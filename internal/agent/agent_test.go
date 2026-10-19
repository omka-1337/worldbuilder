package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The recorded output of `claude -p … --output-format stream-json` reading
// one file and answering, in /world.
func TestStreamEvents(t *testing.T) {
	f, err := os.Open("testdata/stream.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var got []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var e streamEvent
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			got = append(got, e.events("/world")...)
		}
	}
	var text strings.Builder
	var kinds []string
	for _, e := range got {
		if e.Type == "text" {
			text.WriteString(e.Text)
			continue
		}
		k := e.Type
		if e.Type == "tool" {
			k += ":" + e.Tool + ":" + e.Target
		}
		kinds = append(kinds, k)
	}
	if g := strings.Join(kinds, " "); g != "session tool:Read:Тест.md done" {
		t.Errorf("events = %s", g)
	}
	last := got[len(got)-1]
	if text.String() != last.Text || last.Session == "" || got[0].Session != last.Session {
		t.Errorf("streamed %q, done %+v", text.String(), last)
	}
}

func TestRules(t *testing.T) {
	r := Rules(World{
		Name:     "ANDROMEDA",
		MainPage: "Головна",
		Units:    []Unit{{Name: "Епоха", Write: "roman"}, {Name: "Рік"}, {Name: "Частина"}},
		Formats:  []string{"{Рік}[.{Частина}][ р.] {Епоха} Епохи"},
		Dates:    []string{"145.126 III Епохи", "145 р. III Епохи"},
		Types: []Type{{Key: "держава", Label: "Держава", Fields: []Field{
			{Key: "дата-утворення", Label: "Дата утворення", Kind: "date", End: "дата-розпаду"},
			{Key: "дата-розпаду", Label: "Дата розпаду", Kind: "date"},
			{Key: "економіка", Label: "Економіка", Kind: "section"},
		}}},
		Extra:     "Назви кораблів пиши в лапках «».",
		ExtraPath: ".worldbuilder/agent.md",
	})
	for _, want := range []string{
		`world "ANDROMEDA"`,
		"{{date | short description}}",
		"- Епоха — written in Roman numerals",
		"`{Рік}[.{Частина}][ р.] {Епоха} Епохи`",
		"`145.126 III Епохи`, `145 р. III Епохи`",
		"- `дата-утворення` — Дата утворення (date, period ends with `дата-розпаду`)",
		"  - section `економіка` — Економіка",
		"[[Головна]]",
		"## The author's rules for this world",
		"Назви кораблів пиши в лапках «».",
	} {
		if !strings.Contains(r, want) {
			t.Errorf("rules lack %q", want)
		}
	}
	if plain := Rules(World{Name: "X", Era: "ЕП"}); !strings.Contains(plain, "`343 ЕП`") || strings.Contains(plain, "author's rules") {
		t.Errorf("plain-year rules:\n%s", plain)
	}
}
