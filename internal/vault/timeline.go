package vault

import (
	"math"
	"sort"
	"strings"
)

// Timeline is everything the timeline view draws. Dates are read with the
// world's calendar and placed on axes measured in its step unit (years,
// usually). Units above the step either offset one shared axis, when their
// lengths are known, or split the timeline into several axes (tabs).
type Timeline struct {
	Axes       []TimelineAxis  `json:"axes"`
	Points     []TimelinePoint `json:"points"`
	Boundaries []TimelineMark  `json:"boundaries"`    // where each parent value starts on a shared axis
	Now        *TimelineMark   `json:"now,omitempty"` // the latest date in the world
}

type TimelineAxis struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type TimelineMark struct {
	Axis  string  `json:"axis"`
	Pos   float64 `json:"pos"`
	Label string  `json:"label"`
}

// TimelinePoint groups the entries starting in one step (one year). Main is
// the entry that names it: the event with the longest article, or the
// longest article of any kind when the step has no events.
type TimelinePoint struct {
	Axis    string          `json:"axis"`
	Pos     float64         `json:"pos"`
	Label   string          `json:"label"`
	Main    TimelineEntry   `json:"main"`
	Entries []TimelineEntry `json:"entries"`
}

// TimelineEntry is one dated fact: an article and which of its date fields
// put it here.
type TimelineEntry struct {
	Title     string  `json:"title"`
	Type      string  `json:"type,omitempty"`
	TypeLabel string  `json:"typeLabel,omitempty"`
	Field     string  `json:"field"` // label of the date field, e.g. "Народження"
	Date      string  `json:"date"`  // the field's text, as the author wrote it
	Text      int     `json:"text"`  // article length in characters
	Event     bool    `json:"event"` // the article's type is an event type
	Axis      string  `json:"axis"`
	Pos       float64 `json:"pos"`
	// A range (340–347) or a period over two fields (born → died). Bar is
	// set when it spans more than its starting step; OpenEnd when the end is
	// not filled in, so it runs on to the present.
	Bar      bool    `json:"bar,omitempty"`
	HasEnd   bool    `json:"hasEnd,omitempty"`
	EndAxis  string  `json:"endAxis,omitempty"`
	EndPos   float64 `json:"endPos,omitempty"`
	EndField string  `json:"endField,omitempty"`
	EndDate  string  `json:"endDate,omitempty"`
	OpenEnd  bool    `json:"openEnd,omitempty"`

	start, end tdate
}

// dated is a date field of an article, read with the calendar.
type dated struct {
	page  *Page
	def   TypeDef
	field FieldDef
	value string
	r     dateRange
	ok    bool
}

// datedFields reads every date field in the world. Callers hold v.mu.
func (v *Vault) datedFields(cal *calendar) []dated {
	var out []dated
	for _, p := range v.idx.pages {
		def, ok := v.world.Types[p.Type]
		if !ok {
			continue
		}
		values := p.typeValues()
		for _, fd := range def.Fields {
			if fd.Type != "date" || strings.TrimSpace(values[fd.Key]) == "" {
				continue
			}
			r, ok := cal.parse(values[fd.Key])
			out = append(out, dated{page: p, def: def, field: fd, value: values[fd.Key], r: r, ok: ok})
		}
	}
	return out
}

// Timeline collects every infobox field declared as a date.
func (v *Vault) Timeline() Timeline {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.timeline(v.cal)
}

