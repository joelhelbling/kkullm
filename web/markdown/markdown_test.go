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

func TestRenderBody_StripsScriptTag(t *testing.T) {
	got := string(RenderBody("<script>alert(1)</script>"))
	if strings.Contains(got, "<script>") {
		t.Errorf("expected <script> to be escaped or dropped, got: %s", got)
	}
}

func TestRenderBody_EscapesInlineHTML(t *testing.T) {
	got := string(RenderBody("a <b>bold?</b> word"))
	if strings.Contains(got, "<b>bold?</b>") {
		t.Errorf("expected raw <b> to be escaped, got: %s", got)
	}
}

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

func TestRenderBody_FencedCodeHighlighted(t *testing.T) {
	src := "```go\nfunc main() {}\n```\n"
	got := string(RenderBody(src))
	if !strings.Contains(got, "<pre") || !strings.Contains(got, "func") {
		t.Errorf("expected highlighted code block, got: %s", got)
	}
	if !strings.Contains(got, "class=") {
		t.Errorf("expected chroma class attributes, got: %s", got)
	}
}

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

func TestRenderTitle_CodeBlockTextPreserved(t *testing.T) {
	got := string(RenderTitle("```\nhello world\n```"))
	if strings.Contains(got, "<pre") || strings.Contains(got, "<code") {
		t.Errorf("expected no <pre>/<code>, got: %s", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected code text preserved, got: %q", got)
	}
}

func TestRenderTitle_TableFlattens(t *testing.T) {
	got := string(RenderTitle("| a | b |\n|---|---|\n| 1 | 2 |\n"))
	if strings.Contains(got, "<table") || strings.Contains(got, "<td") || strings.Contains(got, "<tr") || strings.Contains(got, "<th") {
		t.Errorf("expected no table tags, got: %s", got)
	}
	if strings.Contains(got, "|") || strings.Contains(got, "---") {
		t.Errorf("expected no pipe/separator chars, got: %q", got)
	}
	for _, want := range []string{"a", "b", "1", "2"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected cell text %q present, got: %q", want, got)
		}
	}
}

func TestRenderTitle_BlockquoteFlattens(t *testing.T) {
	got := string(RenderTitle("> quoted text"))
	if strings.Contains(got, "<blockquote") {
		t.Errorf("expected no <blockquote>, got: %s", got)
	}
	if !strings.Contains(got, "quoted text") {
		t.Errorf("expected blockquote text preserved, got: %q", got)
	}
}

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
