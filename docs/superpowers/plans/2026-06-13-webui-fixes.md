# Web UI Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix seven web-UI bugs/polish items (GitHub #56–#62) found during manual testing of the roadmap work: drag-revert position loss, Alt-force in Chrome/Safari, a 3-way blocked-move dialog, the stale Blocked view after unblock, the project selector showing "All projects" on load, and cramped/constrained drawer forms.

**Architecture:** All seven fixes are client-side (JavaScript / HTML templates / CSS) with **no server contract changes** — the existing API/handlers already support status changes on blocked cards, `?unblock=1`, and `?force=1`. Because `web/static/js/app.js` is the shared hot file for five of the seven issues, tasks are **serialized** and each lands as its own PR merged to `main` before the next starts, keeping `app.js` current for every subagent and eliminating merge collisions. Task 5 (CSS/templates only) is file-disjoint from the JS work and goes last.

**Tech Stack:** Go `html/template` server-rendered views, htmx, Alpine.js, SortableJS, SSE. No JS test runner exists in this repo — JS behaviour is verified by careful code review plus the explicit manual browser steps in each task. CI (`task build` / `task test` / `go vet` / `gofmt -l`) runs on every PR and will pass trivially since no Go changes.

**Execution conventions (per task):**
- Work only inside the task's worktree: `git worktree add ../kkullm-issue-NN -b issue-NN-slug main`.
- Commit with `git commit --no-gpg-sign` (signing fails non-interactively); trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- Open the PR with `gh pr create --body-file /tmp/prNN.md` (shell is fish — never inline multi-line strings). PR body ends with `Closes #NN` + Claude Code attribution.
- The orchestrator reviews CI + diff, merges with `gh pr merge NN --rebase --delete-branch`, then `git fetch origin && git reset --hard origin/main`, removes the worktree, and deletes the local branch before starting the next task.

**Manual verification setup (used by Tasks 1–4):**
```bash
task build && ./kkullm serve --addr :7799 --db /tmp/webui-verify.db
# in another shell, seed demo data so there are blocked cards + multiple projects:
KKULLM_SERVER=http://localhost:7799 KKULLM_DB=/tmp/webui-verify.db bash scripts/dev-seed.sh
# then open http://localhost:7799 in Chrome, Safari, and Firefox.
```

---

## File Structure

| File | Responsibility | Touched by |
|------|----------------|------------|
| `web/static/js/app.js` | Alpine component: drag/drop (`onCardDrop`, `initSortable`), SSE handlers (`handleCardUpdated`), bootstrap (`bootstrapData`, `init`) | Tasks 1, 2, 3, 4 |
| `web/templates/layout.html` | Nav shell, project `<select>`, board-container load trigger, modal mount points | Tasks 2, 4 |
| `web/static/css/app.css` | All styling; drawer form rules (`.drawer-unblock-form`, `.drawer-block-form`, `.drawer-edit-form`), modal styles | Tasks 2, 5 |
| `web/templates/drawer.html` | Card drawer: edit form, block/unblock forms | Task 5 |

---

## Task 1: Drag revert to original position + Alt-force in all browsers (#61, #62)

**Files:**
- Modify: `web/static/js/app.js` — `onCardDrop` (lines ~520–575), `initSortable` (lines ~506–518)

