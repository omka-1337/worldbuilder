package vault

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fixture is a small world exercising every index feature. Titles are
// Cyrillic on purpose: case folding, slugs and collation must handle them.
var fixture = map[string]string{
	"world.yaml": `
name: Test
era: ЕП
types:
  character:
    label: Персонаж
    fields:
      - { key: born, label: Народження, type: date }
      - { key: died, label: Смерть, type: date }
      - { key: affiliation, label: Приналежність }
  location:
    label: Локація
    fields:
      - { key: founded, label: Засновано, type: date }
  event:
    label: Подія
    event: true
    fields:
      - { key: date, label: Дата, type: date }
`,
	"Головна.md": "[[Аедрін Сірий]], [[Сірий Вовк]] і [[Війна Попелу]].\n",
	"Аедрін Сірий.md": `---
type: character
aliases: [Сірий Вовк]
categories: [Персонажі, Маги]
infobox:
  born: 312 ЕП, [[Вальмор]]
  died: 389 ЕП
  affiliation: "[[Орден Попелу]]"
  Титул: Перший Магістр
---
**Аедрін Сірий** — маг, засновник ордену. Див. [[#Спадщина]].

## Ранні роки

Навчався в [[Академія Тлінь|Академії Тлінь]]. Воював у [[Війна Попелу|Війні Попелу]].
Ця стаття навмисно довша за будь-яку подію, щоб перевірити ранжування на таймлайні.

## Заснування ордену

Текст.

## Спадщина

Текст.
`,
	"Вальмор.md": `---
type: location
infobox:
  founded: 120 ЕП
---
Тут народився [[Сірий Вовк]].
`,
	"Орден Попелу.md": "Заснований [[Аедрін Сірий|Аедріном]].\n",
	"Битва під Тліню.md": `---
type: event
infobox:
  date: 343 ЕП
---
Вирішальна битва війни, у якій маги вільних міст уперше об'єднали сили.
`,
	"Облога Вальмора.md": `---
type: event
infobox:
  date: 343–345 ЕП
---
Невдала облога.
`,
	"Падіння Сірої Вежі.md": `---
type: event
infobox:
  date: 389 ЕП
---
Пожежа.
`,
}

func writeWorld(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func openFixture(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(writeWorld(t, fixture), "")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRedirectAndInfobox(t *testing.T) {
	v := openFixture(t)
	view, err := v.Get("сірий_вовк")
	if err != nil {
		t.Fatal(err)
	}
	if view.Title != "Аедрін Сірий" || view.RedirectedFrom != "сірий_вовк" {
		t.Errorf("got title %q redirected from %q", view.Title, view.RedirectedFrom)
	}
	if view.TypeLabel != "Персонаж" {
		t.Errorf("type label = %q", view.TypeLabel)
	}
	// Schema fields come first in schema order, then extra fields by key.
	var labels []string
	for _, r := range view.Infobox {
		labels = append(labels, r.Label)
	}
	want := []string{"Народження", "Смерть", "Приналежність", "Титул"}
	if !slices.Equal(labels, want) {
		t.Errorf("infobox labels = %v, want %v", labels, want)
	}
	if !strings.Contains(view.Infobox[0].HTML, `href="/wiki/%D0%92%D0%B0%D0%BB%D1%8C%D0%BC%D0%BE%D1%80"`) {
		t.Errorf("infobox link not rendered: %s", view.Infobox[0].HTML)
	}
}

func TestLinksAndHeadings(t *testing.T) {
	v := openFixture(t)
	view, err := v.Get("Аедрін Сірий")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.HTML, `class="wikilink new" href="/edit/`) {
		t.Error("missing page should render as a red link to the editor")
	}
	if !strings.Contains(view.HTML, `href="#%D1%81%D0%BF%D0%B0%D0%B4%D1%89%D0%B8%D0%BD%D0%B0"`) {
		t.Errorf("same-page fragment link not rendered: %s", view.HTML)
	}
	ids := []string{}
	for _, h := range view.Headings {
		ids = append(ids, h.ID)
	}
	if !slices.Equal(ids, []string{"ранні-роки", "заснування-ордену", "спадщина"}) {
		t.Errorf("heading ids = %v", ids)
	}
	// The main page links both the title and the alias: still one backlink.
	if !slices.Equal(view.Backlinks, []string{"Вальмор", "Головна", "Орден Попелу"}) {
		t.Errorf("backlinks = %v", view.Backlinks)
	}
}

func TestWantedAndCategories(t *testing.T) {
	v := openFixture(t)
	wanted := v.Wanted()
	if len(wanted) != 2 || wanted[0].Title != "Війна Попелу" || len(wanted[0].From) != 2 {
		t.Fatalf("wanted = %+v", wanted)
	}
	members := v.CategoryMembers("маги")
	if len(members) != 1 || members[0].Title != "Аедрін Сірий" {
		t.Errorf("category members = %+v", members)
	}
}

func TestSaveCreatesAndRejectsBadYAML(t *testing.T) {
	v := openFixture(t)
	title, err := v.Save("Академія Тлінь", []byte("---\ntype: location\n---\nШкола магів.\n"))
	if err != nil || title != "Академія Тлінь" {
		t.Fatalf("save = %q, %v", title, err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), "Академія Тлінь.md")); err != nil {
		t.Fatal(err)
	}
	for _, w := range v.Wanted() {
		if w.Title == "Академія Тлінь" {
			t.Error("created page still listed as wanted")
		}
	}

	_, err = v.Save("Вальмор", []byte("---\ninfobox: [unclosed\n---\n"))
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Errorf("expected ParseError, got %v", err)
	}
}

