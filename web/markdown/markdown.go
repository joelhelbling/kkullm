// Package markdown converts markdown source to safe HTML for the web UI.
//
// The store, API, and CLI all deal in raw markdown text; only this package
// produces HTML, and only the web template layer calls it.
package markdown

import (
	"bytes"
	"html/template"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type linkAttrTransformer struct{}

func (linkAttrTransformer) Transform(doc *gmast.Document, _ text.Reader, _ parser.Context) {
	_ = gmast.Walk(doc, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}
		switch link := n.(type) {
		case *gmast.Link:
			link.SetAttributeString("target", []byte("_blank"))
			link.SetAttributeString("rel", []byte("noopener noreferrer"))
		case *gmast.AutoLink:
			link.SetAttributeString("target", []byte("_blank"))
			link.SetAttributeString("rel", []byte("noopener noreferrer"))
		case *gmast.Image:
			dest := string(link.Destination)
			if !isAllowedImageScheme(dest) {
				parent := link.Parent()
				for c := link.FirstChild(); c != nil; {
					next := c.NextSibling()
					link.RemoveChild(link, c)
					parent.InsertBefore(parent, link, c)
					c = next
				}
				parent.RemoveChild(parent, link)
				return gmast.WalkContinue, nil
			}
		}
		return gmast.WalkContinue, nil
	})
}

var bodyRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
		extension.TaskList,
		highlighting.NewHighlighting(
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(
			util.Prioritized(linkAttrTransformer{}, 100),
		),
	),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
	),
)

func isAllowedImageScheme(dest string) bool {
	return strings.HasPrefix(dest, "https://") || strings.HasPrefix(dest, "http://")
}

// RenderBody renders full markdown to safe HTML. Used for card bodies and
// comment bodies.
func RenderBody(md string) template.HTML {
	var buf bytes.Buffer
	if err := bodyRenderer.Convert([]byte(md), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(md))
	}
	return template.HTML(buf.String())
}

// titleParser is a goldmark instance used only to parse — we never call its
// renderer. We walk the AST ourselves and emit inline-only HTML, flattening
// block constructs and dropping links/images.
var titleParser = goldmark.New(
	goldmark.WithExtensions(
		extension.Strikethrough,
		extension.Linkify, // so bare URLs still flatten cleanly
		extension.Table,   // so GFM tables parse as table AST nodes, not raw text
	),
)

// RenderTitle renders inline-only markdown to safe HTML. Block constructs
// flatten to their text content; links flatten to their visible text; images
// are dropped entirely; newlines collapse to single spaces. Used for card
// titles.
func RenderTitle(md string) template.HTML {
	doc := titleParser.Parser().Parse(text.NewReader([]byte(md)))
	var sb strings.Builder
	renderTitleNode(&sb, doc, []byte(md))
	// Collapse any residual whitespace runs (newlines, tabs) to single spaces.
	out := collapseWhitespace(sb.String())
	return template.HTML(out)
}

func renderTitleNode(sb *strings.Builder, n gmast.Node, src []byte) {
	switch node := n.(type) {
	case *gmast.Text:
		// Text segment from source.
		seg := node.Segment
		sb.WriteString(template.HTMLEscapeString(string(seg.Value(src))))
		// Soft/hard line breaks inside a Text node become spaces.
		if node.SoftLineBreak() || node.HardLineBreak() {
			sb.WriteByte(' ')
		}
		return
	case *gmast.String:
		sb.WriteString(template.HTMLEscapeString(string(node.Value)))
		return
	case *gmast.CodeSpan:
		sb.WriteString("<code>")
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			renderTitleNode(sb, c, src)
		}
		sb.WriteString("</code>")
		return
	case *gmast.Emphasis:
		tag := "em"
		if node.Level == 2 {
			tag = "strong"
		}
		sb.WriteString("<")
		sb.WriteString(tag)
		sb.WriteString(">")
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			renderTitleNode(sb, c, src)
		}
		sb.WriteString("</")
		sb.WriteString(tag)
		sb.WriteString(">")
		return
	case *gmast.Link:
		// Drop the <a>; render only inline children (the link text).
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			renderTitleNode(sb, c, src)
		}
		return
	case *gmast.AutoLink:
		// Render the URL text but without an <a> wrapper.
		sb.WriteString(template.HTMLEscapeString(string(node.URL(src))))
		return
	case *gmast.Image:
		// Dropped entirely — no <img>, no alt text rendered as content.
		return
	case *gmast.FencedCodeBlock:
		// Fenced code blocks store content in Lines(), not child nodes.
		for i := 0; i < node.Lines().Len(); i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			seg := node.Lines().At(i)
			sb.WriteString(template.HTMLEscapeString(string(seg.Value(src))))
		}
		return
	case *gmast.CodeBlock:
		// Indented code blocks also store content in Lines().
		for i := 0; i < node.Lines().Len(); i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			seg := node.Lines().At(i)
			sb.WriteString(template.HTMLEscapeString(string(seg.Value(src))))
		}
		return
	}

	// Table nodes from the GFM table extension.
	switch node := n.(type) {
	case *extast.Table:
		// Recurse into header and rows, spacing between children.
		first := true
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if !first {
				sb.WriteByte(' ')
			}
			first = false
			renderTitleNode(sb, c, src)
		}
		return
	case *extast.TableHeader:
		first := true
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if !first {
				sb.WriteByte(' ')
			}
			first = false
			renderTitleNode(sb, c, src)
		}
		return
	case *extast.TableRow:
		first := true
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if !first {
				sb.WriteByte(' ')
			}
			first = false
			renderTitleNode(sb, c, src)
		}
		return
	case *extast.TableCell:
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			renderTitleNode(sb, c, src)
		}
		sb.WriteByte(' ')
		return
	}

	// Strikethrough from the extension package.
	if isStrikethrough(n) {
		sb.WriteString("<del>")
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			renderTitleNode(sb, c, src)
		}
		sb.WriteString("</del>")
		return
	}

	// Default: recurse into children, dropping any block structure.
	// Insert a space between consecutive block-level children so list items
	// or paragraphs don't run together.
	first := true
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if !first && c.Type() == gmast.TypeBlock {
			sb.WriteByte(' ')
		}
		first = false
		renderTitleNode(sb, c, src)
	}
}

func isStrikethrough(n gmast.Node) bool {
	_, ok := n.(*extast.Strikethrough)
	return ok
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			r = ' '
		}
		if r == ' ' {
			if space {
				continue
			}
			space = true
		} else {
			space = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
