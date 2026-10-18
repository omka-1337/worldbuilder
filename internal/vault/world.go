package vault

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
	"golang.org/x/text/language"
)

// World is the vault-wide configuration read from world.yaml.
type World struct {
	Name        string             `yaml:"name" json:"name"`
	Description string             `yaml:"description" json:"description,omitempty"`
	MainPage    string             `yaml:"mainPage" json:"mainPage"`
	Era         string             `yaml:"era" json:"era,omitempty"` // after years, for worlds without a calendar: "343 ЕП"
	Language    string             `yaml:"language" json:"language"`
	Types       map[string]TypeDef `yaml:"types" json:"types"`
	Calendar    Calendar           `yaml:"calendar" json:"calendar"`
}

// TypeDef describes an article type and the infobox fields it offers.
type TypeDef struct {
	Label  string     `yaml:"label" json:"label"`
	Color  string     `yaml:"color" json:"color,omitempty"` // "#rrggbb", used on the timeline
	Event  bool       `yaml:"event" json:"event,omitempty"` // articles of this type are timeline events
	Fields []FieldDef `yaml:"fields" json:"fields"`
	// Former keeps the labels of fields taken out of the type, by key:
	// articles still hold their values and show them under these names.
	Former map[string]string `yaml:"former" json:"former,omitempty"`
}

type FieldDef struct {
	Key   string `yaml:"key" json:"key"`
	Label string `yaml:"label" json:"label"`
	Type  string `yaml:"type" json:"type,omitempty"` // "date" puts the article on the timeline
	// End, on a date field, names the date field that closes the period it
	// opens (born → died, founded → dissolved): the timeline draws the two as
	// one bar.
	End string `yaml:"end" json:"end,omitempty"`
}

// fieldLabel names an infobox field that is not in the article's type: one
// taken out of it shows under its old label, one some other type has under
// that type's label. Anything else is the article's own field and its key is
// the name the author typed, unless it reads like a generated key
// ("дата-народження"), which is turned back into words.
func (w World) fieldLabel(typ, key string) string {
	if label, ok := w.Types[typ].Former[key]; ok {
		return label
	}
	keys := make([]string, 0, len(w.Types))
	for k := range w.Types {
		keys = append(keys, k)
	}
	sort.Strings(keys) // the same label every time when types disagree
	for _, k := range keys {
		for _, f := range w.Types[k].Fields {
			if f.Key == key && f.Type != "section" {
				return f.Label
			}
		}
	}
	if key == "" || slug(key) != key {
		return key
	}
	label := []rune(strings.ReplaceAll(key, "-", " "))
	label[0] = unicode.ToUpper(label[0])
	return string(label)
}

const worldFile = "world.yaml"

// LoadWorld reads root/world.yaml, filling defaults for anything missing.
func LoadWorld(root string) (World, error) {
	w := World{}
	raw, err := os.ReadFile(filepath.Join(root, worldFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return w, err
	default:
		if err := yaml.Unmarshal(raw, &w); err != nil {
			return w, &ParseError{Path: worldFile, Err: err}
		}
	}

	if w.Name == "" {
		abs, _ := filepath.Abs(root)
		w.Name = filepath.Base(abs)
	}
	if w.MainPage == "" {
		w.MainPage = "Головна"
	}
	if w.Language == "" {
		w.Language = "uk"
	}
	if w.Types == nil {
		w.Types = map[string]TypeDef{}
	}
	if w.Calendar.Units == nil {
		w.Calendar.Units = []TimeUnit{}
	}
	if w.Calendar.Formats == nil {
		w.Calendar.Formats = []string{}
	}
	return w, nil
}

func (w World) lang() language.Tag {
	tag, err := language.Parse(w.Language)
	if err != nil {
		return language.Ukrainian
	}
	return tag
}
