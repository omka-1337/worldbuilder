package vault

import (
	"bytes"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
)

// Page is a single article parsed from a Markdown file.
type Page struct {
	Title      string
	Path       string // slash-separated, relative to the vault root
	Type       string
	Aliases    []string
	Categories []string
	Infobox    []Field     // in file order, without the pictures
	Images     []InfoImage // the infobox pictures, first shown first
	Banner     string      // image embed shown across the top of the article
	Gallery    []InfoImage // the article's gallery, shown at its end when not empty
	BannerY    float64     // 0..1, which part of the banner stays visible
	Raw        []byte      // the whole file, frontmatter included
	Body       []byte      // Markdown after the frontmatter
	ModTime    time.Time

	textLen    int      // characters in the body, how much has been written
	links      []string // wikilink targets as written, body and infobox
	images     []string // embedded image names as written, banner first
	searchRaw  string   // title, aliases, infobox and body, for snippets
	searchText string   // searchRaw lowercased, for matching
}

// Field is one infobox row as written in the frontmatter. An article's own
// field can sit in one of its type's sections: it is then written nested
// under the section's key,
//
//	infobox:
//	  діяльність:
//	    "211-224": Найманець
type Field struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Section string `json:"section,omitempty"` // the section's key, for a field nested in one
}

// typeValues maps the article's fields outside sections by key: the
// fields its type can define. Fields nested in a section are always the
// article's own.
func (p *Page) typeValues() map[string]string {
	values := map[string]string{}
	for _, f := range p.Infobox {
		if f.Section == "" {
			values[f.Key] = f.Value
		}
	}
	return values
}

// InfoImage is one picture at the top of the infobox. With several, the
// infobox shows them one at a time under tabs named by their labels:
//
//	infobox:
//	  image:
//	    - file: "![[young.png]]"
//	      label: 211 р.
//	      caption: Койл у молодості
//	    - file: "![[old.png]]"
//	      label: 224 р.
//
// One picture is written plainly, `image: "![[x.png]]"` with `caption`.
type InfoImage struct {
	File    string `json:"file"`              // ![[name.png]]
	Label   string `json:"label,omitempty"`   // its tab, among several
	Caption string `json:"caption,omitempty"` // the line under it
	URL     string `json:"url,omitempty"`     // where the editor previews it
}

type frontmatter struct {
	Title      string        `yaml:"title"`
	Type       string        `yaml:"type"`
	Aliases    any           `yaml:"aliases"`
	Categories any           `yaml:"categories"`
	Infobox    yaml.MapSlice `yaml:"infobox"`
	Banner     string        `yaml:"banner"`   // Obsidian Banners plugin: ![[wide.png]]
	BannerY    any           `yaml:"banner_y"` // its vertical focus, 0 (top) to 1
	Gallery    any           `yaml:"gallery"`  // pictures: a list of embeds or {file, caption}
}

// ParseError reports a malformed frontmatter block.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string { return fmt.Sprintf("%s: %v", e.Path, e.Err) }
func (e *ParseError) Unwrap() error { return e.Err }

func parsePage(rel string, raw []byte) (*Page, error) {
	fmRaw, body := splitFrontmatter(raw)

	var fm frontmatter
	if len(fmRaw) > 0 {
		// Ordered maps keep a section's own fields in the order written.
		if err := yaml.UnmarshalWithOptions(fmRaw, &fm, yaml.UseOrderedMap()); err != nil {
			return nil, &ParseError{Path: rel, Err: err}
		}
	}

	p := &Page{
		Title:      strings.TrimSpace(fm.Title),
		Path:       rel,
		Type:       strings.TrimSpace(fm.Type),
		Aliases:    stringList(fm.Aliases),
		Categories: stringList(fm.Categories),
		Raw:        raw,
		Body:       body,
	}
	if p.Title == "" {
		p.Title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	}
	p.textLen = utf8.RuneCount(bytes.TrimSpace(body))
	p.Banner = strings.TrimSpace(fm.Banner)
	if fm.Gallery != nil {
		p.Gallery = infoImages(fm.Gallery)
	}
	p.BannerY = 0.5
	if y, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(fm.BannerY)), 64); err == nil && y >= 0 && y <= 1 {
		p.BannerY = y
	}
	caption := ""
	for _, item := range fm.Infobox {
		key := fmt.Sprint(item.Key)
		switch key {
		case ImageKey:
			p.Images = infoImages(item.Value)
			continue
		case CaptionKey:
			caption = formatValue(item.Value)
			continue
		}
		if nested, ok := item.Value.(yaml.MapSlice); ok {
			for _, sub := range nested {
				p.Infobox = append(p.Infobox, Field{Key: fmt.Sprint(sub.Key), Value: formatValue(sub.Value), Section: key})
			}
			continue
		}
		p.Infobox = append(p.Infobox, Field{Key: key, Value: formatValue(item.Value)})
	}
	// The plain form's caption belongs to its one picture.
	if len(p.Images) == 1 && p.Images[0].Caption == "" {
		p.Images[0].Caption = caption
	}

	var search strings.Builder
	search.WriteString(p.Title)
	for _, a := range p.Aliases {
		search.WriteString("\n" + a)
	}
	for _, f := range p.Infobox {
		search.WriteString("\n" + f.Value)
	}
	for _, img := range p.Images {
		search.WriteString("\n" + img.Caption)
	}
	for _, img := range p.Gallery {
		search.WriteString("\n" + img.Caption)
	}
	search.WriteString("\n")
	search.Write(body)
	p.searchRaw = search.String()
	p.searchText = strings.ToLower(p.searchRaw)

	return p, nil
}

