# Mobile-Responsive Kkullm Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Kkullm web UI usable on phones (≤640px) without changing the desktop experience.

**Architecture:** CSS-only mobile mode behind a single `@media (max-width: 640px)` breakpoint, plus a handful of new template partials (top bar, picker sheet, overflow menu, quick-capture) hidden on desktop. New interactive behavior (sheet swipe, board column paging, FAB submit) lives inside the existing Alpine root in `web/static/js/app.js`.

**Tech Stack:** Go html/template, Alpine.js 3, htmx, vanilla CSS, SortableJS (untouched on phone).

**Spec:** `docs/superpowers/specs/2026-05-17-mobile-responsive-design.md`

**Testing notes:**
- The project has no JS test framework; every UI task ends with a manual verification step. Desktop regressions are caught by visually checking the desktop layout at 1280px after each task.
- `go test ./...` must remain green throughout. It will fail loudly if any template fails to parse.
- A second `kkullm serve` is already running on `:7733` against `/tmp/kkullm-mobile-brainstorm.db` (seeded with `beehive`, `birds_nest`, `ant_hill`). Reuse it for verification. Recompile and restart it after every code change: `task build && pkill -f 'kkullm serve --addr :7733' ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &`.

**Conventions:**
- Each task is one commit.
- Phone mode CSS lives inside the single `@media (max-width: 640px)` block in `web/static/css/app.css`. Tasks add to it; the block is created in Task 1.
- New partials use the underscore prefix (`_topbar_mobile.html`) to signal they're includes, not entry points. Existing partials don't use underscores; the convention is new and applies only to additions.
- Alpine state additions live on the existing `kkullm()` data object — no new x-data scopes.

---

## File Structure

**New files:**
- `web/templates/_topbar_mobile.html` — sticky top bar shown only on phone.
- `web/templates/_picker_sheet.html` — bottom-sheet project/agent picker.
- `web/templates/_overflow_sheet.html` — bottom-sheet overflow menu (Archive / Admin / Theme).
- `web/templates/_quick_capture.html` — FAB button + quick-capture bottom sheet.

**Modified files:**
- `web/templates/layout.html` — viewport meta update, render new partials, no other behavioral change.
- `web/templates/board.html` — wrap columns with a sticky status header partial inside the existing `.board` container.
- `web/templates/drawer.html` — add a phone-only back button at the top.
- `web/static/css/app.css` — consolidate two existing `(max-width: 720px)` blocks into one new `(max-width: 640px)` block; add all new phone-mode rules there.
- `web/static/js/app.js` — add Alpine state and methods for picker, overflow, board pager, quick capture.
- `web/web.go` — register the new partial templates so `html/template` parses them at startup. *(Verify in Task 1 whether template registration is needed or globbed.)*

**Untouched at >640px:** Desktop nav, board, drawer chrome, compose modal, admin sidebar — verified visually after each task at 1280px.

---

## Task 1: Foundation — viewport meta, breakpoint consolidation, base mobile styles

**Files:**
- Modify: `web/templates/layout.html:5` (viewport meta)
- Modify: `web/static/css/app.css:1844-1860` and `web/static/css/app.css:2298-2317` (consolidate two `(max-width: 720px)` blocks into one new `(max-width: 640px)` block at the end of the file)

- [ ] **Step 1: Check how templates are loaded**

Open `web/web.go` and find how `html/template` parses files. We need to know whether new partials are auto-discovered (via `template.ParseGlob` or `embed.FS`) or must be added explicitly.

Run:
```bash
grep -n 'ParseFiles\|ParseGlob\|ParseFS\|template\.Must' web/web.go web/*.go | head -20
```

Expected: a line showing the parse mechanism. Note for later tasks:
- If `ParseGlob("web/templates/*.html")` or `ParseFS(... "*.html")`: new files auto-load.
- If `ParseFiles(...)` with explicit names: each new partial in Tasks 2–7 must be appended to that list.

No change in this step — just record what you found.

- [ ] **Step 2: Update the viewport meta**

In `web/templates/layout.html` line 5, replace:

```html
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
```

with:

```html
  <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
```

- [ ] **Step 3: Remove the two existing 720px blocks**

In `web/static/css/app.css`, delete lines 1843–1860 (the first `/* ===== Responsive niceties ===== */` block) and lines 2298–2317 (the admin 720px block). The content of these blocks moves into the new 640px block in Step 4.

- [ ] **Step 4: Append the consolidated 640px block at the end of `web/static/css/app.css`**

Append this entire block to the end of the file:

```css
/* ===== Phone mode (≤640px) =====
   Single breakpoint that contains all phone-specific overrides.
   Tablets and resized desktop windows fall through to desktop layout. */
@media (max-width: 640px) {
  /* Body safe-area paddings */
  body {
    padding-left: env(safe-area-inset-left);
    padding-right: env(safe-area-inset-right);
  }

  /* Suppress iOS auto-zoom on input focus */
  input, select, textarea {
    font-size: 16px;
  }

  /* Minimum tap target */
  button, .btn, a.btn {
    min-height: 44px;
  }

  /* Show/hide helpers (no-op on desktop) */
  .phone-only { display: revert; }
  .desktop-only { display: none !important; }

  /* Hide the existing desktop nav entirely; the mobile top bar takes over.
     The mobile top bar (Task 2) gets .phone-only so it stays hidden >640px. */
  .nav { display: none !important; }

  /* Carry-over from the previous 720px blocks */
  .board { padding-left: 16px; padding-right: 16px; }
  .drawer { width: 100%; }
  .compose-modal { max-width: none; margin: 0 8px; }
  .compose-grid { grid-template-columns: 1fr; }
  .drawer-meta { grid-template-columns: 1fr; gap: 12px; }
  .admin {
    grid-template-columns: 1fr;
    gap: 24px;
    padding: 20px 16px 40px;
  }
  .admin-menu {
    flex-direction: row;
    flex-wrap: wrap;
    border-right: none;
    border-bottom: 1px solid var(--hairline);
    padding-right: 0;
    padding-bottom: 12px;
  }
  .admin-menu-divider { display: none; }
  .admin-row { grid-template-columns: 1fr; gap: 10px; }
}

/* Phone-only / desktop-only base state (outside the media query) */
.phone-only { display: none; }
.desktop-only { display: revert; }
```

