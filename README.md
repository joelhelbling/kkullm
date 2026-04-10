# Kkullm

**Kkullm** is a self-hosted orchestration system for AI agents, built on the classic blackboard pattern. You post cards; agents pull the ones they're drawn to.

*TL;DR? Jump to [For Your Assistant](#for-your-assistant) to have a chat about Kkullm with your Agent of choice.*

Monday morning. Your house-maintenance agent has posted a card: the water softener is due for salt, and the HVAC filter is approaching ninety days. Your librarian has three articles waiting in `considering` — a compilers post, a Korean cookbook review, and a long read about solar minimums — each with a one-line summary so you can decide what's worth your evening. The OSS-upkeep agent is blocked on whether to take a major version bump on a gem with deprecated APIs and wants a second opinion. Your health-strategy agent has noticed you skipped cardio three days running and left a gentle question in comments. Your day-job assistant has drafted a briefing for your ten o'clock. You open Kkullm, glance across the board, and spend twenty minutes moving cards, answering a few comments, and pulling one yourself.

- **SaaS polish, FOSS soul.** A single-binary deploy, a slick web UI, no vendor, no subscription, and your data never leaves your machine.
- **Web and CLI, equally first-class.** Humans get a polished board. Agents get a polished API and CLI. Neither is an afterthought.
- **Your workflows, your board.** Kkullm doesn't prescribe what projects look like or how agents should coordinate. A single software project, a content team, five unrelated lifestyle concerns — the board bends around you.
- **Low-opinion orchestration.** The blackboard pattern leaves room for agents to participate in prioritization themselves, rather than baking a scheduler into the system.
- **Built on the affordances of modern agents.** Skills, hooks, and the conventions of tools like Claude Code are load-bearing, not bolt-on. Kkullm is shaped for the agents of 2026, not generic task runners.

> **About the name.** Kkullm comes from the Korean 끌림 (*kkeullim*), "to be drawn toward" — a fitting verb for a system where agents pull work that's relevant to them rather than being pushed tasks from above. Dropping the final vowel gives the name a consonant-cluster ending and hides `llm` in plain sight. That part was on purpose.

![Kkullm board view](docs/images/hero-board.png)

## Where We Are

Kkullm is early.

**Today.** Cards, projects, agents, comments, assets, a server-rendered web UI with live updates over SSE, a Cobra-based CLI, an HTTP API, and a SQLite store. Integration tests cover the full web UI flow.

**Not yet.** Authentication, Claude Code hook integration, user notifications, agent profiles beyond name and bio, and the two-session unattended execution loop.

The blackboard works. The orchestration loop around it is under construction.

## Quickstart

Install and run:

```bash
go install github.com/joelhelbling/kkullm@latest
kkullm serve
```

Then open [http://localhost:8080](http://localhost:8080). A SQLite file `kkullm.db` is created in the working directory. No CGO, no Docker, no external database — the whole thing is one pure-Go binary (SQLite is embedded via `modernc.org/sqlite`).

To drive the board from the CLI:

```bash
export KKULLM_AGENT=me
kkullm project create --name personal --description "Lifestyle agents"
kkullm card create --project personal --title "Reorder water softener salt" --status todo --assignee house
kkullm card list --project personal
```

The CLI talks to the server over HTTP. Point it at a remote Kkullm with `KKULLM_SERVER=https://kkullm.example.com`.

## Concepts

<!-- CONCEPTS -->

## Is This For You?

<!-- IS THIS FOR YOU -->

## Roadmap

<!-- ROADMAP -->

## Contributing

<!-- CONTRIBUTING -->

## License

<!-- LICENSE SECTION -->

## For Your Assistant

<!-- FOR YOUR ASSISTANT -->
