package vault

import (
	"strings"
	"testing"
)

func TestChangesAgainstSaved(t *testing.T) {
	saved := `---
type: character
infobox:
  born: 312 ЕП
  affiliation: "[[Орден Попелу]]"
  Титул: Перший Магістр
---
**Аедрін** — маг.

## Ранні роки

Навчався в Академії.
Воював у війні.

- перший пункт
- другий пункт

Кінець.
`
	now := `---
type: character
infobox:
  born: 313 ЕП
  affiliation: "[[Орден Попелу]]"
  Прізвисько: Сірий Вовк
---
**Аедрін** — маг.

## Ранні роки

Навчався в Академії Тлінь.
Воював у війні.

- перший пункт
- новий пункт
- другий пункт

Новий абзац.
`
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["Аедрін Сірий.md"] = now
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}
	view, err := v.GetCompared("Аедрін Сірий", func(string) ([]byte, bool, error) { return []byte(saved), true, nil })
	if err != nil {
		t.Fatal(err)
	}
	h := view.HTML
	for _, want := range []string{
		`<p class="changed">Навчався в Академії Тлінь.`, // a changed line marks its paragraph
		`<li class="changed">новий пункт</li>`,
		`<p class="changed">Новий абзац.</p>`, // "Кінець." replaced: changed, not removed
		`<h2 id="ранні-роки">Ранні роки</h2>`,               // untouched blocks stay plain
		`<li>перший пункт</li>`,
	} {
		if !strings.Contains(h, want) {
			t.Errorf("missing %s in:\n%s", want, h)
		}
	}
	rows := map[string]string{}
	for _, r := range view.Infobox {
		rows[r.Label] = r.Changed
	}
	if rows["Народження"] != "modified" || rows["Приналежність"] != "" || rows["Прізвисько"] != "added" || rows["Титул"] != "removed" {
		t.Errorf("rows = %v", rows)
	}
	if view.Changes == nil || view.Changes.New || view.Changes.Fields != 3 || view.Changes.Blocks == 0 {
		t.Errorf("changes = %+v", view.Changes)
	}

	// Unchanged: no marks; never saved: new; plain Get: nothing.
	same, _ := v.GetCompared("Аедрін Сірий", func(string) ([]byte, bool, error) { return []byte(now), true, nil })
	fresh, _ := v.GetCompared("Аедрін Сірий", func(string) ([]byte, bool, error) { return nil, false, nil })
	plain, _ := v.Get("Аедрін Сірий")
	if same.Changes != nil || strings.Contains(same.HTML, "changed") {
		t.Errorf("unchanged article marked: %+v", same.Changes)
	}
	if fresh.Changes == nil || !fresh.Changes.New || strings.Contains(fresh.HTML, "changed") {
		t.Errorf("new article: %+v", fresh.Changes)
	}
	if plain.Changes != nil {
		t.Errorf("plain get: %+v", plain.Changes)
	}
}

func TestRemovedBlockMarked(t *testing.T) {
	files := map[string]string{}
	for k, v := range fixture {
		files[k] = v
	}
	files["Нотатки.md"] = "Перший.\n\nТретій.\n"
	v, err := Open(writeWorld(t, files), "")
	if err != nil {
		t.Fatal(err)
	}
	view, _ := v.GetCompared("Нотатки", func(string) ([]byte, bool, error) {
		return []byte("Перший.\n\nДругий.\n\nТретій.\n"), true, nil
	})
	if !strings.Contains(view.HTML, `<p class="removed-before">Третій.</p>`) || strings.Contains(view.HTML, `class="changed"`) {
		t.Errorf("removal:\n%s", view.HTML)
	}
	view, _ = v.GetCompared("Нотатки", func(string) ([]byte, bool, error) {
		return []byte("Перший.\n\nТретій.\n\nЧетвертий.\n"), true, nil
	})
	if !strings.Contains(view.HTML, `<p class="removed-after">Третій.</p>`) {
		t.Errorf("removal at the end:\n%s", view.HTML)
	}
}

func TestLineChanges(t *testing.T) {
	added, removed := lineChanges("a\nb\nc\nd", "a\nX\nc")
	if !added[1] || len(added) != 1 || !removed[3] || len(removed) != 1 {
		t.Errorf("added %v removed %v", added, removed)
	}
	added, removed = lineChanges("", "a\nb")
	if len(added) != 2 || len(removed) != 0 {
		t.Errorf("from nothing: added %v removed %v", added, removed)
	}
}
