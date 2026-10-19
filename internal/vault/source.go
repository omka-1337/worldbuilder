package vault

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

// Source is an article split into the parts the editor form shows. The
// file's frontmatter is an implementation detail the user never sees.
type Source struct {
	Exists     bool        `json:"exists"`
	Create     bool        `json:"create"`           // on save: fail if the title is taken
	Title      string      `json:"title"`            // on save: a different title renames the page
	Folder     string      `json:"folder,omitempty"` // on creating: the folder to put it in
	Type       string      `json:"type"`
	Aliases    []string    `json:"aliases"`
	Categories []string    `json:"categories"`
	Infobox    []Field     `json:"infobox"` // in file order
	Images     []InfoImage `json:"images"`  // the infobox pictures, with preview URLs when loading
	Gallery    []InfoImage `json:"gallery"` // the article's gallery, likewise
	Body       string      `json:"body"`
	Notes      string      `json:"notes"` // the author's notes, apart from the text
}

// Source returns the editable parts of a page; for a missing page it
// returns an empty Source with Exists false.
func (v *Vault) Source(title string) Source {
	v.mu.RLock()
	defer v.mu.RUnlock()

	p, ok := v.lookup(title)
	if !ok {
		return Source{Title: title, Aliases: []string{}, Categories: []string{}, Infobox: []Field{}, Images: []InfoImage{}, Gallery: []InfoImage{}}
	}
	src := Source{
		Exists:     true,
		Title:      p.Title,
		Type:       p.Type,
		Aliases:    nonNil(p.Aliases),
		Categories: nonNil(p.Categories),
		Infobox:    nonNil(p.Infobox),
		Images:     []InfoImage{},
		Gallery:    []InfoImage{},
		Body:       string(p.Body),
		Notes:      string(p.Notes),
	}
	for _, img := range p.Gallery {
		img.URL = v.imageURL(img.File)
		src.Gallery = append(src.Gallery, img)
	}
	for _, img := range p.Images {
		img.URL = v.imageURL(img.File)
		src.Images = append(src.Images, img)
	}
	return src
}

// SaveSource writes the editor's parts back into the page's file, keeping
// any frontmatter keys the editor does not know about. Empty values are left
// out so files stay clean.
//
// title names the page being edited. A different src.Title renames it, the
// way Wikipedia moves a page: the file is renamed and the old title becomes
// an alias, so existing links keep working. With src.Create, an existing
// page under either title is an error rather than overwritten.
func (v *Vault) SaveSource(title string, src Source) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	newTitle := strings.Join(strings.Fields(src.Title), " ")
	if newTitle == "" {
		newTitle = strings.Join(strings.Fields(title), " ")
	}
	if newTitle == "" {
		return "", &InvalidError{"an article needs a title"}
	}

	page, exists := v.lookup(title)
	other, taken := v.lookup(newTitle)
	if src.Create && (exists || taken) {
		return "", ErrExists
	}

	var rel, old string
	aliases := cleanList(src.Aliases)
	switch {
	case !exists:
		dir, err := cleanFolder(src.Folder)
		if err != nil {
			return "", err
		}
		rel = path.Join(dir, fileNameFor(newTitle))
		if taken || fileExists(filepath.Join(v.root, filepath.FromSlash(rel))) {
			return "", ErrExists
		}
		if err := os.MkdirAll(v.abs(dir), 0o755); err != nil {
			return "", err
		}
	case Key(newTitle) == Key(page.Title):
		rel = page.Path
		if newTitle != page.Title { // a case-only change still renames the file
			rel, old = path.Join(path.Dir(page.Path), fileNameFor(newTitle)), page.Path
		}
	default:
		if taken && other != page {
			return "", ErrExists
		}
		rel, old = path.Join(path.Dir(page.Path), fileNameFor(newTitle)), page.Path
		if fileExists(filepath.Join(v.root, filepath.FromSlash(rel))) {
			return "", ErrExists
		}
		// The new title may have been one of the aliases; the old one becomes one.
		aliases = slices.DeleteFunc(aliases, func(a string) bool { return Key(a) == Key(newTitle) })
		aliases = cleanList(append(aliases, page.Title))
	}

	var doc yaml.MapSlice
	if exists {
		raw, err := os.ReadFile(filepath.Join(v.root, filepath.FromSlash(page.Path)))
		if err != nil {
			return "", err
		}
		if fm, _ := splitFrontmatter(raw); len(fm) > 0 {
			if err := yaml.Unmarshal(fm, &doc); err != nil {
				return "", &ParseError{Path: page.Path, Err: err}
			}
		}
	}

	for _, item := range doc {
		if item.Key == "title" { // an explicit title wins over the file name
			doc = setItem(doc, "title", newTitle)
		}
	}
	doc = setOrDelete(doc, "type", strings.TrimSpace(src.Type))
	doc = setOrDelete(doc, "aliases", aliases)
	doc = setOrDelete(doc, "categories", cleanList(src.Categories))
	doc = setOrDelete(doc, "infobox", append(imagesYAML(src.Images), infoboxYAML(src.Infobox)...))
	doc = setOrDelete(doc, "gallery", galleryYAML(src.Gallery))

	var out bytes.Buffer
	if len(doc) > 0 {
		fm, err := yaml.Marshal(doc)
		if err != nil {
			return "", err
		}
		out.WriteString("---\n")
		out.Write(fm)
		out.WriteString("---\n")
	}
	out.WriteString(joinNotes(strings.TrimLeft(src.Body, "\n"), src.Notes))
	// Both the old and the new name have a folder of children: they would
	// collide. A folder only under the new name is simply adopted.
	if old != "" {
		from := filepath.Join(v.root, filepath.FromSlash(stem(old)))
		to := filepath.Join(v.root, filepath.FromSlash(stem(rel)))
		if isDir(from) && isDir(to) && !sameFile(from, to) {
			return "", ErrExists
		}
	}
	saved, err := v.writePage(rel, old, out.Bytes())
	if err != nil || old == "" {
		return saved, err
	}
	// A renamed parent takes its children's folder along.
	moved, err := v.renameFolder(old, rel)
	if err != nil {
		return "", err
	}
	if moved {
		if err := v.rescanLocked(); err != nil {
			return "", err
		}
	}
	return saved, nil
}

