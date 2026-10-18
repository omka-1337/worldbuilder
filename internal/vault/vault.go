package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"golang.org/x/text/collate"
)

var (
	ErrNotFound = errors.New("page not found")
	ErrExists   = errors.New("a file with this name already exists")
)

// Vault is a directory of Markdown articles plus an in-memory index of
// titles, aliases, links and categories. The files are the source of truth;
// the index can always be rebuilt from them.
type Vault struct {
	root string
	base string // URL prefix for links: pages at base/wiki/…, files at base/files/…
	md   goldmark.Markdown

	mu    sync.RWMutex
	world World
	idx   *index

	cfgMu sync.Mutex // serializes writes to world.yaml
}

type index struct {
	pages     map[string]*Page   // by path
	byKey     map[string]*Page   // by Key(title) and Key(alias)
	files     map[string]string  // attachments: lowercased base name → path
	backlinks map[*Page][]*Page  // target → pages linking to it
	wanted    map[string]*wanted // Key(missing title) → who links to it
}

type wanted struct {
	title string // first spelling seen
	from  []*Page
}

// Open loads every article under root. base prefixes the URLs that rendered
// links point to; "" gives /wiki/Title.
func Open(root, base string) (*Vault, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	v := &Vault{root: root, base: strings.TrimSuffix(base, "/")}
	v.md = newMarkdown(v)
	if err := v.Reload(); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *Vault) Root() string { return v.root }

// Reload rescans the whole vault from disk. A file with broken frontmatter
// is logged and skipped rather than failing the whole load.
func (v *Vault) Reload() error {
	world, err := LoadWorld(v.root)
	if err != nil {
		return err
	}
	idx := &index{pages: map[string]*Page{}, files: map[string]string{}}
	err = filepath.WalkDir(v.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(v.root, p)
		rel = filepath.ToSlash(rel)
		if rel != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || rel == worldFile {
			return nil
		}
		if !strings.EqualFold(path.Ext(rel), ".md") {
			idx.files[strings.ToLower(path.Base(rel))] = rel
			return nil
		}
		page, err := v.readPage(rel)
		if err != nil {
			log.Printf("skipping %v", err)
			return nil
		}
		idx.pages[rel] = page
		return nil
	})
	if err != nil {
		return err
	}
	v.link(idx)

	v.mu.Lock()
	v.world, v.idx = world, idx
	v.mu.Unlock()
	return nil
}

