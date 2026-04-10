package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/store"
)

func TestRootHandler(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)

	checks := []string{
		"kkullm",
		"/static/css/app.css",
		"/static/vendor/htmx.min.js",
		"x-data",
		"hx-get",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected body to contain %q", check)
		}
	}
}

func TestBoardProjectScoped(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	_, err := st.CreateCard(store.CardCreateParams{
		Title:     "Test card",
		Status:    "todo",
		ProjectID: 1, // orchestration project from seed
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET /ui/board: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if !strings.Contains(body, "Test card") {
		t.Error("expected board to contain card title")
	}
	if !strings.Contains(body, `data-status="todo"`) {
		t.Error("expected board to contain todo column with data-status")
	}
}

func TestBoardAgentScoped(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Look up the seeded user agent to get its actual name and id
	userAgent, err := st.GetAgent(1)
	if err != nil {
		t.Fatalf("get seeded agent: %v", err)
	}

	_, err = st.CreateCard(store.CardCreateParams{
		Title:     "Agent card",
		Status:    "in_flight",
		ProjectID: 1,
		Assignees: []string{userAgent.Name},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + fmt.Sprintf("/ui/board?agent=%d", userAgent.ID))
	if err != nil {
		t.Fatalf("GET /ui/board?agent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if !strings.Contains(body, "Agent card") {
		t.Error("expected board to contain agent's card")
	}
	// Verify ShowProject=true by checking for project-badge class
	if !strings.Contains(body, "project-badge") {
		t.Error("expected agent-scoped board to show project-of-origin badges")
	}
}