// imagesYAML writes the infobox pictures first in the infobox: one plain,
// `image` with `caption`; several (or one with a tab label) as a list.
func imagesYAML(images []InfoImage) yaml.MapSlice {
	var list []InfoImage
	for _, img := range images {
		img.File, img.Label, img.Caption = strings.TrimSpace(img.File), strings.TrimSpace(img.Label), strings.TrimSpace(img.Caption)
		if img.File != "" {
			list = append(list, img)
		}
	}
	switch {
	case len(list) == 0:
		return nil
	case len(list) == 1 && list[0].Label == "":
		out := yaml.MapSlice{{Key: ImageKey, Value: list[0].File}}
		if list[0].Caption != "" {
			out = append(out, yaml.MapItem{Key: CaptionKey, Value: list[0].Caption})
		}
		return out
	}
	items := make([]yaml.MapSlice, len(list))
	for i, img := range list {
		items[i] = yaml.MapSlice{{Key: "file", Value: img.File}}
		if img.Label != "" {
			items[i] = append(items[i], yaml.MapItem{Key: "label", Value: img.Label})
		}
		if img.Caption != "" {
			items[i] = append(items[i], yaml.MapItem{Key: "caption", Value: img.Caption})
		}
	}
	return yaml.MapSlice{{Key: ImageKey, Value: items}}
}

// galleryYAML writes the gallery as a list: a picture with a caption as
// {file, caption}, one without as its embed alone.
func galleryYAML(gallery []InfoImage) []any {
	var out []any
	for _, img := range gallery {
		file, caption := strings.TrimSpace(img.File), strings.TrimSpace(img.Caption)
		switch {
		case file == "":
		case caption == "":
			out = append(out, file)
		default:
			out = append(out, yaml.MapSlice{{Key: "file", Value: file}, {Key: "caption", Value: caption}})
		}
	}
	return out
}

// infoboxYAML lays out infobox fields for the frontmatter: fields of no
// section first, in order, then each section's own fields nested under its
// key, sections in the order they first appear.
func infoboxYAML(fields []Field) yaml.MapSlice {
	top := yaml.MapSlice{}
	var order []string
	sections := map[string]yaml.MapSlice{}
	seen := map[[2]string]bool{}
	for _, f := range fields {
		key, value, section := strings.TrimSpace(f.Key), strings.TrimSpace(f.Value), strings.TrimSpace(f.Section)
		if key == "" || value == "" || seen[[2]string{section, key}] {
			continue
		}
		seen[[2]string{section, key}] = true
		if section == "" {
			top = append(top, yaml.MapItem{Key: key, Value: value})
			continue
		}
		if _, ok := sections[section]; !ok {
			order = append(order, section)
		}
		sections[section] = append(sections[section], yaml.MapItem{Key: key, Value: value})
	}
	for _, section := range order {
		if !seen[[2]string{"", section}] { // a value of the same name wins
			top = append(top, yaml.MapItem{Key: section, Value: sections[section]})
		}
	}
	return top
}

// setOrDelete sets key to value, or removes it when value is empty.
func setOrDelete(doc yaml.MapSlice, key string, value any) yaml.MapSlice {
	empty := false
	switch v := value.(type) {
	case string:
		empty = v == ""
	case []string:
		empty = len(v) == 0
	case yaml.MapSlice:
		empty = len(v) == 0
	case []any:
		empty = len(v) == 0
	}
	if !empty {
		return setItem(doc, key, value)
	}
	out := doc[:0]
	for _, item := range doc {
		if item.Key != key {
			out = append(out, item)
		}
	}
	return out
}

func cleanList(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[Key(s)] {
			seen[Key(s)] = true
			out = append(out, s)
		}
	}
	return out
}
