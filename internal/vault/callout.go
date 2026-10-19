package vault

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var (
	// "> [!warning] Title" opens an Obsidian callout; + or - marks it
	// foldable, which is ignored here.
	calloutStart = regexp.MustCompile(`^\[!([A-Za-z-]+)\][+-]?\s*(.*)$`)
	// Image embed options after the pipe: a width or a width×height.
	embedSize = regexp.MustCompile(`^(\d+)(?:x\d+)?$`)
)

// Titles for callouts that do not give their own.
var calloutTitles = map[string]string{
	"note":      "Примітка",
	"info":      "Інформація",
	"tip":       "Порада",
	"hint":      "Порада",
	"important": "Важливо",
	"warning":   "Увага",
	"caution":   "Увага",
	"danger":    "Небезпека",
	"error":     "Помилка",
	"question":  "Питання",
	"faq":       "Питання",
	"example":   "Приклад",
	"quote":     "Цитата",
	"author":    "Нотатка автора", // the author's aside in the text, hidden in reading mode
	"abstract":  "Коротко",
	"summary":   "Коротко",
	"todo":      "Зробити",
	"success":   "Готово",
	"bug":       "Помилка",
	"failure":   "Невдача",
}

// calloutTransformer turns Obsidian callouts — blockquotes starting with
// [!type] — into <blockquote class="callout callout-type"> with a title
// paragraph, so they render as styled boxes instead of quotes.
type calloutTransformer struct{}

func (calloutTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var quotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q, ok := n.(*ast.Blockquote); ok && entering {
			quotes = append(quotes, q)
		}
		return ast.WalkContinue, nil
	})
	for _, q := range quotes {
		if !convertCallout(q, src) {
			markSource(q, src)
		}
	}
}

// quoteSource matches the line that names a quote's source: it starts with a
// dash ("— from a speech in parliament").
var quoteSource = regexp.MustCompile(`^(—|–|―|--)\s*\S`)

// markSource gives a plain quote's last paragraph the class quote-source
// when it is such a line, so it shows as the quote's attribution.
func markSource(q *ast.Blockquote, src []byte) {
	para, ok := q.LastChild().(*ast.Paragraph)
	if !ok || para == q.FirstChild() || para.Lines().Len() == 0 {
		return
	}
	line := para.Lines().At(0)
	if quoteSource.Match(bytes.TrimSpace(line.Value(src))) {
		para.SetAttributeString("class", []byte("quote-source"))
	}
}

// convertCallout makes q a callout if it starts with a [!type] marker and
// reports whether it did.
func convertCallout(q *ast.Blockquote, src []byte) bool {
	para, ok := q.FirstChild().(*ast.Paragraph)
	if !ok || para.Lines().Len() == 0 {
		return false
	}
	first := para.Lines().At(0)
	m := calloutStart.FindStringSubmatch(strings.TrimSpace(string(first.Value(src))))
	if m == nil {
		return false
	}
	kind := strings.ToLower(m[1])
	title := strings.TrimSpace(m[2])
	if title == "" {
		title = calloutTitles[kind]
	}
	if title == "" {
		title = strings.ToUpper(kind[:1]) + kind[1:]
	}

	// Drop the inline nodes of the first line: the marker and the title.
	for c := para.FirstChild(); c != nil; {
		next := c.NextSibling()
		start, ok := inlineStart(c)
		if ok && start >= first.Stop {
			break
		}
		endsLine := false
		if t, isText := c.(*ast.Text); isText {
			endsLine = t.SoftLineBreak() || t.HardLineBreak()
		}
		para.RemoveChild(para, c)
		c = next
		if endsLine {
			break
		}
	}
	if para.ChildCount() == 0 {
		q.RemoveChild(q, para)
	}

	q.SetAttributeString("class", []byte("callout callout-"+kind))
	head := ast.NewParagraph()
	head.SetAttributeString("class", []byte("callout-title"))
	head.AppendChild(head, ast.NewString([]byte(title)))
	q.InsertBefore(q, q.FirstChild(), head)
	return true
}

// inlineStart is the source offset where an inline node begins, found from
// its first text descendant.
func inlineStart(n ast.Node) (int, bool) {
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start, true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if s, ok := inlineStart(c); ok {
			return s, true
		}
	}
	return 0, false
}
