package vault

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Date anchors put an article's own moments on its chronology. The author
// writes them in the text,
//
//	Створення ТАТ було проголошене {{145.126 III Епохи | Проголошення ТАТ}}.
//
// The article shows the date where it stands; the description goes to the
// chronology, which lists the anchors in time order. Any article with an
// anchor has one.

var (
	kindDateAnchor = ast.NewNodeKind("DateAnchor")
	anchorsKey     = parser.NewContextKey()
)

// dateAnchor is {{date | description}} in the text.
type dateAnchor struct {
	ast.BaseInline
	Date, Note string
	ID         string // the anchor in the rendered page: date-1, date-2 …
}

func (n *dateAnchor) Kind() ast.NodeKind { return kindDateAnchor }

func (n *dateAnchor) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Date": n.Date, "Note": n.Note}, nil)
}

// anchorParser reads {{date | description}} within one line. Without a
// date it is not an anchor and stays text.
type anchorParser struct{}

func (anchorParser) Trigger() []byte { return []byte{'{'} }

func (anchorParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("{{")) {
		return nil
	}
	end := bytes.Index(line[2:], []byte("}}"))
	if end < 0 {
		return nil
	}
	date, note, _ := strings.Cut(string(line[2:2+end]), "|")
	date = strings.Join(strings.Fields(date), " ")
	if date == "" {
		return nil
	}
	block.Advance(2 + end + 2)
	return &dateAnchor{Date: date, Note: strings.Join(strings.Fields(note), " ")}
}

// ChronoEntry is one anchor of an article's chronology.
type ChronoEntry struct {
	ID   string `json:"id"`   // where it stands in the rendered text
	Date string `json:"date"` // as written
	Note string `json:"note"` // the author's description
	// unread is a date the world's calendar cannot place; it is listed
	// last, in the order of the text.
	unread bool

	r     dateRange
	order int
}

// anchorCollector gathers an article's anchors while it renders.
type anchorCollector struct {
	cal     *calendar
	entries []ChronoEntry
}

// anchorTransformer numbers the anchors in reading order and, when the
// article renders for its page, collects them.
type anchorTransformer struct{}

func (anchorTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	col, _ := pc.Get(anchorsKey).(*anchorCollector)
	n := 0
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		a, ok := node.(*dateAnchor)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		n++
		a.ID = fmt.Sprintf("date-%d", n)
		if col != nil {
			e := ChronoEntry{ID: a.ID, Date: a.Date, Note: a.Note, order: n}
			r, ok := col.cal.parse(a.Date)
			e.r, e.unread = r, !ok || r.start.count() == 0
			col.entries = append(col.entries, e)
		}
		return ast.WalkContinue, nil
	})
}

// sorted returns the entries in time order, equal dates in reading order
// and unreadable ones last.
func (col *anchorCollector) sorted() []ChronoEntry {
	out := append([]ChronoEntry{}, col.entries...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.unread != b.unread {
			return b.unread
		}
		if !a.unread {
			if a.r.start.before(b.r.start) {
				return true
			}
			if b.r.start.before(a.r.start) {
				return false
			}
		}
		return a.order < b.order
	})
	return out
}

// anchorRenderer shows the date where the anchor stands; the description
// is its tooltip.
type anchorRenderer struct{}

func (anchorRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindDateAnchor, func(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		a := node.(*dateAnchor)
		fmt.Fprintf(w, `<time class="date-anchor" id="%s"`, a.ID)
		if a.Note != "" {
			fmt.Fprintf(w, ` title="%s"`, util.EscapeHTML([]byte(a.Note)))
		}
		fmt.Fprintf(w, `>%s</time>`, util.EscapeHTML([]byte(a.Date)))
		return ast.WalkSkipChildren, nil
	})
}
