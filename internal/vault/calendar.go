package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// Calendar describes how time is counted in a world, entirely in the
// author's terms: a hierarchy of time units, largest first, each one a
// subdivision of the one above, and templates for how dates are written in
// articles. Nothing here knows about years or epochs; an epoch is simply a
// unit at the root.
type Calendar struct {
	Units   []TimeUnit `yaml:"units" json:"units"`
	Formats []string   `yaml:"formats" json:"formats"`
	// Step is the unit the timeline counts in; units above it group or
	// offset the axis, units below place a date inside a step.
	Step string `yaml:"step,omitempty" json:"step,omitempty"`
}

// TimeUnit is one level of the hierarchy.
type TimeUnit struct {
	Name string `yaml:"name" json:"name"`
	// Write is how values are written: "number" (the default), "roman", or
	// "names", which picks from Names in order.
	Write string   `yaml:"write,omitempty" json:"write,omitempty"`
	Names []string `yaml:"names,omitempty" json:"names,omitempty"`
	// Per is how many of this unit make one of its parent, when constant.
	Per int `yaml:"per,omitempty" json:"per,omitempty"`
	// Lengths overrides, for particular values of this unit (as written),
	// how many of the next unit down they hold: { Талець: 28 }, { I: 500 }.
	Lengths map[string]int `yaml:"lengths,omitempty" json:"lengths,omitempty"`
}

// A world without a calendar counts plain years.
var defaultCalendar = Calendar{
	Units:   []TimeUnit{{Name: "Рік"}},
	Formats: []string{"{Рік}"},
	Step:    "Рік",
}

// --- compiling -----------------------------------------------------------------

// calendar is a Calendar ready to read dates.
type calendar struct {
	Calendar
	step    int
	formats []format
	custom  bool   // false for the default calendar
	era     string // the world's legacy label after years, default calendar only
}

// format is one compiled template. Partial variants read one side of a
// range, which may leave out units written once for both: the start of
// "144.087 — 144.125 III Епохи" has no epoch.
type format struct {
	full     pattern
	partials []pattern
	items    []item
}

type pattern struct {
	re    *regexp.Regexp
	units []int // unit index of each capture group
}

// item is a piece of a template: literal text, a unit, or an optional group.
type item struct {
	lit  string
	unit int    // -1 unless a unit
	opt  []item // non-nil for [optional] groups
}

func (it item) isUnit() bool { return it.unit >= 0 && it.opt == nil }

func compileCalendar(c Calendar, era string) (*calendar, error) {
	if len(c.Units) == 0 {
		cal, _ := compileCalendar(defaultCalendar, "")
		cal.custom, cal.era = false, era
		return cal, nil
	}
	if err := validateCalendar(c); err != nil {
		return nil, err
	}
	cal := &calendar{Calendar: c, custom: true, step: stepIndex(c)}
	for _, src := range c.Formats {
		f, err := cal.compileFormat(src)
		if err != nil {
			return nil, err
		}
		cal.formats = append(cal.formats, f)
	}
	return cal, nil
}

// stepIndex is the unit named as the step, or else the first numbered unit
// (typically the year), or else the deepest one.
func stepIndex(c Calendar) int {
	for i, u := range c.Units {
		if strings.EqualFold(u.Name, c.Step) {
			return i
		}
	}
	for i, u := range c.Units {
		if u.Write == "" || u.Write == "number" {
			return i
		}
	}
	return len(c.Units) - 1
}

func validateCalendar(c Calendar) error {
	seen := map[string]bool{}
	for _, u := range c.Units {
		name := strings.TrimSpace(u.Name)
		switch {
		case name == "":
			return &InvalidError{"a time unit has no name"}
		case strings.ContainsAny(name, "{}[]"):
			return &InvalidError{fmt.Sprintf("the name %q has brackets in it", name)}
		case seen[strings.ToLower(name)]:
			return &InvalidError{fmt.Sprintf("two time units are called %q", name)}
		}
		seen[strings.ToLower(name)] = true
		switch u.Write {
		case "", "number", "roman":
		case "names":
			if len(u.Names) == 0 {
				return &InvalidError{fmt.Sprintf("%q is written with names but has none", name)}
			}
		default:
			return &InvalidError{fmt.Sprintf("%q: unknown way of writing %q", name, u.Write)}
		}
		if u.Per < 0 {
			return &InvalidError{fmt.Sprintf("%q: a negative count", name)}
		}
		for _, n := range u.Lengths {
			if n < 0 {
				return &InvalidError{fmt.Sprintf("%q: a negative length", name)}
			}
		}
	}
	if len(c.Formats) == 0 {
		return &InvalidError{"the calendar has no date formats"}
	}
	return nil
}

