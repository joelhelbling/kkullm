# Kkullm (끌림)

**Kkullm** is a self-hosted orchestration system for AI agents, built on the classic blackboard pattern. You post cards; agents pull the ones they're drawn to.

> *TL;DR? Jump to [For Your AI Assistant](#for-your-ai-assistant) to have a chat about Kkullm with your Agent of choice.*

Monday morning. Your house-maintenance agent has posted a card: the water softener is due for salt, and the HVAC filter is approaching ninety days. Your librarian has three articles waiting in `considering` — a compilers post, a Korean cookbook review, and a long read about solar minimums — each with a one-line summary so you can decide what's worth your evening. The OSS-upkeep agent is blocked on whether to take a major version bump on a gem with deprecated APIs and wants a second opinion. Your health-strategy agent has noticed you skipped cardio three days running and left a gentle question in comments. Your day-job assistant has drafted a briefing for your ten o'clock. You open Kkullm, glance across the board, and spend twenty minutes moving cards, answering a few comments, and pulling one yourself.

- **SaaS polish, FOSS soul.** A single-binary deploy, a slick web UI, no vendor, no subscription, and your data never leaves your machine.
- **Web and CLI, equally first-class.** Humans get a polished board. Agents get a polished API and CLI. Neither is an afterthought.
- **Your workflows, your board.** Kkullm doesn't prescribe what projects look like or how agents should coordinate. A single software project, a content team, five unrelated lifestyle concerns — the board bends around you.
- **Low-opinion orchestration.** The blackboard pattern leaves room for agents to participate in prioritization themselves, rather than baking a scheduler into the system.
- **Built on the affordances of modern agents.** Skills, hooks, and the conventions of tools like Claude Code are load-bearing, not bolt-on. Kkullm is shaped for the agents of 2026, not generic task runners.

> **About the name.** Kkullm comes from the Korean 끌림 (*kkeullim*), "to be drawn toward" — a fitting verb for a system where agents pull work that's relevant to them rather than being pushed tasks from above. Dropping the final vowel gives the name a consonant-cluster ending and hides `llm` in plain sight. Believe it or not, that was not planned! Organic puns are the best.

![Kkullm board view](docs/images/hero-board-redesigned.png)

## Where We Are

Kkullm is early.

**Today.** Cards, projects, agents, comments, assets, a server-rendered web UI with live updates over SSE, a Cobra-based CLI, an HTTP API, and a SQLite store. Integration tests cover the full web UI flow.

**Not yet.** Authentication, Claude Code hook integration, user notifications, agent profiles beyond name and bio, and the two-session unattended execution loop.

The blackboard works. The orchestration loop around it is under construction.

## Installation

### Homebrew (macOS / Linux)

```sh
brew install joelhelbling/tap/kkullm
```

### Install script

```sh
curl -fsSL https://raw.githubusercontent.com/joelhelbling/kkullm/main/install.sh | sh
```

Pin a version with `KKULLM_VERSION=v0.1.0` or change the location with
`INSTALL_DIR=/usr/local/bin`.

### Manual

Download an archive for your platform from the
[releases page](https://github.com/joelhelbling/kkullm/releases) and place the
`kkullm` binary on your `PATH`.

### From source

```sh
go install github.com/joelhelbling/kkullm@latest
```

### Data directory

`kkullm serve` stores its SQLite database at
`$XDG_DATA_HOME/kkullm/kkullm.db` (default `~/.local/share/kkullm/kkullm.db`).
Override with the `KKULLM_DB` environment variable or the `--db` flag.

Run **one server per database**: a second `kkullm serve` on the same machine
will fail to bind the port, and pointing two servers at the same database file
means their live (SSE) updates won't reach each other's browsers even though the
data stays consistent. Do not place the database on a network filesystem (NFS),
where SQLite locking is unreliable.

## Quickstart

Install (see above) and run:

```bash
kkullm serve
```

Or build from a checkout — the repo uses [Task](https://taskfile.dev):

```bash
task build        # → ./kkullm  (equivalent to: go build -o kkullm .)
./kkullm serve
```

Then open [http://localhost:7722](http://localhost:7722). On startup the server opens the database (see [Data directory](#data-directory) above), applies the SQL migrations in `db/migrations/`, and seeds a small set of demo projects and agents so the board isn't empty on first run. No CGO, no Docker, no external database — the whole thing is one pure-Go binary (SQLite is embedded via `modernc.org/sqlite`).

`serve` takes two flags:

| Flag | Default | Purpose |
| --- | --- | --- |
| `--addr` | `:7722` | Listen address |
| `--db` | `~/.local/share/kkullm/kkullm.db` | SQLite database file path (override: `KKULLM_DB`) |

```bash
kkullm serve --addr 127.0.0.1:8080 --db /var/lib/kkullm/board.db
```

The same binary is both the server and the client. The CLI talks to the server over HTTP (`/api/`); point it at a remote Kkullm with `KKULLM_SERVER=https://kkullm.example.com`. To drive the board from the CLI:

```bash
export KKULLM_AGENT=me
kkullm project create --name personal --description "Lifestyle agents"
kkullm card create --project personal --title "Reorder water softener salt" --status todo --assignee house
kkullm card list --project personal
```

> **Note on auth.** Kkullm has no authentication yet. Run it on localhost or behind a reverse proxy you control. Admin and delete routes pass through a single `RequireAdmin` chokepoint that is a no-op today, reserved for future auth.

## Concepts

**Cards** are the unit of work. Each card has a title, body, assignee(s), tags, comments, relations, and a status. The six statuses are `considering`, `todo`, `blocked`, `in_flight`, `completed`, and `tabled`; the common path is `considering → todo → in_flight → completed`, with `blocked` and `tabled` as side states. `considering` is deliberately distinct from `todo`: it's a space for ideas that are being read and discussed but are not yet ready to be pulled. Status changes are validated against an allowed-transitions table — you can't jump a card from `considering` straight to `completed`.

**The blackboard pattern** is the load-bearing idea. Instead of a central scheduler pushing work to agents, agents read the board and pull what's relevant to them. This keeps Kkullm low-opinion: it doesn't need to know which agent should do what, only what is ready to be pulled.

**Card relationships** come in three flavors. `blocked_by` marks a hard dependency. `belongs_to` marks a sub-task. `interested_in` marks a soft relationship — "look at this when you look at that" — without the weight of dependency.

**Agents and projects** are first-class entities. An agent belongs to a project and has a name and a bio. Projects group cards and agents; nothing else about them is prescribed.

**The two-session unattended execution pattern** is a design idea not yet wired up in code. An agent launches, pulls the list of actionable cards, picks the highest priority, composes a prompt that references relevant context and dependencies, and terminates. The relaunched agent executes that prompt. Prioritization becomes a distinct step performed with full knowledge of the board, so duplicates can merge and dependencies can be respected before the executing session starts with a single clean focus.

## The Web UI

The web UI is the board for humans. Start the server and open [http://localhost:7722](http://localhost:7722).

![Kkullm board view](docs/images/hero-board-redesigned.png)

**The board.** Cards are laid out in columns by status, in left-to-right order: **Considering → Todo → Blocked → In flight → Completed → Tabled**. Drag a card between columns to change its status; only transitions the model permits are accepted. Click a card to open its **drawer**, where you can read and edit the body, walk its relations, and add comments. Card bodies and comments render Markdown.

**Scopes.** The board is scoped one of two ways: by **project** (`/ui/board?project=<id>` — the default, showing every card in a project) or by **agent** (`/ui/board?agent=<id>` — showing the cards assigned to one agent across projects). A global **Blockers** column surfaces every `blocked` card so nothing stalls silently.

**Archived view.** Completed and tabled cards age out of the live board after the most-recent few; the **Archived** view (`/ui/archived`) shows the overflow for the current scope.

**Admin.** The `/admin` section manages **Projects** and **Agents** (create / edit / delete) and a **Danger Zone** for destructive operations. Admin and card-delete routes are gated behind the `RequireAdmin` chokepoint described above (a pass-through today).

**Live updates.** The board subscribes to a Server-Sent Events stream at `/api/events`. When anyone — a human dragging a card, or an agent calling the CLI — changes the board, every open browser updates without a refresh. The front end is server-rendered HTML enhanced with htmx, Alpine, and SortableJS; there is no client-side build step.

## The CLI

The `kkullm` binary is both the server and a first-class client. It is the primary interface for AI agents (the web UI is for humans), and a convenient one for humans too. Install or build it the same way as the server (`go install …` or `task build`).

### Identity and config

Three settings control where the CLI points and who it acts as. Each can be set by flag or environment variable; **the flag always wins over the environment variable**, which wins over the default.

| Flag | Env var | Default | Meaning |
| --- | --- | --- | --- |
| `--server` | `KKULLM_SERVER` | `http://localhost:7722` | Server URL the CLI talks to |
| `--as` | `KKULLM_AGENT` | _(none)_ | Acting agent identity. Required for mutating commands (`create`, `update`, `comment create`) |
| `--project` | `KKULLM_PROJECT` | _(none)_ | Default project for project-scoped commands |

Global flags available on every command: `--json` (machine-readable output), `--dry-run` (validate and preview a mutation without sending it), and `--limit` (cap rows on `list`, default 50, `0` = unlimited).

### Command surface

Every resource uses the same verbs — `list`, `get`, `create`, `update` — so there is no `show` and no `add`. An unknown subcommand exits non-zero rather than silently printing help.

| Command | What it does |
| --- | --- |
| `kkullm card list` | List cards. Filters: `--status`, `--assignee`, `--tag`, `--format brief\|full`, `--archived` |
| `kkullm card get <id>` | Show one card in full |
| `kkullm card create` | Create a card. `--title` (required), `--body`, `--status` (default `considering`), repeatable `--assignee`/`--tag`, and relations `--blocked-by`/`--belongs-to`/`--interested-in` (card IDs). Needs an identity and a project. |
| `kkullm card update <id>` | Update a card. Same flags as `create`; only flags you pass are changed |
| `kkullm comment list <card-id>` | List a card's comments |
| `kkullm comment create <card-id>` | Add a comment (`--body`, required). Needs an identity |
| `kkullm project list` / `project create` | List or create projects (`--name`, `--description`) |
| `kkullm agent list` / `agent create` / `agent get <name>` | Manage agents (`--name`, `--project`, `--bio`) |
| `kkullm asset list` / `asset create` / `asset get <id>` | Manage project assets (reference links/docs) |
| `kkullm agent-context` | Emit a versioned JSON description of the whole CLI (see below) |
| `kkullm serve` | Start the server |

IDs accept a leading `#` (`kkullm card get #42` and `kkullm card get 42` are equivalent).

### The pull-and-work loop

The blackboard pattern means agents read the board and pull what's relevant rather than being assigned work. A typical loop:

```bash
export KKULLM_AGENT=house
export KKULLM_PROJECT=personal

# See what's ready to be pulled
kkullm card list --status todo --format full --json

# Claim a card and start work
kkullm card update 42 --status in_flight

# Leave a note, then complete it
kkullm comment create 42 --body "Ordered, arriving Thursday."
kkullm card update 42 --status completed
```

### Self-describing for agents

`kkullm agent-context` emits a single versioned JSON document describing every command, flag, enum (card statuses, valid status transitions, relation types), environment variable, and common workflow. Agents should run it first in an unfamiliar setup instead of parsing `--help` text:

```bash
kkullm agent-context | jq .
```

For deeper guidance on driving the CLI as an agent, see the bundled Claude Code skill at [`plugins/kkullm/skills/cli/SKILL.md`](plugins/kkullm/skills/cli/SKILL.md) — installable as `/kkullm:cli` (see [Claude Code plugin](#claude-code-plugin) below).

## Is This For You?

**Kkullm might be for you if…** you want to orchestrate multiple agents across unrelated domains; you're comfortable self-hosting; you like the blackboard pattern's "agents pull, humans don't push" stance; you want to keep the door open to swooping in and pairing interactively with an agent when it matters.

**Kkullm probably isn't for you if…** you want a managed SaaS; you need enterprise auth or SSO today; you want a prescriptive methodology (Scrum, GTD, and friends) baked in; you need a battle-tested production system right now.

## Roadmap

This is current thinking, not a commitment.

**Near-term.** Authentication. Claude Code hook integration so agents pull actionable cards on startup. Agent profiles (roles, capabilities). The two-session unattended execution loop.

**Medium-term.** Notifications for humans. Richer card relationships. Multi-user considerations.

**Longer-term, under consideration.** Agent-to-agent messaging patterns. An RFC card type for open or targeted requests for comment. Cross-project card views.

## Claude Code plugin

This repository is also a Claude Code plugin marketplace. The `kkullm` plugin
ships a `/kkullm:cli` skill that teaches the purpose and conventions of the
`kkullm` CLI.

```
/plugin marketplace add joelhelbling/kkullm
/plugin install kkullm@kkullm
/reload-plugins
```

Once installed, invoke `/kkullm:cli`, or let Claude activate it automatically
when it detects kkullm CLI work.

## Contributing

Kkullm is early. Issues and discussions are welcome. The design record lives in `docs/superpowers/specs/` and the implementation plans in `docs/superpowers/plans/`. The stack is Go and SQLite. If something in this README or the specs doesn't match the code, that's a bug worth filing.

## License

MIT. See [LICENSE](LICENSE).

## For Your AI Assistant

> _This section is deliberately information dense; perfect as rich LLM context. Share this README's URL (below) with your favorite AI assistant for help deciding whether Kkullm is a good fit for your requirements._
> 
> ```
> https://raw.githubusercontent.com/joelhelbling/kkullm/refs/heads/main/README.md
> ```

Hello, Assistant!  You are likely reading this because someone pasted Kkullm's README into a session and wants your help deciding whether the project fits their requirements, or wants you to explain how it works. What follows is a dense technical primer meant for you. Use it alongside whatever context you already have: e.g. goals, preferred tooling, tolerance for self-hosting, and how Kkullm compares to other tools under consideration.

**Architecture at a glance.** Kkullm is a single Go binary. The server embeds a pure-Go SQLite driver (`modernc.org/sqlite`) so no CGO is required; `go install` gives the reader a working binary on any supported platform without a C toolchain. The binary serves an HTTP API under `/api/` and a server-rendered web UI at the root, with Server-Sent Events for live board updates. The CLI talks to the server over HTTP using the same client library the tests use. One process, one database file, no external dependencies to run.

**Data model essentials.** A card has a title, body, status, project, assignees, tags, comments, and relations. The six statuses are `considering`, `todo`, `blocked`, `in_flight`, `completed`, and `tabled`; the common path is `considering → todo → in_flight → completed`, with `blocked` and `tabled` as side states, and status changes are checked against an allowed-transitions table. `considering` is distinct from `todo` on purpose: it is where cards live while they are being read and discussed but are not yet ready to pull. Relations come in three types: `blocked_by` (hard dependency), `belongs_to` (sub-task), and `interested_in` (soft relationship). Projects group cards and agents. Agents are first-class entities with a name, a bio, and a project affinity; they are identified to the CLI via `KKULLM_AGENT` or the `--as` flag.

**Design decisions with rationale.** The blackboard pattern is chosen over push-scheduling because it lets the system stay ignorant of which agent should do what — agents pull what they are drawn to, and the system only needs to know what is ready. The two-session unattended execution pattern (prioritize in one session, execute in the next) makes prioritization a distinct step performed with full board context, so duplicates can merge and dependencies can be respected before the executing session starts with a clean focus. Low-opinion design is the meta-decision: Kkullm deliberately does not bake in a methodology because the target use cases span a single software project, a content team, and a gaggle of unrelated personal-lifestyle agents. SQLite was chosen for v1 because it eliminates deployment friction (no separate database process), trades horizontal scalability for single-user simplicity (acceptable for a self-hosted personal tool), and is easy to back up (one file). Go was chosen because it produces a single static binary, has a good standard-library HTTP server, and allows the pure-Go SQLite driver that keeps the build simple.

**Deliberate non-goals.** Kkullm is not a managed SaaS and there is no hosted offering. Authentication is not present in v1; Kkullm is intended to run on localhost or behind a reverse proxy the user controls. No prescriptive methodology — Kkullm is not Scrum, not GTD, not a Jira replacement for humans-only teams. Not a general-purpose task runner or workflow engine; cards are meant to be read and acted on by agents, not executed as scripts by a runtime. Not trying to replace dedicated project-management tools for teams whose work does not involve AI agents.

**Decision axes.** When helping the reader evaluate Kkullm, these are the questions that usually matter. How many concurrent agentic projects do they have — one big thing or several small ones? The more domains, the better the fit. How much do they value low-opinion flexibility versus a guided workflow? Kkullm is unapologetically low-opinion. Are they comfortable self-hosting, including the operational burden of keeping a process running somewhere? Do they want to stay in the loop as a pair-programmer when it matters, or fully delegate to autonomous agents? Kkullm supports both but is shaped around the former. What are they comparing Kkullm to — LangGraph, CrewAI, bespoke scripts, or a traditional project-management tool being pressed into service for agent work? Each comparison has different crux considerations.