func (v *Vault) timeline(cal *calendar) Timeline {
	var entries []TimelineEntry
	for _, d := range v.datedFields(cal) {
		if !d.ok || !d.r.start.has[cal.step] {
			continue
		}
		e := TimelineEntry{
			Title:     d.page.Title,
			Type:      d.page.Type,
			TypeLabel: d.def.Label,
			Field:     d.field.Label,
			Date:      plainDate(d.value),
			Text:      d.page.textLen,
			Event:     d.def.Event,
			start:     d.r.start,
		}
		if d.r.hasEnd && d.r.end.has[cal.step] {
			e.end, e.HasEnd = d.r.end, true
		}
		// A period over two fields: the end comes from the paired field.
		if d.field.End != "" && !e.HasEnd {
			e.EndField = labelOf(d.def, d.field.End)
			endValue := d.page.typeValues()[d.field.End]
			if r, ok := cal.parse(endValue); !ok || !r.start.has[cal.step] {
				e.OpenEnd = true
			} else if e.start.before(r.start) {
				e.end, e.HasEnd, e.EndDate = r.start, true, plainDate(endValue)
			}
		}
		entries = append(entries, e)
	}

	// One shared axis when every parent chain can be offset.
	shared := cal.step > 0
	for _, e := range entries {
		for _, d := range []tdate{e.start, e.end} {
			if d.v == nil || !hasParents(cal, d) {
				continue
			}
			if _, ok := cal.offset(d); !ok {
				shared = false
			}
		}
	}
	axisOf := func(d tdate) string {
		switch {
		case cal.step == 0:
			return ""
		case !hasParents(cal, d):
			return "?"
		case shared:
			return ""
		}
		return cal.parentKey(d)
	}
	posOf := func(d tdate, withFraction bool) float64 {
		off := 0.0
		if shared && hasParents(cal, d) {
			off, _ = cal.offset(d)
		}
		p := off + float64(d.v[cal.step])
		if withFraction {
			p += cal.fraction(d)
		}
		return p
	}

	// The present: the latest date anywhere, ends of periods included.
	var tl Timeline
	var now tdate
	for _, e := range entries {
		for _, d := range []tdate{e.start, e.end} {
			if d.v != nil && (now.v == nil || now.before(d)) {
				now = d
			}
		}
	}
	if now.v != nil {
		tl.Now = &TimelineMark{Axis: axisOf(now), Pos: posOf(now, true), Label: cal.stepLabel(now)}
	}

	type group struct {
		point TimelinePoint
		first tdate
	}
	groups := map[string]*group{}
	axisOrder := map[string]tdate{}
	for i := range entries {
		e := &entries[i]
		e.Axis, e.Pos = axisOf(e.start), posOf(e.start, true)
		if e.HasEnd {
			e.EndAxis, e.EndPos = axisOf(e.end), posOf(e.end, true)
			e.Bar = cal.parentKey(e.start) != cal.parentKey(e.end) || e.end.v[cal.step] > e.start.v[cal.step]
		}
		if e.OpenEnd && now.v != nil {
			e.Bar = cal.parentKey(e.start) != cal.parentKey(now) || now.v[cal.step] > e.start.v[cal.step]
		}
		key := e.Axis + "\x00" + cal.parentKey(e.start) + "\x00" + itoa(e.start.v[cal.step])
		g := groups[key]
		if g == nil {
			g = &group{point: TimelinePoint{Axis: e.Axis, Pos: posOf(e.start, false), Label: cal.stepLabel(e.start)}, first: e.start}
			groups[key] = g
		}
		g.point.Entries = append(g.point.Entries, *e)
		if first, ok := axisOrder[e.Axis]; !ok || e.start.before(first) {
			axisOrder[e.Axis] = e.start
		}
	}

	c := v.collator()
	for _, g := range groups {
		es := g.point.Entries
		sort.Slice(es, func(i, j int) bool {
			a, b := es[i], es[j]
			if a.Event != b.Event {
				return a.Event
			}
			if a.Text != b.Text {
				return a.Text > b.Text
			}
			return c.CompareString(a.Title, b.Title) < 0
		})
		g.point.Main = es[0]
		tl.Points = append(tl.Points, g.point)
	}
	sort.Slice(tl.Points, func(i, j int) bool {
		a, b := tl.Points[i], tl.Points[j]
		if a.Axis != b.Axis {
			return axisLess(a.Axis, b.Axis, axisOrder)
		}
		return a.Pos < b.Pos
	})

	for key, d := range axisOrder {
		label := ""
		if key != "" {
			label = cal.parentLabel(d, cal.step)
		}
		tl.Axes = append(tl.Axes, TimelineAxis{Key: key, Label: label})
	}
	sort.Slice(tl.Axes, func(i, j int) bool { return axisLess(tl.Axes[i].Key, tl.Axes[j].Key, axisOrder) })

	// On a shared axis, mark where each parent value used by a date begins.
	if shared {
		seen := map[string]bool{}
		for _, e := range entries {
			if !hasParents(cal, e.start) {
				continue
			}
			for i := 0; i < cal.step; i++ {
				probe := cal.newDate()
				for k := 0; k <= i; k++ {
					probe.v[k], probe.has[k] = e.start.v[k], true
				}
				for k := i + 1; k < cal.step; k++ {
					probe.v[k], probe.has[k] = 1, true
				}
				pos, ok := cal.offset(probe)
				key := itoa(i) + ":" + cal.parentKey(probe)
				if ok && !seen[key] {
					seen[key] = true
					tl.Boundaries = append(tl.Boundaries, TimelineMark{Pos: pos, Label: cal.valueLabel(i, e.start.v[i])})
				}
			}
		}
		sort.Slice(tl.Boundaries, func(i, j int) bool { return tl.Boundaries[i].Pos < tl.Boundaries[j].Pos })
	}
	if tl.Axes == nil {
		tl.Axes = []TimelineAxis{}
	}
	if tl.Points == nil {
		tl.Points = []TimelinePoint{}
	}
	if tl.Boundaries == nil {
		tl.Boundaries = []TimelineMark{}
	}
	return tl
}

// axisLess orders axes by their earliest date, the unknown one last.
func axisLess(a, b string, first map[string]tdate) bool {
	if a == "?" || b == "?" {
		return b == "?" && a != "?"
	}
	return first[a].before(first[b])
}

