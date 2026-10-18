package vault

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/collate"
)

// Articles nest the way the user groups them: dropping B onto A moves B.md
// into a folder A/ next to A.md. The folder carries the parent's file name,
// so the hierarchy is visible in any file manager, and since links point at
// titles rather than paths, moving never breaks them.

// TreeNode is one entry of the article tree: an article, or a plain folder
// (no Title). Path is the folder the node stands for: a plain folder's own,
// or the one an article's children go in.
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Title    string      `json:"title,omitempty"`
	Type     string      `json:"type,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// stem is a page's path without the extension: also the path of the folder
// that holds its children.
func stem(rel string) string {
	return strings.TrimSuffix(rel, path.Ext(rel))
}

// Tree returns the articles nested by folder, sorted by name at each level.
func (v *Vault) Tree() []*TreeNode {
	v.mu.RLock()
	defer v.mu.RUnlock()

	root := &TreeNode{}
	nodes := map[string]*TreeNode{".": root}
	var nodeFor func(dir string) *TreeNode
	nodeFor = func(dir string) *TreeNode {
		if n, ok := nodes[dir]; ok {
			return n
		}
		n := &TreeNode{Name: path.Base(dir), Path: dir}
		nodes[dir] = n
		parent := nodeFor(path.Dir(dir))
		parent.Children = append(parent.Children, n)
		return n
	}
	for dir := range v.idx.folders {
		nodeFor(dir)
	}
	for _, p := range v.idx.pages {
		n := nodeFor(stem(p.Path))
		n.Name, n.Title, n.Type = p.Title, p.Title, p.Type
	}

	// Groups first, as in a file manager: folders and articles that hold
	// others come before lone articles; each kind in alphabetical order.
	c := v.collator()
	group := func(n *TreeNode) bool { return n.Title == "" || len(n.Children) > 0 }
	var sortAll func(*TreeNode)
	sortAll = func(n *TreeNode) {
		for _, ch := range n.Children {
			sortAll(ch)
		}
		sort.Slice(n.Children, func(i, j int) bool {
			a, b := n.Children[i], n.Children[j]
			if group(a) != group(b) {
				return group(a)
			}
			return c.CompareString(a.Name, b.Name) < 0
		})
	}
	sortAll(root)
	return nonNil(root.Children)
}

// Crumb is one step of an article's location, outermost first.
type Crumb struct {
	Name  string `json:"name"`
	Title string `json:"title,omitempty"` // empty for a plain folder
}

// location returns the folders above p as crumbs and the articles directly
// inside p's own folder. Callers hold v.mu.
func (v *Vault) location(p *Page, c *collate.Collator) ([]Crumb, []string) {
	crumbs := []Crumb{}
	for dir := path.Dir(p.Path); dir != "."; dir = path.Dir(dir) {
		crumb := Crumb{Name: path.Base(dir)}
		if parent, ok := v.idx.pages[dir+path.Ext(p.Path)]; ok {
			crumb = Crumb{Name: parent.Title, Title: parent.Title}
		} else if parent, ok := v.pageAt(dir); ok {
			crumb = Crumb{Name: parent.Title, Title: parent.Title}
		}
		crumbs = append([]Crumb{crumb}, crumbs...)
	}

	children := []string{}
	own := stem(p.Path)
	for _, q := range v.idx.pages {
		if path.Dir(q.Path) == own {
			children = append(children, q.Title)
		}
	}
	c.SortStrings(children)
	return crumbs, children
}

// TitleAt returns the title of the article in file rel, if there is one.
func (v *Vault) TitleAt(rel string) (string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if p, ok := v.idx.pages[rel]; ok {
		return p.Title, true
	}
	return "", false
}

// pageAt finds the article whose folder is dir, whatever its extension case.
func (v *Vault) pageAt(dir string) (*Page, bool) {
	for _, p := range v.idx.pages {
		if stem(p.Path) == dir {
			return p, true
		}
	}
	return nil, false
}

// Move puts the article into folder ("" for the top level); its own
// children move along with it. A folder that is an article's (Path of its
// tree node) makes the article that article's child.
func (v *Vault) Move(title, folder string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	p, ok := v.lookup(title)
	if !ok {
		return ErrNotFound
	}
	dir, err := cleanFolder(folder)
	if err != nil {
		return err
	}
	own := stem(p.Path)
	if dir == own || strings.HasPrefix(dir, own+"/") {
		return &InvalidError{"an article cannot go inside itself"}
	}

	rel := path.Join(dir, path.Base(p.Path))
	if rel == p.Path {
		return nil
	}
	abs := v.abs
	newFolder := stem(rel)
	hasFolder := isDir(abs(own))
	if fileExists(abs(rel)) || (hasFolder && fileExists(abs(newFolder))) {
		return ErrExists
	}
	if err := os.MkdirAll(abs(dir), 0o755); err != nil {
		return err
	}
	if err := os.Rename(abs(p.Path), abs(rel)); err != nil {
		return err
	}
	if hasFolder {
		if err := os.Rename(abs(own), abs(newFolder)); err != nil {
			return err
		}
	}
	return v.rescanLocked()
}

// Rename gives an article a new title, as the editor does: the old one
// stays as an alias so links keep working.
func (v *Vault) Rename(title, newTitle string) (string, error) {
	src := v.Source(title)
	if !src.Exists {
		return "", ErrNotFound
	}
	src.Title = newTitle
	return v.SaveSource(title, src)
}

// Delete moves an article to the world's .trash folder, where it can be
// recovered. Articles nested in it stay, in what becomes a plain folder.
func (v *Vault) Delete(title string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	p, ok := v.lookup(title)
	if !ok {
		return ErrNotFound
	}
	if err := v.toTrash(p.Path); err != nil {
		return err
	}
	return v.rescanLocked()
}

// CreateFolder makes a folder (and any missing ones above it) and returns
// its cleaned path.
func (v *Vault) CreateFolder(folder string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	dir, err := cleanFolder(folder)
	if err != nil || dir == "." {
		return "", &InvalidError{"invalid folder name"}
	}
	if fileExists(v.abs(dir)) {
		return "", ErrExists
	}
	if err := os.MkdirAll(v.abs(dir), 0o755); err != nil {
		return "", err
	}
	return dir, v.rescanLocked()
}

// RenameFolder renames a plain folder. An article's folder follows its
// article instead, through Rename.
func (v *Vault) RenameFolder(folder, name string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	dir, err := cleanFolder(folder)
	if err != nil || dir == "." || !isDir(v.abs(dir)) {
		return "", ErrNotFound
	}
	if _, ok := v.pageAt(dir); ok {
		return "", &InvalidError{"this folder belongs to an article; rename the article"}
	}
	name = SafeName(name)
	if name == "" {
		return "", &InvalidError{"invalid folder name"}
	}
	to := path.Join(path.Dir(dir), name)
	if to == dir {
		return dir, nil
	}
	if fileExists(v.abs(to)) && !sameFile(v.abs(to), v.abs(dir)) {
		return "", ErrExists
	}
	if err := os.Rename(v.abs(dir), v.abs(to)); err != nil {
		return "", err
	}
	return to, v.rescanLocked()
}

// MoveFolder puts a plain folder, with everything in it, into folder dest
// ("" for the top level, or an article's folder) and returns its new path.
// An article's folder follows its article instead, through Move.
func (v *Vault) MoveFolder(folder, dest string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	dir, err := cleanFolder(folder)
	if err != nil || dir == "." || !isDir(v.abs(dir)) {
		return "", ErrNotFound
	}
	if _, ok := v.pageAt(dir); ok {
		return "", &InvalidError{"this folder belongs to an article; move the article"}
	}
	to, err := cleanFolder(dest)
	if err != nil {
		return "", err
	}
	if to == dir || strings.HasPrefix(to, dir+"/") {
		return "", &InvalidError{"a folder cannot go inside itself"}
	}
	target := path.Join(to, path.Base(dir))
	if target == dir {
		return dir, nil
	}
	if fileExists(v.abs(target)) {
		return "", ErrExists
	}
	if err := os.MkdirAll(v.abs(to), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(v.abs(dir), v.abs(target)); err != nil {
		return "", err
	}
	return target, v.rescanLocked()
}

// DeleteFolder moves a plain folder, with everything in it, to .trash.
func (v *Vault) DeleteFolder(folder string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	dir, err := cleanFolder(folder)
	if err != nil || dir == "." || !isDir(v.abs(dir)) {
		return ErrNotFound
	}
	if _, ok := v.pageAt(dir); ok {
		return &InvalidError{"this folder belongs to an article; delete the article"}
	}
	if err := v.toTrash(dir); err != nil {
		return err
	}
	return v.rescanLocked()
}

// toTrash moves rel into the world's hidden .trash folder under a name not
// taken there yet. Callers hold v.mu.
func (v *Vault) toTrash(rel string) error {
	trash := v.abs(".trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return err
	}
	base := path.Base(rel)
	ext := path.Ext(base)
	if isDir(v.abs(rel)) {
		ext = ""
	}
	name := base
	for i := 2; fileExists(filepath.Join(trash, name)); i++ {
		name = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(base, ext), i, ext)
	}
	return os.Rename(v.abs(rel), filepath.Join(trash, name))
}

func (v *Vault) abs(rel string) string {
	return filepath.Join(v.root, filepath.FromSlash(rel))
}

// cleanFolder checks a folder path from the browser: relative, inside the
// world, no hidden parts. "" means the top level, returned as ".".
func cleanFolder(folder string) (string, error) {
	folder = strings.TrimSpace(strings.ReplaceAll(folder, "\\", "/"))
	if folder == "" || folder == "." {
		return ".", nil
	}
	dir := path.Clean(folder)
	if path.IsAbs(dir) || dir == ".." || strings.HasPrefix(dir, "../") {
		return "", &InvalidError{"invalid folder"}
	}
	for _, seg := range strings.Split(dir, "/") {
		if strings.HasPrefix(seg, ".") || SafeName(seg) != seg {
			return "", &InvalidError{"invalid folder name"}
		}
	}
	return dir, nil
}

// renameFolder follows a page renamed from old to rel: its children's folder
// takes the new name too. It reports whether there was a folder to move.
func (v *Vault) renameFolder(old, rel string) (bool, error) {
	from := filepath.Join(v.root, filepath.FromSlash(stem(old)))
	to := filepath.Join(v.root, filepath.FromSlash(stem(rel)))
	if !isDir(from) || from == to {
		return false, nil
	}
	return true, os.Rename(from, to)
}

func isDir(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}