func TestSearch(t *testing.T) {
	v := openFixture(t)
	hits := v.Search("вальмор", 10)
	if len(hits) == 0 || hits[0].Title != "Вальмор" {
		t.Fatalf("hits = %+v", hits)
	}
	for _, h := range hits[1:] {
		if strings.Contains(h.Snippet, "[[") || strings.Contains(h.Snippet, "**") {
			t.Errorf("snippet keeps markup: %q", h.Snippet)
		}
	}
}

func TestTimeline(t *testing.T) {
	v := openFixture(t)
	years := map[int]TimelineYear{}
	var order []int
	for _, y := range v.Timeline() {
		years[y.Year] = y
		order = append(order, y.Year)
	}
	if !slices.Equal(order, []int{120, 312, 343, 389}) {
		t.Errorf("years = %v", order)
	}
	// Two events in 343: the longer article names the year.
	if y := years[343]; len(y.Entries) != 2 || y.Main.Title != "Битва під Тліню" {
		t.Errorf("343 = %+v", y)
	}
	// An event outranks a longer non-event article from the same year.
	if y := years[389]; len(y.Entries) != 2 || y.Main.Title != "Падіння Сірої Вежі" || !y.Main.Event {
		t.Errorf("389 = %+v", y)
	}
	// A year without events is named by its longest article.
	if y := years[312]; y.Main.Title != "Аедрін Сірий" || y.Main.Field != "Народження" {
		t.Errorf("312 = %+v", y)
	}
	for _, e := range years[343].Entries {
		if e.Title == "Облога Вальмора" && e.EndYear != 345 {
			t.Errorf("range end = %d", e.EndYear)
		}
	}
}

func TestParseDate(t *testing.T) {
	type pt = datePoint
	cases := []struct {
		in         string
		start, end pt
		ok         bool
	}{
		{"312 ЕП", pt{0, 312, 0}, pt{}, true},
		{"340–347 ЕП", pt{0, 340, 0}, pt{0, 347, 0}, true},
		{"120 ЕП, [[Вальмор]]", pt{0, 120, 0}, pt{}, true},
		{"-40", pt{0, -40, 0}, pt{}, true},
		{"−200 — −150", pt{0, -200, 0}, pt{0, -150, 0}, true},
		{"139.096 III Епохи", pt{3, 139, 96}, pt{}, true},
		{"144.087 — 144.125 III Епохи", pt{3, 144, 87}, pt{3, 144, 125}, true},
		{"142 р. III Епохи — 145 р. III Епохи", pt{3, 142, 0}, pt{3, 145, 0}, true},
		{"211-224 III Епохи", pt{3, 211, 0}, pt{3, 224, 0}, true},
		{"145 р. ІІІ Епохи", pt{3, 145, 0}, pt{}, true},  // Cyrillic І
		{"224-2XX III Епохи", pt{3, 224, 0}, pt{}, true}, // an unknown end is dropped
		{"V Епоха, 12", pt{5, 12, 0}, pt{}, true},
		{"[[СМ-16]] 450", pt{0, 450, 0}, pt{}, true}, // digits inside links are not dates
		{"невідомо", pt{}, pt{}, false},
	}
	for _, c := range cases {
		r, ok := parseDate(c.in)
		if ok != c.ok || r.start != c.start || r.end != c.end {
			t.Errorf("parseDate(%q) = %+v, %v; want %+v–%+v", c.in, r, ok, c.start, c.end)
		}
	}
}

