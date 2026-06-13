---
name: cli
description: Drive the kkullm CLI — a blackboard-pattern agent-orchestration board where work lives on cards that agents pull rather than being assigned. Use when working with the `kkullm` command, a kkullm board, or kkullm cards/projects/agents; when an agent needs to pull, claim, comment on, or complete cards; or when setting up kkullm projects and agents.
---

# Using the kkullm CLI

Kkullm is a self-hosted agent-orchestration system built on the classic
**blackboard pattern**: work lives on a shared board as *cards*, and agents
*pull* the cards they are drawn to rather than being assigned tasks. The
`kkullm` CLI is the first-class interface for both human operators and AI
agents — the web UI is for humans; the CLI and HTTP API are for agents.

This skill covers what the CLI is for and how to use it well. It deliberately
does not list every command and flag — get that from the CLI itself (see
Discovery below).

## Discovery: ask the CLI, don't guess

Two introspection layers keep you from guessing:

- `kkullm agent-context` — emits a versioned JSON document describing every
  command, flag, enum (card statuses, status transitions, relation types),
  environment variable, and common workflow. **Run this first** in an
  unfamiliar kkullm setup.
- `kkullm <command> --help` — per-command flags and usage.

Prefer these over assuming a command's shape.

## Conventions

The CLI follows a consistent, agent-native contract. Rely on it:

- **Verbs are consistent.** Mutation is always `create` or `update` — nothing
  else changes state. Reading is `list` and `get` (plus the occasional
  resource-specific read such as `card events`, which is list-style). There is
  no `show` and no `add`; blocking, unblocking, and forced moves are *flags on
  `update`*, not new verbs, so the contract holds. An unknown subcommand fails
  with a non-zero exit, so a typo never silently no-ops.
- **`--json` everywhere.** Every data-returning command accepts `--json` and
  emits a parseable document on stdout. Pipe it to `jq`. Diagnostics and
  truncation hints go to stderr, so `--json` stdout stays a clean document.
- **`--dry-run` before mutations.** Any `create` or `update` accepts
  `--dry-run`: it validates the request and prints what *would* be sent
  without sending it. Use it to check a mutation before committing to it.
- **`--limit` bounds lists.** List commands default to 50 rows; pass
  `--limit 0` for everything, or a smaller number to narrow. When output is
  truncated, a hint is printed to stderr.
- **Errors teach.** A failure is one clean line on stderr naming the problem
  and the valid options — an invalid status, for example, lists every valid
  status. Exit code is `0` on success and non-zero on failure; check it.
- **Identity and config.** Three settings resolve as *flag beats env var*:
  `--server` / `KKULLM_SERVER`, `--as` / `KKULLM_AGENT`, and
  `--project` / `KKULLM_PROJECT`. Commands that change state require an agent
  identity, set with `--as` or `KKULLM_AGENT`.

## Cards and their lifecycle

A card is the unit of work: it has a title, body, status, project, assignees,
tags, relations, comments, and an orthogonal `blocked` flag. Status flows
roughly:

```
considering → todo → in_flight → completed
        (tabled: shelved, not completed)
```

`blocked` is NOT a status — it is a separate boolean flag a card carries while
it stays in its real status column. Set or clear it with the flags on
`card update` (see below); it never changes status or assignees.

Transitions are validated server-side — an illegal jump is rejected with a
teaching error. For the exact status set and the full transition map, read the
`enums` section of `kkullm agent-context` rather than memorizing it.

Cards relate to one another three ways: `blocked_by`, `belongs_to`, and
`interested_in`.

## How to use it

### As an operator: set up a board

```
kkullm project create --name acme --description "Acme product work"
kkullm agent create --name scribe --project acme --bio "Writes docs"
kkullm --project acme --as scribe card create --title "Draft the README" --status todo
```

### As a board-worker agent: the pull-and-work loop

Kkullm agents pull work rather than receiving it. The loop:

1. **See what is actionable.**
   `kkullm card list --status todo --format full --json`
2. **Claim one.** Move it to `in_flight` so others know it is taken:
   `kkullm card update <id> --status in_flight --as <agent>`
3. **Work it, leaving a trail.** Log progress as comments:
   `kkullm comment create <id> --body "Found the root cause in ..." --as <agent>`
4. **Finish.**
   `kkullm card update <id> --status completed --as <agent>`

If you get stuck, mark the card blocked with a reason — the card stays in its
current column, and the reason is recorded as a tagged comment in its timeline:
`kkullm card update <id> --blocked --reason "waiting on the auth spec" --as <agent>`
When the blocker clears, unblock it (optionally moving it in the same call):
`kkullm card update <id> --unblocked --status in_flight --as <agent>`

The status transitions are guardrails, not a cage. If you need to move a card
to a status the matrix would normally reject, add `--force` — it bypasses the
transition rule (the target must still be a real status) and the move is
recorded as *forced* in the card's audit trail:
`kkullm card update <id> --status completed --force --as <agent>`

### Reading a card's audit trail

`card events <id>` lists a card's append-only audit trail in chronological
order — every status transition and assignee add/remove, with the acting agent
and a timestamp. It is a list-style read (it accepts `--json` and `--limit`):
`kkullm card events <id> --json`

### Parsing output

Always add `--json` when a script or agent consumes the result:

```
kkullm card list --status todo --json | jq '.[] | {id, title}'
```

## When in doubt

Run `kkullm agent-context` for the authoritative command and enum reference,
and `kkullm <command> --help` for flag detail. The conventions above tell you
*how* the CLI behaves; those two commands tell you *exactly what is there*.
