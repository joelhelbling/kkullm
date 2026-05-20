# Markdown Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render markdown for card titles (inline-only), card bodies, and comments in the web UI while keeping the API and CLI markdown-native.

**Architecture:** A new `web/markdown` package owns two goldmark renderers (`RenderBody`, `RenderTitle`). The web template layer calls them via template funcs. Store, API, and CLI are unchanged — markdown is the source of truth and stays as text everywhere except where the web layer emits HTML.

**Tech Stack:** Go 1.25, `github.com/yuin/goldmark` (+ `goldmark-highlighting/v2` for chroma), `html/template`, existing SSR + HTMX web UI, bash dev-seed script.

**Reference spec:** `docs/superpowers/specs/2026-05-19-markdown-rendering-design.md`

---

## File Structure

**New files:**
- `web/markdown/markdown.go` — `RenderBody`, `RenderTitle`, package-init of goldmark instances, link/image transformers.
- `web/markdown/markdown_test.go` — unit tests for both renderers.
- `web/static/css/markdown.css` — chroma + scoped `.kk-md` styles.

**Modified files:**
- `go.mod`, `go.sum` — add goldmark + chroma highlighter.
- `web/templates.go` — register `renderBody` and `renderTitle` template funcs; add monospace styling note (CSS only, no Go change needed for fonts).
- `web/templates/card.html` — title rendering.
- `web/templates/blockers.html` — title rendering.
- `web/templates/drawer.html` — title (top + delete-confirm), body, and per-comment body rendering.
- `web/templates/layout.html` — link the new `markdown.css`.
- `web/static/css/app.css` — apply monospace font to body/comment textareas; minor `.kk-md` container resets if needed.
- `scripts/dev-seed.sh` — add showcase card, showcase comments, and a few lightly-decorated cards/titles.

**Unchanged:** `model/`, `store/`, `db/`, `api/`, `cmd/`. No schema, no API surface change, no CLI change.

---

## Task 1: Add goldmark dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add goldmark and the chroma highlighter**

Run:
```bash
go get github.com/yuin/goldmark@latest
go get github.com/yuin/goldmark-highlighting/v2@latest
go mod tidy
```

Expected: `go.mod` lists `github.com/yuin/goldmark` and `github.com/yuin/goldmark-highlighting/v2` as direct deps; `go.sum` updated.

- [ ] **Step 2: Verify build still works**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add goldmark + chroma highlighter for markdown rendering"
```

---

## Task 2: Skeleton `web/markdown` package with failing test

**Files:**
- Create: `web/markdown/markdown.go`
- Create: `web/markdown/markdown_test.go`

- [ ] **Step 1: Write the first failing test**

Create `web/markdown/markdown_test.go`:

```go
package markdown

import (
	"strings"
	"testing"
)