Note: `.admin`'s top padding changed from `calc(var(--nav-height) + 20px)` to `20px` because the desktop nav is hidden on phone — the mobile top bar from Task 2 will add its own height to body padding via `padding-top` on `main` or `body`. We'll re-tune in Task 2 if needed.

- [ ] **Step 5: Build and run the tests**

Run:
```bash
task build && go test ./...
```

Expected: build succeeds; all Go tests pass.

- [ ] **Step 6: Restart the brainstorm server**

Run:
```bash
pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1 && curl -sf http://localhost:7733/ -o /dev/null && echo OK
```

Expected: `OK`.

- [ ] **Step 7: Manual verification**

In Chrome devtools mobile mode (iPhone 14 Pro Max, 430×932):
1. Navigate to `http://localhost:7733/?project=beehive`.
2. Confirm the **desktop nav is hidden** (no visible nav bar yet; this is intentional — the mobile top bar lands in Task 2).
3. Confirm the **board renders** below where the nav used to be, with `padding-left: 16px`.
4. Toggle off device mode and confirm at 1280×900 the desktop nav and board look unchanged from before.

- [ ] **Step 8: Commit**

```bash
git add web/templates/layout.html web/static/css/app.css
git commit -m "feat(web): consolidate mobile breakpoint at 640px + viewport-fit"
```

---

## Task 2: Mobile top bar

**Files:**
- Create: `web/templates/_topbar_mobile.html`
- Modify: `web/templates/layout.html` (render the partial, register if needed per Task 1 Step 1)
- Modify: `web/static/css/app.css` (append top-bar styles)
- Modify: `web/static/js/app.js` (add `currentViewName()` getter)

- [ ] **Step 1: Create the partial**

Create `web/templates/_topbar_mobile.html` with:

```html
{{define "topbar_mobile"}}
<header class="topbar-mobile phone-only" role="banner">
  <span class="topbar-brand" aria-hidden="true">끌림</span>
  <button class="topbar-view-name"
          type="button"
          @click="pickerOpen = true"
          aria-haspopup="dialog"
          :aria-expanded="pickerOpen">
    <span x-text="currentViewName()"></span>
    <span class="topbar-caret" aria-hidden="true">▾</span>
  </button>
  <button class="topbar-overflow icon-btn"
          type="button"
          @click="overflowOpen = true"
          aria-haspopup="menu"
          :aria-expanded="overflowOpen"
          aria-label="Menu">
    <svg width="18" height="18" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <circle cx="8" cy="3" r="1.4" fill="currentColor"/>
      <circle cx="8" cy="8" r="1.4" fill="currentColor"/>
      <circle cx="8" cy="13" r="1.4" fill="currentColor"/>
    </svg>
  </button>
</header>
{{end}}
```

- [ ] **Step 2: Include the partial in `layout.html`**

In `web/templates/layout.html`, immediately after the opening `<body ...>` tag (after the `<div class="grain">` is fine), and before `<nav class="nav">`, add:

```html
  <!-- Mobile top bar (phone only) -->
  {{template "topbar_mobile" .}}
```

If Task 1 Step 1 showed `ParseFiles` with explicit filenames in `web/web.go`, also add `"web/templates/_topbar_mobile.html"` to that list. If `ParseGlob` is in use, no Go changes needed.

- [ ] **Step 3: Add temporary Alpine state stubs**

In `web/static/js/app.js`, inside the `return { ... }` object (around line 4), add these fields alongside `composeOpen`:

```javascript
    pickerOpen: false,
    pickerPage: 'projects',
    overflowOpen: false,
    quickCaptureOpen: false,
    boardCol: 'todo',
```

These stubs make the top bar `@click` handlers harmless until later tasks wire them up. The user clicks won't do anything visible yet.

- [ ] **Step 4: Add `currentViewName()` method**

In `web/static/js/app.js`, inside the `return { ... }` object, add this method (place it after `bootstrapData()`):

```javascript
    currentViewName() {
      if (this.viewMode === 'agent') {
        const a = this.agents.find(x => String(x.id) === String(this.currentAgent));
        return a ? a.name : '(no agent)';
      }
      const p = this.projects.find(x => String(x.id) === String(this.currentProject));
      return p ? p.name : '(no project)';
    },
```

- [ ] **Step 5: Append top-bar CSS**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Mobile top bar ===== */
.topbar-mobile {
  display: none;
}

@media (max-width: 640px) {
  .topbar-mobile {
    display: flex;
    align-items: center;
    gap: 12px;
    position: sticky;
    top: 0;
    z-index: 50;
    height: 56px;
    padding: 0 12px;
    padding-top: env(safe-area-inset-top);
    height: calc(56px + env(safe-area-inset-top));
    background: var(--bg-page);
    border-bottom: 1px solid var(--hairline);
  }
  .topbar-brand {
    font-family: var(--font-korean);
    font-size: 18px;
    color: var(--accent);
    line-height: 1;
    flex: 0 0 auto;
  }
  .topbar-view-name {
    flex: 1 1 auto;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: transparent;
    border: none;
    padding: 0 4px;
    font-family: var(--font-display);
    font-size: 18px;
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
    min-height: 44px;
  }
  .topbar-caret {
    font-size: 12px;
    color: var(--text-tertiary);
  }
  .topbar-overflow {
    flex: 0 0 auto;
    min-width: 44px;
    min-height: 44px;
  }
}
```

- [ ] **Step 6: Build, restart server, run tests**

```bash
task build && go test ./... && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

Expected: build OK, tests pass.

- [ ] **Step 7: Manual verification**

At iPhone 14 Pro Max:
1. Top bar is visible, sticky at top, ~56px tall (plus safe-area).
2. Left shows `끌림` mark in red/accent color.
3. Center-left shows `beehive ▾` (the default project name).
4. Right shows the `⋮` icon button.
5. Tap on `beehive ▾` and `⋮` — nothing visible happens yet, but no console errors.

At 1280×900: top bar is invisible, desktop nav reappears.

- [ ] **Step 8: Commit**

```bash
git add web/templates/_topbar_mobile.html web/templates/layout.html web/static/js/app.js web/static/css/app.css
git commit -m "feat(web): mobile top bar with view-name picker trigger"
```

If you added the partial to `web/web.go` in Step 2, include it in `git add`.

