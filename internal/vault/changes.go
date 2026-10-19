package vault

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// An article read next to its last saved version shows what changed since:
// new and changed blocks of text marked, a mark where text was removed, and
// infobox fields marked as added, changed or removed. So whatever the
// writing assistant (or anyone) changed is seen at a glance.

// ViewChanges sums up how an article differs from its last saved version.
type ViewChanges struct {
	New    bool `json:"new,omitempty"` // no saved version has it yet
	Blocks int  `json:"blocks"`        // blocks of text marked changed or after a removal
	Fields int  `json:"fields"`        // infobox rows marked
}

// --- lines ---------------------------------------------------------------------

// lineChanges compares two texts by lines: which lines of after are new or
// changed, and before which lines of after (len = at the end) old lines
// were removed. A line replaced by another counts as changed only.
func lineChanges(before, after string) (added, removed map[int]bool) {
	added, removed = map[int]bool{}, map[int]bool{}
	var a []string
	if before != "" {
		a = strings.Split(before, "\n")
	}
	b := strings.Split(after, "\n")

	start := 0
	for start < len(a) && start < len(b) && a[start] == b[start] {
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	n, m := endA-start, endB-start
	if n*m > 4_000_000 { // too big to compare line by line: the middle counts as new
		for j := start; j < endB; j++ {
			added[j] = true
		}
		if n > 0 && m == 0 {
			removed[endB] = true
		}
		return added, removed
	}
	// Longest common subsequence of the middle, from the end.
	w := m + 1
	lcs := make([]int32, (n+1)*w)
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[start+i] == b[start+j] {
				lcs[i*w+j] = lcs[(i+1)*w+j+1] + 1
			} else {
				lcs[i*w+j] = max(lcs[(i+1)*w+j], lcs[i*w+j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[start+i] == b[start+j]:
			i++
			j++
		case j < m && (i == n || lcs[i*w+j+1] >= lcs[(i+1)*w+j]):
			added[start+j] = true
			j++
		default:
			removed[start+j] = true
			i++
		}
	}
	for at := range removed {
		if added[at] || added[at-1] {
			delete(removed, at)
		}
	}
	return added, removed
}

// --- marking blocks ---------------------------------------------------------------

var changesKey = parser.NewContextKey()

// changeMarks carries the changed lines of the text being rendered, and
// counts the blocks it marks.
type changeMarks struct {
	added, removed map[int]bool
	blocks         int
}

// changeTransformer marks the blocks whose source lines changed with the
// class "changed", and the block a removal happened just before with
// "removed-before" (or the last one with "removed-after").
type changeTransformer struct{}

func (changeTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	marks, _ := pc.Get(changesKey).(*changeMarks)
	if marks == nil || (len(marks.added) == 0 && len(marks.removed) == 0) {
		return
	}
	src := reader.Source()
	lineAt := func(offset int) int { return bytes.Count(src[:offset], []byte("\n")) }

	// The blocks a reader sees as one piece, with the lines they span.
	type block struct {
		node        ast.Node
		first, last int
	}
	var blocks []block
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Paragraph, *ast.Heading, *ast.ListItem, *extast.TableRow, *extast.TableHeader, *ast.FencedCodeBlock, *ast.CodeBlock:
			first, last, ok := spanOf(n, lineAt)
			if !ok {
				return ast.WalkContinue, nil
			}
			blocks = append(blocks, block{n, first, last})
			// A list item is marked whole unless its paragraphs are
			// separate (a loose list); other blocks are leaves.
			if li, isItem := n.(*ast.ListItem); isItem {
				if _, tight := li.FirstChild().(*ast.TextBlock); tight {
					return ast.WalkSkipChildren, nil
				}
				blocks = blocks[:len(blocks)-1]
				return ast.WalkContinue, nil
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	for _, b := range blocks {
		for l := b.first; l <= b.last; l++ {
			if marks.added[l] {
				addClass(b.node, "changed")
				marks.blocks++
				break
			}
		}
	}
	for at := range marks.removed {
		var target *block
		for k := range blocks {
			if blocks[k].last >= at {
				target = &blocks[k]
				break
			}
		}
		switch {
		case target != nil:
			addClass(target.node, "removed-before")
			marks.blocks++
		case len(blocks) > 0:
			addClass(blocks[len(blocks)-1].node, "removed-after")
			marks.blocks++
		}
	}
}

// spanOf is the first and last source line of a block, from its own lines
// or its children's.
func spanOf(n ast.Node, lineAt func(int) int) (int, int, bool) {
	first, last, ok := 0, 0, false
	var visit func(ast.Node)
	visit = func(n ast.Node) {
		if n.Type() == ast.TypeBlock {
			if lines := n.Lines(); lines.Len() > 0 {
				f := lineAt(lines.At(0).Start)
				l := lineAt(max(lines.At(lines.Len()-1).Stop-1, lines.At(lines.Len()-1).Start))
				if !ok || f < first {
					first = f
				}
				if !ok || l > last {
					last = l
				}
				ok = true
			}
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			visit(c)
		}
	}
	visit(n)
	return first, last, ok
}

// addClass adds a class to a node's class attribute.
func addClass(n ast.Node, class string) {
	if old, ok := n.AttributeString("class"); ok {
		if b, isBytes := old.([]byte); isBytes && len(b) > 0 {
			n.SetAttributeString("class", []byte(string(b)+" "+class))
			return
		}
	}
	n.SetAttributeString("class", []byte(class))
}

// --- the infobox ----------------------------------------------------------------

// fieldChanges compares infobox fields with their saved version, by section
// and key: "added", "modified", or, for saved fields gone now, "removed".
func fieldChanges(before, after []Field) (map[[2]string]string, []Field) {
	old := map[[2]string]string{}
	for _, f := range before {
		if f.Value != "" {
			old[[2]string{f.Section, f.Key}] = f.Value
		}
	}
	out := map[[2]string]string{}
	now := map[[2]string]bool{}
	for _, f := range after {
		k := [2]string{f.Section, f.Key}
		now[k] = f.Value != ""
		switch v, was := old[k]; {
		case f.Value == "":
		case !was:
			out[k] = "added"
		case v != f.Value:
			out[k] = "modified"
		}
	}
	var gone []Field
	for _, f := range before {
		if k := [2]string{f.Section, f.Key}; f.Value != "" && !now[k] {
			gone = append(gone, f)
		}
	}
	return out, gone
}
