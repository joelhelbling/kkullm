# Admin & Dangerous Actions — Design

**Status:** Design (awaiting implementation plan)
**Date:** 2026-05-14
**Author:** Joel Helbling (with Claude)

## Summary

Add the ability for the human user to **rename** and **delete** projects, agents, and cards, and to **purge the database** entirely. These are dangerous operations and must only be reachable from the web UI (acting on behalf of a human); they must not be exposed on the agent-facing API. Renames are handled inline on the existing board/card UI. Deletes of individual cards are handled from the card drawer. Project/agent administration and database purge live on a new `/admin` page styled after GitHub's settings pages.

The feature also lays a foundation for future admin/config surfaces (a left-menu shell that more sections can be added to) and a future authn/authz story (a single `requireAdmin` middleware chokepoint, currently a pass-through).

## Goals

- Let the human user rename and delete the entities they create.
- Provide a clearly-marked surface for dangerous operations, with confirmation patterns scaled to blast radius.
- Keep destructive operations out of reach of agents.
- Establish a reusable admin shell that future settings can plug into.

## Non-Goals (Explicit)

- **Auth.** No authentication or authorization is added in this feature. A no-op middleware chokepoint is introduced so the future auth feature has one place to wire in.
- **Persistent user identity across purge.** Purge wipes everything; preserving a "user agent" row across purges is deferred until we build out the broader identity story.
- **Backup-before-purge.** No export/download affordance in this feature.
- **Bulk operations.** No multi-select delete.
- **Undo / trash.** Deletes are immediate and permanent.

## Feature Bundle

1. **Inline rename** of projects (board header) and cards (existing card edit).
2. **Inline delete card** from the card drawer.
3. **New `/admin` surface** with a GitHub-style left menu and three sections at MVP: Projects, Agents, Danger Zone.
4. **Delete project** (cascading) and **delete agent** (unassign-cards semantics) from the admin Projects / Agents lists.
5. **Purge database** (truncate all data tables) from Danger Zone.

## Cascade & Reference Semantics

| Operation | Cascade behavior |
|---|---|
| Delete card | Existing FK cascades handle `comments`, `card_tags`, `card_relations`, `card_assignees`. |
| Delete agent | `card_assignees` rows for this agent are removed (cards stay, just unassigned). Comments authored by this agent **stay**, with `comments.agent_id` set NULL and `comments.author_name` preserving the agent's last name. |
| Delete project | Cascades to all of the project's `cards` (and their dependents via FK cascade), `project_assets`, and project-scoped `agents` — within a single transaction. |
| Purge database | `DELETE FROM` every data table (FK-safe order), reset `sqlite_sequence`. Migrations table untouched. |

### "Deleted agent" snapshot

To preserve comment attribution when an agent is deleted, we add a snapshot column to `comments`:

- New migration `db/migrations/002_comments_author_snapshot.sql`:
  - Add `comments.author_name TEXT` (nullable).
  - Backfill from current agent names.
  - Make `comments.agent_id` nullable (table-rebuild in SQLite).
- `store.CreateComment(...)` writes both `agent_id` and `author_name` going forward. **All existing callers (CLI, API, dev-seed, web composer) get the snapshot behavior for free — no call-site changes.**
- `store.RenameAgent(id, newName)` updates the agent row and runs `UPDATE comments SET author_name = ? WHERE agent_id = ?` in the same transaction, so historical comments reflect the current agent name.

## Confirmation UX Ladder

| Action | Confirmation |
|---|---|
| Rename project / agent | None — inline edit, save on blur/Enter, cancel on Esc. Reversible by re-renaming. |
| Delete card | Modal: title and cascade summary (N comments, M relationships). Single confirm button. |
| Delete agent | Modal: name and impact summary (N cards unassigned, M comments preserved with name snapshot). Single confirm button. |
| Delete project | Modal: type the **project name** to enable the confirm button. Lists what gets deleted (N cards, M agents, K assets). |
| Purge database | Modal: type the fixed phrase `PURGE DATABASE` (case-sensitive, monospace input) to enable the confirm button. Explicit copy: "This cannot be undone. All projects, agents, cards, comments, and assets will be permanently deleted." |

Server-side re-validates typed-confirmation payloads on submit. Client-side gating is a UX aid, not the security boundary.

## Architecture

### Routes

```
GET    /admin                          → redirect to /admin/projects
GET    /admin/projects                 → list projects with rename + delete affordances
GET    /admin/agents                   → list agents with rename + delete affordances
GET    /admin/danger                   → purge-database panel

POST   /admin/projects/{id}/rename     → form-post, redirect back
POST   /admin/projects/{id}/delete     → requires typed project-name match
POST   /admin/agents/{id}/rename       → form-post, redirect back
POST   /admin/agents/{id}/delete       → confirmation modal (no typing)
POST   /admin/danger/purge             → requires typed "PURGE DATABASE" phrase

POST   /cards/{id}/delete              → from card drawer (board UI)
POST   /projects/{id}/rename           → inline rename on board header
```

All `/admin/*` and the destructive endpoints pass through a **`requireAdmin` middleware**. Today this is a pass-through (no auth exists yet) — its job is to provide a single chokepoint so the future auth feature has one place to wire in. **None of these endpoints are exposed on the agent-facing API surface.**

### Store layer

- `store/project.go`: add `Rename(id, name)`, `Delete(id)` (transactional fan-out).
- `store/agent.go`: add `Rename(id, name)` (updates comment snapshots), `Delete(id)` (removes assignments; nulls `comments.agent_id`, preserves `author_name`).
- `store/card.go`: add `Delete(id)` (relies on existing FK cascades).
- `store/comment.go`: existing `Create` writes `author_name` snapshot.
- New `store/admin.go`: `Purge()` — transactional truncate of data tables, reset `sqlite_sequence`, leaves migrations table untouched.

