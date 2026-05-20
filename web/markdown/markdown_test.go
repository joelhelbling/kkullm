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
