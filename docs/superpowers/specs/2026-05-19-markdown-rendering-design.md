# Markdown Rendering for Cards and Comments

**Date:** 2026-05-19
**Status:** Approved (design)

## Goal

Support markdown authoring and rendering for card titles, card bodies, and comments. Markdown is the source of truth: stored as text, returned as text from the API and CLI, rendered to HTML only by the web template layer.

A future "edit existing content" capability (possibly time-limited) is anticipated but out of scope here.

## Scope

In scope:

- Card bodies: full markdown.
- Comment bodies: full markdown, identical treatment to card bodies.
- Card titles: inline-only markdown, no links.
- Server-side HTML rendering in the web UI only.
- Updated dev seed showcasing markdown features.

Out of scope (follow-ups):

- Card cross-references (`#card-123`) and `@mention` autolinking.
- Click-to-toggle interactivity for task list checkboxes (read-only render now).
- Editing existing cards/comments (including any time-limit policy).
- Live preview or split-pane editors.
- Image upload / attachments.
- DB caching of rendered HTML.

## Decisions

| # | Decision | Choice |
|---|---|---|
| 1 | Markdown library | `goldmark` |
| 2 | Raw HTML in markdown | Stripped; no `<script>`, no inline tags |
| 3 | Syntax highlighting | Yes, via chroma extension |
| 4 | GFM extensions | Tables, task lists (read-only), strikethrough, autolinks |
| 5 | Title scope | Inline only, no links |
| 6 | Render location | Web template layer only |
| 7 | Caching | None; render on read |
| 8 | CLI non-JSON output | Print raw markdown as-is |
| 9 | JSON output | Pure markdown; no `*_html` fields, no `?render=` param |
| 10 | Authoring UX | Monospace textarea; no preview |
| 11 | Comments | Same treatment as card body |
| 12 | Link behavior | `target="_blank" rel="noopener noreferrer"` on every rendered `<a>` |
| 13 | Images | `<img>` allowed only for `http://` / `https://` src; others dropped |
| 14 | Cross-refs / mentions | Follow-up, not in this spec |
| 15 | Migration | None; existing text treated as already-markdown |

## Architecture

### Layers

- **Store / model / DB** — unchanged. `title`, `body`, and `Comment.Body` remain `TEXT` columns holding markdown source.
- **API** — unchanged. JSON responses contain markdown text; no rendering, no new fields, no new endpoints.
- **CLI** — unchanged. Prints API response fields verbatim.
- **Web template layer** — invokes goldmark on render. The only layer that produces HTML from markdown.

### New package: `web/markdown`

Owns goldmark configuration and exposes two functions:

```go
package markdown

// RenderBody renders full markdown to safe HTML.
// Used for card bodies and comment bodies.
func RenderBody(md string) template.HTML

// RenderTitle renders inline-only markdown to safe HTML.
// Block constructs flatten to text; links flatten to their visible text.
// Used for card titles.
func RenderTitle(md string) template.HTML
```

Two configured goldmark instances are held as package-level vars. Goldmark renderers are immutable after construction and safe for concurrent use, so no locking is required.

Neither function returns an error — goldmark renders whatever it can from malformed input. Disallowed link/image schemes are dropped silently (debug-logged).

### Body renderer configuration

- Extensions enabled: GFM table, GFM task list, GFM strikethrough, GFM linkify (autolinks), chroma syntax highlighting.
- Raw HTML rendering disabled (`html.WithUnsafe` left off; any raw-HTML extension disabled). Raw HTML in input is escaped or dropped.
- AST transformer (or post-render step) applied to every output document:
  - For each `<a>`: set `target="_blank"` and `rel="noopener noreferrer"`.
  - For each `<img>`: keep only when `src` parses as `http://` or `https://`. Drop the node otherwise.
- Chroma styling: classes-only output. A single CSS file ships the styles.

### Title renderer

Goldmark has no built-in "inline-only" mode. Approach: parse with a goldmark instance, then walk the AST and serialize only inline children, flattening block constructs.

Rules:

- Headings, lists, blockquotes, fenced/indented code blocks, tables, horizontal rules, and block-level images: contribute their inline text content as plain text (no wrapping element).
- Inline emphasis, strong, code spans, strikethrough: render as their HTML tags.
- Link nodes: render their visible text only (no `<a>` wrapper). No links in titles per decision #5.
- Soft and hard breaks: collapse to a single space. Titles are one line.
- Output is wrapped/returned as `template.HTML`.

Because no `<a>` or `<img>` is ever emitted from `RenderTitle`, the link/image transformers used by the body renderer don't apply.

## Components and File Layout

### New files

- `web/markdown/markdown.go` — package described above.
- `web/markdown/markdown_test.go` — unit tests (see Testing).
- `web/static/markdown.css` — chroma class styles plus minimal rules for rendered markdown content (e.g., scoped under a `.kk-md` container: table borders, code block padding, task list checkbox alignment, image max-width).

### Modified files

- `web/templates.go` — register two template functions:
  - `renderBody` → calls `markdown.RenderBody`, returns `template.HTML`.
  - `renderTitle` → calls `markdown.RenderTitle`, returns `template.HTML`.
- `web/templates/*.tmpl` — every template that currently emits a card title, card body, or comment body switches from `{{ .Title }}` / `{{ .Body }}` to `{{ renderTitle .Title }}` / `{{ renderBody .Body }}`. Wrap rendered content in a container element with class `kk-md` so CSS scoping works.
- Base stylesheet (`web/static/styles.css` or equivalent) — link the new `markdown.css`. Set a monospace font stack and reasonable line-height on the body and comment textareas so authors see a clear markdown-friendly editor.
- Dev seed script (`scripts/seed/...`) — update fixtures (see Dev Seed section).

