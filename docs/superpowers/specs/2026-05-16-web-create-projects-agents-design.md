# Design: Create & manage projects and agents in the web admin UI

**Date:** 2026-05-16
**Status:** Approved

## Problem

The admin UI added the ability to rename and delete agents and projects (and
cards), but it provides no way to *create* a project or an agent. It also
cannot edit a project's `description` or an agent's `bio` — the inline rename
forms only touch `name`. Both fields exist in the data model and can only be
set today via the CLI/API.

## Scope

In scope:

- Create a project from the admin UI.
- Create an agent from the admin UI.
- Edit an existing project: `name` and `description`.
- Edit an existing agent: `name` and `bio`.

Out of scope:

- **Project assets** — no web presence today; deferred to a separate feature.
- **Agent project reassignment** — an agent's project is fixed after creation.
  The CLI has no reassign command (`agent` supports only `list`, `create`,
  `get`), so the web UI does not introduce one either.

## UI Design

### Pattern decisions

- **Create** uses a modal opened by a `+ New project` / `+ New agent` button,
  consistent with the existing delete-project modal.
- **Edit** uses an `Edit` button per row that opens a modal pre-filled with the
  row's values. This **replaces** the existing inline rename box — renaming now
  happens inside the edit modal.
- **Form submission** uses plain `POST` forms (no `fetch`/JS submission path),
  consistent with the rest of the server-rendered admin section. On a
  validation error the handler re-renders the same admin page with an error
  banner, the submitted values preserved, and the relevant modal reopened.

### `admin_projects.html`

- Page header gains a `+ New project` button.
- Each project row shows: name, meta (`N cards · N agents`), an `Edit` button,
  and the existing `Delete` button. The inline rename `<form>` is removed.
- A **create modal**: `name` (required text input), `description` (optional
  `<textarea>`).
- An **edit modal**: `name` and `description`, pre-filled. Populated by inline
  script in the same style as the existing `openDeleteProjectModal`.
- An **error banner** slot at the top of the page, rendered only when the
  page-data carries an error.
- The delete modal is unchanged.

### `admin_agents.html`

- Page header gains a `+ New agent` button.
- Each agent row shows: name, meta (`project: X`), an `Edit` button, and the
  existing `Delete` button. The inline rename `<form>` is removed.
- A **create modal**: `name` (required), `project` (required `<select>` listing
  all projects), `bio` (optional `<textarea>`).
- An **edit modal**: `name` and `bio` editable; `project` shown read-only.
- An **error banner** slot at the top of the page.
- The delete confirmation is unchanged.

### Error re-render mechanics

The `adminProjectsData` / `adminAgentsData` structs gain:

- `Error string` — message shown in the banner when non-empty.
- `Form` — the values the user submitted, so the reopened modal is pre-filled
  rather than blank.
- `Reopen` — identifies which modal to reopen: `create`, or `edit` together
  with the row id.

On page load, a few lines of inline script read a data attribute carrying the
`Reopen` value and open the matching modal pre-filled with `Form`. This mirrors
the existing `openDeleteProjectModal` inline-script pattern.

## Routes

In `web/web.go`, the two `/rename` routes are **replaced**:

| Removed                             | Added                                                            |
| ----------------------------------- | ---------------------------------------------------------------- |
| `POST /admin/projects/{id}/rename`  | `POST /admin/projects/create`, `POST /admin/projects/{id}/update` |
| `POST /admin/agents/{id}/rename`    | `POST /admin/agents/create`, `POST /admin/agents/{id}/update`     |

All routes remain gated by `RequireAdmin`.

## Handlers

New handlers in `web/admin_handlers.go`: `handleAdminCreateProject`,
`handleAdminUpdateProject`, `handleAdminCreateAgent`, `handleAdminUpdateAgent`.
The `handleAdminRenameProject` / `handleAdminRenameAgent` handlers are removed.

**Happy path:** parse form → validate → call store → broadcast SSE event →
`303 See Other` redirect to the list page.

**Error path:** the handler re-renders the same admin page, fully populated (it
re-lists projects/agents as the normal `GET` handler does), plus `Error`,
`Form`, and `Reopen`. No raw `http.Error` text page for routine validation
failures.

### Validation

- `name` is required: trimmed, must be non-empty.
- Duplicate `name`: the store `INSERT`/`UPDATE` hits the `UNIQUE` constraint on
  `projects.name` / `agents.name`. A helper `isUniqueViolation(err error) bool`
  detects this (the `modernc.org/sqlite` driver reports
  `UNIQUE constraint failed` in the error string). The handler then shows a
  friendly message, e.g. *"A project named \"X\" already exists."*
- Agent create: `project` must be selected and must resolve to an existing
  project; otherwise an error is shown.

## Store

In the `store` package:

- `UpdateProject(id int, name, description string) error` — validates a
  non-empty name, updates `name`, `description`, and `updated_at`. **Replaces**
  `RenameProject`.
- `UpdateAgent(id int, name, bio string) error` — validates a non-empty name,
  updates `name`, `bio`, and `updated_at`, and retains `RenameAgent`'s
  `comments.author_name` backfill so historical comment attribution stays
  correct. **Replaces** `RenameAgent`.
- `CreateProject` and `CreateAgent` already exist and are reused unchanged.

## SSE events

- Updates reuse the existing `project_renamed` / `agent_renamed` broadcasts.
  These carry the name, which is what board consumers care about;
  `description` and `bio` do not appear on the board, so no extra event is
  needed when only those change.
- **No new event types for creation.** A newly created project or agent has no
  cards and nothing on the board reacts to it, so a creation broadcast would
  have no consumer (YAGNI).

## Testing

Store tests (`store` package):

- `UpdateProject`: success, empty name rejected, duplicate name rejected.
- `UpdateAgent`: success, empty name rejected, duplicate name rejected,
  historical comment `author_name` backfilled.
- Existing `RenameProject` / `RenameAgent` tests are migrated to the new
  `Update*` methods.

Web handler tests (`web`):

- Create project: success; empty name → error re-render; duplicate name →
  error re-render.
- Create agent: success; missing/unknown project → error re-render; duplicate
  name → error re-render.
- Update project / update agent: success; validation error re-renders the page
  with the error banner and reopens the correct modal.

## Out-of-scope confirmations

- The CLI is unchanged, so the `/kkullm:cli` skill needs no update.
- No database migration is required — `description` and `bio` columns already
  exist.
