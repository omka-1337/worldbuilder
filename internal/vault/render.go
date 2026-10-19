package vault

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// Heading is a table-of-contents entry.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id"`
}

// newMarkdown builds the shared converter. Raw HTML in articles is not
// rendered (goldmark's default), so a page cannot inject scripts.
func newMarkdown(v *Vault) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 199),
				util.Prioritized(anchorParser{}, 199),
			),
			parser.WithASTTransformers(
				util.Prioritized(calloutTransformer{}, 100),
				util.Prioritized(anchorTransformer{}, 110),
				util.Prioritized(changeTransformer{}, 130),
			),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(&linkRenderer{v: v}, 199),
				util.Prioritized(anchorRenderer{}, 199),
			),
		),
	)
}

// render converts an article body and collects its headings. Callers hold
// v.mu, since link resolution reads the index.
func (v *Vault) render(src []byte) (string, []Heading, error) {
	return v.renderWith(src, nil, nil)
}

// renderWith renders like render; with a collector it also gathers the
// date anchors of the text for the article's chronology.
func (v *Vault) renderWith(src []byte, col *anchorCollector, marks *changeMarks) (string, []Heading, error) {
	ctx := parser.NewContext(parser.WithIDs(&headingIDs{seen: map[string]bool{}}))
	if col != nil {
		ctx.Set(anchorsKey, col)
	}
	if marks != nil {
		ctx.Set(changesKey, marks)
	}
	doc := v.md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))

	headings := []Heading{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering {
			id, _ := h.AttributeString("id")
			idb, _ := id.([]byte)
			headings = append(headings, Heading{Level: h.Level, Text: plainText(src, h), ID: string(idb)})
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	var buf bytes.Buffer
	if err := v.md.Renderer().Render(&buf, src, doc); err != nil {
		return "", nil, err
	}
	return buf.String(), headings, nil
}

// renderInline renders a short Markdown value, such as an infobox field,
// without the surrounding paragraph.
func (v *Vault) renderInline(s string) string {
	html, _, err := v.render([]byte(s))
	if err != nil {
		return ""
	}
	html = strings.TrimSpace(html)
	if strings.HasPrefix(html, "<p>") && strings.HasSuffix(html, "</p>") && strings.Count(html, "<p>") == 1 {
		html = html[len("<p>") : len(html)-len("</p>")]
	}
	return html
}

// extractLinks returns the targets of every wikilink in the body and the
// infobox, and apart from them the images embedded there (and the banner),
// in the order they appear: image embeds are not links between articles.
func (v *Vault) extractLinks(p *Page) (links, images []string) {
	if p.Banner != "" {
		images = append(images, embedName(p.Banner))
	}
	collect := func(src []byte) {
		doc := v.md.Parser().Parse(text.NewReader(src))
		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if l, ok := n.(*wikilink.Node); ok && entering {
				switch target := wikiTarget(l); {
				case target == "":
				case isImage(target):
					if l.Embed {
						images = append(images, target)
					}
				default:
					links = append(links, target)
				}
			}
			return ast.WalkContinue, nil
		})
	}
	// The infobox pictures come first, as the article shows them.
	for _, img := range p.Images {
		collect([]byte(img.File))
		collect([]byte(img.Caption))
	}
	collect(p.Body)
	for _, f := range p.Infobox {
		collect([]byte(f.Value))
	}
	return links, images
}

// pageURL is the in-app address of an article.
func (v *Vault) pageURL(title string) string {
	return v.base + "/wiki/" + url.PathEscape(strings.ReplaceAll(strings.TrimSpace(title), " ", "_"))
}

// editURL opens the editor; red links go straight there, as on Wikipedia.
func (v *Vault) editURL(title string) string {
	return v.base + "/edit/" + url.PathEscape(strings.ReplaceAll(strings.TrimSpace(title), " ", "_"))
}

func (v *Vault) fileURL(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return v.base + "/files/" + strings.Join(parts, "/")
}

// wikiTarget is the page a wikilink points to. Inside a table, Obsidian
// writes [[Page\|label]] to keep the pipe from splitting the cell; the
// backslash is not part of the title.
func wikiTarget(n *wikilink.Node) string {
	return strings.TrimSpace(strings.TrimSuffix(string(n.Target), `\`))
}

func isImage(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".apng", ".avif", ".gif", ".jpg", ".jpeg", ".png", ".svg", ".webp":
		return true
	}
	return false
}

// linkRenderer renders wikilinks against the index: existing pages get
// class "wikilink", missing ones "wikilink new" (the red links), and image
// embeds become <img> pointing at the attachment.
type linkRenderer struct{ v *Vault }

func (r *linkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilink.Kind, r.render)
}

func (r *linkRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*wikilink.Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T", node)
	}
	target := wikiTarget(n)

	if n.Embed && isImage(target) {
		if !entering {
			return ast.WalkContinue, nil
		}
		rel, ok := r.v.attachment(target)
		if !ok {
			fmt.Fprintf(w, `<span class="missing-file">%s</span>`, util.EscapeHTML(n.Target))
			return ast.WalkSkipChildren, nil
		}
		// The part after | is alt text, or Obsidian-style display options:
		// a width (300), a size (300x200) or an alignment (left, right, center).
		alt, attrs := plainText(src, n), ""
		switch opt := strings.ToLower(strings.TrimSpace(alt)); {
		case alt == target || alt == string(n.Target):
			alt = ""
		case embedSize.MatchString(opt):
			attrs = fmt.Sprintf(` width="%s"`, embedSize.FindStringSubmatch(opt)[1])
			alt = ""
		case opt == "left" || opt == "right" || opt == "center":
			attrs = ` class="align-` + opt + `"`
			alt = ""
		}
		fmt.Fprintf(w, `<img src="%s" alt="%s"%s>`, util.EscapeHTML([]byte(r.v.fileURL(rel))), util.EscapeHTML([]byte(alt)), attrs)
		return ast.WalkSkipChildren, nil
	}

	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}

	var href, class string
	switch page, ok := r.v.lookup(target); {
	case target == "":
		href, class = "", "wikilink"
	case ok:
		href, class = r.v.pageURL(page.Title), "wikilink"
	default:
		href, class = r.v.editURL(target), "wikilink new"
	}
	if len(n.Fragment) > 0 {
		href += "#" + url.PathEscape(slug(string(n.Fragment)))
	}
	fmt.Fprintf(w, `<a class="%s" href="%s">`, class, util.EscapeHTML([]byte(href)))
	return ast.WalkContinue, nil
}

// headingIDs implements parser.IDs with slugs that keep Cyrillic and other
// non-Latin letters, so [[Page#Розділ]] can target them.
type headingIDs struct{ seen map[string]bool }

func (h *headingIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := slug(string(value))
	if base == "" {
		base = "розділ"
	}
	id := base
	for i := 1; h.seen[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	h.seen[id] = true
	return []byte(id)
}

func (h *headingIDs) Put(value []byte) { h.seen[string(value)] = true }

func plainText(src []byte, n ast.Node) string {
	var b strings.Builder
	writeText(&b, src, n)
	return b.String()
}

func writeText(w io.StringWriter, src []byte, n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			_, _ = w.WriteString(string(c.Segment.Value(src)))
			if c.SoftLineBreak() {
				_, _ = w.WriteString(" ")
			}
		case *ast.String:
			_, _ = w.WriteString(string(c.Value))
		case *dateAnchor:
			_, _ = w.WriteString(c.Date)
		default:
			writeText(w, src, c)
		}
	}
}