---

## Task 3: Picker bottom sheet (project ↔ agent, swipeable)

**Files:**
- Create: `web/templates/_picker_sheet.html`
- Modify: `web/templates/layout.html` (render partial)
- Modify: `web/static/css/app.css` (sheet + picker styles)
- Modify: `web/static/js/app.js` (picker state, methods, IntersectionObserver, persistence)

- [ ] **Step 1: Create the partial**

Create `web/templates/_picker_sheet.html`:

```html
{{define "picker_sheet"}}
<div class="sheet-backdrop phone-only"
     x-show="pickerOpen"
     x-transition.opacity.duration.150ms
     @click="pickerOpen = false"
     x-cloak></div>
<div class="sheet picker-sheet phone-only"
     :class="{ open: pickerOpen }"
     role="dialog"
     aria-modal="true"
     aria-label="Choose project or agent"
     x-cloak>
  <div class="sheet-handle" aria-hidden="true"></div>
  <div class="picker-tabs">
    <button type="button"
            class="picker-chevron"
            @click="setPickerPage('projects')"
            aria-label="Projects page">◀</button>
    <button type="button"
            class="picker-tab"
            :class="{ active: pickerPage === 'projects' }"
            @click="setPickerPage('projects')">Projects</button>
    <span class="picker-dots" aria-hidden="true">
      <span class="picker-dot" :class="{ active: pickerPage === 'projects' }"></span>
      <span class="picker-dot" :class="{ active: pickerPage === 'agents' }"></span>
    </span>
    <button type="button"
            class="picker-tab"
            :class="{ active: pickerPage === 'agents' }"
            @click="setPickerPage('agents')">Agents</button>
    <button type="button"
            class="picker-chevron"
            @click="setPickerPage('agents')"
            aria-label="Agents page">▶</button>
  </div>
  <div class="picker-pages" x-ref="pickerPages">
    <div class="picker-page" data-page="projects">
      <ul class="picker-list">
        <template x-for="p in projects" :key="p.id">
          <li>
            <button type="button"
                    class="picker-row"
                    :class="{ active: viewMode === 'project' && String(currentProject) === String(p.id) }"
                    @click="selectProject(p.id)">
              <span class="picker-row-name" x-text="p.name"></span>
              <span class="picker-row-check" aria-hidden="true"
                    x-show="viewMode === 'project' && String(currentProject) === String(p.id)">✓</span>
            </button>
          </li>
        </template>
      </ul>
    </div>
    <div class="picker-page" data-page="agents">
      <ul class="picker-list">
        <template x-if="agents.length === 0">
          <li class="picker-empty">No agents.</li>
        </template>
        <template x-for="a in agents" :key="a.id">
          <li>
            <button type="button"
                    class="picker-row"
                    :class="{ active: viewMode === 'agent' && String(currentAgent) === String(a.id) }"
                    @click="selectAgent(a.id)">
              <span class="picker-row-name">
                <span x-text="a.name"></span>
                <span class="picker-row-meta" x-text="'· ' + a.project"></span>
              </span>
              <span class="picker-row-check" aria-hidden="true"
                    x-show="viewMode === 'agent' && String(currentAgent) === String(a.id)">✓</span>
            </button>
          </li>
        </template>
      </ul>
    </div>
  </div>
</div>
{{end}}
```

- [ ] **Step 2: Include the partial in `layout.html`**

In `web/templates/layout.html`, immediately before the closing `</body>` tag (after the toast container, before `<script id="boot-data">`), add:

```html
  <!-- Mobile picker sheet (phone only) -->
  {{template "picker_sheet" .}}
```

If `web/web.go` uses explicit `ParseFiles`, add the new filename there too.

- [ ] **Step 3: Add picker methods to Alpine root**

In `web/static/js/app.js`, inside the `return { ... }` object (after `currentViewName()` from Task 2), add:

```javascript
    initPicker() {
      const saved = localStorage.getItem('kkullm-picker-page');
      if (saved === 'projects' || saved === 'agents') {
        this.pickerPage = saved;
      }
      // Sync horizontal scroll position when pickerOpen flips true.
      this.$watch('pickerOpen', (open) => {
        if (open) this.$nextTick(() => this.syncPickerScroll());
      });
      this.$watch('pickerPage', (page) => {
        localStorage.setItem('kkullm-picker-page', page);
      });
    },

    setPickerPage(page) {
      this.pickerPage = page;
      this.$nextTick(() => this.syncPickerScroll());
    },

    syncPickerScroll() {
      const container = this.$refs.pickerPages;
      if (!container) return;
      const target = container.querySelector('[data-page="' + this.pickerPage + '"]');
      if (target) target.scrollIntoView({ behavior: 'smooth', inline: 'start', block: 'nearest' });
    },

    selectProject(id) {
      this.viewMode = 'project';
      this.currentProject = String(id);
      this.pickerOpen = false;
      this.loadBoard();
    },

    selectAgent(id) {
      this.viewMode = 'agent';
      this.currentAgent = String(id);
      this.pickerOpen = false;
      this.loadBoard();
    },
```

- [ ] **Step 4: Call `initPicker()` from `init()`**

In `web/static/js/app.js`, find the `init()` method (around line 24) and add a call at the bottom of its body (after `connectSSE()` and after the existing `htmx:afterSettle` handler):

```javascript
      this.initPicker();
```

- [ ] **Step 5: Add IntersectionObserver to track swipe**

Inside `initPicker()`, after the existing body, append:

```javascript
      // Track horizontal scroll position to keep pickerPage in sync with swipe.
      this.$nextTick(() => {
        const container = this.$refs.pickerPages;
        if (!container) return;
        const io = new IntersectionObserver((entries) => {
          for (const entry of entries) {
            if (entry.intersectionRatio >= 0.6) {
              const page = entry.target.dataset.page;
              if (page && page !== this.pickerPage) this.pickerPage = page;
            }
          }
        }, { root: container, threshold: [0.6] });
        container.querySelectorAll('.picker-page').forEach(el => io.observe(el));
      });
```

