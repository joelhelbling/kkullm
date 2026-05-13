# Drawer Scroll Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the card drawer's header/meta/body stay fixed at the top and the composer stay fixed at the bottom, with the comments list scrolling between them (defaulting to the bottom), and subtle fade indicators when content extends past the visible scroll edges.

**Architecture:** Convert the drawer container from a single `overflow-y: auto` block into a vertical flex column with three children rendered by the template: `.drawer-top`, `.drawer-comments`, `.drawer-composer`. An inline `<script>` at the end of the drawer template runs on every HTMX swap to scroll the comments list to its bottom and to toggle two CSS classes that drive the top/bottom fade gradients.

**Tech Stack:** Go `html/template`, HTMX, vanilla JS, CSS (existing CSS custom-property theme system).

**Spec:** `docs/superpowers/specs/2026-05-13-drawer-scroll-layout-design.md`

---

## File map

- **Modify** `web/templates/drawer.html` — wrap contents in three rows; move the comment form out of the Comments section; append inline script.
- **Modify** `web/static/css/app.css` — change `.drawer` to a flex column; add `.drawer-top`, `.drawer-comments`, `.drawer-composer`, `::before` and `::after` shadow rules, and `has-shadow-top` / `has-shadow-bottom` toggles.
- **Modify** `web/handlers_test.go` — small regression test that the rendered drawer has `.drawer-composer` as a *sibling* of `.drawer-comments`, not nested inside it.

---

## Task 1: Failing test for new drawer structure

We assert that the rendered drawer HTML contains the three new wrapper classes in the right order. This pins the structural contract.

**Files:**
- Modify: `web/handlers_test.go`

- [ ] **Step 1: Append the test**

Append to `web/handlers_test.go`:

```go
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

	for _, cls := range []string{"drawer-top", "drawer-comments", "drawer-composer"} {
		if !strings.Contains(body, cls) {
			t.Errorf("expected rendered drawer to contain class %q, got: %s", cls, body)
		}
	}

	topIdx := strings.Index(body, "drawer-top")
	commentsIdx := strings.Index(body, "drawer-comments")
	composerIdx := strings.Index(body, "drawer-composer")
	if !(topIdx < commentsIdx && commentsIdx < composerIdx) {
		t.Errorf("expected drawer-top < drawer-comments < drawer-composer in source order; got %d < %d < %d", topIdx, commentsIdx, composerIdx)
	}

	// Sibling check: the composer must NOT be nested inside the comments section.
	// We slice the substring between drawer-comments and its presumed close, and
	// verify drawer-composer does not appear inside the comments list block.
	// A practical proxy: the composer must not appear inside #comments-list.
	listOpen := strings.Index(body, `id="comments-list"`)
	if listOpen < 0 {
		t.Fatalf("expected #comments-list in rendered drawer")
	}
	// The list's contents end at composerIdx if composer is a sibling AFTER the list.
	listSegment := body[listOpen:composerIdx]
	if strings.Contains(listSegment, "drawer-composer") {
		t.Errorf("drawer-composer should be a sibling of, not nested inside, the comments list")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./web -run TestDrawerHasThreeRowStructure -v`
Expected: FAIL — the template doesn't have these wrappers yet.

- [ ] **Step 3: Commit**

```bash
git add web/handlers_test.go
git commit -m "test(web): failing test for drawer three-row structure"
```

---

## Task 2: Restructure the drawer template into three rows

Wrap the existing content in three `<div>`s and move the form out of the comments section.

**Files:**
- Modify: `web/templates/drawer.html`

- [ ] **Step 1: Read the current template**

Run: `cat web/templates/drawer.html`

Identify these existing blocks:

- **Top block (everything above Comments):** the drawer-header `<div>`, the Status `<div class="drawer-section">`, the Meta `<div class="drawer-section">`, the optional Relations `{{if .Card.Relations}}` block, and the optional Body `{{if .Card.Body}}` block.
- **Comments section:** the `<div class="drawer-section" style="border-bottom:none;">` that contains the section label, the `<div id="comments-list">`, and the `<form class="comment-form">` (added by the previous feature).

- [ ] **Step 2: Rewrite the template**

Overwrite `web/templates/drawer.html` with the following. The content inside each row is the same as today — only the wrappers and the form's location have changed.