func (c *calendar) unitIndex(name string) int {
	for i, u := range c.Units {
		if strings.EqualFold(strings.TrimSpace(u.Name), strings.TrimSpace(name)) {
			return i
		}
	}
	return -1
}

// parseTemplate splits "{Рік}[.{Частина}] {Епоха} Епохи" into items.
func (c *calendar) parseTemplate(src string, nested bool) ([]item, string, error) {
	var items []item
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			items = append(items, item{lit: lit.String(), unit: -1})
			lit.Reset()
		}
	}
	for len(src) > 0 {
		switch src[0] {
		case '{':
			end := strings.IndexByte(src, '}')
			if end < 0 {
				return nil, "", &InvalidError{"a { without a closing }"}
			}
			u := c.unitIndex(src[1:end])
			if u < 0 {
				return nil, "", &InvalidError{fmt.Sprintf("no time unit called %q", src[1:end])}
			}
			flush()
			items = append(items, item{unit: u})
			src = src[end+1:]
		case '[':
			if nested {
				return nil, "", &InvalidError{"[ ] cannot be nested"}
			}
			flush()
			inner, rest, err := c.parseTemplate(src[1:], true)
			if err != nil {
				return nil, "", err
			}
			if !strings.HasPrefix(rest, "]") {
				return nil, "", &InvalidError{"a [ without a closing ]"}
			}
			items = append(items, item{unit: -1, opt: inner})
			src = rest[1:]
		case ']':
			if nested {
				flush()
				return items, src, nil
			}
			return nil, "", &InvalidError{"a ] without an opening ["}
		default:
			lit.WriteByte(src[0])
			src = src[1:]
		}
	}
	flush()
	return items, "", nil
}

func (c *calendar) compileFormat(src string) (format, error) {
	items, _, err := c.parseTemplate(src, false)
	if err != nil {
		return format{}, err
	}
	var top []int // positions of units outside [ ]
	for i, it := range items {
		if it.isUnit() {
			top = append(top, i)
		}
	}
	if len(top) == 0 && !hasUnit(items) {
		return format{}, &InvalidError{fmt.Sprintf("the format %q names no time unit", src)}
	}
	f := format{items: items, full: c.pattern(items)}
	// Prefixes ending at each top-level unit, and suffixes starting at one:
	// the start of a range drops trailing units, its end leading ones.
	for k := 0; k+1 < len(top); k++ {
		f.partials = append(f.partials, c.pattern(trailingOptional(items[:top[k+1]], top[k])))
	}
	for k := 1; k < len(top); k++ {
		f.partials = append(f.partials, c.pattern(items[top[k]:]))
	}
	return f, nil
}

func hasUnit(items []item) bool {
	for _, it := range items {
		if it.isUnit() || hasUnit(it.opt) {
			return true
		}
	}
	return false
}

// trailingOptional makes the literals after position last optional, so a
// prefix of "{Рік}.{Частина} {Епоха}" still matches "144.087" at the end of
// a string.
func trailingOptional(items []item, last int) []item {
	out := append([]item(nil), items...)
	for i := last + 1; i < len(out); i++ {
		if out[i].opt == nil && !out[i].isUnit() {
			out[i] = item{unit: -1, opt: []item{out[i]}}
		}
	}
	return out
}

var spaceRun = regexp.MustCompile(`\s+`)

func (c *calendar) pattern(items []item) pattern {
	var b strings.Builder
	var units []int
	var write func([]item)
	write = func(items []item) {
		for _, it := range items {
			switch {
			case it.opt != nil:
				b.WriteString("(?:")
				write(it.opt)
				b.WriteString(")?")
			case it.isUnit():
				b.WriteString(c.unitRegex(it.unit))
				units = append(units, it.unit)
			default:
				parts := spaceRun.Split(it.lit, -1)
				for i, p := range parts {
					if i > 0 {
						b.WriteString(`\s*`)
					}
					b.WriteString(regexp.QuoteMeta(p))
				}
			}
		}
	}
	write(items)
	return pattern{re: regexp.MustCompile(`(?i)` + b.String()), units: units}
}