- [ ] **Step 6: Append picker + sheet CSS**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Bottom sheet (shared chrome) ===== */
.sheet-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(26, 22, 18, 0.45);
  z-index: 60;
  display: none;
}
.sheet {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 61;
  background: var(--bg-surface);
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  box-shadow: var(--shadow-lg);
  padding-bottom: env(safe-area-inset-bottom);
  transform: translateY(100%);
  transition: transform 0.22s ease;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  display: none;
}
.sheet.open { transform: translateY(0); }
.sheet-handle {
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--hairline);
  margin: 10px auto 6px;
}

@media (max-width: 640px) {
  .sheet, .sheet-backdrop { display: flex; }
  .sheet { display: flex; }
}

/* ===== Picker sheet ===== */
.picker-sheet { height: 70vh; }
.picker-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px 10px;
  border-bottom: 1px solid var(--hairline);
}
.picker-chevron {
  background: transparent;
  border: none;
  font-size: 16px;
  color: var(--text-tertiary);
  min-width: 36px;
  min-height: 36px;
  cursor: pointer;
}
.picker-tab {
  background: transparent;
  border: none;
  font-family: var(--font-mono);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--text-secondary);
  padding: 8px 12px;
  cursor: pointer;
}
.picker-tab.active { color: var(--accent); }
.picker-dots {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  margin: 0 6px;
}
.picker-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--hairline);
}
.picker-dot.active { background: var(--accent); }
.picker-pages {
  flex: 1 1 auto;
  display: flex;
  overflow-x: auto;
  scroll-snap-type: x mandatory;
  scrollbar-width: none;
}
.picker-pages::-webkit-scrollbar { display: none; }
.picker-page {
  flex: 0 0 100%;
  scroll-snap-align: start;
  overflow-y: auto;
  padding: 6px 0 16px;
}
.picker-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.picker-row {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 56px;
  padding: 0 18px;
  background: transparent;
  border: none;
  text-align: left;
  font-family: var(--font-body);
  font-size: 16px;
  color: var(--text-primary);
  cursor: pointer;
  border-bottom: 1px solid var(--border-light);
}
.picker-row:hover { background: var(--bg-card-hover); }
.picker-row.active { color: var(--accent-strong); }
.picker-row-meta {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-left: 6px;
}
.picker-row-check { color: var(--accent); font-size: 18px; }
.picker-empty { padding: 24px 18px; color: var(--text-tertiary); }
```

- [ ] **Step 7: Build, restart server, run tests**

```bash
task build && go test ./... && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

- [ ] **Step 8: Manual verification**

At iPhone 14 Pro Max:
1. Tap `beehive ▾` in the top bar → picker sheet slides up.
2. Three projects (`beehive`, `birds_nest`, `ant_hill`) are listed, beehive marked with `✓`.
3. Tap `birds_nest` → sheet closes, top bar updates to `birds_nest ▾`, board reloads.
4. Tap view name → picker reopens.
5. Tap `Agents` tab label → page swipes to agents list.
6. Swipe horizontally on the list area → tab indicator dots follow.
7. Select an agent → sheet closes, top bar updates to agent name, board reloads.
8. Reopen picker → it lands on the page (Projects or Agents) you last used.
9. Tap backdrop → picker closes.

At 1280×900: sheet/backdrop never render.

- [ ] **Step 9: Commit**

```bash
git add web/templates/_picker_sheet.html web/templates/layout.html web/static/js/app.js web/static/css/app.css
git commit -m "feat(web): mobile picker bottom sheet for project/agent switching"
```

---

## Task 4: Overflow menu sheet

**Files:**
- Create: `web/templates/_overflow_sheet.html`
- Modify: `web/templates/layout.html`
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Create the partial**

Create `web/templates/_overflow_sheet.html`:

```html
{{define "overflow_sheet"}}
<div class="sheet-backdrop phone-only"
     x-show="overflowOpen"
     x-transition.opacity.duration.150ms
     @click="overflowOpen = false"
     x-cloak></div>
<div class="sheet overflow-sheet phone-only"
     :class="{ open: overflowOpen }"
     role="menu"
     aria-label="Menu"
     x-cloak>
  <div class="sheet-handle" aria-hidden="true"></div>
  <ul class="overflow-list">
    <li>
      <button type="button"
              class="overflow-row"
              @click="overflowOpen = false; loadArchived()"
              role="menuitem">
        <span class="overflow-row-icon" aria-hidden="true">🗂</span>
        <span>Archive</span>
      </button>
    </li>
    <li>
      <a href="/admin" class="overflow-row" role="menuitem"
         @click="overflowOpen = false">
        <span class="overflow-row-icon" aria-hidden="true">⚙</span>
        <span>Admin</span>
      </a>
    </li>
    <li class="overflow-divider" aria-hidden="true"></li>
    <li>
      <button type="button"
              class="overflow-row"
              @click="toggleTheme()"
              role="menuitem">
        <span class="overflow-row-icon" aria-hidden="true">🌓</span>
        <span>Theme: <span x-text="theme === 'dark' ? 'dark' : 'light'"></span></span>
      </button>
    </li>
  </ul>
</div>
{{end}}
```

- [ ] **Step 2: Include the partial in `layout.html`**

In `web/templates/layout.html`, immediately after the picker sheet include from Task 3, add:

```html
  <!-- Mobile overflow menu (phone only) -->
  {{template "overflow_sheet" .}}
```

If `web/web.go` uses explicit `ParseFiles`, add the new filename there too.

- [ ] **Step 3: Append overflow CSS**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Overflow menu sheet ===== */
.overflow-sheet { max-height: 60vh; }
.overflow-list {
  list-style: none;
  margin: 0;
  padding: 6px 0 12px;
}
.overflow-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 48px;
  padding: 0 18px;
  background: transparent;
  border: none;
  text-align: left;
  font-family: var(--font-body);
  font-size: 16px;
  color: var(--text-primary);
  text-decoration: none;
  cursor: pointer;
}
.overflow-row:hover { background: var(--bg-card-hover); }
.overflow-row-icon { font-size: 18px; width: 22px; text-align: center; }
.overflow-divider {
  height: 1px;
  margin: 6px 18px;
  background: var(--hairline);
}
```

- [ ] **Step 4: Build, restart server, run tests**

```bash
task build && go test ./... && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

- [ ] **Step 5: Manual verification**

