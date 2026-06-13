# Remove Status-Transition Rules and Force/Alt-Drag — Design

**Date:** 2026-06-13
**Status:** Approved (design)

## Summary

Remove the card status-transition rules engine and everything that exists to
bypass it: the `--force` CLI flag, the `?force=1` web/API parameter, the
web-UI Alt-drag detection, and the `forced` audit distinction. After this
change any status may transition to any other status; the only remaining
status-change validation is that the target is a real status (typos are still
rejected). The append-only card-event audit log is the sole record of moves.

The blocked-card feature is **orthogonal** to transition rules and is kept
entirely intact.

## Motivation

The transition rules were intended to prevent "nonsensical" moves. In practice
they create friction (every legitimate-but-unanticipated move requires a force
override), and the Alt-drag force mechanism in the web UI has proven
unreliable across browsers despite repeated fixes (issues #62, #69, #71). Now
that every status change is recorded in the card-event audit trail, the log
provides accountability without the rules needing to prevent moves. Removing
the rules removes both the friction and the fragile force machinery.

## Scope

### In scope (removed)
- The transition matrix and its helpers: `ValidTransitions`, `CanTransition`,
  `AllowedTransitions`.
- The `Force` parameter end-to-end: CLI `--force`, API JSON `force`, web
  `?force=1`, the store `Force` branch, and the client request field.
- The web-UI Alt-drag machinery: `altHeld` state, the keydown/keyup/blur
  listeners, the `onStart`/`onMove` altKey capture, and the `?force=1` query
  construction in `onCardDrop`.
- The `forced` audit distinction: the `Forced` field on `CardEvent`, its
  read/write in the store, and the `forced` DB column (via migration).

### Explicitly kept
- **`ValidStatuses`** validation: an unknown status (e.g. a typo) is still
  rejected with **HTTP 422** on the web and the equivalent error on the API/CLI.
- **The blocked feature**, unchanged: the `Blocked` flag, the block/unblock
  forms and handlers, `?unblock=1`, and the 3-way "Unblock and move / Keep
  blocked / Cancel" modal (issue #60). Dragging a *blocked* card still prompts
  the modal.
- **The status-change audit events**: every move continues to write a
  `status_changed` `CardEvent` (now always un-forced).
- **The `_pendingDrops` self-echo suppression** (issue #69): still required —
  any successful drop can trigger a redundant SSE-driven FLIP on the
  originating client, independent of whether rules exist.

### Out of scope
- No change to the set of valid statuses themselves
  (`considering`, `todo`, `in_flight`, `completed`, `tabled`).
- No change to assignee, project, tag, or relationship handling.

## Behavior changes

| Action | Before | After |
| --- | --- | --- |
| Drag/PATCH `considering → completed` (previously illegal) | 422 unless `?force=1` | 200, records `status_changed` |
| Drag/PATCH to an unknown status (typo) | 422 | 422 (unchanged) |
| Alt-drag in web UI | Appends `?force=1` | Alt does nothing; all drags allowed |
| `card update --force` | Bypasses rule, recorded as forced | Flag removed; all updates allowed |
| Drawer status pills | Reachable pills active, others greyed | All pills clickable; current status visually marked |
| Drag a blocked card | Modal prompts unblock/keep/cancel | Unchanged |
| Audit event for a forced move | `forced = 1` | No `forced` concept; plain `status_changed` |

## Affected components

### Model — `model/model.go`
- Delete `ValidTransitions` (the map), `CanTransition`, `AllowedTransitions`.
- Delete the `Forced bool` field from `CardEvent`.
- Keep `ValidStatuses`, the `Blocked` field, and `ValidCommentKinds`.

### Store — `store/card.go`
- `UpdateCard`: remove the `CanTransition` check and the entire `if p.Force`
  branch. Retain: detection of `oldStatus != newStatus`, validation that the
  new status is in `ValidStatuses` (returns an error otherwise), and writing
  the `status_changed` `CardEvent`.
- Remove the `Force bool` field from `CardUpdateParams`.

### Store audit — `store/audit.go`
- Remove `forced` from the `INSERT` column list and value binding, and from
  every `SELECT` and `Scan`.

### Migration — `db/migrations/005_drop_forced_column.sql` (new)
- `ALTER TABLE card_events DROP COLUMN forced;`
- Register `"migrations/005_drop_forced_column.sql"` in the migration list in
  `db/db.go` (the `for _, name := range []string{...}` slice).
- Safe: `card_events` is append-only with a single index; DROP COLUMN rebuilds
  the table, trivial at this volume. modernc.org/sqlite supports SQLite 3.35+
  DROP COLUMN.

### Web handlers — `web/handlers.go`
- Delete `wantsForce` and the `Force: wantsForce(r)` assignment in
  `handleStatusChange`. The illegal-transition 422 path disappears; the
  invalid-*status* 422 path remains (driven by the store's `ValidStatuses`
  error).
- `buildStatusPills`: replace the `CanTransition`-based reachable flag with an
  `is-current` marker. Every pill is clickable; only the card's current status
  gets a distinguishing marker (no greyed/unreachable state).

### API — `api/cards.go`
- Remove the `Force` field read from the JSON request body in `updateCard`.

### Client — `client/client.go`
- Remove the `Force bool` field (and its comments) from `CardUpdateRequest`.

### CLI — `cmd/card.go`
- Remove the `cardUpdateForce` variable, the `req.Force = true` assignment, and
  the `--force` flag registration in `init()`.

### Agent context — `cmd/context.go`
- Remove `ValidTransitions` from the exported agent-context schema.
- Bump `agentContextSchemaVersion` from v2 to v3 and update its comment to note
  transition rules were removed.
- Keep exporting `ValidStatuses` and `ValidCommentKinds`.

### Web UI — `web/static/js/app.js`
- Remove the `altHeld` state field.
- Remove the `keydown`/`keyup` document listeners and the `window` `blur`
  listener that maintained `altHeld`.
- Remove the `onStart` altKey snapshot and the `onMove` altKey capture. If
  `onMove` exists solely for altKey capture, remove the callback entirely
  (SortableJS defaults to allowing the move); otherwise leave it returning
  `true`.
- In `onCardDrop`, remove the `force` variable and the
  `if (force) qs.push('force=1')` line. Keep the `?unblock=1` construction and
  the `askBlockMove` modal flow intact.
- Keep `_pendingDrops` and its use in `handleCardUpdated`.

### Drawer template — `web/templates/drawer.html`
- Re-render the status pills as all-clickable, with a marker on the current
  status (matching the new `buildStatusPills` output). Keep the `?unblock=1`
  parameter on status changes for blocked cards.

## Tests

Rewrite or remove the transition/force tests; add coverage for the new
"anything goes" behavior.

- **`model/model_test.go`**: remove the `CanTransition`/transition-matrix tests.
- **`store/card_test.go`**:
  - `TestUpdateCardRejectsInvalidTransition` → invert into
    `TestUpdateCardAllowsAnyTransition`: a previously-illegal transition
    (`considering → completed`) now succeeds and records a `status_changed`
    event.
  - Remove `TestUpdateCardForceBypassesTransition` and
    `TestUpdateCardLegalTransitionNotForced` (the `Forced` concept is gone).
  - Replace `TestUpdateCardForceStillRejectsInvalidStatus` with
    `TestUpdateCardRejectsInvalidStatus`: an unknown status is still rejected
    (no force involved).
- **`cmd/card_test.go`**: remove `TestCardUpdateForceBypassesTransition` and
  the `cardUpdateForce` reset in the test setup.
- **`web/handlers_test.go`**:
  - Remove `TestStatusChangeForce`.
  - `TestStatusChangeWithoutForceStillRejects` → invert into
    `TestStatusChangeAllowsAnyTransition`: the previously-illegal transition now
    returns 200.
  - Add/keep a case asserting an unknown status still returns 422.
- **`test/e2e_test.go`**: the invalid-transition-rejected assertion
  (`tabled → completed`) becomes a success assertion.

No JS test runner exists; the `app.js` changes are verified by code review plus
a manual browser check (drag a card across any columns — it always moves; Alt
has no special effect; blocked-card drag still prompts the modal).

## Documentation

- **`plugins/kkullm/skills/cli/SKILL.md`**: remove the `--force` flag
  documentation; update the lifecycle/transition section to state that any
  status may move to any other status and there is no transition validation
  (only that the target is a real status). Keep the `--blocked`/`--unblocked`/
  `--reason` documentation.
- **`docs/superpowers/specs/2026-04-09-kkullm-prd-design.md`** and the historical
  backend plan are left as-is (historical record); this spec supersedes the
  transition-validation requirement.

## Risks & mitigations

- **Existing forced history loses its flag.** Acceptable: the audit events
  remain; only the boolean distinction is dropped. The user chose the migration
  approach.
- **Agent-context schema bump.** Agents reading the schema see v3 without
  `ValidTransitions`. This is a deliberate, documented change; `ValidStatuses`
  still tells agents the legal status values.
- **Drawer pill regression.** Mitigated by updating `buildStatusPills` and its
  template together and by handler tests asserting all statuses render
  clickable.
