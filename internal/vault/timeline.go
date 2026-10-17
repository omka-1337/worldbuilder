package vault

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TimelineEntry is one dated fact: an article and which of its date fields
// put it on this year.
type TimelineEntry struct {
	Title     string `json:"title"`
	Type      string `json:"type,omitempty"`
	TypeLabel string `json:"typeLabel,omitempty"`
	Field     string `json:"field"` // label of the date field, e.g. "Народження"
	Year      int    `json:"year"`
	EndYear   int    `json:"endYear,omitempty"` // for ranges like 340–347
	Text      int    `json:"text"`              // article length in characters
	Event     bool   `json:"event"`             // the article's type is an event type
}

// TimelineYear groups the entries starting in one year. Main is the entry
// that names the year: the event with the longest article, or the longest
// article of any kind when the year has no events.
type TimelineYear struct {
	Year    int             `json:"year"`
	Main    TimelineEntry   `json:"main"`
	Entries []TimelineEntry `json:"entries"`
}

// Timeline collects every infobox field declared as `type: date` and groups
// the articles by year, earliest first.
func (v *Vault) Timeline() []TimelineYear {
	v.mu.RLock()
	defer v.mu.RUnlock()

	byYear := map[int][]TimelineEntry{}
	for _, p := range v.idx.pages {
		def, ok := v.world.Types[p.Type]
		if !ok {
			continue
		}
		values := map[string]string{}
		for _, f := range p.Infobox {
			values[f.Key] = f.Value
		}
		for _, fd := range def.Fields {
			if fd.Type != "date" {
				continue
			}
			start, end, ok := parseYears(values[fd.Key])
			if !ok {
				continue
			}
			byYear[start] = append(byYear[start], TimelineEntry{
				Title:     p.Title,
				Type:      p.Type,
				TypeLabel: def.Label,
				Field:     fd.Label,
				Year:      start,
				EndYear:   end,
				Text:      p.textLen,
				Event:     def.Event,
			})
		}
	}

	c := v.collator()
	out := make([]TimelineYear, 0, len(byYear))
	for year, entries := range byYear {
		sort.Slice(entries, func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.Event != b.Event {
				return a.Event
			}
			if a.Text != b.Text {
				return a.Text > b.Text
			}
			return c.CompareString(a.Title, b.Title) < 0
		})
		out = append(out, TimelineYear{Year: year, Main: entries[0], Entries: entries})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year < out[j].Year })
	return out
}

// A year, optionally negative, optionally followed by a range end:
// "312 ЕП", "-40", "340–347 ЕП", "120 ЕП, [[Вальмор]]".
var yearRe = regexp.MustCompile(`([-−]?)(\d+)(?:\s*[–—-]\s*([-−]?)(\d+))?`)

func parseYears(s string) (start, end int, ok bool) {
	m := yearRe.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	num := func(sign, digits string) int {
		n, _ := strconv.Atoi(digits)
		if strings.TrimSpace(sign) != "" {
			n = -n
		}
		return n
	}
	start = num(m[1], m[2])
	if m[4] != "" {
		end = num(m[3], m[4])
		if end <= start {
			end = 0
		}
	}
	return start, end, true
}