```html
{{define "drawer"}}
<div class="drawer-top">
  <div class="drawer-header">
    <div>
      <div class="drawer-card-id">#{{.Card.ID}} · {{.Card.Project}}</div>
      <div class="drawer-title">{{.Card.Title}}</div>
    </div>
    <button class="drawer-close" @click="closeDrawer()">✕</button>
  </div>

  <div class="drawer-section">
    <div class="drawer-section-label">Status</div>
    <div class="drawer-status-pills">
      <span class="status-pill active active-{{.Card.Status}}">{{.Card.Status}} ✓</span>
      {{range .StatusPills}}
      {{if .Reachable}}
      <span class="status-pill"
            hx-patch="/ui/cards/{{$.Card.ID}}/status"
            hx-vals='{"status":"{{.Status}}"}'
            hx-target="#drawer-container"
            hx-swap="innerHTML">{{.Status}}</span>
      {{else}}
      <span class="status-pill status-pill-disabled"
            title="Not allowed from {{$.Card.Status}}">{{.Status}}</span>
      {{end}}
      {{end}}
    </div>
  </div>

  <div class="drawer-section">
    <div class="drawer-meta">
      <div>
        <div class="drawer-section-label">Assignees</div>
        <div class="drawer-meta-item">{{if .Card.Assignees}}{{joinStrings .Card.Assignees ", "}}{{else}}unassigned{{end}}</div>
      </div>
      <div>
        <div class="drawer-section-label">Tags</div>
        <div class="card-tile-tags">
          {{range .Card.Tags}}
          <span class="tag" style="background:{{tagBg .}};color:{{tagColor .}};">{{.}}</span>
          {{end}}
          {{if not .Card.Tags}}<span class="drawer-meta-item">none</span>{{end}}
        </div>
      </div>
      <div>
        <div class="drawer-section-label">Created</div>
        <div class="drawer-meta-item">{{timeAgo .Card.CreatedAt}}</div>
      </div>
    </div>
  </div>

  {{if .Card.Relations}}
  <div class="drawer-section">
    <div class="drawer-section-label">Relations</div>
    {{range .Card.Relations}}
    <div class="drawer-relation">
      <span class="drawer-relation-type">{{.RelationType}}</span>
      <a class="drawer-relation-link"
         hx-get="/ui/cards/{{.RelatedCardID}}/drawer"
         hx-target="#drawer-container"
         hx-swap="innerHTML">#{{.RelatedCardID}}</a>
    </div>
    {{end}}
  </div>
  {{end}}

  {{if .Card.Body}}
  <div class="drawer-section">
    <div class="drawer-section-label">Description</div>
    <div class="drawer-body">{{.Card.Body}}</div>
  </div>
  {{end}}
</div>

<div class="drawer-comments">
  <div class="drawer-section-label drawer-comments-label">Comments ({{len .Comments}})</div>
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
</div>

<div class="drawer-composer">
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
{{end}}
```

- [ ] **Step 3: Run the structure test**

Run: `go test ./web -run TestDrawerHasThreeRowStructure -v`
Expected: PASS.

- [ ] **Step 4: Run the full web suite**

Run: `go test ./web -v`
Expected: all PASS (the prior comment tests still pass — same content, just reorganized).

- [ ] **Step 5: Commit**

```bash
git add web/templates/drawer.html
git commit -m "refactor(web): drawer template uses three-row structure"
```

---

## Task 3: Layout CSS — flex column with internal scroll

Change `.drawer` to a flex column with hidden overflow, and add the three row rules.

**Files:**
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Edit `.drawer` to remove its outer scroll and become a flex column**

Locate the existing `.drawer` block (currently around line 427) and replace it with:

```css
.drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: var(--drawer-width);
  background: var(--bg-surface);
  border-left: 1px solid var(--border);
  z-index: 201;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transform: translateX(100%);
  transition: transform 0.25s ease;
}
```

The `overflow-y: auto` is GONE — scrolling now happens inside the rows.

- [ ] **Step 2: Append the row rules**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Drawer three-row layout ===== */
.drawer-top {
  flex: 0 1 auto;
  overflow-y: auto;
}

.drawer-comments {
  flex: 1 1 0;
  min-height: 120px;
  overflow-y: auto;
  position: relative;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
}

.drawer-comments-label {
  margin-bottom: 6px;
}

.drawer-composer {
  flex: 0 0 auto;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  background: var(--bg-surface);
}