At iPhone 14 Pro Max:
1. Tap `⋮` → overflow sheet slides up with three rows: Archive, Admin, Theme.
2. Tap Admin → navigates to `/admin`.
3. Back to board (tap browser back). Tap `⋮` → Tap Theme → theme flips dark/light, label updates.
4. Tap backdrop or close swipe (tap outside) → sheet closes.
5. Tap Archive — currently 400s on this scope, which is the known pre-existing bug. Confirm the *menu interaction* worked (sheet closed, fetch was attempted); the broken response is not our problem in this task.

At 1280×900: overflow sheet never renders.

- [ ] **Step 6: Commit**

```bash
git add web/templates/_overflow_sheet.html web/templates/layout.html web/static/css/app.css
git commit -m "feat(web): mobile overflow menu sheet"
```

---

## Task 5: Phone board — always show Blocked column

**Files:**
- Modify: `web/static/css/app.css` (one rule inside the 640 block)

- [ ] **Step 1: Add Blocked-always-visible rule**

Inside the existing `@media (max-width: 640px)` block in `web/static/css/app.css`, add this rule (anywhere within the block):

```css
  /* On phone, always show the Blocked column even when empty.
     The .blocked-hidden class is toggled by JS to hide it on desktop. */
  .column.column-blocked.blocked-hidden { display: flex; }
```

- [ ] **Step 2: Build, restart, verify**

```bash
task build && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

At iPhone 14 Pro Max on a project with zero blocked cards (e.g. `birds_nest` if no cards are blocked — confirm by inspecting the demo data):

```bash
sqlite3 /tmp/kkullm-mobile-brainstorm.db "SELECT id, name FROM projects;"
sqlite3 /tmp/kkullm-mobile-brainstorm.db "SELECT p.name, COUNT(c.id) FROM projects p LEFT JOIN cards c ON c.project_id=p.id AND c.status='blocked' GROUP BY p.id;"
```

Switch to a project with `blocked` count = 0 via the picker. Confirm the Blocked column header is visible (count `0`). At desktop width, that column remains hidden — the JS toggles `.blocked-hidden`, but our CSS only overrides at ≤640px.

- [ ] **Step 3: Commit**

```bash
git add web/static/css/app.css
git commit -m "feat(web): always show Blocked column on phone"
```

---

## Task 6: Board pager — sticky status header, scroll-snap, IntersectionObserver

**Files:**
- Modify: `web/templates/board.html`
- Modify: `web/static/css/app.css`
- Modify: `web/static/js/app.js`

- [ ] **Step 1: Add the status header to `board.html`**

In `web/templates/board.html`, replace the opening line (currently `<div class="board" id="board" x-ref="board">`) with:

```html
<div class="board-status-header phone-only" x-show="boardLoaded" x-cloak>
  <button type="button"
          class="board-chevron"
          @click="boardPagerStep(-1)"
          aria-label="Previous column">◀</button>
  <span class="board-status-name" x-text="boardColLabel()"></span>
  <span class="board-status-count" x-text="'(' + boardColCount() + ')'"></span>
  <button type="button"
          class="board-chevron"
          @click="boardPagerStep(1)"
          aria-label="Next column">▶</button>
  <span class="board-dots" aria-hidden="true">
    <template x-for="s in boardPagerStatuses()" :key="s">
      <span class="board-dot" :class="{ active: boardCol === s }"></span>
    </template>
  </span>
</div>
<div class="board" id="board" x-ref="board">
```

The closing `</div>` at the end of the file stays. (You're adding a sibling header above the existing `.board` div, not wrapping it.)

- [ ] **Step 2: Append pager CSS**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Board status header (phone) ===== */
.board-status-header {
  display: none;
}

@media (max-width: 640px) {
  .board-status-header {
    display: flex;
    align-items: center;
    gap: 10px;
    position: sticky;
    top: calc(56px + env(safe-area-inset-top));
    z-index: 40;
    background: var(--bg-page);
    padding: 8px 12px;
    border-bottom: 1px solid var(--hairline);
    font-family: var(--font-display);
  }
  .board-chevron {
    background: transparent;
    border: none;
    font-size: 16px;
    color: var(--text-tertiary);
    min-width: 36px;
    min-height: 36px;
    cursor: pointer;
  }
  .board-status-name {
    font-size: 16px;
    color: var(--text-primary);
    text-transform: capitalize;
  }
  .board-status-count {
    color: var(--text-tertiary);
    font-size: 14px;
  }
  .board-dots {
    margin-left: auto;
    display: inline-flex;
    gap: 5px;
  }
  .board-dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--hairline);
  }
  .board-dot.active { background: var(--accent); }

  /* Make .board into a horizontal pager */
  .board {
    scroll-snap-type: x mandatory;
    scroll-padding: 12px;
    gap: 0;
  }
  .board > .column {
    flex: 0 0 calc(100% - 24px);
    scroll-snap-align: center;
    margin-right: 12px;
  }
  .board > .column:first-child { margin-left: 12px; }
}
```

- [ ] **Step 3: Add Alpine state for pager**

The `boardCol` stub was added in Task 2 Step 3. Now expand it with helpers.

In `web/static/js/app.js`, inside the `return { ... }` object, add these methods (placement: just before `loadBoard()`):

