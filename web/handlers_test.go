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

func TestDrawerHandler(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Drawer test card",
		Body:      "This is the card body",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
		Tags:      []string{"bug"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	// Add a comment (user agent has ID 1 from seed)
	_, err = st.CreateComment(card.ID, 1, "Test comment")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + fmt.Sprintf("/ui/cards/%d/drawer", card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	checks := []string{
		"Drawer test card",
		"This is the card body",
		"Test comment",
		"bug",
		"user",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected drawer to contain %q", check)
		}
	}

	// From status "todo", valid transitions are in_flight, blocked, tabled
	if !strings.Contains(body, "in_flight") {
		t.Error("expected drawer to show in_flight as valid transition")
	}
}

func TestStatusChange(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Status test",
		Status:    "considering",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Valid transition: considering -> todo
	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=todo"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, _ := st.GetCard(card.ID)
	if updated.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", updated.Status)
	}
}

func TestStatusChangeInvalid(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Invalid transition test",
		Status:    "considering",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Invalid transition: considering -> in_flight (must go through todo first)
	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=in_flight"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
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

func TestBlockersHandler(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create a card, move it to todo, then blocked
	card, _ := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked card",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
	})

	blockedStatus := "blocked"
	st.UpdateCard(card.ID, store.CardUpdateParams{Status: &blockedStatus})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/blockers")
	if err != nil {
		t.Fatalf("GET /ui/blockers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if !strings.Contains(body, "Blocked card") {
		t.Error("expected blockers to contain blocked card")
	}
}

func TestFullFlow(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Flow test card",
		Body:      "Testing the full flow",
		Status:    "considering",
		ProjectID: 1,
		Assignees: []string{"user"},
		Tags:      []string{"test"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Load root page
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("root: expected 200, got %d", resp.StatusCode)
	}

	// 2. Fetch board
	resp, err = http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET /ui/board: %v", err)
	}
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(buf), "Flow test card") {
		t.Error("board should contain the test card")
	}

	// 3. Open drawer
	resp, err = http.Get(ts.URL + fmt.Sprintf("/ui/cards/%d/drawer", card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(buf), "Testing the full flow") {
		t.Error("drawer should contain card body")
	}

	// 4. Change status: considering -> todo
	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=todo"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status change: expected 200, got %d", resp.StatusCode)
	}

	// 5. Verify card is now in todo
	updated, _ := st.GetCard(card.ID)
	if updated.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", updated.Status)
	}

	// 6. Fetch blockers (should be empty of this card)
	resp, err = http.Get(ts.URL + "/ui/blockers")
	if err != nil {
		t.Fatalf("GET blockers: %v", err)
	}
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(buf), "Flow test card") {
		t.Error("blockers should not contain the test card (it's in todo, not blocked)")
	}

	// 7. Move to blocked
	req, _ = http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=blocked"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	// 8. Fetch blockers (should contain the card now)
	resp, err = http.Get(ts.URL + "/ui/blockers")
	if err != nil {
		t.Fatalf("GET blockers: %v", err)
	}
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(buf), "Flow test card") {
		t.Error("blockers should contain the blocked card")
	}
}
