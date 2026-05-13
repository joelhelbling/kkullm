# Web UI Comment Composer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the human operator post comments on a card from the web UI's drawer, attributed to a fixed `user` agent, reusing the existing HTMX + SSE patterns.

**Architecture:** Add a single `POST /ui/cards/{id}/comments` route handled by a new `handleAddComment` in `web/handlers.go`. It mirrors `handleStatusChange`: parse, mutate via `store`, publish an SSE event, and re-render the full drawer for HTMX to swap. The drawer template gains a `<form>` below the comments list. The `user` agent is already seeded by `db.Seed()`, so no schema change is required — we just add a regression test that pins this convention.

**Tech Stack:** Go (`net/http`), `html/template`, HTMX, SQLite via `modernc.org/sqlite`, existing internal packages: `web`, `store`, `api` (EventBus), `model`, `db`.

**Spec:** `docs/superpowers/specs/2026-05-13-web-ui-comment-composer-design.md`

---

## File map

- **Modify** `web/handlers.go` — add `CommentError` field to `drawerData`; add `handleAddComment`; refactor drawer-rendering into a small helper (DRY between `handleDrawer`, `handleStatusChange`, `handleAddComment`).
- **Modify** `web/web.go` — register `POST /ui/cards/{id}/comments`.
- **Modify** `web/templates/drawer.html` — render `CommentError` (when present) and a `<form>` posting to the new route.
- **Modify** `web/static/css/app.css` — add `.comment-form`, `.comment-form-actions`, `.form-error` styles.
- **Modify** `web/handlers_test.go` — happy-path, empty-body, and bad-id tests for the new route.
- **Modify** `db/db_test.go` — regression test that `Seed` creates a `user` agent and is idempotent.

---

## Task 1: Pin the `user` agent seeding contract with a test

The spec depends on a `user` agent existing after schema init. `db.Seed()` already does this (see `db/db.go`), but there is no test pinning it. Add one so the convention can't silently regress.

**Files:**
- Modify: `db/db_test.go`

- [ ] **Step 1: Read the existing test file to find the right style and helpers**

Run: `cat db/db_test.go`

Note the existing test functions and how they open + migrate + seed an in-memory DB.

- [ ] **Step 2: Add the failing test**

Append to `db/db_test.go`:

```go
func TestSeedCreatesUserAgent(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := Seed(database); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM agents WHERE name = 'user'`).Scan(&count); err != nil {
		t.Fatalf("query user agent: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 'user' agent after Seed, got %d", count)
	}

	// Idempotency: re-running Seed must not duplicate.
	if err := Seed(database); err != nil {
		t.Fatalf("re-Seed: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agents WHERE name = 'user'`).Scan(&count); err != nil {
		t.Fatalf("query user agent after re-seed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 'user' agent after re-Seed, got %d", count)
	}
}
```

- [ ] **Step 3: Run the test to verify it passes**

Run: `go test ./db -run TestSeedCreatesUserAgent -v`
Expected: PASS (the seeding is already implemented; this test pins it).

- [ ] **Step 4: Commit**

```bash
git add db/db_test.go
git commit -m "test: pin user agent seeding contract"
```

---

## Task 2: Add `CommentError` field to `drawerData`

The handler will need to surface a validation error inline. Add the field first; later tasks consume it.

**Files:**
- Modify: `web/handlers.go:192-196`

- [ ] **Step 1: Update the struct**

In `web/handlers.go`, change:

```go
type drawerData struct {
	Card        *model.Card
	Comments    []model.Comment
	StatusPills []statusPill
}
```

to:

```go
type drawerData struct {
	Card         *model.Card
	Comments     []model.Comment
	StatusPills  []statusPill
	CommentError string
}
```

- [ ] **Step 2: Verify the package still builds**

Run: `go build ./web`
Expected: no output, exit 0.

- [ ] **Step 3: Run the existing web tests to confirm no regression**

Run: `go test ./web`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/handlers.go
git commit -m "feat(web): add CommentError field to drawerData"
```

---

## Task 3: Extract a `renderDrawer` helper

Currently `handleDrawer` and the `HX-Target == "drawer-container"` branch in `handleStatusChange` both load comments and execute the drawer template. The new comment handler will too. DRY this into a helper before adding the third caller.

**Files:**
- Modify: `web/handlers.go`

- [ ] **Step 1: Add the helper**

Add this function in `web/handlers.go` (near the other helpers, e.g., just after `buildStatusPills`):

```go
// renderDrawer loads comments for the card and writes the rendered drawer
// template. commentError, when non-empty, is shown above the comment form.
func (ws *WebServer) renderDrawer(w http.ResponseWriter, card *model.Card, commentError string) {
	comments, err := ws.store.ListComments(card.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if comments == nil {
		comments = []model.Comment{}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "drawer", drawerData{
		Card:         card,
		Comments:     comments,
		StatusPills:  buildStatusPills(card.Status),
		CommentError: commentError,
	}); err != nil {
		log.Printf("render drawer: %v", err)
	}
}
```