func (c *calendar) unitRegex(i int) string {
	u := c.Units[i]
	switch u.Write {
	case "roman":
		return `([IVXLCDMІХСМ]+)` // Cyrillic look-alikes too: "ІІІ"
	case "names":
		names := append([]string(nil), u.Names...)
		sort.Slice(names, func(a, b int) bool { return len(names[a]) > len(names[b]) })
		for k, n := range names {
			names[k] = regexp.QuoteMeta(n)
		}
		return `(` + strings.Join(names, "|") + `)`
	default:
		return `([-−]?\d+)`
	}
}

// --- reading dates ---------------------------------------------------------------

// tdate holds a value for each unit, root first; has marks which were given.
type tdate struct {
	v   []int
	has []bool
}

func (c *calendar) newDate() tdate {
	return tdate{v: make([]int, len(c.Units)), has: make([]bool, len(c.Units))}
}

func (d tdate) count() int {
	n := 0
	for _, h := range d.has {
		if h {
			n++
		}
	}
	return n
}

// before compares root first; a unit not given counts as the earliest.
func (d tdate) before(o tdate) bool {
	for i := range d.v {
		a, b := d.v[i], o.v[i]
		if !d.has[i] {
			a = -1 << 31
		}
		if !o.has[i] {
			b = -1 << 31
		}
		if a != b {
			return a < b
		}
	}
	return false
}

type dateRange struct {
	start, end tdate
	hasEnd     bool
	format     int // the format that read it, for statistics
}

var (
	// Links and footnote marks carry digits that are not dates: [[СМ-16]], [^1].
	noiseRe = regexp.MustCompile(`\[\[[^\]]*\]\]|\[\^[^\]]*\]`)
	// Cyrillic letters that look like Roman numerals, as typed in "ІІІ".
	romanLookalikes = strings.NewReplacer("І", "I", "Х", "X", "С", "C", "М", "M", "і", "I", "х", "X", "с", "C", "м", "M")
)

// parse reads a date or a range written in any of the calendar's formats.
// A single date must match a format whole; only the sides of a range may
// leave out units, which they then share.
func (c *calendar) parse(s string) (dateRange, bool) {
	s = noiseRe.ReplaceAllString(s, " ")
	a, b := splitRange(s)
	if b == "" {
		d, f, ok := c.best(a, false)
		return dateRange{start: d, format: f}, ok
	}
	// One side must be complete; the other may be shortened.
	start, f, okS := c.best(a, false)
	end, fe, okE := c.best(b, false)
	switch {
	case !okS && !okE:
		return dateRange{}, false
	case !okS:
		start, f, okS = c.best(a, true)
	case !okE:
		end, fe, okE = c.best(b, true)
	}
	if !okS { // text before a dash that is no date
		return dateRange{start: end, format: fe}, true
	}
	r := dateRange{start: start, format: f}
	if !okE {
		return r, true
	}
	inherit(r.start, end)
	inherit(end, r.start)
	if r.start.before(end) {
		r.end, r.hasEnd = end, true
	}
	if start.count() < end.count() {
		r.format = fe
	}
	return r, true
}

// inherit fills units of d that sit above ones it has from o: the epoch
// written once at the end of "144.087 — 144.125 III Епохи" belongs to both.
func inherit(d, o tdate) {
	deepest := -1
	for i, h := range d.has {
		if h {
			deepest = i
		}
	}
	for i := 0; i < deepest; i++ {
		if !d.has[i] && o.has[i] {
			d.v[i], d.has[i] = o.v[i], true
		}
	}
}

// best tries every format whole, then (when partial) its prefixes and
// suffixes, and keeps the reading that gives the most units.
func (c *calendar) best(s string, partial bool) (tdate, int, bool) {
	try := func(partial bool) (tdate, int, bool) {
		var best tdate
		bestF, found := -1, false
		for fi, f := range c.formats {
			ps := []pattern{f.full}
			if partial {
				ps = f.partials
			}
			for _, p := range ps {
				d, ok := c.read(p, s)
				if ok && (!found || d.count() > best.count()) {
					best, bestF, found = d, fi, true
				}
			}
		}
		return best, bestF, found
	}
	if d, f, ok := try(false); ok || !partial {
		return d, f, ok
	}
	return try(true)
}

