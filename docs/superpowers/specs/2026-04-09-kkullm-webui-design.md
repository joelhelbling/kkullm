# Kkullm Web UI Design — v1

## Overview

The web UI is a single-page application embedded in the same Go binary that serves the REST API. It provides a kanban board interface for humans to observe and interact with cards managed by AI agents. The UI consumes HTML fragments from a `/ui/` route namespace and receives real-time updates via Server-Sent Events.

## Tech Stack

- **htmx** — HTML fragment swaps for all server interactions
- **Alpine.js** — local UI state (drawer open/close, dropdown toggles, dark mode)
- **SortableJS** — drag-and-drop between kanban columns
- **CSS custom properties** — theming (light/dark mode)
- **Go `html/template`** — server-side rendering of HTML fragments
- **`//go:embed`** — static assets and templates bundled into the binary

No Node.js build toolchain. No bundler. Vendor JS files committed directly.

## Architecture

### Route Namespaces

The Go server serves three namespaces on the same mux:

| Namespace | Purpose | Consumers |
|-----------|---------|-----------|
| `/api/` | JSON REST API | CLI, agents, external tools |
| `/ui/` | HTML fragment endpoints | Browser (htmx) |
| `/static/` | CSS, JS, vendor files | Browser |
| `/` | Full page shell | Browser (initial load) |

The existing `/api/` endpoints are unchanged. The `/ui/` namespace is new and returns HTML fragments rendered by Go templates. Both namespaces share the same `store.Store` for data access.

### File Structure

```
web/
├── web.go              # //go:embed directive, RegisterRoutes(mux, store)
├── handlers.go         # /ui/ route handlers
├── templates/
│   ├── layout.html     # Full page shell (nav, board container, drawer container, scripts)
│   ├── board.html      # Kanban board (columns with card tiles)
│   ├── card.html       # Single card tile (used inside columns)
│   ├── drawer.html     # Card detail drawer content
│   └── blockers.html   # Blocked column fragment
├── static/
│   ├── css/
│   │   └── app.css     # All styles, CSS custom properties for theming
│   ├── js/
│   │   └── app.js      # Alpine components, SortableJS init, SSE handling, dark mode
│   └── vendor/
│       ├── htmx.min.js
│       ├── alpine.min.js
│       ├── sortable.min.js
│       └── htmx-sse.js
```

`web.go` embeds `static/` and `templates/` via `//go:embed`. It exports `RegisterRoutes(mux *http.ServeMux, s *store.Store, events *api.EventBus)` which `cmd/serve.go` calls to mount all web routes on the existing mux.

### Server Integration

`cmd/serve.go` imports the `web` package and calls `RegisterRoutes` after registering API routes. The `EventBus` is passed to the web package so that `/ui/` status-change handlers can publish SSE events (same bus the API handlers use). No changes to the store or model packages. The `api.EventBus` type may need to be exported if it isn't already.

## Routes

### Full Page

```
GET /                    → layout.html
```

The page shell contains the nav bar, an empty board container (populated on load via htmx), a hidden drawer container, and script/style tags. The board container has `hx-get="/ui/board?project=1" hx-trigger="load"` to immediately fetch the default project board.

### HTML Fragment Endpoints

```
GET  /ui/board?project=X          → board.html (project-scoped kanban)
GET  /ui/board?agent=X            → board.html (agent-scoped kanban)
GET  /ui/cards/{id}/drawer        → drawer.html (card detail)
GET  /ui/cards/{id}/status        → status selector fragment
PATCH /ui/cards/{id}/status       → updated card.html (form-encoded input, not JSON)
GET  /ui/blockers                 → blockers.html (blocked column content)
```

### Static Files

```
GET /static/*                     → embedded CSS, JS, vendor files
```

## Views

### 1. Project Board

A kanban board showing all cards in a single project, organized by status columns.

**Columns (left to right):** `considering`, `todo`, `in_flight`, `completed`

**Closed statuses** (`done`, `tabled`) are hidden by default. A toggle at the bottom of the board reads "▶ Show closed (done: N, tabled: N)" and expands to show those columns when clicked.

**Card tiles** display:
- Title (clickable, opens drawer)
- Tag pills (color-coded)
- Assignee name(s) (bottom left)
- Comment count with 💬 icon (bottom right, **only shown if > 0**)
- Relation count with 🔗 icon (only shown if > 0)
- In-flight cards get a left border accent (green)

**Column headers** show the status name (uppercase) and a count badge.

**Drag-and-drop:** SortableJS enables dragging cards between columns. On drop, the JS reads the target column's status and fires `hx-patch="/ui/cards/{id}/status"` with the new status. The server validates the transition. On success, it returns the updated card tile. On invalid transition, it returns an error message displayed as a toast, and the card snaps back to its original column.

### 2. Agent Board

Same kanban layout as the project board, but scoped to a single agent across all projects.

