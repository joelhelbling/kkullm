# Kkullm CLI Skill + Plugin Marketplace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the kkullm repo into a Claude Code plugin marketplace that ships a `/kkullm:cli` skill teaching the purpose and conventions of the kkullm CLI.

**Architecture:** Add a marketplace manifest at the repo root (`.claude-plugin/marketplace.json`) and a single plugin under `plugins/kkullm/` containing one skill at `skills/cli/SKILL.md`. No code changes — only JSON manifests, a Markdown skill, and README updates.

**Tech Stack:** Claude Code plugin/marketplace JSON manifests; Markdown skill with YAML frontmatter.

**Spec:** `docs/superpowers/specs/2026-05-16-kkullm-cli-skill-plugin-design.md`

---

## File Structure

- `.claude-plugin/marketplace.json` — marketplace catalog listing one plugin.
- `plugins/kkullm/.claude-plugin/plugin.json` — plugin manifest.
- `plugins/kkullm/skills/cli/SKILL.md` — the skill (`/kkullm:cli`).
- `plugins/kkullm/README.md` — short plugin readme.
- `README.md` — modified: add a "Claude Code plugin" section.

There is no test suite for these files; verification is JSON validity plus a manual install in Claude Code (Task 6).

---

## Task 1: Marketplace manifest

**Files:**
- Create: `.claude-plugin/marketplace.json`

- [ ] **Step 1: Create the marketplace manifest**

Create `.claude-plugin/marketplace.json` with exactly:

```json
{
  "$schema": "https://anthropic.com/claude-code/marketplace.schema.json",
  "name": "kkullm",
  "description": "Plugins for kkullm, the blackboard-pattern agent-orchestration system",
  "owner": {
    "name": "Joel Helbling"
  },
  "plugins": [
    {
      "name": "kkullm",
      "description": "Skills for driving the kkullm CLI and board",
      "author": {
        "name": "Joel Helbling"
      },
      "category": "development",
      "source": "./plugins/kkullm",
      "homepage": "https://github.com/joelhelbling/kkullm"
    }
  ]
}
```

- [ ] **Step 2: Verify it is valid JSON**

Run: `jq . .claude-plugin/marketplace.json`
Expected: the file is echoed back pretty-printed, no parse error.

- [ ] **Step 3: Commit**

```bash
git add .claude-plugin/marketplace.json
git commit -m "feat(plugin): add Claude Code plugin marketplace manifest"
```

---

## Task 2: Plugin manifest

**Files:**
- Create: `plugins/kkullm/.claude-plugin/plugin.json`

- [ ] **Step 1: Create the plugin manifest**

Create `plugins/kkullm/.claude-plugin/plugin.json` with exactly:

```json
{
  "name": "kkullm",
  "description": "Skills for driving the kkullm CLI and board",
  "version": "0.1.0",
  "author": {
    "name": "Joel Helbling"
  },
  "homepage": "https://github.com/joelhelbling/kkullm",
  "license": "MIT"
}
```

- [ ] **Step 2: Verify it is valid JSON**

Run: `jq . plugins/kkullm/.claude-plugin/plugin.json`
Expected: the file is echoed back pretty-printed, no parse error.

- [ ] **Step 3: Commit**

```bash
git add plugins/kkullm/.claude-plugin/plugin.json
git commit -m "feat(plugin): add kkullm plugin manifest"
```

---

## Task 3: The `/kkullm:cli` skill

**Files:**
- Create: `plugins/kkullm/skills/cli/SKILL.md`

- [ ] **Step 1: Create the skill file**

Create `plugins/kkullm/skills/cli/SKILL.md` with exactly this content:

````markdown
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

- **Verbs are consistent.** `list`, `get`, `create`, `update` — every resource
  uses the same verbs. There is no `show` and no `add`. An unknown subcommand
  fails with a non-zero exit, so a typo never silently no-ops.
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
tags, relations, and comments. Status flows roughly:

```
considering → todo → in_flight → completed
                 ↘ blocked ↗
        (tabled: shelved, not completed)
```

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

If you get stuck, set the card to `blocked` and comment why — another agent or
a human can then pick it up.

### Parsing output