- [ ] **Step 2: Replace the body of `handleDrawer` to use the helper**

In `handleDrawer` (currently web/handlers.go:212-242), replace the comments-load + template-execute block at the end of the function with:

```go
ws.renderDrawer(w, card, "")
```

So the full function becomes:

```go
func (ws *WebServer) handleDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	card, err := ws.store.GetCard(id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	ws.renderDrawer(w, card, "")
}
```

- [ ] **Step 3: Replace the drawer-render branch in `handleStatusChange`**

In `handleStatusChange` (web/handlers.go:244-302), replace the entire `if hxTarget == "drawer-container" { ... }` block with:

```go
if hxTarget == "drawer-container" {
	ws.renderDrawer(w, card, "")
	return
}
```

- [ ] **Step 4: Build and run all web tests**

Run: `go test ./web -v`
Expected: PASS (behavior unchanged).

- [ ] **Step 5: Commit**

```bash
git add web/handlers.go
git commit -m "refactor(web): extract renderDrawer helper"
```

---

## Task 4: Write the failing test for the happy-path comment POST

We write the test before the handler exists so we exercise TDD.

**Files:**
- Modify: `web/handlers_test.go`

- [ ] **Step 1: Inspect the existing handler tests to copy the style**

Run: `grep -n "func Test" web/handlers_test.go`

Pick a similar test (e.g., a status-change or drawer test) as a stylistic reference.

- [ ] **Step 2: Add the test**

Append to `web/handlers_test.go`:

```go
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
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./web -run TestAddCommentHappyPath -v`
Expected: FAIL — either 404 (no route) or non-200 status; the route does not exist yet.

- [ ] **Step 4: Commit the failing test**

```bash
git add web/handlers_test.go
git commit -m "test(web): failing test for comment POST happy path"
```

---

## Task 5: Implement the `handleAddComment` route

Make the failing test pass with the minimal implementation.

**Files:**
- Modify: `web/web.go`
- Modify: `web/handlers.go`

- [ ] **Step 1: Register the route in `web/web.go`**

Add this line alongside the other `mux.HandleFunc` calls in `RegisterRoutes`:

```go
mux.HandleFunc("POST /ui/cards/{id}/comments", ws.handleAddComment)
```

- [ ] **Step 2: Add the handler in `web/handlers.go`**

Append this function:

```go
func (ws *WebServer) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	card, err := ws.store.GetCard(id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", 400)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		ws.renderDrawer(w, card, "Comment can't be empty.")
		return
	}

	agent, err := ws.store.GetAgentByName("user")
	if err != nil {
		http.Error(w, "user agent missing — check seed data", 500)
		return
	}

	comment, err := ws.store.CreateComment(card.ID, agent.ID, body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	ws.events.Publish(api.Event{Type: "comment_created", Data: comment})

	ws.renderDrawer(w, card, "")
}
```

- [ ] **Step 3: Add `strings` to the import block in `web/handlers.go` if not already present**

Run: `head -20 web/handlers.go`

If `"strings"` is missing from the imports, add it.

- [ ] **Step 4: Run the happy-path test**

Run: `go test ./web -run TestAddCommentHappyPath -v`
Expected: PASS.

- [ ] **Step 5: Run the full web suite**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/web.go web/handlers.go
git commit -m "feat(web): POST /ui/cards/{id}/comments handler"
```

---

## Task 6: Add the empty-body validation test and confirm behavior

The handler already trims and routes empty bodies through `renderDrawer` with a `CommentError`. Pin it with a test.

**Files:**
- Modify: `web/handlers_test.go`

- [ ] **Step 1: Add the test**

Append to `web/handlers_test.go`:

```go
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
```

Note: Go's `html/template` HTML-escapes the apostrophe in the message, so the test allows either form.

- [ ] **Step 2: Run the test**

Run: `go test ./web -run TestAddCommentRejectsEmptyBody -v`
Expected: FAIL — the template doesn't render `CommentError` yet (Task 7 fixes that).

- [ ] **Step 3: Commit the failing test**

```bash
git add web/handlers_test.go
git commit -m "test(web): failing test for empty-body comment rejection"
```

---

## Task 7: Render the comment form and error in `drawer.html`

Make the empty-body test pass and add the composer UI.

**Files:**
- Modify: `web/templates/drawer.html`

- [ ] **Step 1: Read the current template to locate the comments section**

Run: `cat web/templates/drawer.html`

Find the block at lines 73-89 (the `<div class="drawer-section" style="border-bottom:none;">` containing comments).

- [ ] **Step 2: Replace the comments section**

Replace the entire block (from `<div class="drawer-section" style="border-bottom:none;">` through its closing `</div>` before `{{end}}` of the `drawer` template) with:

```html
<div class="drawer-section" style="border-bottom:none;">
  <div class="drawer-section-label">Comments ({{len .Comments}})</div>
  <div id="comments-list">
    {{range .Comments}}
    <div class="comment">
      <div class="comment-header">
        <span class="comment-agent{{if eq .Agent "user"}} user-agent{{end}}">{{.Agent}}</span>
        <span class="comment-time">{{timeAgo .CreatedAt}}</span>
      </div>
      <div class="comment-body">{{.Body}}</div>
    </div>
    {{end}}
    {{if not .Comments}}
    <div style="font-size:13px;color:var(--text-secondary);">No comments yet.</div>
    {{end}}
  </div>

  <form class="comment-form"
        hx-post="/ui/cards/{{.Card.ID}}/comments"
        hx-target="#drawer-container"
        hx-swap="innerHTML">
    {{if .CommentError}}
    <div class="form-error">{{.CommentError}}</div>
    {{end}}
    <textarea name="body" rows="3" required
              placeholder="Add a comment as user…"></textarea>
    <div class="comment-form-actions">
      <button type="submit">Comment</button>
    </div>
  </form>