**Differences from project board:**
- Each card tile shows a **project-of-origin badge** at the top — a small colored pill with the project name.
- Badge colors are auto-assigned from a preset palette using consistent hashing on the project name. No configuration needed. Colors remain stable across page loads.
- Nav bar shows agent name instead of project name.
- Drag-and-drop works identically; status transitions are still validated.
- Blockers badge still present, same global behavior.

### 3. Card Detail Drawer

A slide-out panel from the right side of the screen. The board remains visible but dimmed behind it.

**Drawer sections (top to bottom):**

1. **Header** — Card ID (`#42`), project name, title, close button (✕)
2. **Status selector** — Current status shown as a highlighted pill. Valid transitions shown as clickable pills (only valid next states per the state machine). Clicking a transition pill fires `hx-patch="/ui/cards/{id}/status"` and refreshes the drawer.
3. **Metadata** — Assignees, tags, created time (relative)
4. **Relations** — Listed by type (`blocked_by`, `belongs_to`, `interested_in`) with linked card titles. Clicking a related card opens that card's drawer (replaces current drawer content).
5. **Description** — Card body text
6. **Comments** — Flat thread, chronological (oldest first). Each comment shows agent name, relative time, and body. The "user" agent name is highlighted in a distinct color (blue). Comments update in real-time via SSE.

**Opening/closing:** Alpine.js manages drawer visibility state. Clicking a card tile triggers `hx-get="/ui/cards/{id}/drawer"` which swaps content into the drawer container, then Alpine transitions the drawer open. Clicking ✕ or the dimmed board area closes it.

### 4. Blockers Column

Not a separate page — an inline column that appears on the board when activated.

**Badge:** The nav bar shows a "Blocked! (N)" badge with red background, visible whenever there are blocked cards (count > 0). **Badge is completely hidden when count is zero.**

**Toggle interaction:**
1. User clicks "Blocked! (N)" badge.
2. The `todo` and `in_flight` columns animate apart (CSS transition).
3. A `blocked` column slides in between them.
4. Badge text changes to "Hide Blocked ✕".
5. Clicking again collapses the column and restores the original layout.

**Column styling:**
- Red border (2px solid)
- Light red background (`#fff5f5` light / adjusted for dark mode)
- Subtle red box shadow/glow
- Warning icon (⚠) in column header

**Column content:**
- Shows ALL blocked cards across ALL projects (this is a global view of what needs user attention).
- Each card shows a project-of-origin badge (same color-coding as agent board).
- Each card shows its blocking reason: the `blocked_by` relation target (e.g., "blocked_by #40") or a brief context line.
- Cards are clickable — opens the drawer, same as any board card.

