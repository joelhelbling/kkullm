# Kkullm PRD — v1

## Overview

Kkullm (from Korean 끌림, "kkeullim" — to be drawn/pulled toward) is an agent orchestration system based on the classic blackboard pattern. It combines concepts from kanban boards and Slack-like team chat to enable messaging between a user and AI agents, and between agents themselves. The name embeds "LLM" intentionally.

Kkullm is open-source and self-hosted. The primary v1 persona is a solo developer running a personal fleet of AI agents.

## Architecture

A single Go binary serves all roles:

- **`kkullm serve`** — starts the REST API server, serves the embedded SPA, manages SSE connections, owns the SQLite database (WAL mode).
- **`kkullm <command>`** — CLI subcommands that are thin REST clients talking to the server.

The CLI and web UI consume the same REST API. The CLI shapes output for token efficiency (agents); the web UI shapes output for human readability.

The SPA is built with htmx + Alpine.js + SortableJS (drag-and-drop). Static files are embedded in the Go binary via `//go:embed` and served by the Go server. No Node.js build toolchain required.

### Why one binary

- Single artifact to build, distribute, install (`curl | install` or `brew install`).
- Embedded web assets add to file size, not runtime memory — only loaded when `kkullm serve` actually serves them.
- CLI subcommands start in single-digit milliseconds regardless of binary size.
- Cross-compilation gives macOS, Linux, Windows binaries from one build step.

### Why CLI over MCP

- Scriptability — shell scripts, hooks, and pipelines compose naturally with a CLI.
- Hook integration — Claude Code hooks shell out to commands; `kkullm card list` in a hook is trivial.
- Token efficiency — CLI output can be tuned for minimal token consumption.
- No protocol overhead.

## Data Model

### Project

| Column | Type | Notes |
|--------|------|-------|
| id | int | PK |
| name | string | unique |
| description | text | optional |
| created_at | datetime | |
| updated_at | datetime | |

A project is a domain boundary — an area of focus inhabited by one or more agents. It could be a single agent working on one repository, multiple agents on the same repo, or a single agent working across multiple services and data sources.

An **"orchestration"** project is seeded at server startup. It represents oversight of the Kkullm board itself.

### Agent

| Column | Type | Notes |
|--------|------|-------|
| id | int | PK |
| name | string | unique |
| project_id | int | FK → Project (home project) |
| bio | text | optional |
| created_at | datetime | |
| updated_at | datetime | |

An agent's `project_id` is its home project — where it primarily works. Agents can be assigned cards in any project (cross-project assignment is unrestricted).

A **"user"** agent is seeded at server startup, with `project_id` pointing to the "orchestration" project. The web UI acts as this agent. This eliminates polymorphic author columns — every actor in the system is an agent.

### ProjectAsset

| Column | Type | Notes |
|--------|------|-------|
| id | int | PK |
| project_id | int | FK → Project |
| name | string | |
| description | text | optional |
| url | string | nullable |
| created_at | datetime | |
| updated_at | datetime | |

Assets are the connective tissue for cross-project discovery. An agent can query assets by name or URL pattern to find which project owns a resource, then discover that project's agents.

Discovery flow: agent sees a repo URL in a task → queries assets for that URL → finds the owning project → lists project's agents → creates a card assigned to the relevant agent.

### Card

| Column | Type | Notes |
|--------|------|-------|
| id | int | PK |
| title | string | |
| body | text | optional |
| status | enum | see Card Lifecycle |
| project_id | int | FK → Project |
| created_at | datetime | |
| updated_at | datetime | |

### CardAssignee

| Column | Type | Notes |
|--------|------|-------|
| card_id | int | FK → Card |
| agent_id | int | FK → Agent |

No project constraint. Any agent can be assigned to any card regardless of project membership.

### CardTag

| Column | Type | Notes |
|--------|------|-------|
| card_id | int | FK → Card |
| tag | string | |

### CardRelation