func hasParents(cal *calendar, d tdate) bool {
	for i := 0; i < cal.step; i++ {
		if !d.has[i] {
			return false
		}
	}
	return true
}

func labelOf(def TypeDef, key string) string {
	for _, f := range def.Fields {
		if f.Key == key {
			return f.Label
		}
	}
	return key
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// --- previewing a draft calendar ---------------------------------------------

// DateRef points at a date field of an article.
type DateRef struct {
	Title string `json:"title"`
	Field string `json:"field"`
	Date  string `json:"date"`
}

// CalendarPreview shows how a calendar, perhaps not yet saved, reads this
// world: what it makes of a sample date, how many dates each format reads,
// which dates it cannot read or that do not fit, and the present moment.
type CalendarPreview struct {
	Error   string    `json:"error,omitempty"` // the calendar itself is invalid
	Test    *ReadDate `json:"test,omitempty"`
	Formats []int     `json:"formats"`
	Read    int       `json:"read"`
	Unread  []DateRef `json:"unread"`
	Beyond  []DateRef `json:"beyond"` // read, but a value exceeds its parent: part 231 of 100
	Axes    int       `json:"axes"`   // more than one: units above the step lack lengths
	Shared  bool      `json:"shared"`
	Now     string    `json:"now,omitempty"`
	NowFrom *DateRef  `json:"nowFrom,omitempty"`
	Points  int       `json:"points"`
}

// ReadDate is a sample date taken apart unit by unit.
type ReadDate struct {
	OK    bool       `json:"ok"`
	Start []UnitPart `json:"start,omitempty"`
	End   []UnitPart `json:"end,omitempty"`
	Label string     `json:"label,omitempty"`
}

type UnitPart struct {
	Unit  string `json:"unit"`
	Value string `json:"value"`
}

// PreviewCalendar reads the world with cal instead of its own calendar.
func (v *Vault) PreviewCalendar(cal Calendar, sample string) CalendarPreview {
	compiled, err := compileCalendar(cleanCalendar(cal), "")
	if err != nil {
		return CalendarPreview{Error: err.Error(), Formats: []int{}, Unread: []DateRef{}, Beyond: []DateRef{}}
	}
	v.mu.RLock()
	defer v.mu.RUnlock()

	out := CalendarPreview{Formats: make([]int, len(compiled.formats)), Unread: []DateRef{}, Beyond: []DateRef{}}
	for _, d := range v.datedFields(compiled) {
		ref := DateRef{Title: d.page.Title, Field: d.field.Label, Date: plainDate(d.value)}
		if !d.ok || !d.r.start.has[compiled.step] {
			out.Unread = append(out.Unread, ref)
			continue
		}
		out.Read++
		if d.r.format >= 0 && d.r.format < len(out.Formats) {
			out.Formats[d.r.format]++
		}
		if compiled.beyond(d.r.start) || (d.r.hasEnd && compiled.beyond(d.r.end)) {
			out.Beyond = append(out.Beyond, ref)
		}
	}
	c := v.collator()
	byTitle := func(list []DateRef) {
		sort.Slice(list, func(i, j int) bool { return c.CompareString(list[i].Title, list[j].Title) < 0 })
	}
	byTitle(out.Unread)
	byTitle(out.Beyond)

	tl := v.timeline(compiled)
	out.Axes, out.Points = len(tl.Axes), len(tl.Points)
	out.Shared = len(tl.Boundaries) > 0
	if tl.Now != nil {
		out.Now = tl.Now.Label
		for _, p := range tl.Points {
			for _, e := range p.Entries {
				if e.Pos == tl.Now.Pos && e.Axis == tl.Now.Axis {
					out.NowFrom = &DateRef{Title: e.Title, Field: e.Field, Date: e.Date}
				}
				if e.HasEnd && math.Abs(e.EndPos-tl.Now.Pos) < 1e-9 && e.EndAxis == tl.Now.Axis {
					field := e.Field
					if e.EndField != "" {
						field = e.EndField
					}
					out.NowFrom = &DateRef{Title: e.Title, Field: field, Date: e.Date}
				}
			}
		}
	}

	if strings.TrimSpace(sample) != "" {
		r, ok := compiled.parse(sample)
		t := &ReadDate{OK: ok && r.start.count() > 0}
		if t.OK {
			t.Start = compiled.parts(r.start)
			if r.hasEnd {
				t.End = compiled.parts(r.end)
			}
			if r.start.has[compiled.step] {
				t.Label = compiled.stepLabel(r.start)
			}
		}
		out.Test = t
	}
	return out
}

func (c *calendar) parts(d tdate) []UnitPart {
	var out []UnitPart
	for i, u := range c.Units {
		if d.has[i] {
			out = append(out, UnitPart{Unit: u.Name, Value: c.written(i, d.v[i])})
		}
	}
	return out
}