**Background:** When a drag is cancelled (blocked-card confirm declined, `app.js:536`) or rejected by the server (`app.js:557`), the code does `evt.from.appendChild(cardEl)`, which moves the tile to the **bottom** of its original column instead of its original slot (#61, and the cancel path of #60's predecessor). Separately, `onCardDrop` reads `evt.originalEvent.altKey` (`app.js:544`) to detect a force-move; in Chrome/Safari the event SortableJS surfaces at drop time doesn't carry the modifier state, so Alt-force only works in Firefox (#62).

- [ ] **Step 1: Capture the original index before any revert path**

In `onCardDrop`, SortableJS provides `evt.oldDraggableIndex` (the dragged item's index among draggable siblings in `evt.from`, before the move). Add a single reusable revert helper that restores the card to that index instead of appending:

```javascript
// Restore a dropped card to its pre-drag position in the original column.
// SortableJS has already moved the DOM node to evt.to by the time onEnd
// fires, so appendChild(cardEl) would drop it at the bottom. Re-insert at
// the original index instead.
revertDrag(evt) {
  const cardEl = evt.item;
  const siblings = evt.from.children;
  const idx = evt.oldDraggableIndex;
  if (idx == null || idx >= siblings.length) {
    evt.from.appendChild(cardEl);
  } else {
    evt.from.insertBefore(cardEl, siblings[idx]);
  }
}
```

Replace **both** `evt.from.appendChild(cardEl);` calls in `onCardDrop` (the cancel path ~line 536 and the `!resp.ok` error path ~line 557) with `this.revertDrag(evt);`.

- [ ] **Step 2: Capture Alt-key state reliably across browsers**

Reading `altKey` at drop time is unreliable in Chrome/Safari. Instead, track the live modifier state on the Alpine component and read it in `onCardDrop`. Add a field and wire `keydown`/`keyup` listeners in `initSortable` (or `init`), and have SortableJS's `onStart` snapshot it too:

```javascript
// in the returned object's state block, add:
altHeld: false,
```

In `init()` (after `this.connectSSE();`), add document listeners so the flag tracks the physical key regardless of focus:

```javascript
document.addEventListener('keydown', (e) => { if (e.key === 'Alt') this.altHeld = true; });
document.addEventListener('keyup',   (e) => { if (e.key === 'Alt') this.altHeld = false; });
window.addEventListener('blur',      ()  => { this.altHeld = false; });
```

In `initSortable`, also snapshot the modifier at drag start from the pointer event, which is reliable cross-browser at `onStart`:

```javascript
column._sortable = new Sortable(column, {
  group: { name: 'cards', pull: true, put: true },
  animation: 200,
  ghostClass: 'sortable-ghost',
  chosenClass: 'sortable-chosen',
  onStart: (evt) => {
    const oe = evt.originalEvent;
    if (oe && typeof oe.altKey === 'boolean') this.altHeld = oe.altKey;
  },
  onEnd: (evt) => this.onCardDrop(evt),
});
```

In `onCardDrop`, replace:

```javascript
const orig = evt.originalEvent;
const force = !!(orig && orig.altKey);
```

with:

```javascript
// altHeld is tracked via document keydown/keyup + snapshotted at drag
// start (onStart), because evt.originalEvent.altKey is unreliable at drop
// time in Chrome/Safari (it works in Firefox). See #62.
const force = this.altHeld;
```

- [ ] **Step 3: Manual verification**

Start the server and seed data (see "Manual verification setup"). In **each of Chrome, Safari, and Firefox**:
1. **#61:** Drag a non-blocked card from a middle position in a column to an *illegal* status column. Expect: error toast appears, and the card returns to its **original slot** (not the bottom). Reload to confirm DOM matches.
2. **#62:** Hold **Alt** and drag a card across a normally-illegal transition. Expect: the move **succeeds** (force) in all three browsers and persists after reload.
3. Regression: a normal legal drag still works; cancelling a blocked-card drag (decline the confirm) returns the card to its original slot in all browsers.

- [ ] **Step 4: Commit**

```bash
git add web/static/js/app.js
git commit --no-gpg-sign -m "fix(web): restore drag position on revert; reliable Alt-force across browsers

Closes #61
Closes #62

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 2: Three-way blocked-move dialog (#60)

**Files:**
- Modify: `web/static/js/app.js` — `onCardDrop` (blocked-card branch), add a small modal-driven prompt + state
- Modify: `web/templates/layout.html` — add the modal markup near the other modals (after the compose modal include, ~line 130)
- Modify: `web/static/css/app.css` — modal styling (reuse existing modal/overlay tokens)

**Background:** Today a blocked card dragged to a new column uses `window.confirm` (2 buttons): confirm → unblock + move (`?unblock=1`), cancel → revert (#60's current behaviour, now reverting correctly thanks to Task 1). The native confirm can't offer a third option. We want **three** choices: *Unblock and move* / *Move but keep blocked* / *Cancel*. "Move but keep blocked" PATCHes the status **without** `?unblock=1`, leaving the flag set — the server already permits a status change on a blocked card because `blocked` is orthogonal to status. Force (Alt) remains orthogonal and applies to whichever move proceeds.

- [ ] **Step 1: Add modal markup to `layout.html`**

After the compose modal include (`{{template "compose" .}}`, ~line 130), add a blocked-move modal driven by Alpine state on the root component:

```html
  <!-- Blocked-move decision modal -->
  <div class="modal-overlay" x-show="blockMove.open" x-cloak
       @click="resolveBlockMove(null)"></div>
  <div class="modal blockmove-modal" x-show="blockMove.open" x-cloak role="dialog" aria-modal="true">
    <div class="modal-title">Move a blocked card</div>
    <p class="modal-body" x-text="blockMove.message"></p>
    <div class="modal-actions blockmove-actions">
      <button type="button" class="btn-primary" @click="resolveBlockMove('unblock')">Unblock and move</button>
      <button type="button" class="btn-secondary" @click="resolveBlockMove('keep')">Move but keep blocked</button>
      <button type="button" class="link-cancel" @click="resolveBlockMove(null)">Cancel</button>
    </div>
  </div>
```

(Match the existing modal class names actually present in `app.css`/compose markup — inspect the compose modal in `layout.html` and reuse its overlay/container classes rather than inventing new ones. The names above are indicative; align them to what exists.)

- [ ] **Step 2: Add modal state + a promise-based resolver to `app.js`**

Add to the state block:

```javascript
blockMove: { open: false, message: '', _resolve: null },
```

Add methods:

```javascript
// Opens the 3-way blocked-move modal and resolves to 'unblock', 'keep',
// or null (cancel). Promise-based so onCardDrop can await the choice.
askBlockMove(newStatus) {
  this.blockMove.message =
    'This card is blocked. Move it to ' + newStatus + '?';
  this.blockMove.open = true;
  return new Promise((resolve) => { this.blockMove._resolve = resolve; });
},

resolveBlockMove(choice) {
  this.blockMove.open = false;
  const r = this.blockMove._resolve;
  this.blockMove._resolve = null;
  if (r) r(choice);
},
```

- [ ] **Step 3: Rewire the blocked branch of `onCardDrop` to use the modal**

`onCardDrop` is currently synchronous. Convert it to `async` and replace the `window.confirm` block (lines ~531–539) with the 3-way flow. The resulting structure:

```javascript
async onCardDrop(evt) {
  const cardEl = evt.item;
  const cardId = cardEl.dataset.cardId;
  const newStatus = evt.to.dataset.status;
  const oldStatus = evt.from.dataset.status;

  if (newStatus === oldStatus) return;

  let unblock = false;
  if (cardEl.dataset.blocked === 'true') {
    const choice = await this.askBlockMove(newStatus);
    if (choice === null) { this.revertDrag(evt); return; }
    if (choice === 'unblock') unblock = true;
    // choice === 'keep' → proceed with the move, leave blocked flag set
  }

  const force = this.altHeld;
  const qs = [];
  if (unblock) qs.push('unblock=1');
  if (force) qs.push('force=1');
  // ... unchanged fetch/PATCH + revert-on-error (this.revertDrag(evt)) ...
}
```

Keep the rest of the fetch/response handling from Task 1 intact (including `this.revertDrag(evt)` on `!resp.ok`).

- [ ] **Step 4: Style the modal in `app.css`**

Reuse the existing modal overlay/container tokens. Add only what's needed for the three stacked/inline action buttons, e.g.:

```css
.blockmove-modal .blockmove-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}
```

(If `.modal`, `.modal-overlay`, `.modal-title`, `.modal-actions`, `.btn-primary`, `.btn-secondary` don't already exist with these names, reuse the compose modal's actual classes — do not introduce a parallel modal system.)

- [ ] **Step 5: Manual verification**

Start server + seed. With a blocked card:
1. Drag it to another column → modal shows three buttons.
2. **Unblock and move:** card moves, badge clears, board reflects unblocked state (a `card_updated` SSE refreshes the tile).
3. **Move but keep blocked:** card moves to the new column and **keeps** its blocked badge.
4. **Cancel:** card returns to its original slot (Task 1 revert), still blocked.
5. Alt+drag a blocked card across an illegal transition, choose "Move but keep blocked" → forced move succeeds, stays blocked.
6. Escape/overlay click cancels the modal (acts as Cancel).

- [ ] **Step 6: Commit**

```bash
git add web/static/js/app.js web/templates/layout.html web/static/css/app.css
git commit --no-gpg-sign -m "feat(web): 3-way dialog when dragging a blocked card

Unblock-and-move, move-but-keep-blocked, or cancel.

Closes #60

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 3: Blocked view updates after unblock (#56)

**Files:**
- Modify: `web/static/js/app.js` — `handleCardUpdated` (lines ~669–712)

**Background:** The orchestrator Blocked view (`/ui/blocked`) is loaded into `#board-container` via htmx but is **not** a tracked Alpine `viewMode`. When a card is unblocked from the drawer, the `card_updated` SSE fires `handleCardUpdated`, which on a block-state change calls `loadBoard()` (`app.js:687`) — that reloads the *regular* project board, not `/ui/blocked`, so the Blocked view shows a stale, still-badged card. Desired behaviour (from #56): **keep** the card in the Blocked list but **drop its blocked badge in place**; it stays listed until the Blocked view is next loaded from the server.

- [ ] **Step 1: Detect the Blocked view and handle it in place**

The Blocked view's fragment sets `inArchive = true` (see `blocked_view.html`), and its header has class `.archived-title` reading "Blocked". A robust signal that the Blocked view is mounted: `#board-container` contains an element rendered only by `blocked_view.html`. Add a marker to rely on rather than scraping text — in `web/templates/blocked_view.html`, the root `.archived-view` is shared with the archive, so add a distinguishing attribute. **This task touches one template line** in addition to `app.js`:

In `web/templates/blocked_view.html`, change the opening view wrapper to carry a marker:
```html
<div class="archived-view" data-view="blocked">
```

In `handleCardUpdated`, before the existing block-state branch (the `wasBlocked !== !!card.blocked` check ~line 686), special-case the Blocked view:

```javascript
const blockedView = document.querySelector('#board-container [data-view="blocked"]');
const wasBlocked = cardEl.dataset.blocked === 'true';

if (blockedView && wasBlocked && !card.blocked) {
  // On the orchestrator Blocked view: keep the card listed but clear its
  // blocked affordances in place. It stays until the view is reloaded. (#56)
  cardEl.removeAttribute('data-blocked');
  cardEl.classList.remove('card-tile-blocked');
  const badge = cardEl.querySelector('.card-blocked-badge');
  if (badge) badge.remove();
  const reason = cardEl.closest('.blocked-card-wrap')?.querySelector('.blocked-reason');
  if (reason) reason.remove();
  cardEl.classList.add('highlight');
  setTimeout(() => cardEl.classList.remove('highlight'), 1500);
  if (this.drawerOpen && this.drawerCardId === card.id) {
    htmx.ajax('GET', '/ui/cards/' + card.id + '/drawer', {
      target: '#drawer-container', swap: 'innerHTML',
    });
  }
  return;
}
```

Leave the existing non-blocked-view behaviour (the `loadBoard()` branch) unchanged for the normal board.

- [ ] **Step 2: Manual verification**

Start server + seed. Hamburger → **Blocked**:
1. Click a blocked card → drawer opens.
2. Enter a reason, click **Unblock**, close the drawer.
3. Expect: the card **stays in the Blocked list** but its "Blocked" badge and the block-reason footer are **gone**, and it briefly highlights.
4. Reload the Blocked view (hamburger → Blocked again) → the unblocked card is now **absent** from the list (server-side filter).
5. Regression: unblocking from the *regular* board still clears the badge there (existing `loadBoard()` path).

- [ ] **Step 3: Commit**

```bash
git add web/static/js/app.js web/templates/blocked_view.html
git commit --no-gpg-sign -m "fix(web): clear blocked badge in place on the Blocked view after unblock

Closes #56

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 4: Project selector reflects the loaded project on initial load (#59)

**Files:**
- Modify: `web/static/js/app.js` — `bootstrapData` / `init` (lines ~32–143)

**Background:** On first paint the board loads the default project (the `#board-container` has `hx-get="/ui/board?project={{.DefaultProjectID}}"`), and `bootstrapData()` sets `this.currentProject = String(data.defaultProjectId)`. But the nav `<select>` uses `x-model="currentProject"` while its real `<option>`s are generated by `x-for="p in projects"`. Alpine binds `x-model` before the `x-for` options exist, so the value can't match and the select falls back to its first static option, **"All projects"** (`value="all"`). Internally `currentProject` holds the real id, so the displayed "all" is already the select's DOM value — re-selecting it fires **no** `change` event, which is why clicking "All projects" appears to do nothing (#59). You only get a working "all" after switching to another project first (which produces a real change event).

- [ ] **Step 1: Defer seeding `currentProject` until after the options render**

The fix is to let the `x-for` options exist before assigning the value Alpine's `x-model` should select. In `bootstrapData()`, parse and store `projects`/`agents`/default as today, but **do not** set `currentProject` synchronously from `defaultProjectId`; instead capture it and apply it on the next tick. Concretely:

In `bootstrapData()`, replace:

```javascript
if (data.defaultProjectId) {
  this.currentProject = String(data.defaultProjectId);
}
```

with:

```javascript
// Defer seeding currentProject until after Alpine renders the project
// <option>s (x-for). Setting it synchronously here runs before the
// options exist, so x-model can't match and the <select> sticks on its
// first static option ("All projects") even though state holds the id. (#59)
this._defaultProjectId = data.defaultProjectId
  ? String(data.defaultProjectId) : null;
```

Then in `init()`, after `this.bootstrapData();`, add:

```javascript
// Apply the default project after the x-for options have rendered so the
// nav <select> visibly reflects it (see bootstrapData / #59).
this.$nextTick(() => {
  if (this._defaultProjectId) this.currentProject = this._defaultProjectId;
});
```

Note `currentAgent` seeding in `bootstrapData` can stay as-is (the agent select has the same pattern but isn't the reported bug; if the agent selector shows the same symptom, apply the identical `$nextTick` deferral for `currentAgent` — verify in Step 2).

- [ ] **Step 2: Manual verification**

Start server + seed (multiple projects exist). Reload the site:
1. The nav project `<select>` shows the **default project's name** (e.g. `ant_hill`), matching the cards on the board — not "All projects".
2. Open the dropdown and choose **All projects** → board now shows cards from every project (a real `change` fires).
3. Choose a specific project again → board scopes to it; selector label matches.
4. Switch to **Agent** view and back to **Project** → selector still correct.

- [ ] **Step 3: Commit**

```bash
git add web/static/js/app.js
git commit --no-gpg-sign -m "fix(web): nav selector reflects the loaded project on initial load

Closes #59

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Task 5: Drawer form styling — full-width edit form + consistent block/unblock controls (#57, #58)

**Files:**
- Modify: `web/templates/drawer.html` — move the edit form out of the title row so it spans full width (#58)
- Modify: `web/static/css/app.css` — `.drawer-block-form` / `.drawer-unblock-form` rules (~lines 842–855) to match site form styling (#57)

**Background:**
- **#58:** The edit form lives inside `.drawer-header-main`, sharing the header row with the `✕` close button (`drawer.html:34`), so it's constrained narrower than the comment edit form. It should span the full drawer width like `.drawer-edit-body`/comment forms already do.
- **#57:** `.drawer-unblock-form` / `.drawer-block-form` use bare `<input>`/`<button>` laid out inline (`flex` + `flex-wrap`), so the text input and blue button render small and cramped, and inconsistently across browsers. They should reuse the same input styling as `.drawer-edit-title`/`.drawer-edit-body` and the same button styling as `.drawer-edit-actions button[type="submit"]`, laid out as a full-width stacked form.

- [ ] **Step 1 (#58): Make the edit form full-width in `drawer.html`**

The edit `<form x-show="editing">` currently sits inside `.drawer-header-main` (sharing the row that the `✕` button constrains). Restructure the header so that in **edit mode** the form spans the full header width. Minimal approach: when `editing` is true, hide the close button's column influence by rendering the edit form as a block below the header row rather than beside the close button.

In `drawer.html`, wrap the title row and edit form so the form is a full-width sibling that is not constrained by the close button. Replace the `.drawer-header-main` inner content (lines ~11–33) structure so the `<form class="drawer-edit-form">` is **not** boxed beside `.drawer-close`. Concretely, move the close button so it only shares the row with the (non-edit) title view, and let the edit form occupy full width:

```html
  <div class="drawer-header-main">
    <div class="drawer-card-id">#{{.Card.ID}} · {{.Card.Project}}</div>
    <div x-show="!editing" class="drawer-title-row">
      <div class="drawer-title kk-md kk-md-title">{{renderTitle .Card.Title}}</div>
      <button type="button" class="drawer-edit-toggle" @click="editing = true" title="Edit title and description">Edit</button>
    </div>
    <form x-show="editing" x-cloak class="drawer-edit-form"
          hx-post="/ui/cards/{{.Card.ID}}/edit"
          hx-target="#drawer-container"
          hx-swap="innerHTML">
      {{if .EditError}}
      <div class="form-error">{{.EditError}}</div>
      {{end}}
      <input class="drawer-edit-title" name="title" type="text" required
             value="{{.Card.Title}}" placeholder="Title">
      <textarea class="drawer-edit-body" name="body" rows="6"
                placeholder="Description (Markdown)">{{.Card.Body}}</textarea>
      <div class="drawer-edit-actions">
        <button type="submit">Save</button>
        <button type="button" class="link-cancel" @click="editing = false">Cancel</button>
      </div>
    </form>
  </div>
  <button class="drawer-close" x-show="!editing" @click="closeDrawer()">✕</button>
```

The key change is `x-show="!editing"` on the `.drawer-close` button so it disappears while editing, letting `.drawer-edit-form` (already `width:100%` via `.drawer-edit-title`/`.drawer-edit-body`) use the full drawer width. Verify the header is a flex row where `.drawer-header-main` has `flex:1` (it does, `app.css:1048`), so removing the close button frees the full width.

- [ ] **Step 2 (#57): Restyle block/unblock forms to match the edit form**

In `app.css`, replace the cramped inline layout (lines ~842–855) with a stacked, full-width form that reuses the edit-form input and button styling:

```css
.drawer-block-form,
.drawer-unblock-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.drawer-block-form input[type="text"],
.drawer-unblock-form input[type="text"] {
  width: 100%;
  font: inherit;
  font-family: var(--font-mono);
  font-size: 0.95em;
  line-height: 1.45;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg-input);
  color: var(--text-primary);
  transition: border-color 0.15s, background 0.15s;
}

.drawer-block-form input[type="text"]:focus,
.drawer-unblock-form input[type="text"]:focus {
  outline: none;
  border-color: var(--accent);
  background: var(--bg-elevated);
}

/* The unblock form has a bare submit button (no .drawer-edit-actions
   wrapper); style it to match the edit form's primary submit button. */
.drawer-unblock-form button[type="submit"] {
  align-self: flex-end;
  font-family: var(--font-mono);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  padding: 7px 14px;
  border: 1px solid var(--text-primary);
  border-radius: 999px;
  background: var(--text-primary);
  color: var(--bg-page);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.drawer-unblock-form button[type="submit"]:hover {
  background: var(--accent);
  border-color: var(--accent);
  color: white;
}
```

The block form's buttons already live in a `.drawer-edit-actions` wrapper (`drawer.html:83`), so they inherit the existing submit styling — only its input needs the rules above (covered by the shared selector). The unblock form's submit button is bare (`drawer.html:71`), hence the dedicated rule.

- [ ] **Step 3: Manual verification**

Start server + seed.
1. **#58:** Open any card → click **Edit**. The title input and description textarea span the **full drawer width**; the `✕` close button is hidden while editing and returns on Cancel/Save.
2. **#57:** Open a blocked card (or Block an unblocked one). The reason input is full-width and matches the edit form's input styling; the **Unblock**/**Block** button matches the site's pill submit buttons. Check Chrome, Safari, and Firefox — controls look consistent.
3. Regression: comment form, assignee form, and status pills are unchanged.

- [ ] **Step 4: Commit**

```bash
git add web/templates/drawer.html web/static/css/app.css
git commit --no-gpg-sign -m "fix(web): full-width drawer edit form; consistent block/unblock controls

Closes #57
Closes #58

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:** #56 → Task 3; #57 → Task 5; #58 → Task 5; #59 → Task 4; #60 → Task 2; #61 → Task 1; #62 → Task 1. All seven issues mapped.

**Placeholder scan:** No TBD/TODO. Each step shows concrete code. The two soft spots are intentional and flagged: (a) Task 2 modal class names must be reconciled with the *actual* compose-modal classes in `layout.html`/`app.css` rather than invented — the implementer must inspect and reuse; (b) Task 1's `evt.oldDraggableIndex` is the SortableJS-provided original index. Both are explicit instructions, not vague placeholders.

**Type/name consistency:** `revertDrag(evt)` (Task 1) is reused in Task 2's `onCardDrop`. `altHeld` (Task 1) is read in Task 2. `this._defaultProjectId` (Task 4) is set in `bootstrapData` and read in `init`. `blockMove` state shape (`{open, message, _resolve}`) is consistent between Steps 2 and 3 of Task 2. `data-view="blocked"` marker (Task 3) is added to `blocked_view.html` and queried in `app.js`.

**No-JS-test-runner honesty:** Tasks 1–4 are JS/template-only with no automated test; each carries explicit cross-browser manual steps. Reviewers verify by reading + the manual steps. CI passes trivially (no Go changes).
