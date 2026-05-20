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
