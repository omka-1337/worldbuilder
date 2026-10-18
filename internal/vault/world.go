package vault

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"golang.org/x/text/language"
)

// World is the vault-wide configuration read from world.yaml.
type World struct {
	Name        string             `yaml:"name" json:"name"`
	Description string             `yaml:"description" json:"description,omitempty"`
	MainPage    string             `yaml:"mainPage" json:"mainPage"`
	Era         string             `yaml:"era" json:"era,omitempty"` // shown after years, e.g. "ЕП"
	Language    string             `yaml:"language" json:"language"`
	Types       map[string]TypeDef `yaml:"types" json:"types"`
}

// TypeDef describes an article type and the infobox fields it offers.
type TypeDef struct {
	Label  string     `yaml:"label" json:"label"`
	Color  string     `yaml:"color" json:"color,omitempty"` // "#rrggbb", used on the timeline
	Event  bool       `yaml:"event" json:"event,omitempty"` // articles of this type are timeline events
	Fields []FieldDef `yaml:"fields" json:"fields"`
}

type FieldDef struct {
	Key   string `yaml:"key" json:"key"`
	Label string `yaml:"label" json:"label"`
	Type  string `yaml:"type" json:"type,omitempty"` // "date" puts the article on the timeline
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
	return w, nil
}

func (w World) lang() language.Tag {
	tag, err := language.Parse(w.Language)
	if err != nil {
		return language.Ukrainian
	}
	return tag
}
