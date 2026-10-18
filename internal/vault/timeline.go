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
	Date      string `json:"date"`  // the field's text, as the author wrote it
	Epoch     int    `json:"epoch,omitempty"`
	Year      int    `json:"year"`
	Day       int    `json:"day,omitempty"`
	EndEpoch  int    `json:"endEpoch,omitempty"` // for ranges like 340–347
	EndYear   int    `json:"endYear,omitempty"`
	// A period spans two fields (born → died): EndField is the closing
	// field's label and EndDate its text. OpenEnd marks a period whose end is
	// not filled in (still alive, still standing).
	EndField string `json:"endField,omitempty"`
	EndDate  string `json:"endDate,omitempty"`
	OpenEnd  bool   `json:"openEnd,omitempty"`
	Text     int    `json:"text"`  // article length in characters
	Event    bool   `json:"event"` // the article's type is an event type
}

// TimelineYear groups the entries starting in one year. Main is the entry
// that names the year: the event with the longest article, or the longest
// article of any kind when the year has no events.
type TimelineYear struct {
	Epoch   int             `json:"epoch,omitempty"`
	Year    int             `json:"year"`
	Main    TimelineEntry   `json:"main"`
	Entries []TimelineEntry `json:"entries"`
}

// Timeline collects every infobox field declared as `type: date` and groups
// the articles by year, earliest first.
func (v *Vault) Timeline() []TimelineYear {
	v.mu.RLock()
	defer v.mu.RUnlock()

	type yearKey struct{ epoch, year int }
	byYear := map[yearKey][]TimelineEntry{}
	for _, p := range v.idx.pages {
		def, ok := v.world.Types[p.Type]
		if !ok {
			continue
		}
		values := map[string]string{}
		for _, f := range p.Infobox {
			values[f.Key] = f.Value
		}
		labels := map[string]string{}
		for _, fd := range def.Fields {
			labels[fd.Key] = fd.Label
		}
		for _, fd := range def.Fields {
			if fd.Type != "date" {
				continue
			}
			d, ok := parseDate(values[fd.Key])
			if !ok {
				continue
			}
			var endField, endDate string
			openEnd := false
			if fd.End != "" && d.end.year == 0 {
				endField = labels[fd.End]
				if e, ok := parseDate(values[fd.End]); !ok {
					openEnd = true
				} else if d.start.before(e.start) {
					d.end, endDate = e.start, plainDate(values[fd.End])
				}
			}
			k := yearKey{d.start.epoch, d.start.year}
			byYear[k] = append(byYear[k], TimelineEntry{
				EndField:  endField,
				EndDate:   endDate,
				OpenEnd:   openEnd,
				Title:     p.Title,
				Type:      p.Type,
				TypeLabel: def.Label,
				Field:     fd.Label,
				Date:      plainDate(values[fd.Key]),
				Epoch:     d.start.epoch,
				Year:      d.start.year,
				Day:       d.start.day,
				EndEpoch:  d.end.epoch,
				EndYear:   d.end.year,
				Text:      p.textLen,
				Event:     def.Event,
			})
		}
	}

	c := v.collator()
	out := make([]TimelineYear, 0, len(byYear))
	for k, entries := range byYear {
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
		out = append(out, TimelineYear{Epoch: k.epoch, Year: k.year, Main: entries[0], Entries: entries})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Epoch != out[j].Epoch {
			return out[i].Epoch < out[j].Epoch
		}
		return out[i].Year < out[j].Year
	})
	return out
}

// A point in a world's calendar. Years restart with each epoch, so the
// epoch comes first; 0 means the date names none. Day is the day of the
// year when given as year.day ("139.096").
type datePoint struct{ epoch, year, day int }

func (a datePoint) before(b datePoint) bool {
	if a.epoch != b.epoch {
		return a.epoch < b.epoch
	}
	if a.year != b.year {
		return a.year < b.year
	}
	return a.day < b.day
}

type dateRange struct{ start, end datePoint } // end.year is 0 for a single date

var (
	numberRe = regexp.MustCompile(`([-−]?)(\d+)(?:\.(\d+))?`)
	// A Roman numeral standing alone: "2XX" (an unknown year) is not epoch 20.
	romanRe = regexp.MustCompile(`(?:^|[^\p{L}\d])([IVXLC]+)(?:[^\p{L}\d]|$)`)
	// Links and footnote marks carry digits that are not dates: [[СМ-16]], [^1].
	noiseRe = regexp.MustCompile(`\[\[[^\]]*\]\]|\[\^[^\]]*\]`)
	// Cyrillic letters that look like Roman numerals, as typed in "ІІІ Епохи".
	romanLookalikes = strings.NewReplacer("І", "I", "Х", "X", "С", "C", "Ⅰ", "I", "Ⅱ", "II", "Ⅲ", "III", "Ⅳ", "IV", "Ⅴ", "V")
)

// parseDate reads dates as worldbuilders write them: "312 ЕП", "-40",
// "139.096 III Епохи" (year.day of the third epoch), and ranges such as
// "340–347", "211-224 III Епохи" or "142 р. III Епохи — 145 р. III Епохи".
func parseDate(s string) (dateRange, bool) {
	s = noiseRe.ReplaceAllString(s, " ")
	startText, endText := splitRange(s)
	start, ok := parsePoint(startText)
	if !ok {
		return dateRange{}, false
	}
	r := dateRange{start: start}
	if end, ok := parsePoint(endText); ok {
		// "142 — 145 р. III Епохи": one epoch written once covers both ends.
		if end.epoch == 0 {
			end.epoch = start.epoch
		}
		if start.epoch == 0 {
			r.start.epoch = end.epoch
		}
		if r.start.before(end) {
			r.end = end
		}
	}
	return r, true
}

func parsePoint(s string) (datePoint, bool) {
	m := numberRe.FindStringSubmatch(s)
	if m == nil {
		return datePoint{}, false
	}
	p := datePoint{}
	p.year, _ = strconv.Atoi(m[2])
	if m[1] != "" {
		p.year = -p.year
	}
	if m[3] != "" {
		p.day, _ = strconv.Atoi(m[3])
	}
	if r := romanRe.FindStringSubmatch(romanLookalikes.Replace(s)); r != nil {
		p.epoch = fromRoman(r[1])
	}
	return p, true
}

// splitRange cuts "A — B" into its two ends. An en or em dash always
// separates; a hyphen does when spaced ("A - B") or squeezed between digits
// ("211-224"), but not as a minus sign ("-40").
func splitRange(s string) (string, string) {
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '–' || r == '—':
		case r == '-' && i > 0 && i+1 < len(rs) &&
			((rs[i-1] == ' ' && rs[i+1] == ' ') || (isDigit(rs[i-1]) && isDigit(rs[i+1]))):
		default:
			continue
		}
		return string(rs[:i]), string(rs[i+1:])
	}
	return s, ""
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func fromRoman(s string) int {
	values := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100}
	total, prev := 0, 0
	rs := []rune(s)
	for i := len(rs) - 1; i >= 0; i-- {
		v := values[rs[i]]
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total
}

// plainDate is a date field's text for display: links become their labels.
func plainDate(s string) string {
	s = regexp.MustCompile(`\[\^[^\]]*\]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`).ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}
