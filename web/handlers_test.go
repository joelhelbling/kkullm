package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/model"
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

func TestBoardAllProjects(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Seed card lives in the seeded "orchestration" project (id 1).
	if _, err := st.CreateCard(store.CardCreateParams{
		Title:     "Orchestration card",
		Status:    "todo",
		ProjectID: 1,
	}); err != nil {
		t.Fatalf("create card in project 1: %v", err)
	}

	other, err := st.CreateProject("other-project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := st.CreateCard(store.CardCreateParams{
		Title:     "Other project card",
		Status:    "todo",
		ProjectID: other.ID,
	}); err != nil {
		t.Fatalf("create card in other project: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// All view: both projects' cards appear.
	resp, err := http.Get(ts.URL + "/ui/board?project=all")
	if err != nil {
		t.Fatalf("GET /ui/board?project=all: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)
	if !strings.Contains(body, "Orchestration card") {
		t.Error("expected All view to contain card from project 1")
	}
	if !strings.Contains(body, "Other project card") {
		t.Error("expected All view to contain card from other project")
	}

	// Specific project remains scoped: only that project's card.
	resp2, err := http.Get(ts.URL + fmt.Sprintf("/ui/board?project=%d", other.ID))
	if err != nil {
		t.Fatalf("GET /ui/board?project=%d: %v", other.ID, err)
	}
	defer resp2.Body.Close()
	buf2, _ := io.ReadAll(resp2.Body)
	body2 := string(buf2)
	if !strings.Contains(body2, "Other project card") {
		t.Error("expected scoped view to contain its own card")
	}
	if strings.Contains(body2, "Orchestration card") {
		t.Error("scoped view leaked a card from another project")
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
	_, err = st.CreateComment(card.ID, 1, "Test comment", "")
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

	// From status "todo", every other status appears as a clickable pill.
	// There is no longer any transition validation, so no pill is disabled.
	// blocked is not a status, so it is not a status pill.
	for _, s := range []string{"in_flight", "tabled", "considering", "completed"} {
		if !strings.Contains(body, ">"+s+"<") {
			t.Errorf("expected drawer to show %q as a status pill", s)
		}
	}
	if strings.Contains(body, "status-pill-disabled") {
		t.Error("expected no disabled status pills now that any transition is allowed")
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

func TestStatusChangeAllowsAnyTransition(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Any transition test",
		Status:    "considering",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// considering -> completed was previously illegal; it now succeeds.
	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=completed"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for any transition, got %d", resp.StatusCode)
	}

	updated, _ := st.GetCard(card.ID)
	if updated.Status != "completed" {
		t.Errorf("status = %q, want %q", updated.Status, "completed")
	}

	// An unknown status is still rejected.
	bad, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=bogus"))
	bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badResp, err := http.DefaultClient.Do(bad)
	if err != nil {
		t.Fatalf("PATCH bad status: %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != 422 {
		t.Fatalf("expected 422 for unknown status, got %d", badResp.StatusCode)
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

func TestBoardAgentScopedIncludesFormerlyAssignedBlocked(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	alice, err := st.GetAgent(1)
	if err != nil {
		t.Fatalf("get seeded agent: %v", err)
	}
	if _, err := st.CreateAgent("bob", 1, ""); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	// Current card: blocked & still assigned to alice.
	current, err := st.CreateCard(store.CardCreateParams{
		Title: "Current alice card", Status: "in_flight", ProjectID: 1,
		Assignees: []string{alice.Name},
	})
	if err != nil {
		t.Fatalf("create current: %v", err)
	}
	bTrue := true
	if _, err := st.UpdateCard(current.ID, store.CardUpdateParams{Blocked: &bTrue}); err != nil {
		t.Fatalf("block current: %v", err)
	}

	// Formerly-assigned card: blocked, assigned to alice then reassigned to bob.
	former, err := st.CreateCard(store.CardCreateParams{
		Title: "Formerly alice card", Status: "in_flight", ProjectID: 1,
		Assignees: []string{alice.Name},
	})
	if err != nil {
		t.Fatalf("create former: %v", err)
	}
	if _, err := st.UpdateCard(former.ID, store.CardUpdateParams{
		Blocked: &bTrue, Assignees: []string{"bob"},
	}); err != nil {
		t.Fatalf("reassign former: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + fmt.Sprintf("/ui/board?agent=%d", alice.ID))
	if err != nil {
		t.Fatalf("GET /ui/board?agent: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if !strings.Contains(body, "Current alice card") {
		t.Error("expected board to contain alice's current card")
	}
	if !strings.Contains(body, "Formerly alice card") {
		t.Error("expected board to contain the formerly-assigned blocked card")
	}
	if !strings.Contains(body, "card-formerly-badge") {
		t.Error("expected the formerly-assigned card to be marked with card-formerly-badge")
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

func TestAssignCardAddsAssignee(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// A second agent in the same (orchestration) project, alongside the
	// seeded "user" agent.
	if _, err := st.CreateAgent("scout", 1, "Recon agent"); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Assign me",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Set assignees to {user, scout} — add "scout".
	form := strings.NewReader("assignee=user&assignee=scout")
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/assignees", ts.URL, card.ID), form)
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

	if !strings.Contains(body, "drawer-header") {
		t.Errorf("expected drawer fragment in response, got: %s", body)
	}
	if !strings.Contains(body, "scout") {
		t.Errorf("expected re-rendered drawer to show new assignee 'scout', got: %s", body)
	}

	updated, err := st.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	want := map[string]bool{"user": true, "scout": true}
	if len(updated.Assignees) != 2 {
		t.Fatalf("expected 2 assignees, got %v", updated.Assignees)
	}
	for _, a := range updated.Assignees {
		if !want[a] {
			t.Errorf("unexpected assignee %q in %v", a, updated.Assignees)
		}
	}
}

func TestAssignCardRemovesAssignee(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	if _, err := st.CreateAgent("scout", 1, "Recon agent"); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Unassign me",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user", "scout"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Set assignees to {user} only — remove "scout".
	form := strings.NewReader("assignee=user")
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/assignees", ts.URL, card.ID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, err := st.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if len(updated.Assignees) != 1 || updated.Assignees[0] != "user" {
		t.Fatalf("expected assignees [user], got %v", updated.Assignees)
	}
}

func TestAssignCardClearsAllAssignees(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Clear me",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Empty submission (form present, no assignee values) clears all.
	form := strings.NewReader("")
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/ui/cards/%d/assignees", ts.URL, card.ID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, err := st.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if len(updated.Assignees) != 0 {
		t.Fatalf("expected no assignees after clear, got %v", updated.Assignees)
	}
}

func TestAssignCardBadID(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	form := strings.NewReader("assignee=user")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/ui/cards/99999/assignees", form)
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

func TestDrawerShowsAssigneePicker(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	if _, err := st.CreateAgent("scout", 1, "Recon agent"); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Pick assignees",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + fmt.Sprintf("/ui/cards/%d/drawer", card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	// The picker posts to the assignees endpoint and offers every project agent.
	if !strings.Contains(body, fmt.Sprintf("/ui/cards/%d/assignees", card.ID)) {
		t.Errorf("expected drawer to contain assignee form posting to /assignees, got: %s", body)
	}
	if !strings.Contains(body, `value="scout"`) {
		t.Errorf("expected drawer picker to offer project agent 'scout', got: %s", body)
	}
	if !strings.Contains(body, `value="user"`) {
		t.Errorf("expected drawer picker to offer project agent 'user', got: %s", body)
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

	// 6. Board should not show the blocked badge yet (card is in todo).
	resp, err = http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET board: %v", err)
	}
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(buf), "card-blocked-badge") {
		t.Error("board should not show a blocked badge before the card is blocked")
	}

	// 7. Block the card via the web endpoint.
	resp, err = http.Post(
		ts.URL+fmt.Sprintf("/ui/cards/%d/block", card.ID),
		"application/x-www-form-urlencoded",
		strings.NewReader("reason=blocked+in+flow"))
	if err != nil {
		t.Fatalf("POST block: %v", err)
	}
	resp.Body.Close()
	blockedCard, _ := st.GetCard(card.ID)
	if !blockedCard.Blocked {
		t.Error("expected card to be blocked after POST /block")
	}

	// 8. Board should now render the card in place with the blocked badge.
	resp, err = http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET board: %v", err)
	}
	buf, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(buf)
	if !strings.Contains(body, "Flow test card") {
		t.Error("board should still contain the card in its real column")
	}
	if !strings.Contains(body, "card-blocked-badge") {
		t.Error("board should show the blocked badge for the blocked card")
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

// --- Issue #32: blocked badge, block/unblock controls, unblock-on-edit ---

func TestCardTileBadgeWhenBlocked(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked tile card",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if _, err := st.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET board: %v", err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if !strings.Contains(body, "card-tile-blocked") {
		t.Errorf("expected blocked card tile to carry card-tile-blocked marker; got: %s", body)
	}
	if !strings.Contains(body, "card-blocked-badge") {
		t.Errorf("expected blocked badge markup on tile; got: %s", body)
	}
}

func TestCardTileNoBadgeWhenNotBlocked(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	_, err := st.CreateCard(store.CardCreateParams{
		Title:     "Unblocked tile card",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET board: %v", err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if strings.Contains(body, "card-blocked-badge") {
		t.Errorf("did not expect blocked badge on a non-blocked board; got: %s", body)
	}
}

func TestBlockEndpointSetsFlagAndComment(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Block me",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(
		ts.URL+fmt.Sprintf("/ui/cards/%d/block", card.ID),
		"application/x-www-form-urlencoded",
		strings.NewReader("reason=waiting+on+deps"))
	if err != nil {
		t.Fatalf("POST block: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, _ := st.GetCard(card.ID)
	if !updated.Blocked {
		t.Error("expected card to be blocked after POST /block")
	}

	comments, _ := st.ListComments(card.ID)
	var found *model.Comment
	for i := range comments {
		if comments[i].Kind == "block" {
			found = &comments[i]
		}
	}
	if found == nil {
		t.Fatalf("expected a kind=block comment, got %+v", comments)
	}
	if !strings.Contains(found.Body, "waiting on deps") {
		t.Errorf("expected block comment to carry reason, got %q", found.Body)
	}

	buf, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(buf), "drawer-header") {
		t.Error("expected block endpoint to re-render the drawer")
	}
}

func TestUnblockEndpointClearsFlagAndComment(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Unblock me",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if _, err := st.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(
		ts.URL+fmt.Sprintf("/ui/cards/%d/unblock", card.ID),
		"application/x-www-form-urlencoded",
		strings.NewReader("reason=resolved"))
	if err != nil {
		t.Fatalf("POST unblock: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, _ := st.GetCard(card.ID)
	if updated.Blocked {
		t.Error("expected card to be unblocked after POST /unblock")
	}

	comments, _ := st.ListComments(card.ID)
	var found bool
	for _, c := range comments {
		if c.Kind == "unblock" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a kind=unblock comment, got %+v", comments)
	}
}

func TestStatusChangeWithUnblockSignalClearsFlag(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked drag card",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if _, err := st.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status?unblock=1", card.ID),
		strings.NewReader("status=in_flight"))
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
	if updated.Status != "in_flight" {
		t.Errorf("expected status in_flight, got %q", updated.Status)
	}
	if updated.Blocked {
		t.Error("expected card to be unblocked with ?unblock=1 signal")
	}
	comments, _ := st.ListComments(card.ID)
	var found bool
	for _, c := range comments {
		if c.Kind == "unblock" {
			found = true
		}
	}
	if !found {
		t.Error("expected an unblock comment when status change clears the flag")
	}
}

func TestStatusChangeWithoutSignalLeavesBlocked(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Still blocked card",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if _, err := st.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=in_flight"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	defer resp.Body.Close()

	updated, _ := st.GetCard(card.ID)
	if !updated.Blocked {
		t.Error("expected card to remain blocked without unblock signal")
	}
}

func TestAssignWithUnblockSignalClearsFlag(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked assign card",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if _, err := st.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(
		ts.URL+fmt.Sprintf("/ui/cards/%d/assignees?unblock=1", card.ID),
		"application/x-www-form-urlencoded",
		strings.NewReader("assignee=user"))
	if err != nil {
		t.Fatalf("POST assignees: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, _ := st.GetCard(card.ID)
	if updated.Blocked {
		t.Error("expected card to be unblocked with ?unblock=1 on assign")
	}
}

func TestDrawerShowsBlockStateAndReason(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Drawer block state",
		Status:    "todo",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	if _, err := st.UpdateCard(card.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}
	if _, err := st.CreateComment(card.ID, 1, "stuck on upstream API", "block"); err != nil {
		t.Fatalf("create block comment: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + fmt.Sprintf("/ui/cards/%d/drawer", card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	if !strings.Contains(body, "stuck on upstream API") {
		t.Error("expected drawer to surface the block reason")
	}
	// An unblock control should be present for a blocked card.
	if !strings.Contains(body, "/unblock") {
		t.Error("expected drawer to offer an unblock control for a blocked card")
	}
}

func boolPtr32(b bool) *bool { return &b }

// TestBlockedViewShowsOnlyBlockedCardsAcrossProjects exercises the orchestrator
// "blocked" triage view: it lists every blocked card, across all projects, each
// in its real status column, surfacing the latest kind="block" reason. Unblocked
// cards must not appear.
func TestBlockedViewShowsOnlyBlockedCardsAcrossProjects(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Blocked card in the seeded "orchestration" project (id 1), status in_flight.
	blocked, err := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked orchestration card",
		Status:    "in_flight",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create blocked card: %v", err)
	}
	if _, err := st.UpdateCard(blocked.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked: %v", err)
	}
	// The "why" lives in a kind="block" comment (user agent id 1 from seed).
	if _, err := st.CreateComment(blocked.ID, 1, "waiting on credentials", "block"); err != nil {
		t.Fatalf("create block comment: %v", err)
	}

	// A blocked card in a second project, status todo.
	other, err := st.CreateProject("other-project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	blocked2, err := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked other-project card",
		Status:    "todo",
		ProjectID: other.ID,
	})
	if err != nil {
		t.Fatalf("create blocked card 2: %v", err)
	}
	if _, err := st.UpdateCard(blocked2.ID, store.CardUpdateParams{Blocked: boolPtr32(true)}); err != nil {
		t.Fatalf("set blocked 2: %v", err)
	}

	// A NON-blocked card that must be excluded from the view.
	if _, err := st.CreateCard(store.CardCreateParams{
		Title:     "Unblocked card",
		Status:    "todo",
		ProjectID: 1,
	}); err != nil {
		t.Fatalf("create unblocked card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/blocked")
	if err != nil {
		t.Fatalf("GET /ui/blocked: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	buf, _ := io.ReadAll(resp.Body)
	body := string(buf)

	// Both blocked cards appear, across projects.
	if !strings.Contains(body, "Blocked orchestration card") {
		t.Error("expected blocked view to contain the blocked orchestration card")
	}
	if !strings.Contains(body, "Blocked other-project card") {
		t.Error("expected blocked view to contain the blocked other-project card")
	}
	// The unblocked card must be excluded.
	if strings.Contains(body, "Unblocked card") {
		t.Error("blocked view leaked an unblocked card")
	}
	// The block reason from the kind=\"block\" comment is surfaced.
	if !strings.Contains(body, "waiting on credentials") {
		t.Error("expected blocked view to surface the block reason")
	}
	// Cards span projects, so the cross-project label/badge is shown.
	if !strings.Contains(body, "other-project") {
		t.Error("expected blocked view to show the project label (showProject)")
	}
	// Each blocked card still renders in its real status column. The blocked
	// card sits in in_flight; its column header must be present.
	if !strings.Contains(body, `data-status="in_flight"`) {
		t.Error("expected blocked view to render real status columns")
	}
}