// infoImages reads the infobox's `image`: one embed, or a list of embeds or
// of {file, label, caption}.
func infoImages(v any) []InfoImage {
	var out []InfoImage
	add := func(img InfoImage) {
		if img.File = strings.TrimSpace(img.File); img.File != "" {
			out = append(out, img)
		}
	}
	switch v := v.(type) {
	case []any:
		for _, e := range v {
			switch e := e.(type) {
			case yaml.MapSlice:
				var img InfoImage
				for _, kv := range e {
					switch fmt.Sprint(kv.Key) {
					case "file", "image":
						img.File = formatValue(kv.Value)
					case "label":
						img.Label = formatValue(kv.Value)
					case "caption":
						img.Caption = formatValue(kv.Value)
					}
				}
				add(img)
			default:
				add(InfoImage{File: formatValue(e)})
			}
		}
	default:
		add(InfoImage{File: formatValue(v)})
	}
	return out
}

// splitFrontmatter separates a leading "---" YAML block from the body.
func splitFrontmatter(raw []byte) (fm, body []byte) {
	s := bytes.TrimPrefix(raw, []byte("\ufeff"))
	if !bytes.HasPrefix(s, []byte("---\n")) && !bytes.HasPrefix(s, []byte("---\r\n")) {
		return nil, raw
	}
	rest := s[bytes.IndexByte(s, '\n')+1:]
	for off := 0; off < len(rest); {
		nl := bytes.IndexByte(rest[off:], '\n')
		line := rest[off:]
		if nl >= 0 {
			line = rest[off : off+nl]
		}
		if string(bytes.TrimRight(line, " \t\r")) == "---" {
			if nl < 0 {
				return rest[:off], nil
			}
			return rest[:off], rest[off+nl+1:]
		}
		if nl < 0 {
			break
		}
		off += nl + 1
	}
	return nil, raw // unterminated: treat the whole file as body
}

// stringList accepts a string or a (possibly nested) list. Nesting happens
// when someone writes `categories: [[Маги]]` unquoted, which YAML reads as a
// list inside a list.
func stringList(v any) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case nil:
		case []any:
			for _, e := range v {
				walk(e)
			}
		default:
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
				out = append(out, s)
			}
		}
	}
	walk(v)
	return out
}

// formatValue turns an infobox value into Markdown. Lists become
// comma-separated, and a single-element inner list is the unquoted
// `[[Link]]` YAML gotcha, so it is restored as a wikilink.
func formatValue(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, e := range v {
			if inner, ok := e.([]any); ok && len(inner) == 1 {
				if s, ok := inner[0].(string); ok {
					parts = append(parts, "[["+s+"]]")
					continue
				}
			}
			if s := formatValue(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(v)
	}
}

// Key normalizes a title for lookup: case-insensitive, "_" equals " ",
// runs of whitespace collapse.
func Key(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(title, "_", " ")), " "))
}

// slug makes an HTML id from heading text, keeping non-Latin letters.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash && (unicode.IsSpace(r) || r == '-' || r == '_'):
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// fileNameFor picks a file name for a new page with the given title.
func fileNameFor(title string) string {
	name := SafeName(title)
	if name == "" {
		name = "Без назви"
	}
	return name + ".md"
}

// SafeName strips characters that are not allowed in file names on some
// system. It may return "".
func SafeName(title string) string {
	name := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(title))
	return strings.Trim(name, ". ")
}
