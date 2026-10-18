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
	Exists     bool     `json:"exists"`
	Create     bool     `json:"create"` // on save: fail if the title is taken
	Title      string   `json:"title"`  // on save: a different title renames the page
	Type       string   `json:"type"`
	Aliases    []string `json:"aliases"`
	Categories []string `json:"categories"`
	Infobox    []Field  `json:"infobox"` // in file order
	Body       string   `json:"body"`
}

// Source returns the editable parts of a page; for a missing page it
// returns an empty Source with Exists false.
func (v *Vault) Source(title string) Source {
	v.mu.RLock()
	defer v.mu.RUnlock()

	p, ok := v.lookup(title)
	if !ok {
		return Source{Title: title, Aliases: []string{}, Categories: []string{}, Infobox: []Field{}}
	}
	return Source{
		Exists:     true,
		Title:      p.Title,
		Type:       p.Type,
		Aliases:    nonNil(p.Aliases),
		Categories: nonNil(p.Categories),
		Infobox:    nonNil(p.Infobox),
		Body:       string(p.Body),
	}
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
		rel = fileNameFor(newTitle)
		if taken || fileExists(filepath.Join(v.root, rel)) {
			return "", ErrExists
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
	infobox := yaml.MapSlice{}
	seen := map[string]bool{}
	for _, f := range src.Infobox {
		key, value := strings.TrimSpace(f.Key), strings.TrimSpace(f.Value)
		if key == "" || value == "" || seen[key] {
			continue
		}
		seen[key] = true
		infobox = append(infobox, yaml.MapItem{Key: key, Value: value})
	}
	doc = setOrDelete(doc, "infobox", infobox)

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
	out.WriteString(strings.TrimLeft(src.Body, "\n"))
	return v.writePage(rel, old, out.Bytes())
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
