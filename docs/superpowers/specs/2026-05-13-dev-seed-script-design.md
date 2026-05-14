# Dev Seed Script

## Purpose

Provide a representative set of cards, agents, and projects for development
and manual testing of the Kkullm web UI and CLI. The seed must be safe to run
repeatedly against a development database while never silently destroying
data the user might want to keep.

The script is built on top of the `kkullm` CLI rather than direct DB writes
for creation; this exercises the CLI as a side benefit and guarantees the
seed data is reachable through the same paths real users take.

## Surface

- **Script**: `scripts/dev-seed.sh` (bash, executable, `set -euo pipefail`)
- **Taskfile entry**: `task dev-seed` runs the script
- **Env vars**:
  - `KKULLM_SERVER` (default `http://localhost:7722`) — passed to all CLI calls
  - `KKULLM_DB` (default `kkullm.db`) — used for existence detection and cleanup
- **Preconditions**: a `kkullm serve` process must already be running against
  `$KKULLM_DB`. The script does not start the server.

## Flow

1. **First confirmation.** Print the target server URL and DB path; require
   the user to type `yes` to continue. Anything else aborts.
2. **Server reachability check.** Probe `$KKULLM_SERVER/api/projects`. If the
   request fails, print a message instructing the user to start
   `kkullm serve` and exit non-zero.
3. **Detect existing seed data.** Query `$KKULLM_DB` with `sqlite3` for the
   three seeded project names (`beehive`, `birds_nest`, `ant_hill`). For each
   project that exists, count its cards and home agents.
4. **Second confirmation (only if any seed projects exist).** Display the
   counts and warn that ALL cards, comments, agents, and the projects
   themselves will be deleted and recreated. Require typing `yes` again.
5. **Cleanup.** For each existing target project, delete in order: cards
   (cascades to comments, tags, assignees, relations), agents, project.
   Performed via `sqlite3 $KKULLM_DB` because the CLI does not currently
   support delete operations. This is the only step that bypasses the CLI.
6. **Create via CLI.** Project → agents → cards → comments. Card IDs are
   captured from `kkullm card create` output (which prints
   `Created card #N: Title`) and stored in bash variables so subsequent
   calls can reference them with `--blocked-by`, `--belongs-to`, or
   `--interested-in`.

## Seed contents

Three projects, seven home agents, ~50 cards (15–20 per project).

### Projects and agents

- `beehive` — `worker_bee`, `drone`, `queen_bee`
- `birds_nest` — `robin`
- `ant_hill` — `worker_ant`, `soldier_ant`, `queen_ant`

Themes:
- beehive: pollen logistics, comb maintenance, royal jelly politics, swarm planning
- birds_nest: nest engineering, worm pipeline, egg watch, fledgling launch
- ant_hill: tunnel operations, aphid farming, crumb logistics, leaf-cutting

Tone: humorous and topical, with cross-over jokes when agents from one
project interact with another's cards.

### Required scenarios

| Requirement                          | How it appears                                                                                  |
| ------------------------------------ | ----------------------------------------------------------------------------------------------- |
| Agent with cards in multiple projects | `robin` is assigned to at least one beehive card and comments on at least one ant_hill card     |
| Unassigned `considering` cards       | At least one per project                                                                        |
| Unassigned `todo` cards              | At least one per project                                                                        |
| Dependency relations                 | Several pairs created with `--blocked-by`; visible as references between cards                  |
| Blocked cards                        | 1–2 per project, status `blocked`, with a `blocked_by` relation                                 |
| Multi-agent comments                 | Several cards with comments from two or more agents in the same project                         |
| Cross-project comments               | At least one beehive card with a comment from `queen_ant`; one ant_hill card with one from `robin` |
| Each home agent has ≥3 cards          | Assignee distribution is planned per agent in the script                                        |

### Status distribution per project (target)

Approximate, to exercise every board column:

- `considering` ×3
- `todo` ×4
- `in_flight` ×4
- `blocked` ×1–2
- `completed` ×3
- `tabled` ×1

## Failure handling

`set -euo pipefail` aborts on any failing CLI call. A partial seed left
behind by an abort is cleaned up by the next successful run (which will
detect the existing target projects and offer the reset confirmation).
No transactional rollback within a run; the CLI does not support it.

## Out of scope

- Adding `delete` subcommands to the CLI. Cleanup uses `sqlite3` directly.
- Starting or stopping `kkullm serve`.
- Seeding `project_assets`.
- Localization or i18n of the humorous content.
