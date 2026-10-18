package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// TypeInfo is an article type as the types editor sees it.
type TypeInfo struct {
	Key    string     `json:"key"` // empty for a new type; generated on save
	Label  string     `json:"label"`
	Color  string     `json:"color"`
	Event  bool       `json:"event"`
	Fields []FieldDef `json:"fields"`
	Count  int        `json:"count"` // articles of this type; ignored on save
}

// InvalidError reports input the types editor should not have sent.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

// Types lists the world's article types with usage counts, by label.
func (v *Vault) Types() []TypeInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()

	counts := map[string]int{}
	for _, p := range v.idx.pages {
		counts[p.Type]++
	}
	out := make([]TypeInfo, 0, len(v.world.Types))
	for key, def := range v.world.Types {
		out = append(out, TypeInfo{
			Key:    key,
			Label:  def.Label,
			Color:  def.Color,
			Event:  def.Event,
			Fields: nonNil(def.Fields),
			Count:  counts[key],
		})
	}
	c := v.collator()
	sort.Slice(out, func(i, j int) bool { return c.CompareString(out[i].Label, out[j].Label) < 0 })
	return out
}

// SaveTypes replaces all article types and writes them to world.yaml,
// keeping the file's other settings. Keys are generated from labels for new
// types and fields and never change afterwards, so renaming a type or a
// field in the editor does not break the articles that use it.
func (v *Vault) SaveTypes(in []TypeInfo) ([]TypeInfo, error) {
	types, order, err := normalizeTypes(in)
	if err != nil {
		return nil, err
	}

	v.cfgMu.Lock()
	defer v.cfgMu.Unlock()

	path := filepath.Join(v.root, worldFile)
	var doc yaml.MapSlice
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, &ParseError{Path: worldFile, Err: err}
		}
	}

	// Ordered so the file lists types the way the editor showed them.
	section := yaml.MapSlice{}
	for _, key := range order {
		section = append(section, yaml.MapItem{Key: key, Value: types[key]})
	}
	doc = setItem(doc, "types", section)

	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	if err := writeAtomic(path, out); err != nil {
		return nil, err
	}
	if err := v.Reload(); err != nil {
		return nil, err
	}
	return v.Types(), nil
}

// typeOut is how a type is written to world.yaml.
type typeOut struct {
	Label  string     `yaml:"label"`
	Color  string     `yaml:"color,omitempty"`
	Event  bool       `yaml:"event,omitempty"`
	Fields []fieldOut `yaml:"fields"`
}

type fieldOut struct {
	Key   string `yaml:"key"`
	Label string `yaml:"label"`
	Type  string `yaml:"type,omitempty"`
}

func normalizeTypes(in []TypeInfo) (map[string]typeOut, []string, error) {
	types := map[string]typeOut{}
	var order []string

	// Existing keys are claimed first so a new type never takes one.
	taken := map[string]bool{}
	for _, t := range in {
		if t.Key != "" {
			if !validKey(t.Key) || taken[t.Key] {
				return nil, nil, &InvalidError{fmt.Sprintf("invalid or duplicate type key %q", t.Key)}
			}
			taken[t.Key] = true
		}
	}

	for _, t := range in {
		label := strings.TrimSpace(t.Label)
		if label == "" {
			return nil, nil, &InvalidError{"a type has no name"}
		}
		key := t.Key
		if key == "" {
			key = uniqueKey(slug(label), "type", taken)
			taken[key] = true
		}

		color := strings.ToLower(strings.TrimSpace(t.Color))
		if !colorRe.MatchString(color) {
			color = "" // the editor only offers valid colors; anything else means none
		}
		out := typeOut{Label: label, Color: color, Event: t.Event, Fields: []fieldOut{}}
		fieldKeys := map[string]bool{ImageKey: true, CaptionKey: true} // every type has these
		for _, f := range t.Fields {
			if f.Key != "" {
				if !validKey(f.Key) || fieldKeys[f.Key] {
					return nil, nil, &InvalidError{fmt.Sprintf("invalid or duplicate field key %q in %q", f.Key, label)}
				}
				fieldKeys[f.Key] = true
			}
		}
		for _, f := range t.Fields {
			flabel := strings.TrimSpace(f.Label)
			if flabel == "" {
				return nil, nil, &InvalidError{fmt.Sprintf("a field of %q has no name", label)}
			}
			fkey := f.Key
			if fkey == "" {
				fkey = uniqueKey(slug(flabel), "field", fieldKeys)
				fieldKeys[fkey] = true
			}
			kind := ""
			if f.Type == "date" {
				kind = "date"
			}
			out.Fields = append(out.Fields, fieldOut{Key: fkey, Label: flabel, Type: kind})
		}
		types[key] = out
		order = append(order, key)
	}
	return types, order, nil
}

var colorRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// validKey accepts what slug produces: letters, digits and dashes.
func validKey(k string) bool {
	return k != "" && slug(k) == k
}

func uniqueKey(base, fallback string, taken map[string]bool) string {
	if base == "" {
		base = fallback
	}
	key := base
	for i := 2; taken[key]; i++ {
		key = fmt.Sprintf("%s-%d", base, i)
	}
	return key
}

func setItem(doc yaml.MapSlice, key string, value any) yaml.MapSlice {
	for i, item := range doc {
		if item.Key == key {
			doc[i].Value = value
			return doc
		}
	}
	return append(doc, yaml.MapItem{Key: key, Value: value})
}