```javascript
    boardPagerStatuses() {
      return ['considering', 'todo', 'blocked', 'in_flight', 'completed', 'tabled'];
    },

    boardColLabel() {
      const map = {
        considering: 'Considering', todo: 'Todo', blocked: 'Blocked',
        in_flight: 'In Flight', completed: 'Completed', tabled: 'Tabled',
      };
      return map[this.boardCol] || '';
    },

    boardColCount() {
      const col = this.boardCol === 'blocked'
        ? document.getElementById('blocked-column')
        : document.querySelector('.column[data-status="' + this.boardCol + '"]');
      if (!col) return 0;
      return col.querySelectorAll('.card-tile').length;
    },

    boardScopeKey() {
      return this.viewMode === 'agent' ? 'a:' + this.currentAgent : 'p:' + this.currentProject;
    },

    boardPagerStep(delta) {
      const order = this.boardPagerStatuses();
      const idx = Math.max(0, order.indexOf(this.boardCol));
      const next = order[Math.min(order.length - 1, Math.max(0, idx + delta))];
      this.scrollBoardToColumn(next);
    },

    scrollBoardToColumn(status) {
      const board = this.$refs.board;
      if (!board) return;
      const sel = status === 'blocked'
        ? '#blocked-column'
        : '.column[data-status="' + status + '"]';
      const col = board.querySelector(sel);
      if (col) col.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
    },

    initBoardPager() {
      if (!window.matchMedia('(max-width: 640px)').matches) return;

      const board = this.$refs.board;
      if (!board) return;

      // Determine landing column.
      const key = 'kkullm-board-col:' + this.boardScopeKey();
      const remembered = localStorage.getItem(key);
      let landing;
      if (remembered) {
        landing = remembered;
      } else {
        const counts = {
          blocked: document.querySelectorAll('#blocked-cards .card-tile').length,
          in_flight: document.querySelectorAll('[data-status="in_flight"] .card-tile').length,
        };
        if (counts.blocked > 0) landing = 'blocked';
        else if (counts.in_flight > 0) landing = 'in_flight';
        else landing = 'todo';
      }
      this.boardCol = landing;

      // Scroll without smooth on initial landing.
      const sel = landing === 'blocked'
        ? '#blocked-column'
        : '.column[data-status="' + landing + '"]';
      const col = board.querySelector(sel);
      if (col) col.scrollIntoView({ behavior: 'instant', inline: 'center', block: 'nearest' });

      // Observe which column is centered.
      if (this._boardIO) this._boardIO.disconnect();
      this._boardIO = new IntersectionObserver((entries) => {
        for (const entry of entries) {
          if (entry.intersectionRatio >= 0.6) {
            const col = entry.target;
            const status = col.id === 'blocked-column'
              ? 'blocked'
              : col.dataset.status;
            if (status && status !== this.boardCol) {
              this.boardCol = status;
              localStorage.setItem('kkullm-board-col:' + this.boardScopeKey(), status);
            }
          }
        }
      }, { root: board, threshold: [0.6] });
      board.querySelectorAll('.column').forEach(el => this._boardIO.observe(el));
    },
```

- [ ] **Step 4: Wire `initBoardPager()` into the htmx settle handler**

In `web/static/js/app.js`, find the existing `htmx:afterSettle` handler in `init()` (around line 31). Inside the `if (e.detail.target.id === 'board-container') { ... }` block, add a call to `initBoardPager()` after `syncBlockedColumnVisibility()`:

```javascript
        if (e.detail.target.id === 'board-container') {
          this.boardLoaded = true;
          this.$nextTick(() => this.initSortable());
          this.updateBlockerCount();
          this.syncBlockedColumnVisibility();
          this.$nextTick(() => this.initBoardPager());
        }
```

(Only the last line is new.)

- [ ] **Step 5: Build, restart, verify**

```bash
task build && go test ./... && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

At iPhone 14 Pro Max:
1. Load `http://localhost:7733/?project=beehive` (clear localStorage first via devtools → Application → Local Storage).
2. Board lands on **Blocked** (if `beehive` has blocked cards), else **In Flight**, else **Todo**. Confirm against the seed data.
3. Status header shows column name + count + dots; the dot for the current column is filled.
4. Swipe left/right between columns → header label, count, and dot indicator update.
5. Tap `◀` / `▶` chevrons → smooth scroll one column over.
6. Switch to a different project via picker → board reloads and lands on heuristic for that scope.
7. Reload page (Cmd-R) → lands on the column you were last on (per-scope memory). For projects you've never visited before, the heuristic applies.

At 1280×900: board renders as before, no status header visible, columns side-by-side.

- [ ] **Step 6: Commit**

```bash
git add web/templates/board.html web/static/css/app.css web/static/js/app.js
git commit -m "feat(web): phone board column pager with sticky status header"
```

---

## Task 7: Quick-capture FAB + sheet

**Files:**
- Create: `web/templates/_quick_capture.html`
- Modify: `web/templates/layout.html`
- Modify: `web/static/css/app.css`
- Modify: `web/static/js/app.js`

- [ ] **Step 1: Create the partial**

Create `web/templates/_quick_capture.html`:

```html
{{define "quick_capture"}}
<button class="quick-fab phone-only"
        type="button"
        @click="openQuickCapture()"
        aria-label="Quick add card to Considering">
  <svg width="24" height="24" viewBox="0 0 14 14" fill="none" aria-hidden="true">
    <path d="M7 1v12M1 7h12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
  </svg>
</button>

<div class="sheet-backdrop phone-only"
     x-show="quickCaptureOpen"
     x-transition.opacity.duration.150ms
     @click="quickCaptureOpen = false"
     x-cloak></div>
<div class="sheet quick-capture-sheet phone-only"
     :class="{ open: quickCaptureOpen }"
     role="dialog"
     aria-modal="true"
     aria-label="Add card to Considering"
     x-cloak>
  <div class="sheet-handle" aria-hidden="true"></div>
  <form class="quick-capture-form" @submit.prevent="submitQuickCapture($event)">
    <label class="quick-capture-label" for="qc-title">Title</label>
    <textarea id="qc-title"
              name="title"
              x-ref="quickCaptureTitle"
              rows="2"
              required
              placeholder="What needs considering?"></textarea>

    <div class="quick-capture-row">
      <span class="quick-capture-meta-label">Project:</span>
      <select name="project" class="quick-capture-project">
        <template x-for="p in projects" :key="p.id">
          <option :value="p.name" :selected="String(currentProject) === String(p.id)" x-text="p.name"></option>
        </template>
      </select>
    </div>

    <div class="quick-capture-row">
      <span class="quick-capture-meta-label">Status:</span>
      <span class="quick-capture-status">considering</span>
    </div>

    <div class="quick-capture-error" x-show="quickCaptureError" x-text="quickCaptureError"></div>

    <div class="quick-capture-actions">
      <button type="button" class="btn btn-secondary" @click="quickCaptureOpen = false">Cancel</button>
      <button type="submit" class="btn btn-primary" :disabled="quickCaptureBusy">
        <span x-text="quickCaptureBusy ? 'Adding…' : 'Add card'"></span>
      </button>
    </div>
  </form>
</div>
{{end}}
```

- [ ] **Step 2: Include the partial in `layout.html`**

Inside `web/templates/layout.html`, immediately after the overflow sheet include from Task 4, add:

```html
  <!-- Mobile quick-capture (phone only) -->
  {{template "quick_capture" .}}
```

If `web/web.go` uses explicit `ParseFiles`, add the new filename there too.