| Column | Type | Notes |
|--------|------|-------|
| card_id | int | FK → Card |
| related_card_id | int | FK → Card |
| relation_type | enum | `blocked_by`, `belongs_to`, `interested_in` |

### Comment

| Column | Type | Notes |
|--------|------|-------|
| id | int | PK |
| card_id | int | FK → Card |
| agent_id | int | FK → Agent |
| body | text | |
| created_at | datetime | |

## Card Lifecycle

### Main flow

```
considering → todo → in_flight → completed → done
```

- **considering** — card is being discussed; read and comment, but do not work on it yet.
- **todo** — card is ready to be pulled by an agent. 끌림!
- **in_flight** — card is being actively worked on by an agent.
- **completed** — work is done, awaiting human review/acceptance.
- **done** — card is closed, no further work expected.

### Side states

- **blocked** — card cannot proceed; waiting on another card or human input. Can enter from `todo` or `in_flight`. Returns to `todo` or `in_flight` when unblocked.
- **tabled** — card is shelved, not completed; may be reopened later. Can enter from any active status. Returns to `considering` or `todo`.

### Transition rules

- **considering → todo** — human decision only. The user decides when a card is ready for agents.
- **todo → in_flight** — agent claims the card.
- **in_flight → completed** — agent marks work done.
- **completed → done** — human accepts the work.
- **any active → tabled** — human shelves the card.
- **blocked ↔ todo/in_flight** — unblocks when dependency resolves.

### Agent-created cards

Agents can create cards in `considering` or `todo` status. Use cases:
- Sub-tasks of an in-flight card → `todo`
- RFCs or questions for human consideration → `considering`

### Human as assignee

The "user" agent can be assigned cards like any other agent. When an agent needs human input on an in-flight card, it assigns the card to the user and moves it to `blocked`. The user's **blocked-and-assigned-to-me** queue is the highest priority view — these are the cards blocking agent work.

Blocked-queue dwell time is a future metric for identifying systemic collaboration bottlenecks.

## CLI Interface

### Configuration via environment

| Variable | Purpose | Override flag |
|----------|---------|---------------|
| `KKULLM_SERVER` | Server URL (default: `http://localhost:8080`) | `--server` |
| `KKULLM_AGENT` | Agent identity for commands | `--as` |
| `KKULLM_PROJECT` | Default project scope | `--project` |

Agent identity is required for all state-modifying commands. Error if neither env var nor flag is set. Agent and assignee references in CLI commands use agent **name** (not numeric ID) for readability and token efficiency.

### Commands

```
kkullm serve                              # start the server

# Cards
kkullm card list [--status X] [--assignee X] [--tag X] [--format brief|full]
kkullm card show <id>
kkullm card create --title "..." [--body "..."] [--status considering|todo]
                   [--assignee X] [--tag X]
                   [--blocked-by <id>] [--belongs-to <id>] [--interested-in <id>]
kkullm card update <id> [--status X] [--assignee X] [--title "..."] [--body "..."]
                        [--tag X] [--blocked-by <id>] [--belongs-to <id>]
                        [--interested-in <id>]

# Comments
kkullm comment list <card-id>
kkullm comment add <card-id> --body "..."

# Projects
kkullm project list
kkullm project create --name "..." [--description "..."]

# Agents
kkullm agent list [--project X]
kkullm agent create --name "..." --project X [--bio "..."]
kkullm agent show <id>

# Assets
kkullm asset list [--name "<glob>"] [--url "<glob>"]
kkullm asset create --name "..." [--description "..."] [--url "..."]
kkullm asset show <id>
```

### Output format

- Default: terse structured text optimized for token efficiency.
- `--json` flag: machine-parseable JSON output.
- `--format brief` (default for list): compact one-line-per-card summary.
- `--format full` (for prioritization sessions): card details with relations, comment counts, tags, age.

Relation flags are repeatable for multiple relations of the same type (e.g., `--blocked-by 37 --blocked-by 38`).

## REST API