func TestSplitFrontmatter(t *testing.T) {
	fm, body := splitFrontmatter([]byte("---\na: 1\n---\nbody"))
	if string(fm) != "a: 1\n" || string(body) != "body" {
		t.Errorf("got %q / %q", fm, body)
	}
	fm, body = splitFrontmatter([]byte("---\nunterminated"))
	if fm != nil || string(body) != "---\nunterminated" {
		t.Errorf("unterminated: %q / %q", fm, body)
	}
}

func TestSaveTypes(t *testing.T) {
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["world.yaml"] += "mainPage: Головна\ncustom: kept\n"
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}

	types := v.Types()
	var character *TypeInfo
	for i := range types {
		if types[i].Key == "character" {
			character = &types[i]
		}
	}
	if character == nil || character.Count != 1 {
		t.Fatalf("character type = %+v", character)
	}

	// Rename a type and a field, add a field and a new type; keys of
	// existing entries must survive, new ones are generated from labels.
	character.Label = "Герой"
	character.Color = "#8E5BD4"
	character.Fields[0].Label = "Дата народження"
	character.Fields = append(character.Fields, FieldDef{Label: "Рід занять"})
	types = append(types, TypeInfo{Label: "Магічний артефакт", Color: "red", Fields: []FieldDef{
		{Label: "Створено", Type: "date"},
		{Label: "Створено"}, // same label: key must still be unique
	}})
	saved, err := v.SaveTypes(types)
	if err != nil {
		t.Fatal(err)
	}

	byKey := map[string]TypeInfo{}
	for _, ti := range saved {
		byKey[ti.Key] = ti
	}
	if c := byKey["character"]; c.Label != "Герой" || c.Color != "#8e5bd4" || c.Fields[0].Key != "born" || c.Fields[3].Key != "рід-занять" {
		t.Errorf("character after save = %+v", c)
	}
	a, ok := byKey["магічний-артефакт"]
	if !ok || a.Color != "" || a.Fields[0].Key != "створено" || a.Fields[1].Key != "створено-2" || a.Fields[0].Type != "date" {
		t.Errorf("new type = %+v (types %v)", a, saved)
	}

	// The article still resolves its (renamed) type; other settings survive.
	view, err := v.Get("Аедрін Сірий")
	if err != nil || view.TypeLabel != "Герой" || view.Infobox[0].Label != "Дата народження" {
		t.Errorf("article after rename: %+v, %v", view, err)
	}
	raw, _ := os.ReadFile(filepath.Join(v.Root(), "world.yaml"))
	if !strings.Contains(string(raw), "custom: kept") || v.World().Era != "ЕП" {
		t.Errorf("other settings lost:\n%s", raw)
	}

	if _, err := v.SaveTypes([]TypeInfo{{Label: " "}}); !errors.As(err, new(*InvalidError)) {
		t.Errorf("empty label: %v", err)
	}
	if _, err := v.SaveTypes([]TypeInfo{{Key: "a", Label: "A"}, {Key: "a", Label: "B"}}); !errors.As(err, new(*InvalidError)) {
		t.Errorf("duplicate key: %v", err)
	}
}