### Handlers / templates

- New `web/admin_handlers.go` (or extension of `web/handlers.go` if it stays focused).
- New templates under `web/templates/admin/`:
  - `shell.html` — two-column layout, reuses top nav from `layout.html`.
  - `projects.html`, `agents.html`, `danger.html` — main-pane content per section.
  - Confirmation modal partials.
- Form posts follow the existing no-JS-friendly pattern; the typed-confirmation input uses a tiny inline script to toggle the submit button's `disabled` attribute on exact-string match.

### SSE / live updates

`api/sse.go` broadcasts:

- `card:deleted` — drops the card from other open boards.
- `project:deleted` — other tabs viewing the project redirect to a generic landing.
- `project:renamed`, `agent:renamed` — update headers and attribution in real time.
- `dataset:reset` (new) — fires on purge; clients show "Database was purged. Reloading…" toast and reload after ~1s.

## UI Look-and-Feel

### Inline rename

Click-to-edit on the project name (board header) and card title: text becomes input, saves on Enter or blur, cancels on Esc. UNIQUE-constraint or empty-string errors render inline; the input stays open.

### Card delete

A muted "Delete card" link near the bottom of the drawer (subordinate styling — it's an option, not a primary action). Opens a centered confirmation modal.

### `/admin` shell

- Two-column layout below the existing top nav: ~220px left menu, main pane fills the rest.
- Left menu items: **Projects**, **Agents**, **Danger Zone** (last item visually separated — extra top margin + thin divider; no red text in the menu itself).
- Active item highlighted with the existing kkullm accent.
- Main pane: section heading, short description, then list or panel content.

### Projects / Agents lists

Each row: name (inline-rename affordance on hover), small metadata line (e.g., "12 cards • 3 agents" for a project), right-aligned **Delete** button (outlined, not solid).

### Danger Zone

Single red-bordered panel: heading "Purge database", plain-English consequence paragraph, monospace typed-confirmation input (`PURGE DATABASE`), disabled-by-default confirm button that enables on exact-string match. The same red-bordered panel pattern is reused inside the **Delete project** modal.

### Styling additions

- New CSS tokens `--danger` and `--danger-bg`, used sparingly (delete buttons, danger-zone borders, danger confirm buttons).
- No new icon library — use existing icons or minimal inline SVG.

## Error Handling & Edge Cases

| Scenario | Behavior |
|---|---|
| Rename to duplicate name (UNIQUE) | Inline error on the input: "A project named '<name>' already exists." Input stays open. |
| Rename to empty string | Inline error: "Name cannot be empty." |
| Delete project/agent/card with stale id | Redirect to the corresponding list with a flash message. |
| Typed-confirmation mismatch (server) | 400 with the form re-rendered and an inline error. |
| Purge during active SSE connections | Transaction completes, broadcast `dataset:reset`, clients reload to empty state. |
| Purge mid-transaction failure (e.g., disk full) | Rollback. Render Danger Zone with the underlying error. No partial wipe. |

### Concurrency

SQLite is single-writer; transactions serialize naturally. With no auth and single-user usage, competing admin operations are not a practical concern.

### Idempotency

POST delete endpoints are not idempotent in the strict REST sense. A double-submit on a stale link surfaces the "not found" / redirect-with-flash path, not a 500. Rename is naturally idempotent for the same name.

## Testing Plan

### Store layer (`store/*_test.go`)

- `TestRenameProject_OK / DuplicateName / EmptyName`
- `TestRenameAgent_OK / BackfillsAuthorName_OnHistoricalComments`
- `TestDeleteCard_CascadesCommentsTagsRelations`
- `TestDeleteAgent_UnassignsCards / KeepsCommentsWithSnapshot / NullsAgentId`
- `TestDeleteProject_CascadesAllChildren / IsTransactional_RollsBackOnError`
- `TestPurge_EmptiesAllDataTables / ResetsAutoincrement / LeavesMigrationsTableAlone`
- `TestCreateComment_SnapshotsAuthorName`

### API / web handler layer

- Rename/delete endpoints: happy path, validation errors, stale-id redirect path.
- Typed-confirmation endpoints: server-side rejects mismatched payloads with 400.
- `requireAdmin` middleware: present on all admin routes (verified by test).
- SSE: `dataset:reset` event fires on purge; `card:deleted` event fires on card delete.

### End-to-end / web UI (extending `web/web_test.go`)

- Render `/admin/*` pages, assert key elements present (left menu, danger styling).
- Submit forms, follow redirects, assert flash messages.

### Manual smoke checklist

- Rename a project inline on the board → reflected in admin Projects list.
- Delete a card from drawer → other tabs viewing the board update via SSE.
- Delete an agent with cards + comments → cards unassigned, comments retain author name.
- Delete a project from admin → its board route 404s; other open tabs redirect.
- Purge with wrong phrase → button stays disabled; force-submit returns 400.
- Purge with correct phrase → all pages return to empty state, SSE clients reload.

## Migration

`db/migrations/002_comments_author_snapshot.sql`:

1. `ALTER TABLE comments ADD COLUMN author_name TEXT;`
2. Backfill: `UPDATE comments SET author_name = (SELECT name FROM agents WHERE agents.id = comments.agent_id) WHERE author_name IS NULL;`
3. Make `comments.agent_id` nullable via SQLite table-rebuild (create `comments_new`, `INSERT SELECT`, drop original, rename), preserving the FK cascade to `cards`.

## Open Questions

None at design time. Implementation plan will sequence the work and identify any review checkpoints.
