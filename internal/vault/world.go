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
	Calendar    Calendar           `yaml:"calendar" json:"calendar"`
}

// Calendar describes how time is counted in a world. Everything is
// optional: a world that sets nothing counts plain years.
type Calendar struct {
	// YearParts is how many parts a year has: the number after the dot in
	// "139.096". PartName is what one is called, if the author has decided.
	YearParts int    `yaml:"yearParts,omitempty" json:"yearParts,omitempty"`
	PartName  string `yaml:"partName,omitempty" json:"partName,omitempty"`
	// Epochs in order, the first being epoch I. Years restart with each; a
	// known length lets the timeline lay epochs end to end on one axis.
	Epochs []EpochDef `yaml:"epochs,omitempty" json:"epochs"`
}

type EpochDef struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty"`
	Years int    `yaml:"years,omitempty" json:"years,omitempty"` // 0: not known
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
	// End, on a date field, names the date field that closes the period it
	// opens (born → died, founded → dissolved): the timeline draws the two as
	// one bar.
	End string `yaml:"end" json:"end,omitempty"`
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
	if w.Calendar.Epochs == nil {
		w.Calendar.Epochs = []EpochDef{}
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
