package vault

import (
	"strings"
	"testing"
)

const anchorsWorld = `
name: Test
calendar:
  units:
    - { name: Епоха, write: roman }
    - { name: Рік }
    - { name: Частина }
  formats:
    - "{Рік}[.{Частина}][ р.] {Епоха} Епохи"
  step: Рік
`

func TestDateAnchors(t *testing.T) {
	v, err := Open(writeWorld(t, map[string]string{
		"world.yaml": anchorsWorld,
		"ТАТ.md": `Створення ТАТ було проголошене {{145.126 III Епохи | Проголошення <ТАТ>}}. Через кілька днів,
{{ 145.131  III Епохи|Перший уряд }}, був сформований уряд. Існувала з 145 по 160 р. III Епохи.

## Кінець {{161.231 III Епохи | Розпад}}

- {{142 р. III Епохи — 145 р. III Епохи | Перша війна}}
- {{колись | Невідомо коли}}
- Не якір: {{}} і {{ | без дати}} і {не} і ` + "`{{150 р. III Епохи | у коді}}`" + `.
`,
		"Інше.md": "Без якорів, 145 р. III Епохи.\n",
	}), "")
	if err != nil {
		t.Fatal(err)
	}
	view, err := v.Get("ТАТ")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range view.Chronology {
		got = append(got, e.Date+" = "+e.Note)
	}
	// In time order (a range by its start), unreadable dates last; only
	// what is written as an anchor counts.
	want := "142 р. III Епохи — 145 р. III Епохи = Перша війна | 145.126 III Епохи = Проголошення <ТАТ> | 145.131 III Епохи = Перший уряд | 161.231 III Епохи = Розпад | колись = Невідомо коли"
	if g := strings.Join(got, " | "); g != want {
		t.Errorf("chronology:\n got %s\nwant %s", g, want)
	}
	h := view.HTML
	for _, s := range []string{
		`<time class="date-anchor" id="date-1" title="Проголошення &lt;ТАТ&gt;">145.126 III Епохи</time>.`,
		`<time class="date-anchor" id="date-2" title="Перший уряд">145.131 III Епохи</time>, був`,
		`{{}}`, `{{ | без дати}}`, `{не}`, `<code>{{150 р. III Епохи | у коді}}</code>`,
	} {
		if !strings.Contains(h, s) {
			t.Errorf("missing %s in:\n%s", s, h)
		}
	}
	if len(view.Headings) != 1 || view.Headings[0].Text != "Кінець 161.231 III Епохи" {
		t.Errorf("heading = %+v", view.Headings)
	}
	// Without anchors there is no chronology, whatever dates the text has.
	if other, _ := v.Get("Інше"); other.Chronology != nil || strings.Contains(other.HTML, "<time") {
		t.Errorf("no anchors: %+v\n%s", other.Chronology, other.HTML)
	}
}
