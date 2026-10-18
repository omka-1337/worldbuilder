package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// Source is an article split into the parts the editor form shows. The
// file's frontmatter is an implementation detail the user never sees.
type Source struct {
	Exists     bool     `json:"exists"`
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
		return Source{Aliases: []string{}, Categories: []string{}, Infobox: []Field{}}
	}
	return Source{
		Exists:     true,
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
func (v *Vault) SaveSource(title string, src Source) (string, error) {
	var doc yaml.MapSlice
	if raw := v.existingFile(title); raw != nil {
		fm, _ := splitFrontmatter(raw)
		if len(fm) > 0 {
			if err := yaml.Unmarshal(fm, &doc); err != nil {
				return "", &ParseError{Path: title, Err: err}
			}
		}
	}

	doc = setOrDelete(doc, "type", strings.TrimSpace(src.Type))
	doc = setOrDelete(doc, "aliases", cleanList(src.Aliases))
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
	return v.Save(title, out.Bytes())
}

func (v *Vault) existingFile(title string) []byte {
	v.mu.RLock()
	p, ok := v.lookup(title)
	v.mu.RUnlock()
	if !ok {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(v.root, filepath.FromSlash(p.Path)))
	if err != nil {
		return nil
	}
	return raw
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
