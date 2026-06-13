package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
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

	// From status "todo", every status except "todo" itself should appear as a pill.
	// in_flight, blocked, tabled are reachable; considering and completed are disabled.
	for _, s := range []string{"in_flight", "blocked", "tabled", "considering", "completed"} {
		if !strings.Contains(body, ">"+s+"<") {
			t.Errorf("expected drawer to show %q as a status pill", s)
		}
	}
	// Unreachable statuses must carry the disabled class and tooltip.
	if got := strings.Count(body, "status-pill-disabled"); got != 2 {
		t.Errorf("expected exactly 2 disabled pills (considering, completed), got %d", got)
	}
	if !strings.Contains(body, "Not allowed from todo") {
		t.Error("expected disabled pills to carry an explanatory title")
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

	// Default response (no ?response= param) is the card-tile fragment,
	// used by board drag-and-drop. Must NOT be the full drawer.
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)
	if !strings.Contains(body, "card-tile") {
		t.Errorf("expected card-tile fragment by default, got: %s", body)
	}
	if strings.Contains(body, "drawer-top") {
		t.Errorf("did not expect drawer fragment by default, got: %s", body)
	}
}

func TestStatusChangeReturnsDrawerOnResponseDrawer(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Drawer response test",
		Status:    "considering",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// The drawer's status pill includes ?response=drawer to request the
	// drawer fragment instead of the card tile.
	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status?response=drawer", card.ID),
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

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)
	if !strings.Contains(body, "drawer-header") {
		t.Errorf("expected drawer fragment when ?response=drawer, got: %s", body)
	}
	if !strings.Contains(body, "drawer-section-label") {
		t.Errorf("expected drawer sections in response, got: %s", body)
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

func TestBoardRejectsBadIDs(t *testing.T) {
	cases := []struct {
		name   string
		query  string
		status int
	}{
		{"unparseable project", "?project=abc", 400},
		{"missing project", "?project=999", 404},
		{"unparseable agent", "?agent=abc", 400},
		{"missing agent", "?agent=999", 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mux := setupTestMux(t)
			ts := httptest.NewServer(mux)
			defer ts.Close()

			resp, err := http.Get(ts.URL + "/ui/board" + tc.query)
			if err != nil {
				t.Fatalf("GET /ui/board%s: %v", tc.query, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.status {
				buf, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected %d, got %d: %s", tc.status, resp.StatusCode, string(buf))
			}

			// #5 invariant: the response must not leak raw DB error strings.
			buf, _ := io.ReadAll(resp.Body)
			body := string(buf)
			if strings.Contains(body, "sql:") || strings.Contains(body, "no rows") {
				t.Errorf("response leaks raw DB error: %q", body)
			}
		})
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

func TestArchivedHandler(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create more completed cards than the cap so some land in the archive.
	// webArchiveLimit is 20, so we need 21+ completed to see archive overflow.
	// Use a smaller proxy: just verify the archived endpoint renders and
	// excludes statuses other than completed/tabled.
	for i := 0; i < 3; i++ {
		c, _ := st.CreateCard(store.CardCreateParams{
			Title:     fmt.Sprintf("Live card %d", i),
			Status:    "todo",
			ProjectID: 1,
		})
		st.UpdateCard(c.ID, store.CardUpdateParams{Status: strPtr("in_flight")})
		st.UpdateCard(c.ID, store.CardUpdateParams{Status: strPtr("completed")})
	}
	tabledCard, _ := st.CreateCard(store.CardCreateParams{
		Title:     "Tabled card",
		ProjectID: 1,
	})
	st.UpdateCard(tabledCard.ID, store.CardUpdateParams{Status: strPtr("tabled")})

	// Active card that must NOT appear on the archived page.
	st.CreateCard(store.CardCreateParams{
		Title:     "Active todo",
		Status:    "todo",
		ProjectID: 1,
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/archived?project=1")
	if err != nil {
		t.Fatalf("GET /ui/archived: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	// With only 3 completed and 1 tabled (well under the cap), archived
	// should be empty for completed and tabled.
	if !strings.Contains(body, "No archived completed cards") {
		t.Error("expected empty completed section when under the cap")
	}
	if !strings.Contains(body, "No archived tabled cards") {
		t.Error("expected empty tabled section when under the cap")
	}
	if strings.Contains(body, "Active todo") {
		t.Error("archived view should never include non-terminal cards")
	}
	if !strings.Contains(body, "Back to board") {
		t.Error("expected a back-to-board link")
	}
}

// TestArchiveNavButtonHiddenInArchivedView covers issue #24: the Archive nav
// button must not be shown as an active control while the Archived view is the
// current view. The nav lives in the layout shell and views are HTMX fragments
// swapped into #board-container, so the Archived fragment signals archive mode
// (inArchive = true) while the board fragment clears it (inArchive = false),
// and the nav button is gated with x-show="!inArchive".
func TestArchiveNavButtonHiddenInArchivedView(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	get := func(path string) string {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET %s: expected 200, got %d", path, resp.StatusCode)
		}
		buf, _ := io.ReadAll(resp.Body)
		return string(buf)
	}

	// The layout nav must gate the Archive control on the inArchive flag so it
	// disappears once the archived view is active.
	layout := get("/")
	if !strings.Contains(layout, `class="nav-archived"`) {
		t.Fatal("expected layout to contain the Archive nav button")
	}
	if !strings.Contains(layout, `x-show="!inArchive"`) {
		t.Error(`expected the Archive nav button to be gated with x-show="!inArchive"`)
	}

	// The archived fragment must put the UI into archive mode.
	archived := get("/ui/archived?project=1")
	if !strings.Contains(archived, "inArchive = true") {
		t.Error("expected archived fragment to set inArchive = true")
	}

	// A normal board view must clear archive mode so the button reappears.
	board := get("/ui/board?project=1")
	if !strings.Contains(board, "inArchive = false") {
		t.Error("expected board fragment to set inArchive = false")
	}
	if strings.Contains(board, "inArchive = true") {
		t.Error("board fragment must not set archive mode")
	}
}

func strPtr(s string) *string { return &s }

func TestAddCommentHappyPath(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Has comments",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("body=hello+from+test")
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/comments", ts.URL, card.ID), form)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)

	if !strings.Contains(body, "hello from test") {
		t.Errorf("expected re-rendered drawer to contain the new comment body, got: %s", body)
	}
	if !strings.Contains(body, "Comments (1)") {
		t.Errorf("expected comment count to update to 1, got: %s", body)
	}

	comments, err := st.ListComments(card.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment in store, got %d", len(comments))
	}
	if comments[0].Body != "hello from test" {
		t.Errorf("expected stored body 'hello from test', got %q", comments[0].Body)
	}
	if comments[0].Agent != "user" {
		t.Errorf("expected web comment author 'user', got %q", comments[0].Agent)
	}
}

func TestEditCardHappyPath(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Original title",
		Body:      "Original body",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("title=New+title&body=New+body+with+%2A%2Abold%2A%2A")
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/edit", ts.URL, card.ID), form)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	// Response is the re-rendered drawer fragment with the new title and the
	// Markdown-rendered body.
	if !strings.Contains(body, "drawer-header") {
		t.Errorf("expected drawer fragment in response, got: %s", body)
	}
	if !strings.Contains(body, "New title") {
		t.Errorf("expected new title in re-rendered drawer, got: %s", body)
	}
	if !strings.Contains(body, "<strong>bold</strong>") {
		t.Errorf("expected Markdown-rendered body in drawer, got: %s", body)
	}

	updated, err := st.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if updated.Title != "New title" {
		t.Errorf("expected stored title 'New title', got %q", updated.Title)
	}
	if updated.Body != "New body with **bold**" {
		t.Errorf("expected stored body 'New body with **bold**', got %q", updated.Body)
	}
}

func TestEditCardRejectsEmptyTitle(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Keep me",
		Body:      "Keep body",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("title=%20%20&body=whatever")
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/edit", ts.URL, card.ID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 (drawer re-render), got %d", resp.StatusCode)
	}

	updated, _ := st.GetCard(card.ID)
	if updated.Title != "Keep me" {
		t.Errorf("expected title unchanged on empty submit, got %q", updated.Title)
	}
}

func TestEditCardBadID(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("title=x&body=y")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/ui/cards/99999/edit", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for missing card, got %d", resp.StatusCode)
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
	updated, err := st.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
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

func TestAddCommentRejectsEmptyBody(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "No empty comments",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("body=%20%20%20") // whitespace only
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/comments", ts.URL, card.ID), form)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 (drawer re-render), got %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)

	if !strings.Contains(body, "Comment can&#39;t be empty.") &&
		!strings.Contains(body, "Comment can't be empty.") {
		t.Errorf("expected error message in re-rendered drawer, got: %s", body)
	}

	comments, err := st.ListComments(card.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected no comments persisted, got %d", len(comments))
	}
}

func TestAddCommentBadID(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("body=should+not+matter")
	req, err := http.NewRequest(http.MethodPost,
		ts.URL+"/ui/cards/99999/comments", form)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for missing card, got %d", resp.StatusCode)
	}
}

func TestDeleteCardFromDrawer_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Doomed card",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/ui/cards/%d/delete", card.ID), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	if got, err := st.GetCard(card.ID); err == nil {
		t.Errorf("expected card to be deleted, got %+v", got)
	}
}

func TestDeleteCardFromDrawer_StaleIDIsIdempotent(t *testing.T) {
	mux := setupTestMux(t)

	req := httptest.NewRequest(http.MethodPost, "/ui/cards/99999/delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect for stale id, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestLoadBlockersSetsToastTriggerOnError(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	s := store.New(database)
	srv := api.NewServer(s)
	ws := &WebServer{store: s, events: srv.EventBus()}

	// Close the underlying DB so any subsequent query fails.
	database.Close()

	rec := httptest.NewRecorder()
	got := ws.loadBlockers(rec)

	if got != nil {
		t.Errorf("expected nil blockers on error, got %d", len(got))
	}
	trigger := rec.Header().Get("HX-Trigger")
	if trigger == "" {
		t.Fatalf("expected HX-Trigger header to be set on error")
	}
	if !strings.Contains(trigger, "showToast") {
		t.Errorf("expected HX-Trigger to fire showToast event, got %q", trigger)
	}
	if !strings.Contains(trigger, "blockers") {
		t.Errorf("expected toast message to mention blockers, got %q", trigger)
	}
}

func TestBoardDefaultsToFirstProjectWhenIDOneMissing(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create a second project, then delete the seeded orchestration project (id 1).
	// An empty ?project= query should still resolve to a working board, not 404.
	other, err := st.CreateProject("other", "second project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := st.DeleteProject(1); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	if _, err := st.CreateCard(store.CardCreateParams{
		Title:     "Card in other project",
		Status:    "todo",
		ProjectID: other.ID,
	}); err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/board")
	if err != nil {
		t.Fatalf("GET /ui/board: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 with id-1 absent, got %d: %s", resp.StatusCode, string(buf))
	}
	buf, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(buf), "Card in other project") {
		t.Errorf("expected board to render cards from fallback project")
	}
}

func TestDrawerBadIDDoesNotLeakDBError(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/cards/99999/drawer")
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)
	if strings.Contains(body, "sql:") || strings.Contains(body, "no rows") {
		t.Errorf("response leaks raw DB error to client: %q", body)
	}
	if !strings.Contains(strings.ToLower(body), "not found") {
		t.Errorf("expected friendly 'not found' message, got %q", body)
	}
}

