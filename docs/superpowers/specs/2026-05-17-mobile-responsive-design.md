# Mobile-Responsive Kkullm — Design

**Status:** Draft, awaiting user review
**Date:** 2026-05-17
**Scope:** Make the Kkullm web UI usable on phones (≤640px) without disrupting the desktop experience.

## Goals

- The web UI works well on a phone in portrait orientation (320–640px wide).
- The desktop UI is untouched at viewports > 640px.
- Tablets and resized desktop windows fall back gracefully on the same desktop layout (no special tablet treatment).
- Primary mobile use cases, in priority order:
  1. Read the board, read individual cards.
  2. Comment on cards and change card status.
  3. **Frictionless "add to considering"** quick-capture from anywhere.
  4. Browse archive / admin / blockers as needed (functional, not necessarily pretty).

## Non-goals

- Native app, PWA installability, or offline support.
- Search (does not exist yet on desktop either).
- Fixing the archive view (desktop button currently 400s — out of scope; mobile inherits the bug and the eventual fix).
- Tablet-specific layouts.
- Drag-and-drop card reordering on touch.

## Approach

**CSS-only, single template tree, with one new breakpoint.** A new `@media (max-width: 640px)` block in `web/static/css/app.css` carries all phone-specific styles. The two existing `(max-width: 720px)` blocks are consolidated into the new 640px block. Markup gains a small number of new partials (top bar, picker sheet, overflow sheet, quick-capture) that are hidden on desktop and shown on phone via CSS.

Alternatives considered and rejected:
- **Separate mobile templates** (route by UA or cookie) — doubles template surface, fragile sniffing.
- **Mobile-first rewrite** — disruptive to desktop users for a feature aimed at on-the-go use.

## Viewport & breakpoint

```html
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
```

- Single breakpoint: `@media (max-width: 640px)` = "phone mode."
- `viewport-fit=cover` enables painting behind the notch; `env(safe-area-inset-*)` paddings keep the top bar and FAB clear.
- All tap targets in phone mode: `min-height: 44px` (Apple HIG / WCAG).
- Form inputs in phone mode: `font-size: 16px` to suppress iOS auto-zoom.

## Components

### Top bar (phone)

Replaces `.nav` at ≤640px. 56px tall + top safe-area inset. Sticky.

```
┌──────────────────────────────────────┐
│  끌림  beehive ⌃              ⋮     │
└──────────────────────────────────────┘
```

- **Left:** small `끌림` brand mark (smaller than desktop, no "kkullm" wordmark).
- **Center-left:** the current view name (project or agent) with a down-chevron. Tap → opens the picker bottom sheet. The view name comes from whichever picker page the user last selected from.
- **Right:** overflow button (`⋮`). Tap → opens the overflow menu sheet.
- The desktop "Project | Agent" segmented toggle does not appear on phone — the picker subsumes it.

### Picker bottom sheet (project ↔ agent)

A bottom-anchored sheet, ~70% of viewport height, with a drag handle and backdrop. Two horizontally swipeable pages.

```
┌──────────────────────────────────────┐
│                ▬▬                    │
│   ◀     Projects   ●  Agents     ▶   │
├──────────────────────────────────────┤
│   ⦿ beehive                          │
│   ○ birds_nest                       │
│   ○ ant_hill                         │
│   …                                  │
└──────────────────────────────────────┘
```