func TestSourceRoundTrip(t *testing.T) {
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["Вальмор.md"] = "---\ntitle: Вальмор\ncustom: kept\ntype: location\ninfobox:\n  founded: 120 ЕП\n---\nТекст.\n"
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}

	src := v.Source("Вальмор")
	if !src.Exists || src.Type != "location" || src.Infobox[0] != (Field{"founded", "120 ЕП"}) || src.Body != "Текст.\n" {
		t.Fatalf("source = %+v", src)
	}

	// Values that need quoting in YAML must survive; empty ones are dropped;
	// unknown keys stay.
	src.Infobox = []Field{{"founded", "[[Аедрін Сірий]]: 120"}, {"ruler", ""}, {"Мотто", "- вогонь"}}
	src.Categories = []string{"Міста", " ", "міста"}
	src.Aliases = nil
	if _, err := v.SaveSource("Вальмор", src); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(v.Root(), "Вальмор.md"))
	if !strings.Contains(string(raw), "custom: kept") || strings.Contains(string(raw), "ruler") || strings.Contains(string(raw), "aliases") {
		t.Errorf("file after save:\n%s", raw)
	}
	again := v.Source("Вальмор")
	if len(again.Infobox) != 2 || again.Infobox[0].Value != "[[Аедрін Сірий]]: 120" || again.Infobox[1].Value != "- вогонь" {
		t.Errorf("infobox after round trip = %+v", again.Infobox)
	}
	if len(again.Categories) != 1 {
		t.Errorf("categories = %v", again.Categories)
	}

	// A new page with nothing but text has no frontmatter at all.
	if _, err := v.SaveSource("Нова", Source{Body: "Просто текст.\n"}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(v.Root(), "Нова.md"))
	if string(raw) != "Просто текст.\n" {
		t.Errorf("new page file = %q", raw)
	}
	if missing := v.Source("Немає"); missing.Exists {
		t.Error("missing page reported as existing")
	}
}

func TestSaveSourceCreateAndRename(t *testing.T) {
	v := openFixture(t)

	// Creating never overwrites, whether the title is a page or an alias.
	for _, title := range []string{"Вальмор", "сірий вовк"} {
		if _, err := v.SaveSource(title, Source{Create: true, Title: title, Body: "x"}); !errors.Is(err, ErrExists) {
			t.Errorf("create %q: %v", title, err)
		}
	}

	// Renaming moves the file and keeps the old title as an alias.
	src := v.Source("Вальмор")
	src.Title = "Вальмор-на-Попелі"
	title, err := v.SaveSource("Вальмор", src)
	if err != nil || title != "Вальмор-на-Попелі" {
		t.Fatalf("rename = %q, %v", title, err)
	}
	if fileExists(filepath.Join(v.Root(), "Вальмор.md")) || !fileExists(filepath.Join(v.Root(), "Вальмор-на-Попелі.md")) {
		t.Error("file not renamed")
	}
	view, err := v.Get("Вальмор")
	if err != nil || view.Title != "Вальмор-на-Попелі" || view.RedirectedFrom == "" {
		t.Errorf("old title does not redirect: %+v, %v", view, err)
	}
	// Links to the old title still count as backlinks.
	if !slices.Contains(view.Backlinks, "Аедрін Сірий") {
		t.Errorf("backlinks after rename = %v", view.Backlinks)
	}

	// Renaming onto another page is refused; onto its own alias is fine.
	src = v.Source("Аедрін Сірий")
	src.Title = "Головна"
	if _, err := v.SaveSource("Аедрін Сірий", src); !errors.Is(err, ErrExists) {
		t.Errorf("rename onto another page: %v", err)
	}
	src.Title = "Сірий Вовк"
	if _, err := v.SaveSource("Аедрін Сірий", src); err != nil {
		t.Fatal(err)
	}
	got := v.Source("Сірий Вовк")
	if got.Title != "Сірий Вовк" || !slices.Equal(got.Aliases, []string{"Аедрін Сірий"}) {
		t.Errorf("after rename to alias: title %q aliases %v", got.Title, got.Aliases)
	}

	// An empty title is rejected.
	if _, err := v.SaveSource("", Source{Create: true}); !errors.As(err, new(*InvalidError)) {
		t.Errorf("empty title: %v", err)
	}
}

