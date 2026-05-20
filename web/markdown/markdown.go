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

// RenderTitle renders inline-only markdown to safe HTML. Placeholder.
func RenderTitle(md string) template.HTML {
	return template.HTML(template.HTMLEscapeString(md))
}