- Two pages laid out in a horizontally scroll-snapping container (`scroll-snap-type: x mandatory`).
- Page label tabs at top are also tappable (fallback for users who don't swipe).
- Chevron arrows at the edges pulse subtly on first open, then dim.
- A two-dot indicator shows the active page.
- Each row is 56px tall: name + minor meta (e.g. card count), checkmark on the currently active item.
- Selecting an item closes the sheet, updates the top bar's view name, and reloads the board for that scope.
- The "kind" (project vs agent) is implicit in which page was last used.
- **Persistence:** the last picker page (projects vs agents) is remembered in `localStorage`.
- Data sources: the same JSON endpoints the desktop selectors already use.

### Overflow menu sheet

Same chrome as the picker but single-page and shorter. Vertical list of 44px rows.

```
┌──────────────────────────────┐
│           ▬▬                 │
│  🗂  Archive                 │
│  ⚙   Admin                   │
│  ────────────────────────    │
│  🌓  Theme                   │
└──────────────────────────────┘
```

- Each row links to an existing route. No new endpoints.
- "Blockers" is **not** in this menu — Blockers is a board column, not a separate destination.
- Search is not present (no search feature exists yet).
- Theme toggle matches whatever the desktop currently exposes.

### Board pager

The board (`/`) on phone is a horizontally-paginated set of full-width status columns, with a sticky status header above the pager.

```
┌──────────────────────────────────────┐
│  ◀     TODO (7)    ▶   ● ○ ○ ○ ○    │  ← sticky status header
├──────────────────────────────────────┤
│  ┌──────────────────────────────┐   │
│  │ Card title                   │   │
│  │ assignee · 3d                │   │
│  └──────────────────────────────┘   │
│  ┌──────────────────────────────┐   │
│  │ Card title                   │   │
│  └──────────────────────────────┘   │
└──────────────────────────────────────┘
```

CSS changes (phone-mode only):
- `.board` gets `scroll-snap-type: x mandatory` and appropriate `scroll-padding`.
- Each `.column` becomes `flex: 0 0 100%` with `scroll-snap-align: center`.
- A new sticky status-header bar sits *outside* the scroll area: shows the current column name and count, with `◀ ▶` chevrons (tappable) and a dots indicator.

Behavior:
- **The same set of status columns as the desktop board appears in the pager, with one difference: Blocked is always present even when empty.** (Desktop hides Blocked when empty.)
- **Landing column on first visit per project/agent:** Blocked if any, else In Flight if any, else Todo.
- **Subsequent visits:** the last column the user viewed (stored per-project/agent in `localStorage`) overrides the heuristic.
- Chevrons call `el.scrollIntoView({ behavior: 'smooth', inline: 'center' })` on the next/prev column.
- An `IntersectionObserver` keeps the sticky header and dots indicator in sync with which column is currently snapped.

### Card detail (drawer)

Already goes full-width at ≤720px via the existing media query. Phone-mode tweaks:
- Clear back/close button in the drawer header (44px tap target, left-arrow icon) since the drawer covers the entire viewport on phone.
- Comment composer textarea grows naturally; submit button full-width.
- Status-change controls become a horizontal row of pill buttons if not already touch-friendly.

### Quick-capture FAB + sheet

A floating circular `+` button, 56×56px, bottom-right, with safe-area padding. Visible on all phone-mode pages (board, archived, blockers, admin, card detail).

Tap → small bottom sheet (intentionally minimal):

```
┌──────────────────────────────┐
│           ▬▬                 │
│  Title                       │
│  ┌────────────────────────┐  │
│  │                        │  │
│  └────────────────────────┘  │
│                              │
│  Project:  beehive ▾         │
│  Status:   considering       │
│                              │
│  [ Cancel ]    [ Add card ]  │
└──────────────────────────────┘
```

- **Title** is the only required field. Autofocus on open (keyboard opens immediately).
- **Project** defaults to the current project view, or last-used if the user is in agent view. Tap to change.
- **Status** is fixed to `considering`. Displayed as a label, not editable.
- Submit posts to the existing card-create endpoint with empty body/tags/assignee.
- After submit: brief toast `"Added to considering"`, sheet closes, FAB returns. No navigation.

### Full compose modal

Distinct from the FAB quick-capture. On phone it becomes a **full-screen sheet** (slides up, occupies the full viewport minus safe-area). The existing 720px media query already makes it full-width; the new 640px block extends it to full-height.

### Admin

In scope only to the extent that it doesn't break.
- Existing media query already stacks the admin sidebar above the content as horizontal chips.
- Multi-column form grids collapse to single column.
- Tap targets and form inputs get the same 44px / 16px treatment as everywhere else.

No information-architecture changes for admin.

### Other pages

- **`archived.html`** — list view; stack to single column, larger row heights. (Fix to the underlying 400 inherits when desktop is fixed.)
- **`blockers.html`** — same stacking rules as board cards in the pager.
- **`card.html`** (standalone permalink) — same drawer-like rendering at full width.

## Files touched

New partials:
- `web/templates/_topbar_mobile.html`
- `web/templates/_picker_sheet.html`
- `web/templates/_overflow_sheet.html`
- `web/templates/_quick_capture.html`

Modified:
- `web/templates/layout.html` — viewport meta, render new partials inside an Alpine root exposing `pickerOpen`, `pickerPage`, `overflowOpen`, `quickCaptureOpen`, `boardCol`.
- `web/templates/board.html` — wrap columns in a pager scroller and include the sticky status header.
- `web/static/css/app.css` — one new `@media (max-width: 640px)` block; consolidate the two existing 720px blocks into it.
- `web/static/js/app.js` — Alpine helpers for picker swipe state, board column tracking via `IntersectionObserver`, quick-capture submit, `localStorage` persistence for last-column-per-scope and last-picker-page.

Untouched at viewports > 640px:
- Desktop nav, board, drawer, full compose modal, admin sidebar.

## Testing & verification

- **Manual sweep at four viewport sizes:**
  - 390×844 (iPhone 14) — phone mode.
  - 430×932 (iPhone 14 Pro Max) — phone mode.
  - 768×1024 (iPad portrait) — should fall through to desktop layout with no regressions.
  - 1280+ (desktop) — no regressions.
- **Existing Go tests** in `web/handlers_test.go` remain green (no handler changes).
- **No new JS test framework**; the project has none and adding one is out of scope.
- Manual sweep covers: nav rendering, picker open/close/swipe, overflow open/close, board column snap + sticky header sync, drawer open/close + back button, quick-capture submit (creates a card in `considering`), full compose on phone, admin pages render.

## Open questions

None at design time. Will surface during implementation if discovered.
