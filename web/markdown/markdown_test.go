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