func TestRenderBody_EmphasisAndStrong(t *testing.T) {
	got := string(RenderBody("This is **bold** and _italic_."))
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("expected <strong>bold</strong> in output, got: %s", got)
	}
	if !strings.Contains(got, "<em>italic</em>") {
		t.Errorf("expected <em>italic</em> in output, got: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/markdown/...`
Expected: FAIL — `markdown.go` doesn't exist; package won't build.

- [ ] **Step 3: Create the minimal implementation**

Create `web/markdown/markdown.go`:

```go
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
		// Goldmark.Convert only returns errors from underlying writers;
		// bytes.Buffer never errors. Fall back to escaped source.
		return template.HTML(template.HTMLEscapeString(md))
	}
	return template.HTML(buf.String())
}

// RenderTitle renders inline-only markdown to safe HTML. Placeholder.
func RenderTitle(md string) template.HTML {
	return template.HTML(template.HTMLEscapeString(md))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./web/markdown/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/markdown/
git commit -m "feat(web/markdown): add package skeleton with RenderBody"
```

---

## Task 3: Body — GFM extensions

**Files:**
- Modify: `web/markdown/markdown_test.go`
- (No code change needed; extensions already registered in Task 2 — these tests pin behavior.)

- [ ] **Step 1: Add failing tests for tables, task lists, strikethrough, autolinks**

Append to `web/markdown/markdown_test.go`:

```go
func TestRenderBody_Table(t *testing.T) {
	src := "| a | b |\n|---|---|\n| 1 | 2 |\n"
	got := string(RenderBody(src))
	if !strings.Contains(got, "<table>") || !strings.Contains(got, "<td>1</td>") {
		t.Errorf("expected GFM table, got: %s", got)
	}
}

func TestRenderBody_TaskList(t *testing.T) {
	src := "- [x] done\n- [ ] pending\n"
	got := string(RenderBody(src))
	if !strings.Contains(got, `type="checkbox"`) {
		t.Errorf("expected task list checkboxes, got: %s", got)
	}
	if !strings.Contains(got, "checked") {
		t.Errorf("expected 'checked' attribute on done item, got: %s", got)
	}
}

func TestRenderBody_Strikethrough(t *testing.T) {
	got := string(RenderBody("~~gone~~"))
	if !strings.Contains(got, "<del>gone</del>") {
		t.Errorf("expected <del>gone</del>, got: %s", got)
	}
}

func TestRenderBody_Autolink(t *testing.T) {
	got := string(RenderBody("see https://example.com for details"))
	if !strings.Contains(got, `href="https://example.com"`) {
		t.Errorf("expected autolink, got: %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./web/markdown/...`
Expected: PASS (extensions were registered in Task 2).

- [ ] **Step 3: Commit**

```bash
git add web/markdown/markdown_test.go
git commit -m "test(web/markdown): pin GFM extension behavior"
```

---

## Task 4: Body — strip raw HTML

**Files:**
- Modify: `web/markdown/markdown.go`
- Modify: `web/markdown/markdown_test.go`

- [ ] **Step 1: Write failing tests for raw HTML stripping**

Append to `web/markdown/markdown_test.go`:

```go
func TestRenderBody_StripsScriptTag(t *testing.T) {
	got := string(RenderBody("<script>alert(1)</script>"))
	if strings.Contains(got, "<script>") {
		t.Errorf("expected <script> to be escaped or dropped, got: %s", got)
	}
}

func TestRenderBody_EscapesInlineHTML(t *testing.T) {
	got := string(RenderBody("a <b>bold?</b> word"))
	// Raw <b> must NOT render as a real tag.
	if strings.Contains(got, "<b>bold?</b>") {
		t.Errorf("expected raw <b> to be escaped, got: %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/markdown/...`
Expected: FAIL — goldmark passes raw HTML through by default in block contexts (and the inline `<b>` test will fail because goldmark renders inline raw HTML by default too).

- [ ] **Step 3: Disable raw HTML in the body renderer**

Replace the `bodyRenderer` definition in `web/markdown/markdown.go` with:

```go
var bodyRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
		extension.TaskList,
	),
	goldmark.WithParserOptions(),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
		// Do NOT set html.WithUnsafe(); default behavior escapes raw HTML.
	),
)
```

The body uses goldmark's default safe renderer which already escapes raw HTML. If tests still fail, also disable the inline raw-HTML parser explicitly:

```go
import (
	// ...
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

var bodyRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
		extension.TaskList,
	),
	goldmark.WithParserOptions(
		parser.WithInlineParsers(
			// rebuild without the RawHTML parser
			util.Prioritized(parser.NewCodeSpanParser(), 100),
			util.Prioritized(parser.NewLinkParser(), 200),
			util.Prioritized(parser.NewAutoLinkParser(), 300),
			util.Prioritized(parser.NewEmphasisParser(), 400),
		),
	),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
	),
)
```

Try the simpler form first — goldmark's default html renderer escapes raw HTML when `WithUnsafe` is not set. If both tests pass without the second form, keep the simpler config.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./web/markdown/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/markdown/
git commit -m "feat(web/markdown): strip raw HTML from body rendering"
```

---

## Task 5: Body — link rel/target transformer

**Files:**
- Modify: `web/markdown/markdown.go`
- Modify: `web/markdown/markdown_test.go`

- [ ] **Step 1: Write failing tests for link attributes**

Append to `web/markdown/markdown_test.go`:

```go
func TestRenderBody_LinkHasTargetBlank(t *testing.T) {
	got := string(RenderBody("[docs](https://example.com)"))
	if !strings.Contains(got, `target="_blank"`) {
		t.Errorf("expected target=\"_blank\" on link, got: %s", got)
	}
	if !strings.Contains(got, `rel="noopener noreferrer"`) {
		t.Errorf("expected rel=\"noopener noreferrer\" on link, got: %s", got)
	}
}

func TestRenderBody_AutolinkHasTargetBlank(t *testing.T) {
	got := string(RenderBody("see https://example.com"))
	if !strings.Contains(got, `target="_blank"`) {
		t.Errorf("expected target=\"_blank\" on autolink, got: %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/markdown/...`
Expected: FAIL — goldmark emits plain `<a href="...">` without target/rel.

- [ ] **Step 3: Add an AST transformer that sets link attributes**

In `web/markdown/markdown.go`, add a transformer and register it:

```go
import (
	// ... existing imports
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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
		}
		return gmast.WalkContinue, nil
	})
}
```

Register it on `bodyRenderer` by adding to `goldmark.New(...)`:

```go
goldmark.WithParserOptions(
	parser.WithASTTransformers(
		util.Prioritized(linkAttrTransformer{}, 100),
	),
),
```

(Merge this with any existing `WithParserOptions` call rather than duplicating.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./web/markdown/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/markdown/
git commit -m "feat(web/markdown): force target=_blank and rel=noopener on links"
```

---

## Task 6: Body — image src scheme filter

**Files:**
- Modify: `web/markdown/markdown.go`
- Modify: `web/markdown/markdown_test.go`

- [ ] **Step 1: Write failing tests for image filtering**

Append to `web/markdown/markdown_test.go`:

```go
func TestRenderBody_ImageHTTPSAllowed(t *testing.T) {
	got := string(RenderBody("![alt](https://example.com/x.png)"))
	if !strings.Contains(got, `<img`) || !strings.Contains(got, `src="https://example.com/x.png"`) {
		t.Errorf("expected https <img>, got: %s", got)
	}
}

func TestRenderBody_ImageHTTPAllowed(t *testing.T) {
	got := string(RenderBody("![alt](http://example.com/x.png)"))
	if !strings.Contains(got, `<img`) {
		t.Errorf("expected http <img>, got: %s", got)
	}
}

func TestRenderBody_ImageDataURIDropped(t *testing.T) {
	got := string(RenderBody("![alt](data:image/png;base64,AAAA)"))
	if strings.Contains(got, "<img") {
		t.Errorf("expected data: <img> to be dropped, got: %s", got)
	}
}

func TestRenderBody_ImageJavascriptURIDropped(t *testing.T) {
	got := string(RenderBody("![alt](javascript:alert(1))"))
	if strings.Contains(got, "<img") {
		t.Errorf("expected javascript: <img> to be dropped, got: %s", got)
	}
}

func TestRenderBody_ImageRelativeDropped(t *testing.T) {
	got := string(RenderBody("![alt](/local/x.png)"))
	if strings.Contains(got, "<img") {
		t.Errorf("expected relative <img> to be dropped, got: %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/markdown/...`
Expected: FAIL — goldmark emits `<img>` for any URI.

- [ ] **Step 3: Extend the AST transformer to filter images**

In `web/markdown/markdown.go`, add image handling inside the existing `linkAttrTransformer.Transform` walk:

```go
case *gmast.Image:
	dest := string(link.Destination)
	if !isAllowedImageScheme(dest) {
		// Replace the image with its alt text by removing the image node
		// and keeping its children (alt-text inlines) in place.
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
```

Add the helper at package level:

```go
func isAllowedImageScheme(dest string) bool {
	return strings.HasPrefix(dest, "https://") || strings.HasPrefix(dest, "http://")
}
```

Add `"strings"` to imports if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./web/markdown/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/markdown/
git commit -m "feat(web/markdown): drop <img> with disallowed src schemes"
```

---

## Task 7: Body — chroma syntax highlighting

**Files:**
- Modify: `web/markdown/markdown.go`
- Modify: `web/markdown/markdown_test.go`

- [ ] **Step 1: Write failing test for highlighted code block**

Append to `web/markdown/markdown_test.go`:

```go
func TestRenderBody_FencedCodeHighlighted(t *testing.T) {
	src := "```go\nfunc main() {}\n```\n"
	got := string(RenderBody(src))
	if !strings.Contains(got, "<pre") || !strings.Contains(got, "func") {
		t.Errorf("expected highlighted code block, got: %s", got)
	}
	// Chroma with classes-only output emits class attributes on spans.
	if !strings.Contains(got, "class=") {
		t.Errorf("expected chroma class attributes, got: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/markdown/...`
Expected: FAIL — without the highlighting extension, code blocks render as plain `<pre><code>`.

- [ ] **Step 3: Register the highlighting extension**

In `web/markdown/markdown.go`, add the import:

```go
import (
	// ... existing imports
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
)
```

Add the extension to `bodyRenderer`'s `WithExtensions(...)` list:

```go
highlighting.NewHighlighting(
	highlighting.WithFormatOptions(
		chromahtml.WithClasses(true),
	),
),
```

Run `go mod tidy` to pick up the chroma direct dep:

```bash
go mod tidy
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./web/markdown/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/markdown/ go.mod go.sum
git commit -m "feat(web/markdown): syntax-highlight fenced code via chroma (classes only)"
```

---

## Task 8: Title renderer

**Files:**
- Modify: `web/markdown/markdown.go`
- Modify: `web/markdown/markdown_test.go`

- [ ] **Step 1: Write failing tests for `RenderTitle`**

Append to `web/markdown/markdown_test.go`:

```go
func TestRenderTitle_InlineEmphasis(t *testing.T) {
	got := string(RenderTitle("hello **world**"))
	if !strings.Contains(got, "<strong>world</strong>") {
		t.Errorf("expected <strong>world</strong>, got: %s", got)
	}
}

func TestRenderTitle_InlineCode(t *testing.T) {
	got := string(RenderTitle("call `foo()` here"))
	if !strings.Contains(got, "<code>foo()</code>") {
		t.Errorf("expected <code>foo()</code>, got: %s", got)
	}
}

func TestRenderTitle_Strikethrough(t *testing.T) {
	got := string(RenderTitle("~~gone~~ now"))
	if !strings.Contains(got, "<del>gone</del>") {
		t.Errorf("expected <del>gone</del>, got: %s", got)
	}
}

func TestRenderTitle_HeadingFlattens(t *testing.T) {
	got := string(RenderTitle("# Just a Title"))
	if strings.Contains(got, "<h1>") || strings.Contains(got, "<h") {
		t.Errorf("expected no heading tags, got: %s", got)
	}
	if !strings.Contains(got, "Just a Title") {
		t.Errorf("expected heading text preserved, got: %s", got)
	}
}

func TestRenderTitle_ListFlattens(t *testing.T) {
	got := string(RenderTitle("- one\n- two"))
	if strings.Contains(got, "<ul>") || strings.Contains(got, "<li>") {
		t.Errorf("expected no list tags, got: %s", got)
	}
}

func TestRenderTitle_CodeBlockFlattens(t *testing.T) {
	got := string(RenderTitle("```\ncode\n```"))
	if strings.Contains(got, "<pre>") || strings.Contains(got, "<code") && strings.Contains(got, "<pre") {
		t.Errorf("expected no <pre>, got: %s", got)
	}
}

func TestRenderTitle_LinkBecomesText(t *testing.T) {
	got := string(RenderTitle("[label](https://example.com)"))
	if strings.Contains(got, "<a ") {
		t.Errorf("expected no <a> in title, got: %s", got)
	}
	if !strings.Contains(got, "label") {
		t.Errorf("expected link label preserved, got: %s", got)
	}
}

func TestRenderTitle_ImageDropped(t *testing.T) {
	got := string(RenderTitle("![alt](https://example.com/x.png)"))
	if strings.Contains(got, "<img") {
		t.Errorf("expected no <img> in title, got: %s", got)
	}
}

func TestRenderTitle_NewlinesCollapseToSpaces(t *testing.T) {
	got := string(RenderTitle("line1\nline2"))
	if strings.Contains(got, "\n") || strings.Contains(got, "<br") {
		t.Errorf("expected newlines collapsed, got: %q", got)
	}
	if !strings.Contains(got, "line1 line2") {
		t.Errorf("expected single-space join, got: %q", got)
	}
}

func TestRenderTitle_EscapesRawHTML(t *testing.T) {
	got := string(RenderTitle("safe <script>x</script> title"))
	if strings.Contains(got, "<script>") {
		t.Errorf("expected <script> escaped, got: %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./web/markdown/...`
Expected: FAIL — `RenderTitle` currently just escapes input.

- [ ] **Step 3: Implement `RenderTitle` via AST walking**

In `web/markdown/markdown.go`, replace the placeholder `RenderTitle` with the implementation below. Add necessary imports (`strings`, `gmast`, `text`).

```go
// titleParser is a goldmark instance used only to parse — we never call its
// renderer. We walk the AST ourselves and emit inline-only HTML, flattening
// block constructs and dropping links/images.
var titleParser = goldmark.New(
	goldmark.WithExtensions(
		extension.Strikethrough,
		extension.Linkify, // so bare URLs still flatten cleanly
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
		// (Alt children are skipped intentionally to keep titles tidy.)
		return
	}

	// Strikethrough from the extension package — use type name match via
	// interface check since it's not in core ast.
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
```

Add the strikethrough detector:

```go
import (
	// ... existing imports
	extast "github.com/yuin/goldmark/extension/ast"
)

func isStrikethrough(n gmast.Node) bool {
	_, ok := n.(*extast.Strikethrough)
	return ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./web/markdown/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/markdown/
git commit -m "feat(web/markdown): inline-only RenderTitle (flatten blocks, drop links/images)"
```

---

## Task 9: Concurrency smoke test

**Files:**
- Modify: `web/markdown/markdown_test.go`

- [ ] **Step 1: Add the race-smoke test**

Append to `web/markdown/markdown_test.go`:

```go
func TestRender_ConcurrentSafe(t *testing.T) {
	src := "# h\n\n**b** and `c` and [l](https://example.com)\n\n- [x] done\n"
	const n = 32
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			_ = RenderBody(src)
			_ = RenderTitle(src)
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
}
```

- [ ] **Step 2: Run with -race**

Run: `go test -race ./web/markdown/...`
Expected: PASS, no race reported.

- [ ] **Step 3: Commit**

```bash
git add web/markdown/markdown_test.go
git commit -m "test(web/markdown): concurrency smoke test"
```

---

## Task 10: Wire template funcs

**Files:**
- Modify: `web/templates.go`

- [ ] **Step 1: Add the funcs to `funcMap`**

In `web/templates.go`, add to the imports:

```go
"github.com/joelhelbling/kkullm/web/markdown"
```

Update `funcMap`:

```go
var funcMap = template.FuncMap{
	"projectColor": projectColor,
	"tagBg":        tagBg,
	"tagColor":     tagColor,
	"joinStrings":  joinStrings,
	"timeAgo":      timeAgo,
	"renderBody":   markdown.RenderBody,
	"renderTitle":  markdown.RenderTitle,
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 3: Run existing web tests to confirm nothing broke**

Run: `go test ./web/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/templates.go
git commit -m "feat(web): register renderBody and renderTitle template funcs"
```

---

## Task 11: Use renderers in templates

**Files:**
- Modify: `web/templates/card.html`
- Modify: `web/templates/blockers.html`
- Modify: `web/templates/drawer.html`

- [ ] **Step 1: Update title rendering in `card.html`**

Replace:

```html
<div class="card-tile-title">{{.Title}}</div>
```

with:

```html
<div class="card-tile-title kk-md kk-md-title">{{renderTitle .Title}}</div>
```

- [ ] **Step 2: Update title rendering in `blockers.html`**

Replace:

```html
<div class="card-tile-title">{{.Title}}</div>
```

with:

```html
<div class="card-tile-title kk-md kk-md-title">{{renderTitle .Title}}</div>
```

- [ ] **Step 3: Update `drawer.html` — title, body, comments**

Replace the drawer title:

```html
<div class="drawer-title">{{.Card.Title}}</div>
```

with:

```html
<div class="drawer-title kk-md kk-md-title">{{renderTitle .Card.Title}}</div>
```

Replace the drawer body:

```html
<div class="drawer-body">{{.Card.Body}}</div>
```

with:

```html
<div class="drawer-body kk-md">{{renderBody .Card.Body}}</div>
```

Replace the comment body:

```html
<div class="comment-body">{{.Body}}</div>
```

with:

```html
<div class="comment-body kk-md">{{renderBody .Body}}</div>
```

Leave the delete-confirm `onclick="return confirm('Delete card &quot;{{.Card.Title}}&quot;? ...')"` as-is — it's a JS string, not user-facing HTML, and rendering markdown there would be wrong.

- [ ] **Step 4: Build and run existing tests**

Run:
```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/templates/
git commit -m "feat(web): render markdown in card titles, bodies, and comments"
```

---

## Task 12: Handler/template smoke test

**Files:**
- Modify: `web/handlers_test.go` (or `web/web_test.go` — pick whichever already exercises card rendering)

- [ ] **Step 1: Inspect existing tests to find the right place**

Run: `grep -n 'drawer\|card.html\|renderBody\|Render' web/*_test.go`

Use the existing test that renders a card view to model the new test. If no test renders a card with a body, add one in `web/handlers_test.go` following existing patterns (test server + HTTP request against an endpoint that returns the drawer for a card with a markdown body).

- [ ] **Step 2: Add a test that the drawer HTML contains rendered markdown**

Add to `web/handlers_test.go`:

```go
func TestDrawer_RendersBodyMarkdown(t *testing.T) {
	// Use the existing test-server / fixture helpers in this file.
	// Create a card whose body is "Hello **world** with `code`."
	// GET the drawer fragment for that card.
	// Assert the response body contains "<strong>world</strong>" and "<code>code</code>".
	t.Skip("TODO: hook into existing test helpers in this package")
}
```

If `web/handlers_test.go` already has a helper like `newTestServer` or similar, replace the `Skip` with a real test using those helpers. The pattern should be:

1. Create a card with body `"Hello **world** with `code`."`.
2. Issue an HTTP request to the drawer endpoint.
3. Assert `<strong>world</strong>` and `<code>code</code>` appear in the response body.
4. Also assert no `<script>` tag if body contains `"safe <script>x</script>"`.

- [ ] **Step 3: Run tests**

Run: `go test ./web/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/handlers_test.go
git commit -m "test(web): drawer renders markdown bodies as HTML"
```

---

## Task 13: Add `markdown.css`

**Files:**
- Create: `web/static/css/markdown.css`
- Modify: `web/templates/layout.html`
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Create `web/static/css/markdown.css`**

```css
/* markdown.css — styles for rendered markdown content (.kk-md container)
   and chroma class-based syntax highlighting. */

.kk-md > *:first-child { margin-top: 0; }
.kk-md > *:last-child  { margin-bottom: 0; }

.kk-md h1, .kk-md h2, .kk-md h3, .kk-md h4, .kk-md h5, .kk-md h6 {
  margin: 0.8em 0 0.4em;
  font-weight: 600;
  line-height: 1.25;
}
.kk-md h1 { font-size: 1.4em; }
.kk-md h2 { font-size: 1.25em; }
.kk-md h3 { font-size: 1.1em; }
.kk-md h4, .kk-md h5, .kk-md h6 { font-size: 1em; }

.kk-md p { margin: 0.5em 0; }

.kk-md ul, .kk-md ol { margin: 0.4em 0; padding-left: 1.5em; }
.kk-md li { margin: 0.1em 0; }
.kk-md li > input[type="checkbox"] {
  margin-right: 0.4em;
  vertical-align: middle;
}

.kk-md blockquote {
  margin: 0.5em 0;
  padding: 0.2em 0.8em;
  border-left: 3px solid rgba(0, 0, 0, 0.15);
  color: rgba(0, 0, 0, 0.7);
}

.kk-md code {
  font-family: var(--font-mono);
  font-size: 0.92em;
  background: rgba(0, 0, 0, 0.05);
  padding: 0.1em 0.3em;
  border-radius: 3px;
}
.kk-md pre {
  font-family: var(--font-mono);
  font-size: 0.9em;
  background: #f6f8fa;
  padding: 0.6em 0.8em;
  border-radius: 6px;
  overflow-x: auto;
  margin: 0.5em 0;
}
.kk-md pre code { background: transparent; padding: 0; }

.kk-md table {
  border-collapse: collapse;
  margin: 0.5em 0;
}
.kk-md th, .kk-md td {
  border: 1px solid rgba(0, 0, 0, 0.15);
  padding: 0.25em 0.5em;
  text-align: left;
}
.kk-md th { background: rgba(0, 0, 0, 0.04); }

.kk-md img {
  max-width: 100%;
  height: auto;
}

.kk-md a { text-decoration: underline; }

.kk-md hr {
  border: none;
  border-top: 1px solid rgba(0, 0, 0, 0.15);
  margin: 0.8em 0;
}

/* Title variant: keep titles single-line and tight. */
.kk-md-title code,
.kk-md-title em,
.kk-md-title strong,
.kk-md-title del { font-size: inherit; }

/* Chroma classes (subset — github-light-ish). Class names match
   github.com/alecthomas/chroma/v2 short token classes. */
.kk-md .chroma .k  { color: #d73a49; }            /* keyword */
.kk-md .chroma .kd { color: #d73a49; }            /* keyword declaration */
.kk-md .chroma .kn { color: #d73a49; }            /* keyword namespace */
.kk-md .chroma .kr { color: #d73a49; }            /* keyword reserved */
.kk-md .chroma .kt { color: #d73a49; }            /* keyword type */
.kk-md .chroma .s, .kk-md .chroma .s1, .kk-md .chroma .s2 { color: #032f62; }
.kk-md .chroma .sb, .kk-md .chroma .sc, .kk-md .chroma .sd, .kk-md .chroma .sh { color: #032f62; }
.kk-md .chroma .c, .kk-md .chroma .c1, .kk-md .chroma .cm { color: #6a737d; font-style: italic; }
.kk-md .chroma .n  { color: #24292e; }
.kk-md .chroma .nb { color: #005cc5; }            /* name builtin */
.kk-md .chroma .nf { color: #6f42c1; }            /* name function */
.kk-md .chroma .nx { color: #24292e; }
.kk-md .chroma .o  { color: #d73a49; }            /* operator */
.kk-md .chroma .p  { color: #24292e; }            /* punctuation */
.kk-md .chroma .mi, .kk-md .chroma .mf { color: #005cc5; }
```

- [ ] **Step 2: Link `markdown.css` from `layout.html`**

In `web/templates/layout.html`, find:

```html
<link rel="stylesheet" href="/static/css/app.css">
```

Add immediately below:

```html
<link rel="stylesheet" href="/static/css/markdown.css">
```

- [ ] **Step 3: Make body/comment textareas monospace**

Find the textarea selectors in `web/static/css/app.css` (search for `textarea` and for the compose/comment form classes). For the card body textarea and comment textarea specifically, set:

```css
font-family: var(--font-mono);
font-size: 0.95em;
line-height: 1.45;
```

If the existing CSS already uses generic `textarea { font-family: var(--font-body); }`, override it for the specific composer classes rather than globally — locate the comment composer styles (likely something like `.comment-composer textarea` or `.compose-body textarea`) and add the monospace rules there. The exact selector should match what's already in `app.css`; do not introduce a new class.

- [ ] **Step 4: Build and verify static file is served**

Run:
```bash
go build ./...
go test ./web/...
```

Expected: PASS. (The static file is served via the existing `web/static/` `embed.FS` — confirm `markdown.css` is picked up by checking the `embed` directive in `web/web.go` or wherever the embed is declared. If the embed is `//go:embed static/*` or `static`, it's already included.)

- [ ] **Step 5: Commit**

```bash
git add web/static/css/markdown.css web/templates/layout.html web/static/css/app.css
git commit -m "feat(web): add markdown.css and monospace composer textareas"
```

---

## Task 14: Dev seed — showcase card and lighter touches

**Files:**
- Modify: `scripts/dev-seed.sh`

- [ ] **Step 1: Add a markdown showcase card under `beehive`**

Locate the beehive section of `scripts/dev-seed.sh` (after the existing `BH_*` card declarations). Add a new showcase card. Use bash multi-line strings (newlines inside double quotes are passed verbatim by cobra's `StringVar`):

```bash
BH_MARKDOWN_SHOWCASE_BODY=$'## Subhead\n\nA paragraph with **bold**, _italic_, ~~strikethrough~~, `inline code`, and an autolink: https://example.com.\n\n### Lists\n\n- bullet one\n- bullet two\n  - nested\n- bullet three\n\n1. ordered\n2. ordered\n3. ordered\n\n### Task list\n\n- [x] design approved\n- [ ] implementation\n- [ ] tests\n\n### Code\n\n```go\nfunc render(md string) template.HTML {\n    return mdRenderer.Render(md)\n}\n```\n\n### Table\n\n| Status     | Count |\n|------------|------:|\n| todo       |     3 |\n| in_flight  |     1 |\n| completed  |     7 |\n\n> A blockquote for emphasis.\n\n![Diagram](https://placehold.co/600x200)\n\n[Link to docs](https://example.com/docs)\n'

BH_SHOWCASE=$(create_card beehive worker_bee \
  "Markdown ~~smoke~~ **showcase** with \`code\`" todo \
  --body "$BH_MARKDOWN_SHOWCASE_BODY" \
  --tag docs)
```

- [ ] **Step 2: Add two showcase comments on that card**

Below the showcase card declaration:

```bash
BH_SHOWCASE_C1=$'Here is what I have in mind:\n\n```bash\ntask build && ./kkullm serve --db kkullm.db\n```\n\n- step 1\n- step 2\n- step 3\n'
add_comment "$BH_SHOWCASE" drone "$BH_SHOWCASE_C1"

BH_SHOWCASE_C2=$'Looks good. Linking the [issue](https://example.com/issues/42) and emphasizing **the deadline**.'
add_comment "$BH_SHOWCASE" queen_bee "$BH_SHOWCASE_C2"
```

- [ ] **Step 3: Light touches on a couple of existing cards**

Pick two existing cards in the seed (one in `birds_nest`, one in `ant_hill`). Change their `--body` strings to include a simple markdown construct each — one with a task list, one with a fenced shell snippet. For example, find a `birds_nest` card's `--body "..."` line and change it to:

```bash
  --body $'Found three worms but only have two beaks worth of time. Plan:\n\n- [x] inventory worms\n- [ ] feed chick A\n- [ ] feed chick B'
```

And an `ant_hill` card:

```bash
  --body $'Crumb yield has dipped. Running:\n\n```sh\nfind /pantry -name "*.crumb" -newer last_haul\n```\n\nResults pending.'
```

Pick any two seed cards that read naturally with these additions — the exact choice doesn't matter, but keep the surrounding context coherent.

- [ ] **Step 4: Light-touch a couple of titles**

Change two existing titles (one beehive, one ant_hill) to use inline markdown for contrast against the plain titles. Examples:

- A beehive card title: change `"Forage south clover field at first light"` to `"Forage *south clover* field at first light"`.
- An ant_hill card title: change something like `"Audit crumb-storage inventory"` to `` "Audit `crumb-storage` inventory" ``.

Do not touch the showcase card title — it already demonstrates inline emphasis, strikethrough, and code.

- [ ] **Step 5: Run the seed against a dev server and spot-check**

Build the binary and run the seed:

```bash
task build
./kkullm serve --db kkullm.db &
SERVE_PID=$!
sleep 1
bash scripts/dev-seed.sh <<< $'yes\nyes\n'
kill $SERVE_PID
```

Open the board in a browser (`http://localhost:7722`) and visually confirm:
- The showcase card title shows bold, strikethrough, and inline code.
- Opening the showcase card's drawer shows headings, lists, task list checkboxes, a syntax-highlighted code block, a table, a blockquote, an external image, and a link with `target="_blank"`.
- The two showcase comments render their markdown.
- The two lightly-touched cards render their inline markdown / code.

(If you cannot open a browser in this environment, skip the visual step and note it; the unit + handler tests cover correctness.)

- [ ] **Step 6: Commit**

```bash
git add scripts/dev-seed.sh
git commit -m "feat(seed): showcase markdown rendering in dev fixtures"
```

---

## Task 15: Final verification

- [ ] **Step 1: Run the full test suite with -race**

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 2: Build the binary**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 3: API/CLI sanity check — markdown stays raw**

Against a running server with the showcase card seeded:

```bash
./kkullm card show "$BH_SHOWCASE_ID"          # prints raw markdown
./kkullm card show "$BH_SHOWCASE_ID" --json | jq -r .body   # raw markdown
```

Expected: output contains literal `**bold**`, `~~strikethrough~~`, fenced code blocks, etc. — **no HTML tags**.

- [ ] **Step 4: Confirm `--json` escaping**

Run: `./kkullm card show "$BH_SHOWCASE_ID" --json | jq .body`

Expected: a single valid JSON string (jq does not error), with newlines as `\n` and backticks intact.

- [ ] **Step 5: No commit needed** unless verification surfaced fixes.

---

## Self-Review

**Spec coverage:**
- Goldmark + GFM (tables, task lists, strikethrough, autolinks) → Tasks 2, 3.
- Raw HTML stripping → Task 4.
- Chroma syntax highlighting (classes only) → Task 7, CSS in Task 13.
- Inline-only titles, no links → Task 8.
- Server-side render, no caching → Tasks 10, 11 (template-funcs only; no store changes).
- API + CLI unchanged → confirmed by Task 15 verification; no tasks touch `api/` or `cmd/`.
- No `*_html` fields, no `?render=` param → no tasks add them.
- Authoring UX (monospace textarea, no preview) → Task 13 step 3.
- Comments identical to body → Task 11 (drawer.html comment-body uses `renderBody`).
- `target="_blank" rel="noopener noreferrer"` → Task 5.
- Images for http/https only → Task 6.
- Out of scope (cross-refs, mentions, click-to-toggle task lists, edit UX, upload, DB caching) → no tasks added; spec says follow-up.
- No migration → no tasks added.
- Dev seed showcase → Task 14.
- Acceptance criteria — each item maps to a test or verification step in Tasks 3–9, 12, 15.

**Placeholder scan:** Task 12 step 2 acknowledges existing helper unknowns and instructs the engineer to inspect `web/handlers_test.go` for the test-server pattern before writing the assertion. This is a known gap because the project's test helpers vary by handler; the step gives concrete assertion targets (`<strong>world</strong>`, `<code>code</code>`, no `<script>`) so the implementer cannot drift.

**Type consistency:** `RenderBody` and `RenderTitle` keep the same signature (`func(string) template.HTML`) across Tasks 2, 8, 10, 11. The template funcs in Task 10 (`renderBody`, `renderTitle`) match the calls in Task 11.