### Unchanged

- `model/`, `store/`, `db/`, `api/`, `cmd/`. No schema, query, model, handler-signature, or CLI changes.

## Data Flow

### Web read (full page or HTMX fragment)

1. Handler loads card + comments from store; fields hold markdown text.
2. Handler renders template; template calls `renderTitle` / `renderBody`.
3. Response is HTML. SSE-driven fragment refreshes go through the same templates, so live updates render markdown consistently.

### Web write (create / future-edit)

1. Form submission carries raw markdown in the textarea field.
2. Handler stores the text verbatim through the existing store API.
3. Response re-renders the relevant template; goldmark runs on the just-saved markdown for display.

### API read (CLI and external clients)

1. Handler loads card from store.
2. Standard `encoding/json` encodes `title`, `body`, `comments[].body` as JSON strings containing markdown. Standard JSON string escaping handles backslashes, quotes, and control characters.
3. CLI prints the field as-is. `--json` mode emits the JSON response unchanged.

### API write

1. JSON body carries markdown text. Stored verbatim.
2. No rendering on the API side, ever.

## Error Handling and Edge Cases

- Malformed markdown: goldmark renders what it can; no error path surfaces.
- Disallowed `href` / `src` schemes (e.g., `javascript:`, `data:`): dropped during the AST transform with a debug-level log; the surrounding text remains.
- Raw HTML in source: escaped or dropped by goldmark with raw-HTML disabled. No `<script>` or other tags reach the page.
- Existing seeded/stored content treated as already-markdown. Stray `*` or `_` in legacy text may format unexpectedly; acceptable given current data volume.
- JSON response strings containing backticks, newlines, or other markdown punctuation: handled by stock `encoding/json` escaping. No custom escaping needed.

## Concurrency

Goldmark renderers are constructed once at package init and reused. No mutexes; safe for the typical handler-per-request model. Tests include a `-race` smoke test to confirm.

## Testing

### Unit tests — `web/markdown/markdown_test.go`

**Body renderer:**

- Each enabled feature renders correctly: GFM tables, task lists (checked + unchecked), strikethrough, autolinks, fenced code with chroma classes, emphasis, lists, blockquotes, headings.
- Raw HTML is stripped or escaped: `<script>alert(1)</script>` does not produce an executable tag; inline tags like `<b>x</b>` are escaped, not interpreted.
- Every rendered `<a>` carries `target="_blank"` and `rel="noopener noreferrer"`.
- `<img>` allowed for `http://` and `https://` src; dropped for `data:`, `javascript:`, relative, and scheme-less src.

**Title renderer:**

- Block constructs flatten to text: `# Header` → `Header`; `- item` → `item`; fenced code block → its inner text; tables and blockquotes likewise.
- Inline emphasis, strong, code spans, strikethrough render as their tags.
- `[label](url)` → `label` only (no `<a>` element).
- Multi-line input collapses to one line (newlines become spaces).
- No `<a>` or `<img>` ever appears in output.

**Concurrency smoke:** render identical input from N goroutines under `-race`.

### Handler / template tests

- A card view renders body markdown into expected HTML structure (e.g., presence of `<table>` when body contains a GFM table; presence of an `<a target="_blank" rel="noopener noreferrer">` for a body link).
- HTMX drawer fragment renders body the same way as the full page.
- Comment list renders comment markdown.
- JSON API endpoint returns markdown text unchanged (no HTML tags in the response body).

### Manual verification

- Run dev seed, open the board, open the showcase card, confirm the rendered body matches the markdown source.
- `kkullm card show <id>` prints raw markdown.
- `kkullm card show <id> --json | jq .body` returns the markdown as a JSON string.

## Dev Seed Updates

Update the seed script so example data exercises rendering.

**One showcase card** with a title that uses inline emphasis, strikethrough, and inline code (e.g., `` Markdown ~~smoke~~ **showcase** with `code` ``) and a body that demonstrates each enabled feature:

- An `h2` subhead and at least one `h3`.
- Paragraphs with `**bold**`, `_italic_`, `~~strikethrough~~`, `` `inline code` ``, and an autolink.
- A bullet list with nesting, and an ordered list.
- A task list with both checked and unchecked items.
- A fenced code block with a language tag (e.g., `go`) so chroma highlighting is visible.
- A GFM table.
- A blockquote.
- An external image via `![alt](https://...)`.
- A regular `[text](url)` link.

**Two showcase comments** on that card:

- One with a multi-line fenced code block and a list.
- One with inline emphasis and a link.

**Light touches on a few other seeded cards** so the showcase isn't an outlier: one card with a paragraph plus a task list; another with a body containing a fenced shell snippet.

**Other seeded titles** mostly remain plain text. One or two get inline `code` or *italic* for contrast against the showcase title.

## Acceptance Criteria

- Card bodies and comment bodies render the full configured markdown feature set on the web UI.
- Card titles render inline-only markdown; block syntax flattens to text; link syntax flattens to its visible text.
- API responses and `kkullm card show` output return raw markdown text identical to what was stored.
- `kkullm card show --json` produces valid JSON; markdown content is correctly JSON-escaped.
- No `<script>` or other raw HTML from authored content ever reaches the rendered page.
- Every rendered link opens in a new tab with `rel="noopener noreferrer"`.
- Images render only for `http(s)` srcs.
- Dev seed produces a board where at least one card visibly demonstrates every enabled markdown feature.
- All unit and handler tests pass, including the `-race` smoke test.
