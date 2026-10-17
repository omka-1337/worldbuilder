package vault

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func openExample(t *testing.T) *Vault {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../examples/Попелясті Землі")); err != nil {
		t.Fatal(err)
	}
	v, err := Open(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRedirectAndInfobox(t *testing.T) {
	v := openExample(t)
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
	var labels []string
	for _, r := range view.Infobox {
		labels = append(labels, r.Label)
	}
	want := []string{"Народження", "Смерть", "Раса", "Приналежність", "Рід занять", "Титул"}
	if !slices.Equal(labels, want) {
		t.Errorf("infobox labels = %v, want %v", labels, want)
	}
	if !strings.Contains(view.Infobox[0].HTML, `href="/wiki/%D0%92%D0%B0%D0%BB%D1%8C%D0%BC%D0%BE%D1%80"`) {
		t.Errorf("infobox link not rendered: %s", view.Infobox[0].HTML)
	}
}

func TestLinksAndHeadings(t *testing.T) {
	v := openExample(t)
	view, err := v.Get("Аедрін Сірий")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.HTML, `class="wikilink new"`) {
		t.Error("missing page should render as a red link")
	}
	if !strings.Contains(view.HTML, `href="#спадщина"`) && !strings.Contains(view.HTML, `href="#%D1%81`) {
		t.Errorf("same-page fragment link not rendered: %s", view.HTML)
	}
	ids := []string{}
	for _, h := range view.Headings {
		ids = append(ids, h.ID)
	}
	if !slices.Equal(ids, []string{"ранні-роки", "заснування-ордену", "спадщина"}) {
		t.Errorf("heading ids = %v", ids)
	}
	// Головна links both [[Аедрін Сірий]] and the alias [[Сірий Вовк]]: one backlink.
	if !slices.Equal(view.Backlinks, []string{"Вальмор", "Головна", "Орден Попелу"}) {
		t.Errorf("backlinks = %v", view.Backlinks)
	}
}

func TestWantedAndCategories(t *testing.T) {
	v := openExample(t)
	wanted := v.Wanted()
	if len(wanted) == 0 || wanted[0].Title != "Війна Попелу" {
		t.Fatalf("most wanted = %+v", wanted)
	}
	members := v.CategoryMembers("маги")
	if len(members) != 1 || members[0].Title != "Аедрін Сірий" {
		t.Errorf("category members = %+v", members)
	}
}

func TestSaveCreatesAndRejectsBadYAML(t *testing.T) {
	v := openExample(t)
	title, err := v.Save("Війна Попелу", []byte("---\ntype: event\n---\nВійна між [[Вальмор]]ом і пустками.\n"))
	if err != nil || title != "Війна Попелу" {
		t.Fatalf("save = %q, %v", title, err)
	}
	if _, err := os.Stat(filepath.Join(v.Root(), "Війна Попелу.md")); err != nil {
		t.Fatal(err)
	}
	for _, w := range v.Wanted() {
		if w.Title == "Війна Попелу" {
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
	v := openExample(t)
	hits := v.Search("вальмор", 10)
	if len(hits) == 0 || hits[0].Title != "Вальмор" {
		t.Fatalf("hits = %+v", hits)
	}
	for _, h := range hits[1:] {
		if strings.Contains(h.Snippet, "[[") {
			t.Errorf("snippet keeps link brackets: %q", h.Snippet)
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