func TestNesting(t *testing.T) {
	v := openFixture(t)
	root := v.Root()
	exists := func(rel string) bool { return fileExists(filepath.Join(root, filepath.FromSlash(rel))) }

	// Drop the battle onto the location, then the location onto the main page:
	// the battle travels along inside its parent's folder.
	if err := v.Move("Битва під Тліню", "Вальмор"); err != nil {
		t.Fatal(err)
	}
	if !exists("Вальмор/Битва під Тліню.md") || !exists("Вальмор.md") {
		t.Fatal("child not moved into the parent's folder")
	}
	if err := v.Move("Вальмор", "Головна"); err != nil {
		t.Fatal(err)
	}
	if !exists("Головна/Вальмор.md") || !exists("Головна/Вальмор/Битва під Тліню.md") || exists("Вальмор") {
		t.Fatal("parent's folder did not move with it")
	}

	// A page cannot go inside itself or its own descendant.
	if err := v.Move("Головна", "Битва під Тліню"); !errors.As(err, new(*InvalidError)) {
		t.Errorf("cycle: %v", err)
	}

	view, err := v.Get("Битва під Тліню")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Location) != 2 || view.Location[0].Title != "Головна" || view.Location[1].Title != "Вальмор" {
		t.Errorf("location = %+v", view.Location)
	}
	if parent, _ := v.Get("Вальмор"); !slices.Equal(parent.Children, []string{"Битва під Тліню"}) {
		t.Errorf("children = %v", parent.Children)
	}
	// Links resolve by title, so moving a page does not break them.
	if aedrin, _ := v.Get("Аедрін Сірий"); !slices.Contains(aedrin.Backlinks, "Вальмор") {
		t.Errorf("link from a moved page lost: %v", aedrin.Backlinks)
	}

	tree := v.Tree()
	var main *TreeNode
	for _, n := range tree {
		if n.Title == "Головна" {
			main = n
		}
	}
	if main == nil || len(main.Children) != 1 || main.Children[0].Title != "Вальмор" || main.Children[0].Children[0].Title != "Битва під Тліню" {
		t.Errorf("tree = %+v", main)
	}

	// Renaming a parent renames its folder.
	src := v.Source("Вальмор")
	src.Title = "Вальмор-на-Попелі"
	if _, err := v.SaveSource("Вальмор", src); err != nil {
		t.Fatal(err)
	}
	if !exists("Головна/Вальмор-на-Попелі/Битва під Тліню.md") || exists("Головна/Вальмор") {
		t.Error("folder not renamed with its article")
	}

	// Back to the top level; emptied folders disappear.
	if err := v.Move("Вальмор-на-Попелі", ""); err != nil {
		t.Fatal(err)
	}
	if !exists("Вальмор-на-Попелі/Битва під Тліню.md") || exists("Головна") {
		t.Error("move to top level or pruning failed")
	}
}

// A minimal valid PNG (1×1 pixel).
var tinyPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")

func TestInfoboxImage(t *testing.T) {
	v := openFixture(t)

	name, url, err := v.SaveImage("Аедрін Сірий", tinyPNG)
	if err != nil || name != "Аедрін Сірий.png" || !strings.HasSuffix(url, "/files/%D0%90%D0%B5%D0%B4%D1%80%D1%96%D0%BD%20%D0%A1%D1%96%D1%80%D0%B8%D0%B9/%D0%90%D0%B5%D0%B4%D1%80%D1%96%D0%BD%20%D0%A1%D1%96%D1%80%D0%B8%D0%B9.png") {
		t.Fatalf("SaveImage = %q, %q, %v", name, url, err)
	}
	// It lives in the article's folder; a second upload gets its own name.
	if !fileExists(filepath.Join(v.Root(), "Аедрін Сірий", name)) {
		t.Error("image not stored in the article's folder")
	}
	if second, _, _ := v.SaveImage("Аедрін Сірий", tinyPNG); second != "Аедрін Сірий-2.png" {
		t.Errorf("second upload = %q", second)
	}
	if _, _, err := v.SaveImage("Аедрін Сірий", []byte("<svg onload=alert(1)>")); !errors.As(err, new(*InvalidError)) {
		t.Errorf("svg accepted: %v", err)
	}

	src := v.Source("Аедрін Сірий")
	src.Infobox = append(src.Infobox, Field{ImageKey, "![[" + name + "]]"}, Field{CaptionKey, "Портрет із [[Вальмор]]а"})
	if _, err := v.SaveSource("Аедрін Сірий", src); err != nil {
		t.Fatal(err)
	}
	view, _ := v.Get("Аедрін Сірий")
	if !strings.Contains(view.Image, "<img src=") || !strings.Contains(view.Caption, `class="wikilink"`) {
		t.Errorf("image %q caption %q", view.Image, view.Caption)
	}
	for _, r := range view.Infobox {
		if r.Label == ImageKey || r.Label == CaptionKey {
			t.Errorf("%s shown as a plain row", r.Label)
		}
	}
	if got := v.Source("Аедрін Сірий").ImageURL; got != url {
		t.Errorf("source image url = %q", got)
	}

	// The image moves along with its article.
	if err := v.Move("Аедрін Сірий", "Головна"); err != nil {
		t.Fatal(err)
	}
	if view, _ := v.Get("Аедрін Сірий"); !strings.Contains(view.Image, "/files/%D0%93%D0%BE%D0%BB%D0%BE%D0%B2%D0%BD%D0%B0/") {
		t.Errorf("image not found after move: %s", view.Image)
	}
}