func (c *calendar) read(p pattern, s string) (tdate, bool) {
	m := p.re.FindStringSubmatch(s)
	if m == nil {
		return tdate{}, false
	}
	d := c.newDate()
	for g, unit := range p.units {
		text := m[g+1]
		if text == "" {
			continue
		}
		v, ok := c.value(unit, text)
		if !ok {
			return tdate{}, false
		}
		d.v[unit], d.has[unit] = v, true
	}
	return d, d.count() > 0
}

func (c *calendar) value(unit int, text string) (int, bool) {
	u := c.Units[unit]
	switch u.Write {
	case "roman":
		return fromRoman(strings.ToUpper(romanLookalikes.Replace(text))), true
	case "names":
		for i, n := range u.Names {
			if strings.EqualFold(n, text) {
				return i + 1, true
			}
		}
		return 0, false
	default:
		n, err := strconv.Atoi(strings.Replace(text, "−", "-", 1))
		return n, err == nil
	}
}

// written renders a unit's value the way the calendar writes it.
func (c *calendar) written(unit, v int) string {
	u := c.Units[unit]
	switch u.Write {
	case "roman":
		return toRoman(v)
	case "names":
		if v >= 1 && v <= len(u.Names) {
			return u.Names[v-1]
		}
	}
	return strconv.Itoa(v)
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
	values := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
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

func toRoman(n int) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	var b strings.Builder
	for _, p := range []struct {
		v int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for n >= p.v {
			b.WriteString(p.s)
			n -= p.v
		}
	}
	return b.String()
}

