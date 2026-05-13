# Drawer Scroll Layout — Design

**Date:** 2026-05-13
**Status:** Approved, ready for implementation plan
**Follows:** `2026-05-13-web-ui-comment-composer-design.md`

## Problem

After the comment composer landed, a card with many comments pushes the
composer (and the most recent comments) off the bottom of the viewport.
Scrolling the drawer to reach the composer hides the card's header, title,
and status — the parts you most want visible while writing a comment.

## Goals

- The card identity (id, title, status, meta, body) stays visible at all
  times.
- The comment composer (textarea + button) stays visible at all times.
- The most recent comments sit just above the textarea by default.
- When more comments exist than fit, the user can scroll up to read older
  ones; a subtle fade at the relevant edge hints there's more.

## Non-Goals (for this iteration)

- **SSE-driven auto-scroll when a new comment arrives from another client.**
  Deferred. See "Deferred Work" below — the human user explicitly wants
  this, just not now.
- Edit/delete comments.
- Markdown rendering, reactions, threading.
- Responsive layout changes for narrow viewports (current drawer is
  desktop-only).

## Design

### Drawer becomes a three-row flex column

The drawer's outer container becomes a vertical flex container that fills
its parent's height. It has exactly three children, in order:

1. **`.drawer-top`** — header, status, meta, relations, body. Sized by its
   content (`flex: 0 0 auto`).
2. **`.drawer-comments`** — comments list. `flex: 1 1 0`, `overflow-y:
   auto`, `min-height: 120px` so the list always has at least some room.
3. **`.drawer-composer`** — comment form. Sized by its content (`flex: 0 0
   auto`).

The existing `#drawer-container` parent must already (or now must)
establish a flex column that fills the viewport. We will verify the
existing styles and adjust as needed; we will not introduce a fixed pixel
height.

If the card body is long, `.drawer-top` grows naturally — the comments
area shrinks but never below `min-height: 120px`. Past that, `.drawer-top`
itself becomes scrollable internally (a single `overflow-y: auto` on
`.drawer-top` is enough; it only kicks in when the top would otherwise
exceed available height).

### Scroll-to-bottom on every drawer render

Each time the drawer re-renders (initial open, status change, comment
added, error path), the `.drawer-comments` scroll position must end at the
bottom — newest comments just above the composer.

Implementation: an inline `<script>` at the end of the drawer template
that scrolls `.drawer-comments` to `scrollHeight`. Because HTMX evaluates
inline scripts in swapped HTML by default, this runs on every swap. The
script is idempotent and small (one line).

### Scroll-shadow fade

When `.drawer-comments` is scrollable:

- If scrolled past the top, fade the top edge to the drawer background
  color (signals "older comments above").
- If not scrolled to the bottom, fade the bottom edge (signals "newer
  comments below").

Implementation: two CSS pseudo-elements (`::before` and `::after`) on
`.drawer-comments`, positioned `sticky` at top: 0 and bottom: 0 (or
absolutely positioned with the comments list as the positioning context),
showing a vertical linear-gradient from the drawer background color to
transparent. Visibility is toggled by two classes on `.drawer-comments`:
`has-shadow-top` and `has-shadow-bottom`.

A small JS handler (vanilla, no framework dependency) on `.drawer-comments`
updates those classes on `scroll` and on initial layout. The script lives
in the same inline block as the scroll-to-bottom code and runs on every
HTMX swap.

The gradient color resolves from the drawer's background CSS variable so
it Just Works in light and dark mode (whatever variable name the existing
CSS uses; the implementation will use it directly rather than hardcoding
`#fff` / `#000`).

Approximate dimensions (subject to taste during implementation): ~24 px
tall, opaque-to-transparent linear gradient.

### Form behavior unchanged

The composer's HTMX behavior, validation, and error rendering are
unchanged. Only its position in the DOM moves (from inside the comments
section to a sibling row below it).

## Affected files

- `web/templates/drawer.html` — restructure into three rows; move form
  outside comments section; append inline script.
- `web/static/css/app.css` — new layout rules for `.drawer-top`,
  `.drawer-comments`, `.drawer-composer`, the two scroll-shadow
  pseudo-elements, and the toggle classes.
- Possibly the parent drawer container's existing CSS (whatever currently
  styles `#drawer-container` / the drawer aside) — adjust to establish the
  flex column context if it doesn't already.

## Testing

- Existing handler/template tests continue to pass (the change is layout
  only; rendered comment content and error path don't change).
- One new template test: assert the rendered drawer contains
  `.drawer-comments` and `.drawer-composer` as siblings (a regression
  guard that the composer didn't get re-nested back inside the comments
  section).
- Manual smoke check: open a card with many comments, confirm header is
  fixed, list scrolls, composer is always visible, fades render correctly,
  list defaults to bottom on each render.

## Deferred Work (track separately)

**SSE-driven auto-scroll behavior** — when a new comment arrives via SSE
on the open drawer:

- If the user is scrolled to the bottom (or within a small threshold),
  auto-scroll to include the new comment.
- If the user is scrolled up reading older comments, do NOT yank them; the
  new comment just appears below, and the bottom scroll-shadow signals
  fresh content.

This requires the drawer to subscribe to the SSE stream (or for the
existing SSE subscriber to target the open drawer with an HTMX OOB
swap or similar). Implement in a follow-up after the SSE stream's role in
the web UI is firmed up.
