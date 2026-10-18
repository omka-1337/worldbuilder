package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// SaveCalendar replaces the world's calendar and the label written after
// epoch numerals (era), keeping the rest of world.yaml.
func (v *Vault) SaveCalendar(cal Calendar, era string) (World, error) {
	if cal.YearParts < 0 {
		return World{}, &InvalidError{"a year cannot have a negative number of parts"}
	}
	cal.PartName = strings.TrimSpace(cal.PartName)
	epochs := []EpochDef{}
	for _, e := range cal.Epochs {
		if e.Years < 0 {
			return World{}, &InvalidError{"an epoch cannot last a negative number of years"}
		}
		epochs = append(epochs, EpochDef{Name: strings.TrimSpace(e.Name), Years: e.Years})
	}
	cal.Epochs = epochs

	v.cfgMu.Lock()
	defer v.cfgMu.Unlock()

	path := filepath.Join(v.root, worldFile)
	var doc yaml.MapSlice
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return World{}, err
	default:
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return World{}, &ParseError{Path: worldFile, Err: err}
		}
	}
	doc = setOrDelete(doc, "era", strings.TrimSpace(era))
	if cal.YearParts == 0 && cal.PartName == "" && len(cal.Epochs) == 0 {
		doc = setOrDelete(doc, "calendar", "")
	} else {
		doc = setItem(doc, "calendar", cal)
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return World{}, err
	}
	if err := writeAtomic(path, out); err != nil {
		return World{}, err
	}
	if err := v.Reload(); err != nil {
		return World{}, err
	}
	return v.World(), nil
}
