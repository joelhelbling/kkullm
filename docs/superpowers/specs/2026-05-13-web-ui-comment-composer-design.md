# Web UI Comment Composer — Design

**Date:** 2026-05-13
**Status:** Approved, ready for implementation plan

## Problem

The web UI's card drawer displays comments but offers no way to add one.
Comment creation is fully implemented in the store, HTTP/JSON API, and CLI
(`kkullm comment add`), so the gap is purely in the server-rendered web UI.

## Goals

- Let a human user post a comment on any card from the drawer.
- Reuse existing HTMX/SSE patterns; no new client framework or JS dependency.
- Keep the implementation small and consistent with the existing web handlers.

## Non-Goals

- Authentication or multi-user identity. The system presumes a single human
  operator; multi-user auth is deferred.
- Editing, deleting, threading, reactions, or markdown rendering of comments.
- Auto-growing textarea (planned as a follow-up polish).

## Design

### Author identity

All web-originated comments are attributed to a fixed agent named `user`.

- The `user` agent is seeded as part of schema initialization, so it is
  guaranteed to exist for any operational instance.
- The web handler resolves the agent on each POST via
  `store.GetAgentByName("user")`. A missing agent is a server-config error
  (500); it should never occur in practice.
- The existing drawer template already applies special CSS to comments whose
  `Agent == "user"` (`comment-agent user-agent`), so this convention is
  already implicit in the codebase.

### Route and handler

Add one route in `web/web.go`:

    POST /ui/cards/{id}/comments  →  ws.handleAddComment

`handleAddComment` (in `web/handlers.go`) mirrors the structure of
`handleStatusChange`:

1. Parse `id` from the path; 404 on bad/missing card.
2. Read `body` from `r.FormValue("body")`; trim whitespace.
3. If body is empty, re-render the drawer with an inline `CommentError` and
   return; do not insert a row.
4. Resolve the `user` agent via `store.GetAgentByName("user")`.
5. Call `store.CreateComment(cardID, agent.ID, body)`.
6. Publish a `comment_created` event on `ws.events` (matches the API
   handler; keeps SSE behavior consistent for other connected clients).
7. Re-render the full drawer (card + comments) and return the HTML. HTMX
   swaps `#drawer-container`.

The web handler talks to `store` directly; it does not go through the JSON
API. This matches the pattern used elsewhere in `web/handlers.go`.

### Template changes

In `web/templates/drawer.html`, add a comment composer at the bottom of the
Comments section:

```html
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
```

Notes:

- `required` provides browser-side empty-check; the server trims and
  re-validates.
- Multi-line `<textarea>` (3 rows initial, `resize: vertical`). Enter inserts
  a newline; the user submits via the button.
- After a successful swap, the drawer is re-rendered with a fresh empty
  textarea and the new comment in the list. The header count
  `Comments ({{len .Comments}})` increments naturally.

### DrawerData

`DrawerData` in `web/handlers.go` gains an optional field:

```go
CommentError string
```

It is set only on the empty-body re-render path; in all other cases the
existing drawer renders unchanged.

### Styling

Add minimal CSS to the existing stylesheet for `.comment-form`,
`.comment-form-actions`, `.form-error`, and the textarea. Visually
consistent with the existing `.comment` block.

### Error and edge-case behavior

- **Empty body (after trim):** server re-renders the drawer with an inline
  `form-error`. No row inserted. No event published.
- **Bad/missing card id:** 404 plain text. HTMX swap will display the error
  message in `#drawer-container`. The drawer is meaningless without a
  card, so this is acceptable.
- **Missing `user` agent:** 500 with a clear message. Indicates a broken
  install; not user-correctable.
- **Concurrent edits:** comments are append-only; no conflict possible.

### Schema seeding

Schema initialization seeds a single agent row with `name = 'user'`
(idempotent — only inserted if not already present). Implementation
detail: this happens in the same place migrations run today.

## Testing

- `web/handlers_test.go`:
  - happy path — POST adds a comment, re-rendered HTML contains the new
    comment, `comment_created` event published.
  - empty-body path — re-rendered drawer contains the error message, no
    new row inserted, no event published.
  - bad-id path — 404.
- `db/db_test.go` (or wherever schema init is tested):
  - `user` agent exists after fresh schema init.
  - Re-running init is idempotent (no duplicate `user` agent).

`store/comment_test.go` already covers `CreateComment`; no new store tests.

## Out of Scope / Future Work

- Auto-growing textarea (option C from brainstorming).
- Edit/delete comments.
- Markdown rendering.
- Reactions, threading.
- Real auth / multi-user identity (will eventually replace the fixed
  `user` agent convention).