func (v *Vault) readPage(rel string) (*Page, error) {
	abs := filepath.Join(v.root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	p, err := parsePage(rel, raw)
	if err != nil {
		return nil, err
	}
	p.ModTime = info.ModTime()
	p.links = v.extractLinks(p)
	return p, nil
}

// link computes everything derived from the set of pages: title and alias
// lookup, backlinks and wanted pages.
func (v *Vault) link(idx *index) {
	paths := make([]string, 0, len(idx.pages))
	for p := range idx.pages {
		paths = append(paths, p)
	}
	sort.Strings(paths) // deterministic winner on duplicate titles

	idx.byKey = map[string]*Page{}
	for _, p := range paths {
		page := idx.pages[p]
		k := Key(page.Title)
		if other, dup := idx.byKey[k]; dup {
			log.Printf("duplicate title %q in %s and %s", page.Title, other.Path, page.Path)
			continue
		}
		idx.byKey[k] = page
	}
	for _, p := range paths { // aliases never shadow real titles
		page := idx.pages[p]
		for _, a := range page.Aliases {
			if _, taken := idx.byKey[Key(a)]; !taken {
				idx.byKey[Key(a)] = page
			}
		}
	}

	idx.backlinks = map[*Page][]*Page{}
	idx.wanted = map[string]*wanted{}
	for _, p := range paths {
		page := idx.pages[p]
		// Dedupe by destination: [[Title]] and [[Alias]] are one link.
		linked := map[*Page]bool{page: true}
		missing := map[string]bool{}
		for _, target := range page.links {
			k := Key(target)
			if to, ok := idx.byKey[k]; ok {
				if !linked[to] {
					linked[to] = true
					idx.backlinks[to] = append(idx.backlinks[to], page)
				}
				continue
			}
			if missing[k] {
				continue
			}
			missing[k] = true
			w := idx.wanted[k]
			if w == nil {
				w = &wanted{title: strings.Join(strings.Fields(strings.ReplaceAll(target, "_", " ")), " ")}
				idx.wanted[k] = w
			}
			w.from = append(w.from, page)
		}
	}
}

// lookup finds a page by title or alias. Callers hold v.mu.
func (v *Vault) lookup(title string) (*Page, bool) {
	p, ok := v.idx.byKey[Key(title)]
	return p, ok
}

func (v *Vault) attachment(name string) (string, bool) {
	p, ok := v.idx.files[strings.ToLower(path.Base(name))]
	return p, ok
}

func (v *Vault) collator() *collate.Collator {
	return collate.New(v.world.lang(), collate.IgnoreCase)
}

func (v *Vault) sortedTitles(pages []*Page) []string {
	titles := make([]string, len(pages))
	for i, p := range pages {
		titles[i] = p.Title
	}
	v.collator().SortStrings(titles)
	return titles
}

// World returns the vault configuration.
func (v *Vault) World() World {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.world
}

// Summary is a page as it appears in lists.
type Summary struct {
	Title      string   `json:"title"`
	Type       string   `json:"type,omitempty"`
	Categories []string `json:"categories"`
}

func summarize(p *Page) Summary {
	return Summary{Title: p.Title, Type: p.Type, Categories: nonNil(p.Categories)}
}

// List returns every page, sorted by title in the world's language.
func (v *Vault) List() []Summary {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.summaries(func(*Page) bool { return true })
}

func (v *Vault) summaries(keep func(*Page) bool) []Summary {
	out := []Summary{}
	for _, p := range v.idx.pages {
		if keep(p) {
			out = append(out, summarize(p))
		}
	}
	c := v.collator()
	sort.Slice(out, func(i, j int) bool { return c.CompareString(out[i].Title, out[j].Title) < 0 })
	return out
}

// View is everything needed to display an article.
type View struct {
	Title          string    `json:"title"`
	Path           string    `json:"path"`
	Type           string    `json:"type,omitempty"`
	TypeLabel      string    `json:"typeLabel,omitempty"`
	RedirectedFrom string    `json:"redirectedFrom,omitempty"`
	Aliases        []string  `json:"aliases"`
	Categories     []string  `json:"categories"`
	Infobox        []Row     `json:"infobox"`
	Image          string    `json:"image,omitempty"` // rendered HTML
	HTML           string    `json:"html"`
	Headings       []Heading `json:"headings"`
	Backlinks      []string  `json:"backlinks"`
	Raw            string    `json:"raw"`
	Modified       time.Time `json:"modified"`
}

// Row is one rendered infobox row.
type Row struct {
	Label string `json:"label"`
	HTML  string `json:"html"`
}

// Get renders the page with the given title or alias.
func (v *Vault) Get(title string) (*View, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	p, ok := v.lookup(title)
	if !ok {
		return nil, ErrNotFound
	}
	html, headings, err := v.render(p.Body)
	if err != nil {
		return nil, err
	}
	view := &View{
		Title:      p.Title,
		Path:       p.Path,
		Type:       p.Type,
		Aliases:    nonNil(p.Aliases),
		Categories: nonNil(p.Categories),
		Infobox:    []Row{},
		HTML:       html,
		Headings:   headings,
		Backlinks:  v.sortedTitles(v.idx.backlinks[p]),
		Raw:        string(p.Raw),
		Modified:   p.ModTime,
	}
	if Key(p.Title) != Key(title) {
		view.RedirectedFrom = title
	}

	// Infobox rows follow the order declared by the type, then any extra
	// fields in file order, labelled by their key.
	def := v.world.Types[p.Type]
	view.TypeLabel = def.Label
	values := map[string]string{}
	for _, f := range p.Infobox {
		values[f.Key] = f.Value
	}
	if img := values["image"]; img != "" {
		view.Image = v.renderInline(img)
	}
	used := map[string]bool{"image": true}
	for _, fd := range def.Fields {
		used[fd.Key] = true
		if val := values[fd.Key]; val != "" {
			view.Infobox = append(view.Infobox, Row{Label: fd.Label, HTML: v.renderInline(val)})
		}
	}
	for _, f := range p.Infobox {
		if !used[f.Key] && f.Value != "" {
			view.Infobox = append(view.Infobox, Row{Label: f.Key, HTML: v.renderInline(f.Value)})
		}
	}
	return view, nil
}

// Save writes content to the page with the given title, creating a new
// file if there is none. It returns the title the page ends up with, which
// differs from the requested one when the frontmatter sets its own.
func (v *Vault) Save(title string, content []byte) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	var rel string
	if p, ok := v.lookup(title); ok {
		rel = p.Path
	} else {
		rel = fileNameFor(title)
		if _, err := os.Stat(filepath.Join(v.root, rel)); err == nil {
			return "", ErrExists
		}
	}

	// Parse first so malformed frontmatter is rejected without touching disk.
	if _, err := parsePage(rel, content); err != nil {
		return "", err
	}
	if err := writeAtomic(filepath.Join(v.root, filepath.FromSlash(rel)), content); err != nil {
		return "", err
	}
	page, err := v.readPage(rel)
	if err != nil {
		return "", err
	}
	v.idx.pages[rel] = page
	v.link(v.idx)
	return page.Title, nil
}

