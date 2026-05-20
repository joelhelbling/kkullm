// Package markdown converts markdown source to safe HTML for the web UI.
//
// The store, API, and CLI all deal in raw markdown text; only this package
// produces HTML, and only the web template layer calls it.
package markdown

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var bodyRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
		extension.TaskList,
	),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
	),
)

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