Always add `--json` when a script or agent consumes the result:

```
kkullm card list --status todo --json | jq '.[] | {id, title}'
```

## When in doubt

Run `kkullm agent-context` for the authoritative command and enum reference,
and `kkullm <command> --help` for flag detail. The conventions above tell you
*how* the CLI behaves; those two commands tell you *exactly what is there*.
````

- [ ] **Step 2: Verify the frontmatter parses**

Run: `head -3 plugins/kkullm/skills/cli/SKILL.md`
Expected: first line is `---`, second line begins `name: cli`, third line begins `description:`.

- [ ] **Step 3: Commit**

```bash
git add plugins/kkullm/skills/cli/SKILL.md
git commit -m "feat(plugin): add /kkullm:cli skill"
```

---

## Task 4: Plugin README

**Files:**
- Create: `plugins/kkullm/README.md`

- [ ] **Step 1: Create the plugin README**

Create `plugins/kkullm/README.md` with exactly:

```markdown
# kkullm plugin

Claude Code plugin for [kkullm](https://github.com/joelhelbling/kkullm), the
blackboard-pattern agent-orchestration system.

It ships one skill:

- **`/kkullm:cli`** — explains the purpose and conventions of the `kkullm` CLI
  and how operators and agents use it to drive a board.

## Install

```
/plugin marketplace add joelhelbling/kkullm
/plugin install kkullm@kkullm
/reload-plugins
```

Then invoke the skill with `/kkullm:cli`, or let Claude activate it
automatically when it detects kkullm CLI work.
```

- [ ] **Step 2: Commit**

```bash
git add plugins/kkullm/README.md
git commit -m "docs(plugin): add kkullm plugin README"
```

---

## Task 5: Root README — Claude Code plugin section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read the README to choose placement**

Run: `grep -n '^## ' README.md`
Pick an insertion point near the end of the document — after the project's
usage/status content and before any closing "For AI assistants"-style section
if one exists; otherwise append at the end.

- [ ] **Step 2: Insert the Claude Code plugin section**

Add this section at the chosen insertion point:

```markdown
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
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document the kkullm Claude Code plugin"
```

---

## Task 6: Verification

**Files:** none (verification only)

- [ ] **Step 1: Validate all JSON manifests**

Run: `jq . .claude-plugin/marketplace.json plugins/kkullm/.claude-plugin/plugin.json`
Expected: both files print pretty-printed with no parse error.

- [ ] **Step 2: Confirm the directory layout**

Run: `find .claude-plugin plugins -type f | sort`
Expected exactly:
```
.claude-plugin/marketplace.json
plugins/kkullm/.claude-plugin/plugin.json
plugins/kkullm/README.md
plugins/kkullm/skills/cli/SKILL.md
```

- [ ] **Step 3: Install the plugin locally in Claude Code**

In a Claude Code session, run:
```
/plugin marketplace add ./
/plugin install kkullm@kkullm
/reload-plugins
```
Expected: the marketplace `kkullm` is added; the plugin `kkullm` installs
without error.

- [ ] **Step 4: Confirm the skill is available**

In Claude Code, type `/kkullm:cli` and confirm it appears in the slash-command
menu and, when invoked, loads the skill body from Task 3.

- [ ] **Step 5: Cross-check skill accuracy against the live CLI**

Run: `kkullm agent-context | jq '.enums'`
Confirm the card statuses and relation types named in `SKILL.md` match the
`enums` output. Confirm `kkullm card list --help` shows `--json`, `--status`,
`--format`, and `--limit` as the skill describes.

---

## Self-Review

- **Spec coverage:** marketplace.json (Task 1), plugin.json (Task 2), SKILL.md
  with both-mode activation and the spec's five body sections (Task 3), plugin
  README and root README (Tasks 4–5), verification incl. install flow and
  CLI cross-check (Task 6). All spec sections are covered. Out-of-scope items
  (extra skills, hook integration) are correctly absent.
- **Placeholders:** none — every file's full content is inline.
- **Consistency:** marketplace name `kkullm`, plugin name `kkullm`, skill name
  `cli`, install target `kkullm@kkullm`, and invocation `/kkullm:cli` are used
  identically in every task and match the spec.
