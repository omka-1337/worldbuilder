package vault

import (
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

// TreeNode is one entry of the article tree. A folder created outside the
// app, with no article of the same name next to it, has no Title.
type TreeNode struct {
	Name     string      `json:"name"`
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
		n := &TreeNode{Name: path.Base(dir)}
		nodes[dir] = n
		parent := nodeFor(path.Dir(dir))
		parent.Children = append(parent.Children, n)
		return n
	}
	for _, p := range v.idx.pages {
		n := nodeFor(stem(p.Path))
		n.Name, n.Title, n.Type = p.Title, p.Title, p.Type
	}

	c := v.collator()
	var sortAll func(*TreeNode)
	sortAll = func(n *TreeNode) {
		sort.Slice(n.Children, func(i, j int) bool { return c.CompareString(n.Children[i].Name, n.Children[j].Name) < 0 })
		for _, ch := range n.Children {
			sortAll(ch)
		}
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

// pageAt finds the article whose folder is dir, whatever its extension case.
func (v *Vault) pageAt(dir string) (*Page, bool) {
	for _, p := range v.idx.pages {
		if stem(p.Path) == dir {
			return p, true
		}
	}
	return nil, false
}

// Move puts the article into parent's folder, or at the top level when
// parent is empty. Its own children move along with it.
func (v *Vault) Move(title, parent string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	p, ok := v.lookup(title)
	if !ok {
		return ErrNotFound
	}
	dir := "."
	if parent != "" {
		q, ok := v.lookup(parent)
		if !ok {
			return ErrNotFound
		}
		dir = stem(q.Path)
		if q == p || dir == stem(p.Path) || strings.HasPrefix(dir, stem(p.Path)+"/") {
			return &InvalidError{"an article cannot go inside itself"}
		}
	}

	rel := path.Join(dir, path.Base(p.Path))
	if rel == p.Path {
		return nil
	}
	abs := func(rel string) string { return filepath.Join(v.root, filepath.FromSlash(rel)) }
	oldFolder, newFolder := stem(p.Path), stem(rel)
	hasFolder := isDir(abs(oldFolder))
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
		if err := os.Rename(abs(oldFolder), abs(newFolder)); err != nil {
			return err
		}
	}
	v.pruneEmpty(path.Dir(p.Path))
	return v.rescanLocked()
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

// pruneEmpty removes dir and its parents while they are empty folders left
// behind by moves, stopping at the vault root.
func (v *Vault) pruneEmpty(dir string) {
	for dir != "." && dir != "/" && dir != "" {
		// os.Remove refuses a folder that still has anything in it.
		if err := os.Remove(filepath.Join(v.root, filepath.FromSlash(dir))); err != nil {
			return
		}
		dir = path.Dir(dir)
	}
}

func isDir(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}