func TestObsidianSyntax(t *testing.T) {
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["pic.png"] = string(tinyPNG)
	files["Нотатки.md"] = `> [!warning] Не канон
> Ця частина ще [[Вальмор|обговорюється]].
> Друга лінія.

> [!question]
> Без заголовка.

> Звичайна цитата.

| Хто | Де |
| --- | --- |
| [[Аедрін Сірий\|Аедрін]] | [[Вальмор]] |

![[pic.png|left]] ![[pic.png|300]] ![[pic.png|Підпис]]
`
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}
	view, err := v.Get("Нотатки")
	if err != nil {
		t.Fatal(err)
	}
	h := view.HTML
	for _, want := range []string{
		`<blockquote class="callout callout-warning">`,
		`<p class="callout-title">Не канон</p>`,
		`<p class="callout-title">Питання</p>`,
		`<blockquote>`, // a plain quote stays a quote
		`class="align-left"`, `width="300"`, `alt="Підпис"`,
	} {
		if !strings.Contains(h, want) {
			t.Errorf("missing %s in:\n%s", want, h)
		}
	}
	if strings.Contains(h, "[!warning]") || strings.Contains(h, "[!question]") {
		t.Errorf("callout marker left in output:\n%s", h)
	}
	if !strings.Contains(h, ">обговорюється</a>") || !strings.Contains(h, "Друга лінія") {
		t.Errorf("callout body lost:\n%s", h)
	}
	// The escaped pipe in a table still makes a working link.
	if strings.Contains(h, `wikilink new`) {
		t.Errorf("escaped-pipe link is red:\n%s", h)
	}
	if a, _ := v.Get("Аедрін Сірий"); !slices.Contains(a.Backlinks, "Нотатки") {
		t.Errorf("escaped-pipe link not indexed: %v", a.Backlinks)
	}
}

func TestInfoboxSectionsAndBanner(t *testing.T) {
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["wide.png"] = string(tinyPNG)
	files["world.yaml"] = `
types:
  war:
    label: Війна
    fields:
      - { key: date, label: Дата, type: date }
      - { key: losses, label: Втрати, type: section }
      - { key: a, label: Сайнора }
      - { key: b, label: Федерація }
      - { key: family, label: Родина, type: section }
      - { key: c, label: Дружина }
`
	files["Війна.md"] = "---\ntype: war\nbanner: \"![[wide.png]]\"\nbanner_y: 0.3\ninfobox:\n  date: 142\n  a: 41 000\n  b: 52 000\n---\nТекст.\n"
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}
	view, _ := v.Get("Війна")
	var rows []string
	for _, r := range view.Infobox {
		if r.Section {
			rows = append(rows, "## "+r.Label)
		} else {
			rows = append(rows, r.Label)
		}
	}
	// "Родина" has no filled field under it, so its heading is left out.
	if !slices.Equal(rows, []string{"Дата", "## Втрати", "Сайнора", "Федерація"}) {
		t.Errorf("rows = %v", rows)
	}
	if !strings.HasSuffix(view.Banner, "/files/wide.png") || view.BannerY != 0.3 {
		t.Errorf("banner = %q at %v", view.Banner, view.BannerY)
	}
	// Editing through the form keeps the banner, which the form does not show.
	src := v.Source("Війна")
	src.Body = "Інший текст.\n"
	if _, err := v.SaveSource("Війна", src); err != nil {
		t.Fatal(err)
	}
	if view, _ := v.Get("Війна"); view.Banner == "" {
		t.Error("banner lost on save")
	}
}

