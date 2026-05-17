# Kkullm CLI Skill + Plugin Marketplace — Design

## Context

PR #14 made kkullm's CLI agent-native (consistent verbs, universal `--json`,
teaching errors, `--dry-run`, `--limit`, an `agent-context` introspection
command). Those conventions are only useful if the agents and operators driving
the CLI know them.

This design adds two things:

1. A **skill** that explains the purpose of the kkullm CLI and how to use it,
   enumerating the conventions established in PR #14. It is invoked as
   `/kkullm:cli` and also auto-activates when Claude detects kkullm work.
2. A **Claude Code plugin marketplace** hosted in the kkullm repository, so a
   user can `add` the marketplace and `install` the plugin that ships the skill.

The kkullm repo therefore becomes both a Go project and a single-plugin
marketplace.

## Naming

Plugin skills are always namespaced `plugin-name:skill-name` — a bare
`/kkullm-cli` is not possible for a marketplace-distributed skill. The plugin is
named `kkullm` and the skill `cli`, giving `/kkullm:cli` and leaving room for
future skills (`/kkullm:hooks`, etc.).

## Repository layout

New files only; nothing existing moves:

```
kkullm/
├── .claude-plugin/
│   └── marketplace.json          # marketplace catalog (repo root)
└── plugins/
    └── kkullm/
        ├── .claude-plugin/
        │   └── plugin.json       # plugin manifest
        ├── skills/
        │   └── cli/
        │       └── SKILL.md      # the skill → /kkullm:cli
        └── README.md             # short plugin readme
```

The plugin lives in `plugins/kkullm/` rather than the repo root to keep plugin
files from cluttering the Go project and to leave room for future plugins.

## `marketplace.json`

At the repo root, `.claude-plugin/marketplace.json`:

```json
{
  "$schema": "https://anthropic.com/claude-code/marketplace.schema.json",
  "name": "kkullm",
  "description": "Plugins for kkullm, the blackboard-pattern agent-orchestration system",
  "owner": { "name": "Joel Helbling" },
  "plugins": [
    {
      "name": "kkullm",
      "description": "Skills for driving the kkullm CLI and board",
      "author": { "name": "Joel Helbling" },
      "category": "development",
      "source": "./plugins/kkullm",
      "homepage": "https://github.com/joelhelbling/kkullm"
    }
  ]
}
```

`source` is a repo-relative path so the marketplace and plugin ship from one
repo. Install is `/plugin install kkullm@kkullm`.

## `plugin.json`

At `plugins/kkullm/.claude-plugin/plugin.json`:

```json
{
  "name": "kkullm",
  "description": "Skills for driving the kkullm CLI and board",
  "version": "0.1.0",
  "author": { "name": "Joel Helbling" },
  "homepage": "https://github.com/joelhelbling/kkullm",
  "license": "MIT"
}
```

`version` starts at `0.1.0` and is bumped on each release so installed users
receive updates.

## The skill — `plugins/kkullm/skills/cli/SKILL.md`

### Frontmatter

```yaml
---
name: cli
description: <purpose + when-to-use, written with trigger keywords: kkullm,
  card, board, blackboard, agent orchestration — so Claude auto-activates it on
  kkullm work; stays user-invocable as /kkullm:cli>
---
```

No `disable-model-invocation` and no `user-invocable: false`, so the skill is
both user-invocable and model-invoked (the default).

### Body — written for both operators and board-worker agents

The skill teaches purpose, conventions, and judgment; it does **not** re-list
every command and flag — that reference is delegated to `kkullm agent-context`
and `--help`.

1. **Purpose** — kkullm is blackboard-pattern agent orchestration; the CLI is
   the first-class agent/operator interface. Cards are pulled, not pushed; the
   two-session model (prioritize, then execute).

2. **Discovery first** — run `kkullm agent-context` for the authoritative,
   machine-readable command/flag/enum list; `kkullm <cmd> --help` for
   per-command detail. The skill carries conventions; `agent-context` carries
   the reference.

3. **Conventions established in the CLI:**
   - Consistent verbs: `get`, `list`, `create`, `update` — never `show`/`add`.
   - `--json` on every data command for parseable output; data on stdout,
     diagnostics on stderr.
   - `--dry-run` previews any create/update without sending it.
   - `--limit` bounds list output (default 50, `0` = unlimited); a truncation
     hint is written to stderr so `--json` stdout stays clean.
   - Errors are one clean line on stderr and enumerate valid values; exit code
     `0` success, non-zero failure.
   - Unknown subcommands fail loudly (non-zero exit), not a silent help dump.
   - Identity & precedence: `--server`/`--as`/`--project` flags override the
     `KKULLM_SERVER`/`KKULLM_AGENT`/`KKULLM_PROJECT` env vars. Mutating
     commands require an agent identity.

4. **How to use it — two flows:**
   - *Operator setup:* `project create` → `agent create` → seed `card create`.
   - *Board-worker agent loop:* `card list --status todo --format full --json`
     → claim via `card update <id> --status in_flight` → `comment create` to
     log progress → `card update <id> --status completed`.

5. **Card lifecycle** — brief: the status flow
   (`considering`→`todo`→`in_flight`→`completed`, plus `blocked`/`tabled`) and
   that transitions are validated server-side. Point to `agent-context`
   `enums` for the exact status set and transition map rather than duplicating
   it here.

The skill assumes the `kkullm` binary is on PATH and a server is running; it
contains no install/serve setup section.

## README updates

- Root `README.md`: a short "Claude Code plugin" section with the install
  commands (`/plugin marketplace add joelhelbling/kkullm`,
  `/plugin install kkullm@kkullm`, `/reload-plugins`).
- `plugins/kkullm/README.md`: one-paragraph plugin description and the same
  install snippet.

## Verification

1. `marketplace.json`, `plugin.json` are valid JSON and match the documented
   schemas.
2. `SKILL.md` frontmatter parses; `description` is within the length budget.
3. In Claude Code: `/plugin marketplace add ./` (or
   `joelhelbling/kkullm` once pushed) → `/plugin install kkullm@kkullm` →
   `/reload-plugins` → `/kkullm:cli` appears in the slash menu and invokes,
   loading the skill body.
4. Skill content cross-checked against the actual CLI: every convention and
   command named is correct against PR #14's `cmd/` and against
   `kkullm agent-context` output.

## Out of scope

- Additional skills (`/kkullm:hooks`, setup helpers) — the layout leaves room
  for them but they are not built now.
- Claude Code hook integration (pulling cards on session start) — separate
  future work.
- Any change to the CLI itself; this design only documents it.