- [ ] **Step 3: Add Alpine state and methods**

In `web/static/js/app.js`:

The `quickCaptureOpen` stub was added in Task 2. Add error/busy fields alongside it:

```javascript
    quickCaptureBusy: false,
    quickCaptureError: '',
```

Add methods inside the same object (place after `submitCompose()` and `resetComposeForm()`):

```javascript
    openQuickCapture() {
      this.quickCaptureError = '';
      this.quickCaptureBusy = false;
      this.quickCaptureOpen = true;
      this.$nextTick(() => {
        const el = this.$refs.quickCaptureTitle;
        if (el) el.focus();
      });
    },

    async submitQuickCapture(evt) {
      this.quickCaptureError = '';
      this.quickCaptureBusy = true;
      const form = evt.target;
      const fd = new FormData(form);
      try {
        const body = {
          title: (fd.get('title') || '').toString().trim(),
          body: '',
          status: 'considering',
          project: (fd.get('project') || '').toString(),
          assignees: [],
          tags: [],
        };
        const resp = await this.postJSON('/api/cards', body);
        const card = await resp.json();
        if (!resp.ok) throw new Error(card.error || 'Could not add card.');
        form.reset();
        this.quickCaptureOpen = false;
        this.showToast('Added to considering');
        // SSE card_created will refresh the board; nothing else to do.
      } catch (err) {
        this.quickCaptureError = err.message || 'Something went wrong.';
      } finally {
        this.quickCaptureBusy = false;
      }
    },
```

- [ ] **Step 4: Append FAB + sheet CSS**

Append to the end of `web/static/css/app.css`:

```css
/* ===== Quick-capture FAB + sheet ===== */
.quick-fab {
  display: none;
}

@media (max-width: 640px) {
  .quick-fab {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    position: fixed;
    right: calc(16px + env(safe-area-inset-right));
    bottom: calc(20px + env(safe-area-inset-bottom));
    z-index: 55;
    width: 56px;
    height: 56px;
    border-radius: 50%;
    background: var(--accent);
    color: var(--bg-page);
    border: none;
    box-shadow: var(--shadow-lg);
    cursor: pointer;
  }
  .quick-fab:active { transform: translateY(1px); }
}

.quick-capture-sheet { max-height: 70vh; }
.quick-capture-form {
  padding: 4px 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.quick-capture-label {
  font-family: var(--font-mono);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--text-secondary);
}
.quick-capture-form textarea {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 10px 12px;
  font-family: var(--font-body);
  font-size: 16px;
  color: var(--text-primary);
  resize: vertical;
  min-height: 64px;
}
.quick-capture-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.quick-capture-meta-label {
  font-family: var(--font-mono);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--text-tertiary);
  min-width: 70px;
}
.quick-capture-project {
  flex: 1 1 auto;
  min-height: 44px;
}
.quick-capture-status {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--status-considering);
  background: var(--status-considering-tint);
  padding: 4px 10px;
  border-radius: var(--radius-sm);
}
.quick-capture-error {
  color: var(--danger);
  font-size: 14px;
}
.quick-capture-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 4px;
}
```

- [ ] **Step 5: Build, restart, verify**

```bash
task build && go test ./... && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

At iPhone 14 Pro Max:
1. FAB visible bottom-right, accent color.
2. Tap FAB → sheet slides up, title textarea focused, keyboard would open on a real device.
3. Type "Mobile FAB test", verify Project dropdown defaults to current project, status shows `considering`.
4. Tap `Add card` → sheet closes, toast `Added to considering` appears, board refreshes via SSE, new card visible in the Considering column. Swipe to Considering to confirm.
5. Tap FAB again → submit with empty title → required validation prevents submission (no submit; browser shows native required hint).
6. Tap FAB → Cancel → sheet closes with no card created.
7. Tap FAB while on the Admin page → FAB still visible (it's anchored to viewport), opens sheet that creates a card with current project.

At 1280×900: FAB is invisible.

- [ ] **Step 6: Commit**

```bash
git add web/templates/_quick_capture.html web/templates/layout.html web/static/js/app.js web/static/css/app.css
git commit -m "feat(web): mobile quick-capture FAB for adding to Considering"
```

---

## Task 8: Drawer back button on phone

**Files:**
- Modify: `web/templates/drawer.html`
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Add the back button**

The drawer header (`web/templates/drawer.html` lines 3–9) is:

```html
<div class="drawer-header">
  <div>
    <div class="drawer-card-id">#{{.Card.ID}} · {{.Card.Project}}</div>
    <div class="drawer-title">{{.Card.Title}}</div>
  </div>
  <button class="drawer-close" @click="closeDrawer()">✕</button>
</div>
```

Replace those 7 lines with:

```html
<div class="drawer-header">
  <button type="button"
          class="drawer-back phone-only"
          @click="closeDrawer()"
          aria-label="Back">
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M12 4L6 10L12 16" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
    </svg>
  </button>
  <div>
    <div class="drawer-card-id">#{{.Card.ID}} · {{.Card.Project}}</div>
    <div class="drawer-title">{{.Card.Title}}</div>
  </div>
  <button class="drawer-close" @click="closeDrawer()">✕</button>
</div>
```

The back button is the first child of `.drawer-header`, the existing close button stays as-is on desktop. On phone, both render (back on the left, close on the right) — that's fine; either dismisses.

- [ ] **Step 2: Add CSS**

Append to the end of `web/static/css/app.css`:

```css
.drawer-back { display: none; }

@media (max-width: 640px) {
  .drawer-back {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    color: var(--text-primary);
    min-width: 44px;
    min-height: 44px;
    cursor: pointer;
    margin-right: 6px;
  }
}
```

- [ ] **Step 3: Build, restart, verify**

```bash
task build && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

At iPhone 14 Pro Max:
1. Tap any card to open the drawer.
2. Back button is visible top-left of the drawer.
3. Tap back button → drawer closes, board visible again.

At 1280×900: drawer back button invisible; existing close behavior unchanged.

- [ ] **Step 4: Commit**

```bash
git add web/templates/drawer.html web/static/css/app.css
git commit -m "feat(web): phone-only back button in card drawer"
```

---

## Task 9: Compose modal full-screen on phone

**Files:**
- Modify: `web/static/css/app.css`