func TestPeriods(t *testing.T) {
	v := openFixture(t)
	types := v.Types()
	for i := range types {
		if types[i].Key == "character" {
			// Pair by label, as the editor does for a field it just added.
			types[i].Fields[0].End = "Смерть"
		}
		if types[i].Key == "location" {
			types[i].Fields = append(types[i].Fields, FieldDef{Label: "Зруйновано", Type: "date"})
			types[i].Fields[0].End = "Зруйновано"
		}
	}
	saved, err := v.SaveTypes(types)
	if err != nil {
		t.Fatal(err)
	}
	for _, ti := range saved {
		if ti.Key == "character" && ti.Fields[0].End != "died" {
			t.Errorf("character pairing = %q", ti.Fields[0].End)
		}
		if ti.Key == "location" && ti.Fields[0].End != "зруйновано" {
			t.Errorf("location pairing = %q", ti.Fields[0].End)
		}
	}

	byTitle := map[string]TimelineEntry{}
	for _, y := range v.Timeline() {
		for _, e := range y.Entries {
			byTitle[e.Title+"/"+e.Field] = e
		}
	}
	// Born 312, died 389: one period, and the death still has its own dot.
	if e := byTitle["Аедрін Сірий/Народження"]; e.EndYear != 389 || e.EndField != "Смерть" || e.EndDate != "389 ЕП" || e.OpenEnd {
		t.Errorf("born → died = %+v", e)
	}
	if _, ok := byTitle["Аедрін Сірий/Смерть"]; !ok {
		t.Error("end of a period lost its own dot")
	}
	// Founded, never destroyed: the period stays open.
	if e := byTitle["Вальмор/Засновано"]; !e.OpenEnd || e.EndField != "Зруйновано" {
		t.Errorf("open period = %+v", e)
	}
	// A pairing to a non-date field is dropped.
	types = v.Types()
	for i := range types {
		if types[i].Key == "character" {
			types[i].Fields[0].End = "Приналежність"
		}
	}
	saved, _ = v.SaveTypes(types)
	for _, ti := range saved {
		if ti.Key == "character" && ti.Fields[0].End != "" {
			t.Errorf("pairing to a text field kept: %q", ti.Fields[0].End)
		}
	}
}

func TestCalendar(t *testing.T) {
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["world.yaml"] += "custom: kept\n"
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}
	if c := v.World().Calendar; c.YearParts != 0 || len(c.Epochs) != 0 {
		t.Fatalf("a world without a calendar has %+v", c)
	}
	w, err := v.SaveCalendar(Calendar{YearParts: 100, Epochs: []EpochDef{{Name: " Ранній час ", Years: 400}, {}, {Name: "Нова ера"}}}, " Епохи ")
	if err != nil {
		t.Fatal(err)
	}
	c := w.Calendar
	if c.YearParts != 100 || len(c.Epochs) != 3 || c.Epochs[0] != (EpochDef{"Ранній час", 400}) || c.Epochs[2].Name != "Нова ера" || w.Era != "Епохи" {
		t.Errorf("calendar after save = %+v, era %q", c, w.Era)
	}
	raw, _ := os.ReadFile(filepath.Join(v.Root(), "world.yaml"))
	if !strings.Contains(string(raw), "custom: kept") || !strings.Contains(string(raw), "type: date") {
		t.Errorf("other settings lost:\n%s", raw)
	}
	if _, err := v.SaveCalendar(Calendar{YearParts: -1}, ""); !errors.As(err, new(*InvalidError)) {
		t.Errorf("negative parts: %v", err)
	}
	// Clearing everything removes the section.
	if _, err := v.SaveCalendar(Calendar{}, ""); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(v.Root(), "world.yaml"))
	if strings.Contains(string(raw), "calendar") || strings.Contains(string(raw), "era:") {
		t.Errorf("empty calendar left in world.yaml:\n%s", raw)
	}
}