// plainDate is a date field's text for display: links become their labels.
func plainDate(s string) string {
	s = regexp.MustCompile(`\[\^[^\]]*\]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`).ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// --- placing dates on an axis ----------------------------------------------------

// childCount is how many units of the next level fit in d's value at level
// i: an override for that value, the next unit's constant count, or the
// number of its names. 0 means unknown.
func (c *calendar) childCount(i int, d tdate) int {
	if i+1 >= len(c.Units) {
		return 0
	}
	if d.has[i] {
		if n, ok := c.Units[i].Lengths[c.written(i, d.v[i])]; ok {
			return n
		}
	}
	next := c.Units[i+1]
	if next.Per > 0 {
		return next.Per
	}
	if next.Write == "names" {
		return len(next.Names)
	}
	return 0
}

// stepsIn is how many steps one value of level i holds when every value
// holds the same; 0 when unknown or uneven.
func (c *calendar) stepsIn(i int) int {
	if i == c.step {
		return 1
	}
	if i > c.step || len(c.Units[i].Lengths) > 0 {
		return 0
	}
	per := c.childCount(i, c.newDate())
	if per == 0 {
		return 0
	}
	return per * c.stepsIn(i+1)
}

// offset is where d's parent values begin on an axis shared by all of them,
// counted in steps; ok is false when some length on the way is unknown.
// Values of units above the step count from 1.
func (c *calendar) offset(d tdate) (float64, bool) {
	off := 0
	for i := 0; i < c.step; i++ {
		if !d.has[i] || d.v[i] < 1 {
			return 0, false
		}
		probe := c.newDate()
		copy(probe.v, d.v)
		copy(probe.has, d.has)
		for u := 1; u < d.v[i]; u++ {
			probe.v[i] = u
			n := c.childCount(i, probe)
			per := c.stepsIn(i + 1)
			if n == 0 || per == 0 {
				return 0, false
			}
			off += n * per
		}
	}
	return float64(off), true
}

// fraction places d inside its step using the units below it, as far as
// their counts are known.
func (c *calendar) fraction(d tdate) float64 {
	f, scale := 0.0, 1.0
	for j := c.step + 1; j < len(c.Units) && d.has[j]; j++ {
		n := c.childCount(j-1, d)
		if n == 0 {
			break
		}
		v := min(max(d.v[j], 0), n)
		f += scale * float64(v) / float64(n+1)
		scale /= float64(n)
	}
	return f
}

// beyond reports a unit value larger than its parent allows: part 231 in a
// year of 100.
func (c *calendar) beyond(d tdate) bool {
	for j := 1; j < len(c.Units); j++ {
		if !d.has[j] || !d.has[j-1] {
			continue
		}
		if n := c.childCount(j-1, d); n > 0 && d.v[j] > n {
			return true
		}
	}
	return false
}

// parentKey identifies the values above the step; "" when there are none.
func (c *calendar) parentKey(d tdate) string {
	var parts []string
	for i := 0; i < c.step; i++ {
		if !d.has[i] {
			return "?"
		}
		parts = append(parts, c.written(i, d.v[i]))
	}
	return strings.Join(parts, " / ")
}

// parentLabel names an axis: "Епоха III", or a named value as it is.
func (c *calendar) parentLabel(d tdate, upTo int) string {
	var parts []string
	for i := 0; i < upTo; i++ {
		if !d.has[i] {
			return "Без: " + c.Units[i].Name
		}
		parts = append(parts, c.valueLabel(i, d.v[i]))
	}
	return strings.Join(parts, " · ")
}

func (c *calendar) valueLabel(i, v int) string {
	if c.Units[i].Write == "names" {
		return c.written(i, v)
	}
	return c.Units[i].Name + " " + c.written(i, v)
}

// stepLabel writes d down to the step, in the first format that can: one
// using exactly the units from the root to the step. Failing that, values
// are listed.
func (c *calendar) stepLabel(d tdate) string {
	if !c.custom {
		label := strconv.Itoa(d.v[0])
		if c.era != "" {
			label += " " + c.era
		}
		return label
	}
	want := map[int]bool{}
	for i := 0; i <= c.step; i++ {
		if d.has[i] {
			want[i] = true
		}
	}
	for _, f := range c.formats {
		if s, ok := c.render(f.items, d, want); ok {
			return strings.TrimSpace(s)
		}
	}
	parts := []string{c.written(c.step, d.v[c.step])}
	for i := c.step - 1; i >= 0; i-- {
		if d.has[i] {
			parts = append(parts, c.valueLabel(i, d.v[i]))
		}
	}
	return strings.Join(parts, ", ")
}

// render fills a template with the wanted units only. It fails when a
// required unit is not wanted or a wanted one is missing from the template;
// an optional group shows when all its units are wanted.
func (c *calendar) render(items []item, d tdate, want map[int]bool) (string, bool) {
	var walk func([]item) (string, []int, bool)
	walk = func(items []item) (string, []int, bool) {
		var b strings.Builder
		var used []int
		for _, it := range items {
			switch {
			case it.opt != nil:
				if s, u, ok := walk(it.opt); ok {
					b.WriteString(s)
					used = append(used, u...)
				}
			case it.isUnit():
				if !want[it.unit] {
					return "", nil, false
				}
				used = append(used, it.unit)
				b.WriteString(c.written(it.unit, d.v[it.unit]))
			default:
				b.WriteString(it.lit)
			}
		}
		return b.String(), used, true
	}
	s, used, ok := walk(items)
	if !ok {
		return "", false
	}
	seen := map[int]bool{}
	for _, u := range used {
		seen[u] = true
	}
	for u := range want {
		if !seen[u] {
			return "", false
		}
	}
	return s, true
}

// --- saving ------------------------------------------------------------------------

// SaveCalendar replaces the world's calendar, keeping the rest of
// world.yaml. A calendar without units removes the section, so the world
// counts plain years again.
func (v *Vault) SaveCalendar(cal Calendar) (World, error) {
	cal = cleanCalendar(cal)
	if len(cal.Units) > 0 {
		if _, err := compileCalendar(cal, ""); err != nil {
			return World{}, err
		}
	}

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
	if len(cal.Units) == 0 {
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

func cleanCalendar(c Calendar) Calendar {
	out := Calendar{Step: strings.TrimSpace(c.Step), Units: []TimeUnit{}, Formats: []string{}}
	for _, u := range c.Units {
		u.Name = strings.TrimSpace(u.Name)
		if u.Write == "number" {
			u.Write = ""
		}
		if u.Write != "names" {
			u.Names = nil
		}
		var names []string
		for _, n := range u.Names {
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
		u.Names = names
		lengths := map[string]int{}
		for k, n := range u.Lengths {
			if k = strings.TrimSpace(k); k != "" && n > 0 {
				lengths[k] = n
			}
		}
		u.Lengths = nil
		if len(lengths) > 0 {
			u.Lengths = lengths
		}
		out.Units = append(out.Units, u)
	}
	for _, f := range c.Formats {
		if f = strings.TrimSpace(f); f != "" {
			out.Formats = append(out.Formats, f)
		}
	}
	return out
}
