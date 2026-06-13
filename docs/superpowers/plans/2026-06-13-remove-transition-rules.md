# Remove Status-Transition Rules and Force/Alt-Drag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow any card status to transition to any other status, removing the transition-rules engine and the `--force` / `?force=1` / Alt-drag machinery that existed solely to bypass it; the card-event audit log remains the record of moves.

**Architecture:** Three code tasks plus a docs task, ordered so the build and test suite stay green after every commit. Task 1 retires the web-UI Alt-drag and makes the drawer status pills all-clickable (this also removes the handler's only `CanTransition` call). Task 2 removes the transition-rules engine server-side (model, store, agent-context schema) and rewrites the rule tests; after it, any transition is allowed (`--force`/`?force=1` still parse but are redundant). Task 3 removes the now-vestigial force concept end-to-end and drops the `forced` audit column via migration. Task 4 updates the CLI skill docs.

**Tech Stack:** Go (1.26), SQLite via `modernc.org/sqlite` (no CGO), `html/template`, htmx, Alpine.js, SortableJS. Build: `task build` (or `go build ./...`). Tests: `task test` (or `go test ./...`). No JS test runner — JS changes are verified by code review plus a manual browser check.

**Design doc:** `docs/superpowers/specs/2026-06-13-remove-transition-rules-design.md`

**Conventions for every commit in this plan:**
- Commit with `git commit --no-gpg-sign`.
- End each commit message with a trailing:
  ```
  Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>
  ```
- Work only within the assigned git worktree.

---

## File Structure

| File | Task | Responsibility after change |
| --- | --- | --- |
| `web/static/js/app.js` | 1 | Drag-and-drop with no Alt/force logic; keeps blocked-modal + `_pendingDrops`. |
| `web/templates/drawer.html` | 1 | Status pills all clickable; current status keeps its `active` marker. |
| `web/static/css/app.css` | 1 | Drops the dead `.status-pill-disabled` rules. |
| `web/handlers.go` | 1, 2 | `buildStatusPills`/`statusPill` lose `Reachable` (T1); `wantsForce` + `Force` removed (T3); status-change handler unchanged otherwise. |
| `web/handlers_test.go` | 1, 2, 3 | Drawer-pill test loses disabled assertions (T1); transition/force handler tests rewritten/removed (T2/T3). |
| `model/model.go` | 2, 3 | Remove `ValidTransitions`/`CanTransition`/`AllowedTransitions` (T2); remove `CardEvent.Forced` (T3). |
| `model/model_test.go` | 2 | Remove tests referencing the transition map/funcs. |
| `store/card.go` | 2, 3 | `UpdateCard` enforces only status validity (T2); `CardUpdateParams.Force` + audit `Forced` removed (T3). |
| `store/card_test.go` | 2, 3 | Transition tests rewritten (T2); force tests removed/rewritten (T3). |
| `store/audit.go` | 3 | Drop `forced` column from INSERT/SELECT/Scan. |
| `db/migrations/005_drop_forced_column.sql` | 3 | New migration dropping the column. |
| `db/db.go` | 3 | Register migration 005. |
| `api/cards.go` | 3 | Remove `force` from the JSON body. |
| `client/client.go` | 3 | Remove `Force` from `CardUpdateRequest`. |
| `cmd/card.go` | 3 | Remove `--force` flag, its var, and the `req.Force` assignment. |
| `cmd/card_test.go` | 2, 3 | Remove force CLI test (T2); remove `cardUpdateForce` reset (T3). |
| `cmd/context.go` | 2 | Drop `status_transitions` enum; bump schema version to 3. |
| `test/e2e_test.go` | 2 | Invalid-transition assertion becomes a success assertion. |
| `plugins/kkullm/skills/cli/SKILL.md` | 4 | Remove `--force`; document "any status → any status". |

---

## Task 1: Web UI — retire Alt-drag and make status pills all-clickable

**Files:**
- Modify: `web/static/js/app.js`
- Modify: `web/templates/drawer.html:41-56`
- Modify: `web/static/css/app.css:1001-1011`
- Modify: `web/handlers.go:213-216` (the `statusPill` type) and `web/handlers.go:259-271` (`buildStatusPills`)
- Test: `web/handlers_test.go:202-216` (inside `TestDrawerHandler`)

**Context:** The board uses SortableJS for drag-and-drop. Holding Alt during a drop currently appends `?force=1` to the status PATCH to bypass transition rules; that mechanism is unreliable across browsers and is being removed entirely. The drawer renders a row of status pills: the current status as an `active` pill with a ✓, then every other status — reachable ones clickable, unreachable ones greyed with a "Not allowed from X" tooltip. After this task every non-current status is a plain clickable pill. The blocked-card modal (`askBlockMove`) and the `_pendingDrops` self-echo suppression (#69) **stay** — they are independent of rules.

Note: the server still accepts `?force=1` after this task (removed in Task 3); the UI simply stops sending it. The suite stays green.

- [ ] **Step 1: Remove the `altHeld` state field**

In `web/static/js/app.js`, delete the `altHeld: false,` line from the state block (currently line 16). Leave the surrounding `blockMove` and `_pendingDrops` lines intact.

- [ ] **Step 2: Remove the Alt key listeners from `init()`**

In `web/static/js/app.js` `init()`, delete these three lines (currently 48-50):

```javascript
      document.addEventListener('keydown', (e) => { if (e.key === 'Alt') this.altHeld = true; });
      document.addEventListener('keyup',   (e) => { if (e.key === 'Alt') this.altHeld = false; });
      window.addEventListener('blur',      ()  => { this.altHeld = false; });
```

Leave the blank line / surrounding `connectSSE()` and `htmx:afterSettle` code intact.

- [ ] **Step 3: Remove the `onStart` and `onMove` altKey capture in `initSortable()`**

In `web/static/js/app.js` `initSortable()`, the Sortable config currently has `onStart` and `onMove` callbacks that exist only to capture `altKey`. Replace the whole `new Sortable(...)` options object (currently lines 529-550) with this version (no `onStart`, no `onMove`):

```javascript
        column._sortable = new Sortable(column, {
          group: { name: 'cards', pull: true, put: true },
          animation: 200,
          ghostClass: 'sortable-ghost',
          chosenClass: 'sortable-chosen',
          onEnd: (evt) => this.onCardDrop(evt),
        });
```

- [ ] **Step 4: Remove the force logic from `onCardDrop`**

In `web/static/js/app.js` `onCardDrop`, delete the `force` comment block and variable (currently lines 603-607) and the `if (force) qs.push('force=1');` line (currently 611). The relevant region becomes:

```javascript
      const qs = [];
      if (unblock) qs.push('unblock=1');
      const url = '/ui/cards/' + cardId + '/status' + (qs.length ? '?' + qs.join('&') : '');
```

Leave everything else in `onCardDrop` unchanged — the blocked-card `askBlockMove` block above it, the `_pendingDrops` add/timeout, and the `fetch(...)` handling all stay.

- [ ] **Step 5: Make the drawer status pills all-clickable**

In `web/templates/drawer.html`, replace the pills loop (currently lines 43-55, the `{{range .StatusPills}}…{{end}}` containing the `{{if .Reachable}}…{{else}}…{{end}}`) with a single always-clickable pill per status:

```html
        {{range .StatusPills}}
        <span class="status-pill"
              hx-patch="/ui/cards/{{$.Card.ID}}/status?response=drawer{{if $.Card.Blocked}}&unblock=1{{end}}"
              hx-vals='{"status":"{{.Status}}"}'
              {{if $.Card.Blocked}}hx-confirm="This card is blocked. Unblock it as you change the status?"{{end}}
              hx-target="#drawer-container"
              hx-swap="innerHTML">{{.Status}}</span>
        {{end}}
```

Leave line 42 (the current-status `active` pill with ✓) unchanged — that is the current-status marker the design calls for.

- [ ] **Step 6: Remove `Reachable` from the `statusPill` type and `buildStatusPills`**

In `web/handlers.go`, change the type (currently lines 213-216) to drop the field:

```go
type statusPill struct {
	Status string
}
```

Then replace `buildStatusPills` (currently lines 259-271) with a version that no longer calls `model.CanTransition`:

```go
func buildStatusPills(current string) []statusPill {
	pills := make([]statusPill, 0, len(model.AllStatuses)-1)
	for _, s := range model.AllStatuses {
		if s == current {
			continue
		}
		pills = append(pills, statusPill{Status: s})
	}
	return pills
}
```

- [ ] **Step 7: Remove the dead `.status-pill-disabled` CSS**

In `web/static/css/app.css`, delete both rules (currently lines 1001-1011):

```css
.status-pill.status-pill-disabled {
  opacity: 0.35;
  cursor: not-allowed;
  border-style: dashed;
}

.status-pill.status-pill-disabled:hover {
  background: transparent;
  color: var(--text-secondary);
  border-color: var(--border);
}
```

- [ ] **Step 8: Update the `TestDrawerHandler` pill assertions**

In `web/handlers_test.go`, inside `TestDrawerHandler`, replace the block currently at lines 202-216 with a version that asserts all statuses appear as pills and there are no disabled pills:

```go
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
```

- [ ] **Step 9: Build and run the Go tests**

Run: `task build && task test`
Expected: build succeeds; all Go tests pass (`TestDrawerHandler` included).

- [ ] **Step 10: Manual browser check (record the result in the commit/PR)**

Start the server with seed data and, in a browser, drag a card between columns: it moves with no Alt needed. Open a card's drawer: every non-current status is a clickable pill, current status shows the ✓ marker. Drag a *blocked* card to another column: the 3-way modal still appears.

- [ ] **Step 11: Commit**

```bash
git add web/static/js/app.js web/templates/drawer.html web/static/css/app.css web/handlers.go web/handlers_test.go
git commit --no-gpg-sign -m "feat(web): retire alt-drag force and make status pills all-clickable

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 2: Remove the transition-rules engine (server)

**Files:**
- Modify: `model/model.go:99-139` (remove `ValidTransitions`, `CanTransition`, `AllowedTransitions`)
- Modify: `model/model_test.go` (remove tests referencing those symbols)
- Modify: `store/card.go:439-463` (`UpdateCard` validation block)
- Modify: `store/card_test.go` (rewrite transition tests)
- Modify: `web/handlers_test.go` (rewrite the no-force-rejection handler test)
- Modify: `cmd/context.go` (drop `status_transitions` enum; bump schema version)
- Modify: `cmd/card_test.go:281-314` (remove the force-bypasses-transition CLI test)
- Modify: `test/e2e_test.go:125-141`

**Context:** The transition matrix (`ValidTransitions`) and its helpers gate status changes; `UpdateCard` rejects illegal transitions unless `Force` is set. We remove the gate so any change to a *valid* status is allowed. Critically, today the only thing rejecting an unknown status (a typo) on the non-force path is `CanTransition` returning false (the status isn't a map key). After removing it we must add an explicit `model.ValidStatuses` check so typos still error (HTTP 422 via the handler). The `Force` field and the `forced` audit column stay until Task 3 — after this task they are simply redundant (force no longer bypasses anything, but a forced move is still recorded as `forced` in the trail).

- [ ] **Step 1: Rewrite the `UpdateCard` status-validation block**

In `store/card.go`, replace the validation block (currently lines 440-463, the `// Validate status transition…` comment through its closing brace) with a status-validity-only check. The new block:

```go
	// Capture the current status for the audit trail (a status_changed event
	// records from->to). Any change to a valid status is allowed; only an
	// unknown status (e.g. a typo) is rejected.
	var oldStatus string
	if p.Status != nil {
		err := s.db.QueryRow("SELECT status FROM cards WHERE id = ?", id).Scan(&oldStatus)
		if err != nil {
			return nil, fmt.Errorf("get current status for card %d: %w", id, err)
		}
		if oldStatus != *p.Status && !model.ValidStatuses[*p.Status] {
			return nil, fmt.Errorf("invalid status %q", *p.Status)
		}
	}
```

Leave the rest of `UpdateCard` unchanged, including the audit write at lines 576-587 (which still sets `Forced: p.Force` — removed in Task 3).

- [ ] **Step 2: Remove the transition map and helpers from the model**

In `model/model.go`, delete `ValidTransitions` (currently lines 99-105), `CanTransition` (121-127), and `AllowedTransitions` (129-139). Keep `ValidStatuses`, `AllStatuses`, `ValidCommentKinds`, and `ValidCommentKind`.

- [ ] **Step 3: Remove model tests that reference the deleted symbols**

In `model/model_test.go`, delete these three test functions in full: `TestBlockedRemovedFromTransitions`, `TestCannotTransitionToBlocked`, `TestCoreTransitionsStillWork`. Keep `TestBlockedRemovedFromValidStatuses` and `TestBlockedRemovedFromAllStatuses` (they test `ValidStatuses`/`AllStatuses`, which remain). Update the file's leading comment to drop the "transition map" reference:

```go
// blocked is now an orthogonal flag, not a status. It must be absent from the
// status enums entirely.
```

- [ ] **Step 4: Drop the `status_transitions` enum and bump the agent-context schema**

In `cmd/context.go`, remove the `"status_transitions": model.ValidTransitions,` line from the `Enums` map (currently line 114). Then bump the schema version and update its comment (currently lines 11-17):

```go
// agentContextSchemaVersion is bumped whenever the shape of `agent-context`
// output changes, so agents can detect incompatibilities.
//
// v3: status-transition rules removed. status_transitions is gone from enums;
// any status may move to any other (the target must still be a valid status).
// v2: blocked is an orthogonal flag, not a status. It is gone from
// card_statuses and status_transitions; `card update` gains
// --blocked/--unblocked/--reason; comments carry a kind ("block"/"unblock").
const agentContextSchemaVersion = 3
```

- [ ] **Step 5: Rewrite `TestUpdateCardInvalidTransition` as an allows-any-transition test**

In `store/card_test.go`, replace `TestUpdateCardInvalidTransition` (currently lines 461-484) with:

```go
func TestUpdateCardAllowsAnyTransition(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Test card",
		ProjectID: proj.ID,
		Status:    "considering",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	// considering -> completed was previously illegal; it is now allowed.
	updated, err := s.UpdateCard(card.ID, CardUpdateParams{Status: strPtr("completed")})
	if err != nil {
		t.Fatalf("UpdateCard considering -> completed: %v", err)
	}
	if updated.Status != "completed" {
		t.Errorf("status = %q, want %q", updated.Status, "completed")
	}

	// The move is recorded in the audit trail.
	events, err := s.ListCardEvents(card.ID)
	if err != nil {
		t.Fatalf("ListCardEvents: %v", err)
	}
	var sawStatusChange bool
	for _, e := range events {
		if e.EventType == "status_changed" && e.ToValue == "completed" {
			sawStatusChange = true
		}
	}
	if !sawStatusChange {
		t.Error("expected a status_changed event recording the move to completed")
	}
}
```

- [ ] **Step 6: Remove the force CLI test that asserts the rejected transition**

In `cmd/card_test.go`, delete `TestCardUpdateForceBypassesTransition` in full (currently lines 281-314). Its premise — that a transition is rejected without `--force` — no longer holds. (The `--force` flag and its reset in `resetCardFlags` are removed in Task 3.)

- [ ] **Step 7: Rewrite the web handler no-force test as an allows-any-transition test**

In `web/handlers_test.go`, replace `TestStatusChangeWithoutForceStillRejects` (currently lines 385-413) with two assertions — a previously-illegal transition now succeeds, and an unknown status is still rejected with 422:

```go
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
```

Leave `TestStatusChangeForce` (lines 338-383) untouched in this task — it exercises `?force=1`, which still works here (force is removed in Task 3).

- [ ] **Step 8: Rewrite the e2e invalid-transition assertion as a success**

In `test/e2e_test.go`, replace the block at lines 125-141 (the `// 11. Verify invalid transition is rejected` section) with one that confirms the previously-illegal move now succeeds:

```go
	// 11. Any transition is now allowed (no transition rules).
	// completed -> in_flight
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status})
	if err != nil {
		t.Fatalf("completed -> in_flight: %v", err)
	}
	// in_flight -> tabled
	tabled := "tabled"
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &tabled})
	if err != nil {
		t.Fatalf("in_flight -> tabled: %v", err)
	}
	// tabled -> completed was previously rejected; it is now allowed.
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &completed})
	if err != nil {
		t.Fatalf("tabled -> completed should now be allowed: %v", err)
	}
```

- [ ] **Step 9: Build and run the full test suite**

Run: `task build && task test`
Expected: build succeeds; all tests pass. (`TestStatusChangeForce`, `TestUpdateCardForceBypassesTransition` in `store/card_test.go`, `TestUpdateCardForceStillRejectsInvalidStatus`, and `TestUpdateCardLegalTransitionNotForced` still pass here — they are addressed in Task 3.)

- [ ] **Step 10: Sanity-check the agent-context output**

Run: `go run . agent-context | grep -E 'schema_version|status_transitions'`
Expected: `"schema_version": 3` is present and there is no `status_transitions` key.

- [ ] **Step 11: Commit**

```bash
git add model/model.go model/model_test.go store/card.go store/card_test.go web/handlers_test.go cmd/context.go cmd/card_test.go test/e2e_test.go
git commit --no-gpg-sign -m "feat: remove status-transition rules; allow any status change

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 3: Remove the force concept and drop the `forced` audit column

**Files:**
- Modify: `web/handlers.go:335-341` (remove `wantsForce`) and `web/handlers.go:379` (remove `Force:` from params)
- Modify: `web/handlers_test.go:338-383` (remove `TestStatusChangeForce`)
- Modify: `api/cards.go:117-145` (remove `force` from the JSON body)
- Modify: `client/client.go:142-153` (remove `Force` from `CardUpdateRequest`)
- Modify: `cmd/card.go` (remove `--force` flag, `cardUpdateForce` var, `req.Force` block)
- Modify: `cmd/card_test.go:98-102` (remove `cardUpdateForce` reset)
- Modify: `model/model.go:66` (remove `CardEvent.Forced`)
- Modify: `store/card.go:56-60` (remove `Force` from `CardUpdateParams`) and `store/card.go:583` (drop `Forced:`)
- Modify: `store/audit.go` (drop `forced` from INSERT/SELECT/Scan)
- Modify: `store/audit_test.go:33-35,101-103` (remove `.Forced` assertions)
- Modify: `store/card_test.go` (remove/rewrite force tests)
- Create: `db/migrations/005_drop_forced_column.sql`
- Modify: `db/db.go:37` (register migration 005)

**Context:** After Task 2 the `Force` field no longer affects behavior; it only flows to the `forced` audit column. This task removes the force concept everywhere and drops the column. SQLite (via `modernc.org/sqlite`, recent) supports `ALTER TABLE ... DROP COLUMN`. `card_events` is append-only with one index, so the table rebuild DROP COLUMN performs is trivial.

- [ ] **Step 1: Add the migration that drops the column**

Create `db/migrations/005_drop_forced_column.sql`:

```sql
-- Transition rules and the force-move feature were removed; no status change is
-- "forced" anymore. Drop the now-unused column from the audit trail.
ALTER TABLE card_events DROP COLUMN forced;
```

- [ ] **Step 2: Register the migration**

In `db/db.go`, add `"migrations/005_drop_forced_column.sql"` to the end of the migration name slice (currently line 37):

```go
	for _, name := range []string{"migrations/001_initial.sql", "migrations/002_comments_author_snapshot.sql", "migrations/003_blocked_flag.sql", "migrations/004_card_audit.sql", "migrations/005_drop_forced_column.sql"} {
```

- [ ] **Step 3: Remove `forced` from `store/audit.go`**

In `store/audit.go`:
- In `appendCardEvent`, delete the `forced := 0 / if e.Forced { forced = 1 }` block (lines 33-36) and change the INSERT to omit the column and its binding:

```go
	_, err := q.Exec(
		"INSERT INTO card_events (card_id, actor, event_type, from_value, to_value) VALUES (?, ?, ?, ?, ?)",
		e.CardID, e.Actor, e.EventType, e.FromValue, e.ToValue,
	)
```

- In `getCardEvent`, remove `var forced int`, drop `forced` from the SELECT column list, drop `&forced` from the `Scan`, and delete `e.Forced = forced != 0`. Result:

```go
func (s *Store) getCardEvent(id int) (*model.CardEvent, error) {
	e := &model.CardEvent{}
	var from, to sql.NullString
	err := s.db.QueryRow(`
		SELECT id, card_id, actor, event_type, from_value, to_value, created_at
		FROM card_events WHERE id = ?
	`, id).Scan(&e.ID, &e.CardID, &e.Actor, &e.EventType, &from, &to, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get card event %d: %w", id, err)
	}
	e.FromValue = from.String
	e.ToValue = to.String
	return e, nil
}
```

- In `ListCardEvents`, make the same edits to the SELECT, the per-row `var forced int`, the `Scan`, and remove `e.Forced = forced != 0`:

```go
func (s *Store) ListCardEvents(cardID int) ([]model.CardEvent, error) {
	rows, err := s.db.Query(`
		SELECT id, card_id, actor, event_type, from_value, to_value, created_at
		FROM card_events WHERE card_id = ?
		ORDER BY id ASC
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("list card events: %w", err)
	}
	defer rows.Close()

	var events []model.CardEvent
	for rows.Next() {
		var e model.CardEvent
		var from, to sql.NullString
		if err := rows.Scan(&e.ID, &e.CardID, &e.Actor, &e.EventType, &from, &to, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan card event: %w", err)
		}
		e.FromValue = from.String
		e.ToValue = to.String
		events = append(events, e)
	}
	return events, rows.Err()
}
```

- [ ] **Step 3b: Remove the `.Forced` assertions from `store/audit_test.go`**

In `store/audit_test.go`, delete the two assertion blocks that reference the
removed field:
- The `if statusEv.Forced { t.Error("forced should default false") }` block (currently lines 33-35).
- The `if e.Forced { t.Error("forced should be false") }` block (currently lines 101-103).

Leave the surrounding event-creation and other assertions intact.

- [ ] **Step 4: Remove `Forced` from the `CardEvent` model**

In `model/model.go`, delete the `Forced    bool      \`json:"forced"\`` field from the `CardEvent` struct (currently line 66).

- [ ] **Step 5: Remove `Force` from the store params and audit write**

In `store/card.go`:
- Delete the `Force` field and its doc comment from `CardUpdateParams` (currently the `// Force bypasses…` comment at lines 56-59 and the `Force bool` field at line 60). The struct keeps `Title`, `Body`, `Status`, `Blocked`, `Assignees`, `Tags`, `Relations`, `Actor`.
- In the status audit write (currently lines 577-584), delete the `Forced: p.Force,` line so it reads:

```go
		if err := appendCardEvent(tx, model.CardEvent{
			CardID:    id,
			Actor:     p.Actor,
			EventType: "status_changed",
			FromValue: oldStatus,
			ToValue:   *p.Status,
		}); err != nil {
			return nil, err
		}
```

- [ ] **Step 6: Remove `force` from the API handler**

In `api/cards.go` `updateCard`, delete the `Force     bool                 \`json:"force"\`` line from the request `body` struct (currently line 125) and the `Force:     body.Force,` line from the `store.CardUpdateParams{…}` literal (currently line 144).

- [ ] **Step 7: Remove `Force` from the client request type**

In `client/client.go`, delete the `Force` field and its two-line comment from `CardUpdateRequest` (currently lines 150-152). The struct keeps `Title`, `Body`, `Status`, `Blocked`, `Assignees`, `Tags`, `Relations`.

- [ ] **Step 8: Remove the `--force` flag from the CLI**

In `cmd/card.go`:
- Delete `cardUpdateForce        bool` from the var block (currently line 201).
- Delete the `// --force bypasses…` comment and the `if cardUpdateForce { req.Force = true }` block (currently lines 282-286).
- Delete the flag registration `cardUpdateCmd.Flags().BoolVar(&cardUpdateForce, "force", …)` (currently line 413).

- [ ] **Step 9: Remove the `cardUpdateForce` reset from the test helper**

In `cmd/card_test.go` `resetCardFlags`, delete the `cardUpdateForce = false` line (currently line 101).

- [ ] **Step 10: Remove/rewrite the remaining force tests in the store**

In `store/card_test.go`:
- Delete `TestUpdateCardForceBypassesTransition` in full (currently lines 486-530).
- Delete `TestUpdateCardLegalTransitionNotForced` in full (currently lines 551-584) — it asserts on the removed `Forced` field.
- Replace `TestUpdateCardForceStillRejectsInvalidStatus` (currently lines 532-549) with a force-free invalid-status test:

```go
func TestUpdateCardRejectsInvalidStatus(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Invalid status",
		ProjectID: proj.ID,
		Status:    "considering",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	// An unknown status is rejected even though any real transition is allowed.
	if _, err := s.UpdateCard(card.ID, CardUpdateParams{Status: strPtr("bogus")}); err == nil {
		t.Fatal("expected error for invalid status 'bogus'")
	}
}
```

- [ ] **Step 11: Remove the web handler force test**

In `web/handlers_test.go`, delete `TestStatusChangeForce` in full (currently lines 338-383). The any-transition success case is already covered by `TestStatusChangeAllowsAnyTransition` from Task 2.

- [ ] **Step 12: Remove `wantsForce` and the `Force` param from the web handler**

In `web/handlers.go`:
- Delete the `wantsForce` function and its doc comment (currently lines 335-341).
- In `handleStatusChange`, change the params construction (currently line 379) to drop `Force`:

```go
	params := store.CardUpdateParams{Status: &newStatus, Actor: webOperator}
```

- [ ] **Step 13: Build and run the full test suite**

Run: `task build && task test`
Expected: build succeeds; all tests pass. No remaining references to `Force`, `Forced`, `wantsForce`, `force=1`, or `--force`.

- [ ] **Step 14: Verify the migration applies on a fresh DB and the column is gone**

The `db` and `store` test packages each open an in-memory DB and run every
migration (001–005) before their tests, so a malformed migration 005 or an
`audit.go` SELECT that no longer matches the schema makes them fail. Run:

Run: `go test ./db/... ./store/...`
Expected: PASS (this exercises migration 005 on a fresh schema).

Optional direct inspection (the `--db` flag lives on `serve`): create a
migrated file DB by starting the server briefly, then inspect it:
```bash
go run . serve --db /tmp/kkullm-mig.db & SRV=$!; sleep 2; kill $SRV
sqlite3 /tmp/kkullm-mig.db 'PRAGMA table_info(card_events);' | grep -c forced
```
Expected: prints `0`. Skip this block if `sqlite3` is unavailable — the test run above is authoritative.

- [ ] **Step 15: Grep for stragglers**

Run: `grep -rn "Forced\|wantsForce\|force=1\|--force\|cardUpdateForce\|ValidTransitions\|CanTransition\|AllowedTransitions" --include=*.go --include=*.js --include=*.html .`
Expected: no matches in source (matches only in `docs/` are acceptable and handled in Task 4).

- [ ] **Step 16: Commit**

```bash
git add db/migrations/005_drop_forced_column.sql db/db.go store/audit.go store/audit_test.go store/card.go store/card_test.go model/model.go api/cards.go client/client.go cmd/card.go cmd/card_test.go web/handlers.go web/handlers_test.go
git commit --no-gpg-sign -m "feat: remove force-move feature and drop forced audit column

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 4: Update the CLI skill documentation

**Files:**
- Modify: `plugins/kkullm/skills/cli/SKILL.md:57-112`

**Context:** `CLAUDE.md` requires the `/kkullm:cli` skill to stay in sync with CLI behavior. The lifecycle section currently says transitions are validated and documents `--force`; both must change.

- [ ] **Step 1: Reword the lifecycle "transitions are validated" paragraph**

In `plugins/kkullm/skills/cli/SKILL.md`, replace the paragraph currently at lines 72-74:

```
Transitions are validated server-side — an illegal jump is rejected with a
teaching error. For the exact status set and the full transition map, read the
`enums` section of `kkullm agent-context` rather than memorizing it.
```

with:

```
A card may move from any status to any other status — there are no transition
rules. The only check is that the target is a real status; an unknown status is
rejected. For the exact status set, read the `enums` section of
`kkullm agent-context` rather than memorizing it.
```

- [ ] **Step 2: Remove the `--force` paragraph**

In `plugins/kkullm/skills/cli/SKILL.md`, delete the paragraph and example currently at lines 108-112:

```
The status transitions are guardrails, not a cage. If you need to move a card
to a status the matrix would normally reject, add `--force` — it bypasses the
transition rule (the target must still be a real status) and the move is
recorded as *forced* in the card's audit trail:
`kkullm card update <id> --status completed --force --as <agent>`
```

Leave the blocked-flag paragraph (lines 102-106) and the audit-trail section heading (line 114) intact. If removing the paragraph leaves a doubled blank line before `### Reading a card's audit trail`, collapse it to a single blank line.

- [ ] **Step 3: Verify no other `--force`/transition references remain in the skill**

Run: `grep -n "force\|transition" plugins/kkullm/skills/cli/SKILL.md`
Expected: no remaining references to `--force` or to transition *validation* (a passing mention of statuses "flowing" in the lifecycle diagram is fine).

- [ ] **Step 4: Commit**

```bash
git add plugins/kkullm/skills/cli/SKILL.md
git commit --no-gpg-sign -m "docs(cli): drop transition rules and --force from the CLI skill

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Final verification (after all tasks)

- [ ] `task build && task test` — clean build, all tests pass.
- [ ] `go vet ./...` and `gofmt -l .` — no vet issues, no unformatted files (CI runs these).
- [ ] `grep -rn "Forced\|wantsForce\|force=1\|--force\|ValidTransitions\|CanTransition\|AllowedTransitions" --include=*.go --include=*.js --include=*.html .` — no source matches.
- [ ] Manual browser pass: drag any card across any columns (always moves, no Alt); drawer pills all clickable; blocked-card drag still prompts the 3-way modal; block/unblock forms still work.