- [ ] **Step 1: Add full-screen compose rules**

Inside the existing `@media (max-width: 640px)` block in `web/static/css/app.css`, find the line `.compose-modal { max-width: none; margin: 0 8px; }` (added in Task 1 from the old 720px block). Replace it with:

```css
  .compose-modal {
    inset: 0;
    max-width: 100%;
    width: 100%;
    height: 100dvh;
    margin: 0;
    border-radius: 0;
  }
  .kbd-hint { display: none; }
```

- [ ] **Step 2: Build, restart, verify**

```bash
task build && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

At iPhone 14 Pro Max: there is no longer a way to open the desktop compose modal from the UI (the desktop nav with the Compose button is hidden, and `n` is a keyboard shortcut not available on phone). To test the styles, manually trigger the modal:

```javascript
// In Chrome devtools console:
Alpine.$data(document.body).openCompose()
```

Verify the compose modal fills the entire viewport with no border-radius and no outer margin. Press Escape (devtools-level keyboard) to close, or run `Alpine.$data(document.body).closeCompose()`.

At 1280×900: compose modal still appears centered with rounded corners on desktop. No regression.

- [ ] **Step 3: Commit**

```bash
git add web/static/css/app.css
git commit -m "feat(web): full-screen compose modal on phone"
```

---

## Task 10: Polish sweep — admin, archived, blockers, card permalink

**Files:**
- Modify: `web/static/css/app.css` (additions inside the 640 block)

- [ ] **Step 1: Add polish rules**

Inside the existing `@media (max-width: 640px)` block in `web/static/css/app.css`, append these rules near the end of the block (just before its closing `}`):

```css
  /* Hide desktop-only keyboard hints anywhere they appear */
  .kbd-hint, kbd { display: none !important; }

  /* Ensure all forms collapse to single column */
  .form-grid, .compose-grid, .drawer-meta, .admin-row { grid-template-columns: 1fr !important; }

  /* Larger labels and inputs read better at phone size */
  label, .field-label { font-size: 13px; }
  input[type="text"], input[type="email"], input[type="password"],
  input[type="search"], input[type="number"], textarea, select {
    width: 100%;
    box-sizing: border-box;
  }

  /* Cards in any vertical list (archived, blockers, considering pager) */
  .archived-list, .blockers-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
```

- [ ] **Step 2: Build, restart, sweep**

```bash
task build && go test ./... && pkill -f 'kkullm serve --addr :7733' 2>/dev/null ; ./kkullm serve --addr :7733 --db /tmp/kkullm-mobile-brainstorm.db &
sleep 1
```

At iPhone 14 Pro Max, visit each page and confirm:
1. `/admin` (and each admin subpage if there are tabs): sidebar collapses to horizontal chips, no horizontal scrolling, form inputs full-width, every text input renders ≥16px (no iOS auto-zoom on focus).
2. `/admin` → create-project / create-agent forms: each field on its own row, submit button reachable.
3. Tap any card → drawer fills screen, body text readable, comment composer fields full-width, status pills wrap or scroll horizontally without overflowing the viewport.
4. Card permalink `/c/<id>`: page renders, no horizontal overflow, drawer-like display.
5. `/archived?project=…` currently 400s (pre-existing bug, see spec). Page renders error response without breaking the layout.

At 1280×900: spot-check the same pages — no regressions.

- [ ] **Step 3: Commit**

```bash
git add web/static/css/app.css
git commit -m "feat(web): phone polish for admin, drawer, and form pages"
```

---

## Task 11: Final verification sweep

**Files:** none modified.

- [ ] **Step 1: Run the full Go test suite**

```bash
go test ./...
```

Expected: all green.

- [ ] **Step 2: Visual sweep at four viewport sizes**

| Width × Height | Mode | Expected |
|---|---|---|
| 390 × 844 | iPhone 14 | Phone layout: top bar, picker, pager, FAB |
| 430 × 932 | iPhone 14 Pro Max | Same |
| 768 × 1024 | iPad portrait | **Desktop layout** (>640px) |
| 1280 × 900 | Desktop | Desktop layout, no regressions |

At each phone size, walk through:
1. Open picker → swipe between Projects and Agents → select each item → board reloads.
2. Open overflow → tap Admin → navigate back → tap Theme → confirm flip.
3. Board pager: swipe through all 6 status columns → chevrons advance one column → reload page lands on the last column.
4. Tap a card → drawer opens full-screen → tap back → drawer closes.
5. Tap FAB → enter title → submit → toast appears → card visible in Considering.
6. Switch project via picker, repeat 3–5 in another project.

At each desktop size, confirm:
- Existing nav unchanged.
- Board lays out horizontally as before.
- Drawer slides from the right (not full-screen).
- Compose modal is centered with rounded corners.

- [ ] **Step 3: Confirm done**

If anything from Step 2 fails, fix it in a follow-up commit with a clear `fix(web): …` message. Otherwise, the feature is complete.

---

## Spec coverage cross-check

| Spec section | Task(s) |
|---|---|
| Viewport meta + `viewport-fit=cover` | Task 1 |
| Single `@media (max-width: 640px)` breakpoint | Task 1 |
| 44px tap targets + 16px input font-size | Task 1, Task 10 |
| Top bar with small 끌림 mark, view name, ⋮ | Task 2 |
| Picker bottom sheet with swipeable Projects/Agents pages, page label tabs, dots, arrow hints | Task 3 |
| Picker page persistence in localStorage | Task 3 |
| Overflow menu (Archive, Admin, Theme), no Search, no Blockers | Task 4 |
| Phone board: all status columns including Blocked when empty | Task 5 |
| Phone board: pager with scroll-snap, sticky status header, dots, chevrons, IntersectionObserver | Task 6 |
| Landing column heuristic (blocked → in_flight → todo) on first visit only | Task 6 |
| Last-column persistence per scope | Task 6 |
| Quick-capture FAB + sheet, status fixed to `considering`, autofocus title | Task 7 |
| Project defaults to current project | Task 7 |
| Card drawer: phone-only back button | Task 8 |
| Compose modal full-screen on phone | Task 9 |
| Admin / archived / blockers polish | Task 10 |
| Manual sweep + Go tests at four sizes | Task 11 |
| Archive entry inherits the desktop 400 bug | Spec note; no task required (out of scope per spec) |