</div>
```

- [ ] **Step 3: Run the empty-body test**

Run: `go test ./web -run TestAddCommentRejectsEmptyBody -v`
Expected: PASS.

- [ ] **Step 4: Run the happy-path test again to confirm no regression**

Run: `go test ./web -run TestAddComment -v`
Expected: both `TestAddCommentHappyPath` and `TestAddCommentRejectsEmptyBody` PASS.

- [ ] **Step 5: Run the full web suite**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/templates/drawer.html
git commit -m "feat(web): comment composer form in drawer"
```

---

## Task 8: Add the bad-id test

**Files:**
- Modify: `web/handlers_test.go`

- [ ] **Step 1: Add the test**

Append to `web/handlers_test.go`:

```go
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
```

- [ ] **Step 2: Run the test**

Run: `go test ./web -run TestAddCommentBadID -v`
Expected: PASS (handler already returns 404 when `GetCard` fails).

- [ ] **Step 3: Commit**

```bash
git add web/handlers_test.go
git commit -m "test(web): bad-id case for comment POST"
```

---

## Task 9: Style the form

**Files:**
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Append the new styles**

Add to the end of `web/static/css/app.css`:

```css
/* ===== Comment Composer ===== */
.comment-form {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.comment-form textarea {
  width: 100%;
  resize: vertical;
  font: inherit;
  font-size: 13px;
  padding: 8px;
  border: 1px solid var(--border-color, #ccc);
  border-radius: 4px;
  background: var(--bg-input, #fff);
  color: var(--text-primary);
  box-sizing: border-box;
}

.comment-form-actions {
  display: flex;
  justify-content: flex-end;
}

.comment-form-actions button {
  font: inherit;
  font-size: 13px;
  padding: 6px 14px;
  border: 1px solid var(--border-color, #ccc);
  border-radius: 4px;
  background: var(--bg-button, #f6f6f6);
  color: var(--text-primary);
  cursor: pointer;
}

.comment-form-actions button:hover {
  background: var(--bg-button-hover, #ececec);
}

.form-error {
  font-size: 12px;
  color: #c0392b;
}
```

(The CSS variables that already exist in the project will resolve; the fallbacks let the page degrade gracefully if a variable isn't defined.)

- [ ] **Step 2: Verify CSS still parses by serving the file in tests**

Run: `go test ./web -run TestStaticFileServing -v`
Expected: PASS.

- [ ] **Step 3: Manual smoke check**

Run: `go run . serve`

Then in a browser, open the web UI, click into a card's drawer, type a comment, hit "Comment", confirm:
- the comment appears with `user` as the author,
- the count in the section label increments,
- the textarea is empty after the swap,
- submitting an empty/whitespace-only comment shows "Comment can't be empty." above the form.

Stop the server when done.

- [ ] **Step 4: Commit**

```bash
git add web/static/css/app.css
git commit -m "style(web): comment composer styles"
```

---

## Task 10: Final verification

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Verify the new route is wired**

Run: `grep -n "comments" web/web.go`
Expected: the line `mux.HandleFunc("POST /ui/cards/{id}/comments", ws.handleAddComment)` is present.

- [ ] **Step 3: Sanity-check the seed data on a fresh DB**

Run:
```bash
rm -f /tmp/kkullm-smoke.db
go run . serve --db /tmp/kkullm-smoke.db &
SERVER_PID=$!
sleep 1
sqlite3 /tmp/kkullm-smoke.db "SELECT name FROM agents WHERE name='user';"
kill $SERVER_PID
```
Expected: a single row `user`.

- [ ] **Step 4: Confirm there are no uncommitted changes left over**

Run: `git status`
Expected: clean.