```
# Cards
GET    /api/cards?project=X&assignee=X&status=X&tag=X
GET    /api/cards/:id
POST   /api/cards
PATCH  /api/cards/:id
DELETE /api/cards/:id

# Comments
GET    /api/cards/:id/comments
POST   /api/cards/:id/comments

# Projects
GET    /api/projects
POST   /api/projects
GET    /api/projects/:id

# Agents
GET    /api/agents?project=X
POST   /api/agents
GET    /api/agents/:id

# Assets
GET    /api/assets?project=X&name=<glob>&url=<glob>
POST   /api/assets
GET    /api/assets/:id

# Real-time
GET    /api/events          (SSE stream)
```

Card create (`POST`) and update (`PATCH`) accept inline relations and tags in the request body. The SSE stream pushes card status changes, new comments, and new cards.

## Web UI

Built with htmx + Alpine.js + SortableJS. Served as embedded static files from the Go binary.

### Views

1. **Board view (project-scoped)** — Kanban columns by status, all cards in a project. Cards show assignee name, comment count, and relation indicators. Drag-and-drop to change card status.
2. **Board view (agent-scoped)** — Kanban columns, all cards assigned to a specific agent across all projects. Cards show project-of-origin badge (color-coded), making cross-project assignments visible.
3. **Card detail** — Title, body, status, assignees, tags, relations, and flat comment thread. Editable inline.
4. **"My blockers"** — Cards assigned to the user in `blocked` status, across all projects. Persistent count badge in the nav bar.

### Navigation

- Project switcher dropdown in the nav bar.
- Agent switcher to toggle between project view and agent view.
- Blockers badge always visible — red count of cards needing user attention.

### Columns

- Active statuses shown as columns: `considering`, `todo`, `in_flight`, `blocked`, `completed`.
- Closed statuses (`done`, `tabled`) hidden by default with a toggle to show.

### Real-time updates

Server-Sent Events (SSE) push card changes to the browser. The `EventSource` browser API auto-reconnects on disconnect, providing effective polling as a free fallback. No WebSocket complexity needed.

## Two-Session Agent Workflow

The core agent interaction pattern:

### Session 1: Prioritization

1. Claude Code starts → hook calls `kkullm card list --status in_flight,todo,blocked --format full`
2. Agent receives all actionable cards with relations, tags, comment counts, age.
3. Agent checks for `in_flight` cards assigned to itself first — these represent interrupted work and are highest priority (resume before starting new work).
4. Agent reasons about remaining priority, dependencies, duplicates using its own rubric.
5. Agent writes a focused prompt to a file, designating the target card and referencing related cards.
6. Agent terminates. This session is disposable — its purpose is context-heavy triage without polluting the work session.

### Session 2: Execution

1. Claude Code restarts with the composed prompt.
2. Agent calls `kkullm card update <id> --status in_flight` to claim the card.
3. Agent works the card, calling `kkullm comment add <id> --body "..."` as work progresses.
4. Agent calls `kkullm card update <id> --status completed` when done.
5. Agent updates related cards as needed (unblocking, adding comments, creating sub-tasks).

The two-session split is a context hygiene strategy: the prioritization session can be token-heavy (reading many cards), but it's disposable. The work session starts clean.

## Error Handling

- **Agent identity required:** All state-modifying CLI commands require `KKULLM_AGENT` or `--as`. Read-only commands work without it.
- **Status transition validation:** The server enforces valid status transitions. Invalid transitions return a clear error listing allowed transitions.
- **Concurrent claims:** If two agents try to claim the same `todo` card, first one wins (SQLite serialization). Second gets an error with the card's current status and assignee.
- **SQLite WAL mode:** Enabled by default for concurrent read/write support.

## Out of Scope for v1

- Multi-user auth / permissions
- Dashboard / analytics (blocked-queue dwell time is a future metric)
- Agent management via web UI (CLI only)
- Asset management via web UI (CLI only)
- Search (filter by project + status is sufficient for a small board)
- Desktop or push notifications (SSE in the browser is the notification system)
- @-mentions in comments
- Agent auto-matching (agents decide what to pull, Kkullm doesn't suggest)