**Auto-behavior:**
- When an SSE event changes a card to `blocked` and the column isn't visible: the column automatically slides open, badge count increments.
- When the last blocked card is resolved (status changed away from `blocked`): the column automatically slides closed and the badge disappears entirely.
- The blocked column is not draggable and cards cannot be dragged into or out of it (status changes to/from `blocked` happen via the drawer's status selector).

**Semantic note:** `blocked` status is an escalation mechanism — agents use it to get the user's attention. Blocked cards are always implicitly assigned to the user. The blockers view is the user's "inbox" for unblocking agent work.

## Navigation

### Nav Bar (Top, Horizontal)

**Left side:**
- **Logo/wordmark:** "끌림 kkullm"
- **Project/Agent dropdown:** Shows current project name (project view) or agent name (agent view). Selecting a different project/agent fires `hx-get="/ui/board?project=X"` or `hx-get="/ui/board?agent=X"` to swap the board.
- **View switcher:** Dropdown to toggle between "Project Board" and "Agent Board" views. Switching view changes the other dropdown to show agents or projects accordingly.

**Right side:**
- **Blockers badge:** "Blocked! (N)" — red pill, only visible when N > 0. Click to toggle blocked column.
- **Dark mode toggle:** Sun (☀) in dark mode, moon (🌙) in light mode. Click to toggle.

## Real-Time Updates (SSE)

### Connection

The page shell establishes an `EventSource` connection to `/api/events` on load. Managed in `app.js` with custom event handling (not htmx's SSE extension — we need per-event-type logic and FLIP animations).

### Event Handling

| SSE Event | Board Response |
|-----------|---------------|
| `card_created` | New card tile fades in + slides down at the top of the appropriate column. Column count badge updates. |
| `card_updated` (status change) | Card slides from old column to new column using FLIP animation. Brief highlight glow (yellow → transparent, ~1.5s) on arrival. |
| `card_updated` (other fields) | Card tile pulses with highlight glow. Content updates in place. |
| `card_deleted` | Card fades out. Column count updates. |
| `comment_created` | If drawer is open for that card: new comment appends with slide-in animation. Comment count on card tile updates (appears if going from 0 to 1). |

### FLIP Animation for Status Changes

1. SSE event arrives with updated card (new status).
2. JS finds card element by `data-card-id` in old column.
3. **First:** Record card's current bounding rect.
4. **Last:** Move card to new column in DOM, record new bounding rect.
5. **Invert:** Apply CSS transform to make card appear at old position.
6. **Play:** Animate transform to zero (card visually slides to new column).
7. Apply highlight glow on arrival.

### Blocked Column Auto-Management

- SSE `card_updated` with `status: "blocked"`: increment badge count. If blocked column is hidden, auto-open it.
- SSE `card_updated` where card was previously `blocked` and new status is not: decrement badge count. If count reaches 0, auto-close column and hide badge.

### Drawer Updates

If the drawer is open and an SSE event updates that card, the drawer content refreshes via `hx-get="/ui/cards/{id}/drawer"`. Scroll position in the comment thread is preserved.

## Theming

### Light Mode (Default)

Clean, light aesthetic inspired by GitHub's light theme. White surfaces, subtle gray borders, readable contrast.

### Dark Mode

Dark aesthetic inspired by GitHub's dark theme. Dark surfaces, subtle borders, soft accent colors.

### Implementation

CSS custom properties on `:root` for light mode, overridden on `[data-theme="dark"]`.

**Key tokens:**

| Token | Light | Dark |
|-------|-------|------|
| `--bg-page` | `#f0f2f5` | `#0d1117` |
| `--bg-surface` | `#ffffff` | `#161b22` |
| `--bg-card` | `#f8f9fa` | `#1c2128` |
| `--border` | `#e1e4e8` | `#30363d` |
| `--text-primary` | `#24292f` | `#e6edf3` |
| `--text-secondary` | `#656d76` | `#7d8590` |
| `--accent-blocked` | `#dc3545` | `#f85149` |
| `--accent-inflight` | `#1a7f37` | `#3fb950` |

Status badge colors and project badge colors also use CSS custom properties, slightly muted in dark mode.

### Toggle Behavior

1. On page load, check `localStorage` for saved preference.
2. If none, read `window.matchMedia('(prefers-color-scheme: dark)')`.
3. Set `data-theme` attribute on `<html>`.
4. Nav toggle icon: ☀ in dark mode, 🌙 in light mode.
5. Click toggles attribute and saves to `localStorage`.
6. `matchMedia` change listener updates automatically if OS preference changes and user hasn't manually overridden.

## Interaction Flows

### Page Load
1. Browser requests `/` → server returns `layout.html` (full shell).
2. Shell loads CSS, vendor JS, `app.js`.
3. Alpine initializes components (drawer state, nav dropdowns, dark mode).
4. Board container's `hx-trigger="load"` fires `hx-get="/ui/board?project=1"` → board HTML swaps in.
5. `app.js` initializes SortableJS on each column.
6. `EventSource` connects to `/api/events`.

### Project Switch
1. User selects project from dropdown.
2. `hx-get="/ui/board?project=X"` fires → new board HTML swaps in.
3. SortableJS re-initializes on new columns.
4. Blocked column state resets (closes if open).

### Card Click → Drawer
1. Card tile has `hx-get="/ui/cards/{id}/drawer"`.
2. Response swaps into drawer container.
3. Alpine transitions drawer open (slide from right).
4. Board dims (CSS overlay).

### Drag-and-Drop Status Change
1. User drags card from `todo` column to `in_flight` column.
2. SortableJS `onEnd` fires.
3. JS reads target column's `data-status` attribute.
4. Fires `hx-patch="/ui/cards/{id}/status"` with `{status: "in_flight"}`.
5. Server validates transition, updates card, publishes SSE event, returns updated card tile HTML.
6. On success: card stays in new column with fresh HTML.
7. On failure (invalid transition): card snaps back to original column, error toast appears.

### Status Change in Drawer
1. User clicks a valid transition pill in the status selector.
2. `hx-patch="/ui/cards/{id}/status"` fires with new status.
3. Server validates, updates, publishes SSE event, returns updated drawer HTML.
4. Drawer refreshes with new status highlighted.
5. Board also updates via the SSE event (card moves columns).

### Blockers Toggle
1. User clicks "Blocked! (3)" badge.
2. Alpine sets `blockersOpen = true`.
3. CSS transition: `todo` and `in_flight` columns slide apart.
4. `hx-get="/ui/blockers"` fires → blocked column HTML swaps in between them.
5. Badge changes to "Hide Blocked ✕".
6. Click again: Alpine sets `blockersOpen = false`, column slides out, badge reverts.

## Out of Scope for v1

- Card create/edit/delete via web UI (CLI only)
- Agent create/edit via web UI (CLI only)
- Project create/edit via web UI (CLI only)
- Asset management via web UI (CLI only)
- Comment posting via web UI (read-only comment thread; agents post via CLI)
- Search or filtering beyond project/agent scope
- Responsive/mobile layout
- Keyboard shortcuts
- URL routing / deep linking to specific cards