func writeAtomic(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), name)
}

// Category is a category name and how many pages it holds.
type Category struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Categories lists every category used by at least one page.
func (v *Vault) Categories() []Category {
	v.mu.RLock()
	defer v.mu.RUnlock()

	counts := map[string]*Category{}
	for _, p := range v.idx.pages {
		for _, c := range p.Categories {
			k := Key(c)
			if counts[k] == nil {
				counts[k] = &Category{Name: c}
			}
			counts[k].Count++
		}
	}
	out := make([]Category, 0, len(counts))
	for _, c := range counts {
		out = append(out, *c)
	}
	col := v.collator()
	sort.Slice(out, func(i, j int) bool { return col.CompareString(out[i].Name, out[j].Name) < 0 })
	return out
}

// CategoryMembers lists the pages in a category.
func (v *Vault) CategoryMembers(name string) []Summary {
	v.mu.RLock()
	defer v.mu.RUnlock()
	k := Key(name)
	return v.summaries(func(p *Page) bool {
		return slices.ContainsFunc(p.Categories, func(c string) bool { return Key(c) == k })
	})
}

// Wanted is a title that is linked to but has no article yet.
type Wanted struct {
	Title string   `json:"title"`
	From  []string `json:"from"`
}

// Wanted lists missing pages, most linked first.
func (v *Vault) Wanted() []Wanted {
	v.mu.RLock()
	defer v.mu.RUnlock()

	out := make([]Wanted, 0, len(v.idx.wanted))
	for _, w := range v.idx.wanted {
		out = append(out, Wanted{Title: w.title, From: v.sortedTitles(w.from)})
	}
	c := v.collator()
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].From) != len(out[j].From) {
			return len(out[i].From) > len(out[j].From)
		}
		return c.CompareString(out[i].Title, out[j].Title) < 0
	})
	return out
}

// Hit is one search result.
type Hit struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

// Search does a case-insensitive substring search. Title and alias matches
// rank above matches in the text.
func (v *Vault) Search(query string, limit int) []Hit {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return []Hit{}
	}
	v.mu.RLock()
	defer v.mu.RUnlock()

	type scored struct {
		hit   Hit
		score int
	}
	var found []scored
	for _, p := range v.idx.pages {
		i := strings.Index(p.searchText, q)
		if i < 0 {
			continue
		}
		score := 2
		switch {
		case Key(p.Title) == Key(q):
			score = 0
		case strings.Contains(strings.ToLower(p.Title), q) ||
			slices.ContainsFunc(p.Aliases, func(a string) bool { return strings.Contains(strings.ToLower(a), q) }):
			score = 1
		}
		// Lowercasing rarely changes byte lengths; when it does, fall back
		// to the lowercased text so offsets stay valid.
		text := p.searchRaw
		if len(text) != len(p.searchText) {
			text = p.searchText
		}
		found = append(found, scored{Hit{Title: p.Title, Snippet: snippet(text, i, len(q))}, score})
	}
	c := v.collator()
	sort.Slice(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score < found[j].score
		}
		return c.CompareString(found[i].hit.Title, found[j].hit.Title) < 0
	})
	out := []Hit{}
	for i := 0; i < len(found) && i < limit; i++ {
		out = append(out, found[i].hit)
	}
	return out
}

// snippet cuts about 60 bytes of context on each side of a match, on rune
// boundaries, and flattens Markdown link brackets.
func snippet(s string, at, n int) string {
	start, end := max(0, at-60), min(len(s), at+n+60)
	for start > 0 && !utf8.RuneStart(s[start]) {
		start--
	}
	for end < len(s) && !utf8.RuneStart(s[end]) {
		end++
	}
	out := strings.NewReplacer("[[", "", "]]", "", "**", "", "__", "", "\n", " ").Replace(s[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(s) {
		out += "…"
	}
	return strings.TrimSpace(out)
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