func TestDrawerCarriesDataCardID(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Identifiable drawer",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(fmt.Sprintf("%s/ui/cards/%d/drawer", ts.URL, card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()

	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	// The JS afterSettle hook reads data-card-id off the swapped-in drawer
	// fragment to track which card the drawer is showing — needed so SSE
	// card_updated events can refresh the open drawer.
	want := fmt.Sprintf(`data-card-id="%d"`, card.ID)
	if !strings.Contains(body, want) {
		t.Errorf("expected drawer to expose %q for the SSE refresh hook, got: %s", want, body)
	}
}

func TestDrawerHasThreeRowStructure(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Drawer structure",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(fmt.Sprintf("%s/ui/cards/%d/drawer", ts.URL, card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)

	for _, cls := range []string{"drawer-header", "drawer-scroll-wrap", "drawer-composer"} {
		if !strings.Contains(body, cls) {
			t.Errorf("expected rendered drawer to contain class %q, got: %s", cls, body)
		}
	}

	headerIdx := strings.Index(body, "drawer-header")
	scrollIdx := strings.Index(body, "drawer-scroll-wrap")
	composerIdx := strings.Index(body, "drawer-composer")
	if !(headerIdx < scrollIdx && scrollIdx < composerIdx) {
		t.Errorf("expected drawer-header < drawer-scroll-wrap < drawer-composer in source order; got %d < %d < %d", headerIdx, scrollIdx, composerIdx)
	}

	listOpen := strings.Index(body, `id="comments-list"`)
	if listOpen < 0 {
		t.Fatalf("expected #comments-list in rendered drawer")
	}
	listSegment := body[listOpen:composerIdx]
	if strings.Contains(listSegment, "drawer-composer") {
		t.Errorf("drawer-composer should be a sibling of, not nested inside, the comments list")
	}
}

func TestDrawerRendersMarkdownBody(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Markdown body test",
		Body:      "Hello **world** with `code` and a <script>",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
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

	if !strings.Contains(body, "<strong>world</strong>") {
		t.Errorf("expected rendered bold: <strong>world</strong>; got: %s", body)
	}
	if !strings.Contains(body, "<code>code</code>") {
		t.Errorf("expected rendered inline code: <code>code</code>; got: %s", body)
	}
	// The drawer template has its own <script> block for scroll shadows; we must
	// check that the raw <script> from user content was stripped. We do this by
	// confirming the drawer-body section does not contain a <script> tag.
	drawerBodyStart := strings.Index(body, `class="drawer-body`)
	if drawerBodyStart < 0 {
		t.Fatal("could not find drawer-body in response")
	}
	drawerBodyEnd := strings.Index(body[drawerBodyStart:], "</div>")
	if drawerBodyEnd < 0 {
		t.Fatal("could not find end of drawer-body div")
	}
	drawerBodySection := body[drawerBodyStart : drawerBodyStart+drawerBodyEnd]
	if strings.Contains(drawerBodySection, "<script>") {
		t.Errorf("expected raw <script> to be stripped from drawer body, but it was present in: %s", drawerBodySection)
	}
}