.drawer-composer .comment-form {
  /* override the margin-top introduced for the old in-section placement */
  margin-top: 0;
}
```

- [ ] **Step 3: Quick smoke check that CSS still parses**

Run: `go test ./web -run TestStaticFileServing -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/static/css/app.css
git commit -m "feat(web): drawer flex column with internal scroll"
```

---

## Task 4: Inline script — scroll-to-bottom and shadow class toggling

The drawer template gets an `<script>` at the end that runs on every HTMX swap. It scrolls `.drawer-comments` to the bottom and wires up the scroll-shadow classes.

**Files:**
- Modify: `web/templates/drawer.html`

- [ ] **Step 1: Append the script block inside `{{define "drawer"}}`**

In `web/templates/drawer.html`, just before the final `{{end}}` of the `drawer` template, append:

```html
<script>
(function() {
  var el = document.querySelector('#drawer-container .drawer-comments');
  if (!el) return;

  function update() {
    var atTop = el.scrollTop <= 0;
    var atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 1;
    el.classList.toggle('has-shadow-top', !atTop);
    el.classList.toggle('has-shadow-bottom', !atBottom);
  }

  // Default to the bottom on every render.
  el.scrollTop = el.scrollHeight;
  update();

  el.addEventListener('scroll', update, { passive: true });
})();
</script>
```

Final file structure:

```
{{define "drawer"}}
<div class="drawer-top"> ... </div>
<div class="drawer-comments"> ... </div>
<div class="drawer-composer"> ... </div>
<script> ... </script>
{{end}}
```

- [ ] **Step 2: Run the web tests**

Run: `go test ./web -v`
Expected: all PASS (the script is HTML and the tests don't assert against script content).

- [ ] **Step 3: Commit**

```bash
git add web/templates/drawer.html
git commit -m "feat(web): drawer auto-scrolls comments to bottom on render"
```

---

## Task 5: Scroll-shadow gradient CSS

Add the two pseudo-elements driven by `has-shadow-top` / `has-shadow-bottom`.

**Files:**
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Append the gradient rules**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Drawer comments scroll-shadows ===== */
.drawer-comments::before,
.drawer-comments::after {
  content: "";
  position: sticky;
  left: 0;
  right: 0;
  display: block;
  height: 24px;
  margin: 0 -20px; /* extend over the section's horizontal padding */
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.drawer-comments::before {
  top: 0;
  margin-bottom: -24px; /* don't displace content */
  background: linear-gradient(
    to bottom,
    var(--bg-surface) 0%,
    rgba(0, 0, 0, 0) 100%
  );
}

.drawer-comments::after {
  bottom: 0;
  margin-top: -24px; /* don't displace content */
  background: linear-gradient(
    to top,
    var(--bg-surface) 0%,
    rgba(0, 0, 0, 0) 100%
  );
}

.drawer-comments.has-shadow-top::before { opacity: 1; }
.drawer-comments.has-shadow-bottom::after { opacity: 1; }
```

The gradient resolves `var(--bg-surface)` automatically; in dark mode the value is `#161b22` (per `[data-theme="dark"]` at the top of app.css), so the fade matches the drawer background in both themes.

- [ ] **Step 2: Static asset test**

Run: `go test ./web -run TestStaticFileServing -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/static/css/app.css
git commit -m "feat(web): scroll-shadow fades for drawer comments"
```

---

## Task 6: Final verification

- [ ] **Step 1: Full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 2: Manual smoke check**

Start the server with a smoke DB:

```bash
rm -f /tmp/kkullm-smoke.db
go run . serve --db /tmp/kkullm-smoke.db --addr :7733 &
```

Open `http://localhost:7733/`. Create a card via the CLI or API, open its drawer, and add ~20 comments. Verify:

- Header (`#1 · project / title`) and status/meta sections stay visible while comments scroll.
- The textarea stays visible at the bottom at all times.
- On first open and after every post, the comments list is scrolled to the bottom (most recent just above the textarea).
- Scrolling up reveals older comments; a subtle fade appears at the top.
- Scrolling back down — the top fade disappears; if not at bottom, a fade appears at the bottom.
- Dark mode (if you have a toggle): fades match the dark drawer background.

Stop the server (`kill %1` or `kill <pid>`) when done.

- [ ] **Step 3: Verify clean tree**

Run: `git status`
Expected: clean (smoke DB is in /tmp, not the repo).
