# Kkullm Roadmap — Orchestrated Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute this plan. Each task below dispatches a fresh subagent in its own git worktree, with an orchestrator review + merge between tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the 15-issue roadmap (#24–#38) as a Claude Code orchestrator: one subagent per issue, each in an isolated git worktree, scheduled around inter-issue dependencies and shared-file collisions, with a review + merge gate between every task.

**Architecture:** The orchestrator (you, in this session) owns `main`. Each issue is implemented by a **fresh subagent in its own git worktree** (branch `issue-NN-slug`), which follows TDD, gets `task test` green, commits, and opens a PR (`Closes #NN`). The orchestrator reviews each branch, merges it to `main`, then rebases any in-flight worktrees in the same collision lane. Work is ordered so that (a) dependent issues never start before their prerequisite merges, and (b) no two concurrent subagents edit the same hot file.

**Tech Stack:** Go 1.26 (Cobra CLI, `modernc.org/sqlite`, no CGO), server-rendered HTML templates + htmx + Alpine + SortableJS, SSE for live updates. Build: `task build`. Test: `task test` (`go test ./...`). Migrations: `db/migrations/NNN_name.sql`.

---

## Terminology note: worktrees, not subtrees

This plan isolates each subagent with a **git worktree** (`git worktree add`, via `superpowers:using-git-worktrees`) — a separate checkout sharing one `.git`, so parallel subagents never stomp each other's working tree. (This is distinct from `git subtree`, which vendors a subproject under a path and is not used here.)

---

## Orchestration model (read before any task)

**Per-issue lifecycle — the orchestrator runs this for every task:**

1. **Create the worktree** off the latest `main`:
   `git worktree add ../kkullm-issue-NN -b issue-NN-slug main`
2. **Dispatch a subagent** into that worktree with the issue's brief (below). The subagent MUST:
   - Read the issue: `gh issue view NN` — and `docs/superpowers/specs/2026-06-12-roadmap.md` for context.
   - Use `superpowers:test-driven-development`: write a failing Go/handler test first, then the minimal code, per behavior.
   - Get `task test` fully green and `task build` clean.
   - Commit frequently; open a PR whose body contains `Closes #NN`.
3. **Review** the branch with `superpowers:requesting-code-review` (two-stage: spec-compliance, then code quality). Re-run `task test` yourself. Confirm the issue's Acceptance bullets.
4. **Merge** to `main` (squash or rebase-merge), then `git worktree remove ../kkullm-issue-NN`.
5. **Rebase** any still-open worktree in the **same collision lane** onto the new `main` before its subagent continues.

**Collision lanes (the rule: no two *concurrent* subagents on the same hot file):**

- **CORE** hot files: `model/model.go`, `store/card.go`, `store/comment.go`, `api/cards.go`, `api/comments.go`, `cmd/card.go`, `cmd/comment.go`, `db/migrations/*`.
- **WEB** hot files: `web/handlers.go`, `web/web.go`, `web/templates/*.html`, `web/static/js/app.js`.
- **DOCS** (independent files, always parallel-safe): `README.md`, `plugins/kkullm/skills/cli/SKILL.md`.

Some issues straddle CORE+WEB (#30, #35) — they serialize against **both** lanes.

**TDD note for Go web work:** handler behavior is tested in `web/*_test.go` (see existing `web/handlers_test.go`, `web/admin_handlers_test.go`); store behavior in `store/*_test.go`; CLI in `cmd/*_test.go`. Template-only/visual changes that can't be unit-tested get a manual verification note in the PR and, where practical, a handler test asserting the rendered fragment contains the expected markup.

---

## Dependency graph

```
#39 CI (GitHub Actions) .............. FOUNDATION — merges before all other work
#29 README ........................... independent (parallel anytime)
#25 Favicon .......................... independent files (parallel anytime)

#24 Archive-btn ─┐
#26 Edit title ──┤ WEB lane, serial
#27 Re-assign ───┤ (share web/handlers.go + drawer/board templates)
#28 Assets CRUD ─┘
#30 All-projects ── WEB + store/card.go ──┐
                                          │ (must merge before #31 touches store/card.go)
#31 blocked-as-flag (CORE keystone) ◀─────┘
   ├─> #32 web blocked badge + unblock confirm
   ├─> #33 orchestrator blocked view
   └─> #34 agent-view blocked scope ◀── (formerly-assigned needs #36)
#36 audit trail ──> #37 actor identity
#35 force-move (CORE+WEB) ── records "forced" via #36
#38 skill docs ── reflects #31/#35/#36 ── LAST
```

**Hard gates:**
- `#39` (CI) merges before every other task, so all subsequent PRs are tested automatically.
- `#31` must merge before `#32`, `#33`, `#34`, `#35` start.
- `#30` must merge before `#31` starts (both edit `store/card.go`).
- `#36`+`#37` must merge before `#34`'s "formerly-assigned" clause and before `#35` can record `forced`.
- `#38` is last (its content describes #31/#35/#36).

---

## Two execution shapes

**(A) Safe backbone (default):** fully serial in the order of the Waves below. Always correct; slowest. Use if running one subagent at a time.

**(B) Two parallel worktree lanes (faster):** run a **CORE lane** and a **WEB lane** as two concurrent worktree chains, honoring the hard gates:
- CORE lane: `#31 → #36 → #37 → #35`
- WEB lane: `#24 → #26 → #27 → #28 → #30 → ⟨gate: #31 merged⟩ → #32 → #33 → #34`
- DOCS (`#29`, then `#38` last) and `#25` float as parallel-safe singletons.
- Caveat: `#30` (WEB lane) edits `store/card.go`, so it **must merge before** the CORE lane starts `#31`. And `#35` (CORE lane) edits `web/handlers.go`+`app.js`, so it **must merge after** the WEB lane drains `#32–#34`. These two interlocks are the only cross-lane synchronization points.

The Waves below are written for shape (A); the lane labels let you collapse them into shape (B).

---

## Wave 0 — Foundation (do this first, alone)

### Task 0 — #39 CI: run tests and build on GitHub Actions  · lane DOCS/infra (disjoint)
**Brief for subagent — Files:** Create `.github/workflows/ci.yml`.
**Behavior:** On `pull_request` and `push` to `main`: `actions/checkout`; `actions/setup-go` pinned to the repo's Go version (`go.mod`/`mise.toml` → Go 1.26.x) with module caching; build (`go build ./...` or `task build`); test (`go test ./...` ≡ `task test`); `gofmt -l .` (fail if non-empty) and `go vet ./...`. Pick one of {call `go` directly, or set up `go-task/task` via `arduino/setup-task`} and keep it consistent.
**Tests:** none to author — the workflow IS the test harness. Verify by pushing the branch and confirming the Actions run is green on current `main`.
**Done:** workflow runs and passes on a PR; a deliberately failing test turns the check red; PR `Closes #39`.
**Why first:** once merged, every later `issue-NN-*` PR is validated automatically — de-risks the whole roadmap, especially #31's migration.

- [ ] Worktree `../kkullm-issue-39` (branch `issue-39-ci`) off `main`.
- [ ] Dispatch subagent with the brief above (touches only `.github/` — collides with nothing).
- [ ] Review: confirm the run is green on `main` and red on a forced failure; merge; remove worktree.
- [ ] **Gate:** do not start Wave 1 until #39 is merged.

---

## Wave 1 — Independent + Phase 0 WEB polish

### Task 1 — #29 README: document the web UI and the CLI  · lane DOCS
**Brief for subagent — Files:** `README.md` (and reference images under `docs/images/`).
**Behavior:** Document running `kkullm serve`, the board model and views, the CLI (build, `kkullm`, identity/config `--as`/`KKULLM_AGENT` + `--project`/`KKULLM_PROJECT` + `--server`/`KKULLM_SERVER`, the pull-and-work loop, `agent-context`). Link to the `/kkullm:cli` skill. Use existing screenshots.
**Tests:** none (docs). Verify links resolve and commands are accurate against the current CLI.
**Done:** README covers both interfaces; PR `Closes #29`.

- [ ] Create worktree `../kkullm-issue-29` (branch `issue-29-readme`).
- [ ] Dispatch subagent with the brief above.
- [ ] Review (accuracy of commands/links), merge, remove worktree.

### Task 2 — #25 Favicon: stylized 끌림  · lane WEB-light (disjoint files)
**Files:** Create `web/static/favicon.svg` (+ PNG/`.ico` fallbacks under `web/static/`); modify `web/templates/layout.html` (head `<link>`s); modify `web/web.go` if a new static route/path is needed (confirm how `web/static` is served first).
**Behavior:** A 끌림-derived favicon renders in the tab across sizes.
**Tests:** add/extend a `web/web_test.go` assertion that the favicon path returns 200 with the right content-type.
**Done:** favicon visible; PR `Closes #25`. Disjoint from `web/handlers.go`, so safe to run alongside the WEB chain.

- [ ] Worktree `../kkullm-issue-25` (`issue-25-favicon`).
- [ ] Dispatch; confirm static serving path before adding the route.
- [ ] Review, merge, remove worktree.

### Task 3 — #24 Web: "Archive" button appears in the Archived view  · lane WEB
**Files:** `web/templates/archived.html`, `web/templates/layout.html` and/or `web/templates/topbar_mobile.html` (wherever the Archive nav control lives); `web/handlers.go` only if the template needs a "current view" flag passed in.
**Behavior:** In the Archived view, the Archive nav button is hidden or rendered disabled; unchanged elsewhere.
**Tests:** `web/handlers_test.go` — assert the archived-view response does NOT contain the active Archive control, and a normal board response DOES.
**Done:** PR `Closes #24`.

- [ ] Worktree `../kkullm-issue-24` (`issue-24-archive-btn`).
- [ ] Dispatch; locate the Archive control first (grep templates).
- [ ] Review, merge, remove worktree, then rebase the next WEB worktree on new `main`.

### Task 4 — #26 Web: edit a card's title and body  · lane WEB
**Files:** `web/templates/drawer.html` (inline edit affordance), `web/handlers.go` (edit endpoint(s) for title/body; reuse the existing card-update path in `api`/`store`), `web/static/js/app.js` if htmx wiring is needed. `web/web.go` for the new route.
**Behavior:** Operator edits title and body (markdown) in the drawer; persists via the card update path; SSE refreshes the card.
**Tests:** `web/handlers_test.go` — POST/PUT to the edit endpoint updates the card and returns the refreshed fragment; markdown body still renders (markdown pipeline already exists).
**Done:** PR `Closes #26`.

- [ ] Worktree `../kkullm-issue-26` (`issue-26-edit-card`).
- [ ] Dispatch (read #26).
- [ ] Review, merge, remove worktree, rebase next WEB worktree.

### Task 5 — #27 Web: re-assign a card  · lane WEB
**Files:** `web/templates/drawer.html` (assignee editor), `web/handlers.go` (add/remove assignee endpoint; fetch the project's agents for the picker), `web/web.go` route, `web/static/js/app.js` if needed.
**Behavior:** Operator adds/removes assignees (from the project's agents) in the drawer; persists via the card-update assignee path; board + drawer reflect it.
**Tests:** `web/handlers_test.go` — adding and removing an assignee updates `Card.Assignees` and returns the refreshed fragment.
**Note:** the "Unblock this card?" confirm on assignee change is added later in #32 — do NOT build it here.
**Done:** PR `Closes #27`.

- [ ] Worktree `../kkullm-issue-27` (`issue-27-reassign`).
- [ ] Dispatch (read #27).
- [ ] Review, merge, remove worktree, rebase next WEB worktree.

### Task 6 — #28 Web: CRUD project assets  · lane WEB
**Files:** new `web/templates/` partial(s) for an assets section (follow `admin_projects.html`/`admin_agents.html` patterns), `web/handlers.go` (list/create/edit/delete handlers calling the existing `api/assets.go` / `store/asset.go`), `web/web.go` routes. No backend changes expected — audit `api/assets.go` + `store/asset.go` for completeness and note any gap in the PR rather than silently working around it.
**Behavior:** Full CRUD on a project's assets in the web UI, scoped to the current project.
**Tests:** `web/handlers_test.go` — create→list→edit→delete round-trip on an asset returns expected fragments/status.
**Done:** PR `Closes #28`.

- [ ] Worktree `../kkullm-issue-28` (`issue-28-assets-web`).
- [ ] Dispatch (read #28); confirm the assets API surface first.
- [ ] Review, merge, remove worktree, rebase next WEB worktree.

---

## Wave 2 — Cross-project, then the keystone

### Task 7 — #30 Web: "All projects" option in the project selector  · lane WEB + CORE (straddles)
**Files:** `web/templates/board.html` (selector gains an "All" entry), `web/handlers.go` (`handleBoard` project scoping; route the All view through `RequireAdmin` in `web/admin_middleware.go` — pass-through today), `store/card.go` (a list-across-all-projects query). Demote any special-casing of an "orchestration" project to none.
**Behavior:** "All" → every card from every project; a specific project → only its cards. "orchestration" is just a normal project.
**Tests:** `store/card_test.go` — the all-projects list returns cards from multiple projects; `web/handlers_test.go` — selecting All renders cross-project cards, selecting a project scopes correctly.
**Done:** PR `Closes #30`. **MUST merge before #31** (shared `store/card.go`).

- [ ] Worktree `../kkullm-issue-30` (`issue-30-all-projects`) off latest `main` (after Wave 1 WEB merges).
- [ ] Dispatch (read #30).
- [ ] Review, merge, remove worktree.

### Task 8 — #31 Make `blocked` an orthogonal flag, not a status  · lane CORE · KEYSTONE
**Files:**
- `model/model.go`: remove `"blocked"` from `ValidStatuses`, `AllStatuses`, and the `ValidTransitions` map; lifecycle becomes `considering → todo → in_flight → completed` (+ `tabled`). Add a `Blocked bool` field to `Card`. Add a comment `Kind` field to `Comment` (e.g. `""`/`"block"`/`"unblock"`).
- `db/migrations/003_blocked_flag.sql`: add `blocked` boolean column to cards (default 0) and a `kind` column to comments (default `''`). **Data migration:** for existing rows with `status='blocked'`, set `blocked=1` and `status='todo'` — there is no recorded previous status (the audit trail does not exist yet), so `todo` is the deliberate default. Document this lossy step in the PR.
- `store/card.go`, `api/cards.go`, `cmd/card.go`: `card update` gains `--blocked` / `--unblocked` flags (NO new `block`/`unblock` verbs — preserve the list/get/create/update contract). `--reason "..."` with `--blocked` posts a `kind="block"` comment; `--unblocked` posts a `kind="unblock"` comment. Blocking sets only the flag and **keeps the assignee/status**.
- `store/comment.go`, `api/comments.go`, `cmd/comment.go`: thread the comment `kind`.
- **Minimal web keep-alive:** `web/templates/board.html` / `web/templates/blockers.html` currently assume a blocked *column* (they iterate `AllStatuses`). Update just enough that the board still renders correctly with `blocked` removed from `AllStatuses` and blocked cards sitting in their real status column. The full badge + unblock UX is #32 — here, only prevent a broken board.
**Behavior:** matches #31 acceptance: `card update <id> --blocked --reason "..."` sets the flag + tagged comment, leaving status/assignee intact; `--unblocked [--status ...] [--assignee ...]` clears it and applies other edits; `agent-context` reflects the new enum/transition shape.
**Tests:** `model` test — `ValidStatuses`/`ValidTransitions` no longer contain blocked; `store/card_test.go` — set/clear blocked flag, assignee/status preserved; `store/comment_test.go` — kind persisted; `cmd/card_test.go` — `--blocked --reason` produces a kinded comment; migration test (`db/db_test.go` pattern) — a pre-existing `status='blocked'` row becomes `status='todo', blocked=1`.
**Done:** `task test` green, `task build` clean, PR `Closes #31`. **Gate: must merge before #32/#33/#34/#35.**

- [ ] Worktree `../kkullm-issue-31` (`issue-31-blocked-flag`) off `main` AFTER #30 merged.
- [ ] Dispatch (read #31 + roadmap). Emphasize the migration default and the no-new-verbs contract.
- [ ] Review thoroughly (this is the keystone): contract intact, migration correct, board not broken, `agent-context` updated.
- [ ] Merge, remove worktree. Announce the gate is open for #32/#33/#34/#35.

---

## Wave 3 — Blocked UX (all depend on #31)

### Task 9 — #32 Web: render blocked cards in-place + unblock-on-edit confirm  · lane WEB
**Depends on:** #31 merged.
**Files:** `web/templates/board.html` (blocked badge/color on cards in their real column), delete/repurpose `web/templates/blockers.html` (the old blocked column), `web/templates/drawer.html` (block state + reason from the kinded comment), `web/handlers.go` (block/unblock via the new flag path; the confirm flow), `web/static/js/app.js` (the "Unblock this card?" confirm on status/assignee change; SSE refresh of blocked state).
**Behavior:** No blocked column; blocked cards appear in-place, badged. Changing status/assignee on a blocked card prompts "Unblock this card?" → on yes, clear the flag (post `kind="unblock"` comment) as the edit applies; on cancel, leave blocked.
**Tests:** `web/handlers_test.go` — a blocked card renders in its status column with the badge markup; the unblock-on-edit endpoint clears the flag and writes an unblock comment.
**Done:** PR `Closes #32`.

- [ ] Worktree `../kkullm-issue-32` (`issue-32-web-blocked`) off `main` (post-#31).
- [ ] Dispatch (read #32). Reconcile with #27's assignee editor (the confirm wraps it).
- [ ] Review, merge, remove worktree, rebase next WEB worktree.

### Task 10 — #33 Web: orchestrator "blocked" view  · lane WEB
**Depends on:** #31 merged.
**Files:** new `web/templates/blocked_view.html` (filtered board: only blocked cards, each in its real column), `web/handlers.go` (the view handler; surface block reason/who/when from the kinded comment), `web/web.go` route behind `RequireAdmin`.
**Behavior:** A blocked view lists every blocked card in its real column with the reason visible, from which the operator can unblock/reassign (reuse #32's controls).
**Tests:** `web/handlers_test.go` — the blocked view returns only blocked cards and shows the reason.
**Done:** PR `Closes #33`.

- [ ] Worktree `../kkullm-issue-33` (`issue-33-blocked-view`) off `main` (post-#32 to inherit shared template/handler edits).
- [ ] Dispatch (read #33).
- [ ] Review, merge, remove worktree.

---

## Wave 4 — Audit trail (CORE)

### Task 11 — #36 Append-only card audit trail (status + assignment)  · lane CORE
**Depends on:** #31 merged.
**Files:** `db/migrations/004_card_audit.sql` (an append-only `card_events` table: id, card_id, actor, event_type, from_value, to_value, forced bool, created_at), `model/model.go` (a `CardEvent` type), new `store/audit.go` (+`store/audit_test.go`) for append + list, hooks in `store/card.go` / `api/cards.go` to record status transitions and assignee add/remove, and a read path in `api` + `cmd` (e.g. `kkullm card events <id> --json`, staying within the list/get/create/update verb contract — prefer `card get <id> --events` or a `card events` list-style command; pick one and keep it consistent).
**Behavior:** Status and assignment changes produce attributed, timestamped events; schema designed to add title/body/tag/relation events later without breaking.
**Tests:** `store/audit_test.go` — a status change and an assignee add each append the expected event with actor + timestamp; `forced` defaults false.
**Done:** PR `Closes #36`.

- [ ] Worktree `../kkullm-issue-36` (`issue-36-audit-trail`) off `main` (post-#31).
- [ ] Dispatch (read #36). Confirm the read-command shape keeps the verb contract.
- [ ] Review, merge, remove worktree.

### Task 12 — #37 Actor identity (`--as`/`KKULLM_AGENT`) is never recorded  · lane CORE
**Depends on:** #36 merged.
**Files:** `api/cards.go` (accept + persist the acting agent on mutations), `cmd/*` (thread `--as`/`agentName` from `cmd/root.go:35` into the request to the API), `store/audit.go` (store the actor) — verify the chain end to end.
**Behavior:** A mutation made with `--as alice` (or `KKULLM_AGENT=alice`) is attributable to `alice` in the audit trail.
**Tests:** `cmd`/`api` test — a status change issued as `alice` records `actor="alice"` in `card_events`; `--as` overrides `KKULLM_AGENT`.
**Done:** PR `Closes #37`.

- [ ] Worktree `../kkullm-issue-37` (`issue-37-actor-identity`) off `main` (post-#36).
- [ ] Dispatch (read #37).
- [ ] Review (assert the full thread CLI→API→store), merge, remove worktree.

---

## Wave 5 — Agent-view scope + force-move + docs

### Task 13 — #34 Agent view: scope blocked cards to the agent  · lane WEB
**Depends on:** #31 merged; "formerly-assigned" needs #36 merged (now satisfied).
**Files:** `web/handlers.go` (agent view filter), `store/card.go` (query: blocked cards assigned to the agent; plus formerly-assigned via `card_events` assignee history from #36).
**Behavior:** The agent view's blocked cards are scoped to the agent's current assignments, plus cards formerly assigned to them (reassigned away while blocked), read from the audit trail.
**Tests:** `store/card_test.go` — the scoped query returns the agent's currently- and formerly-assigned blocked cards and excludes others; `web/handlers_test.go` — agent view reflects the scope.
**Done:** PR `Closes #34`.

- [ ] Worktree `../kkullm-issue-34` (`issue-34-agent-blocked-scope`) off `main` (post-#33 and post-#36).
- [ ] Dispatch (read #34).
- [ ] Review, merge, remove worktree.

### Task 14 — #35 Force-move a card past the transition rules  · lane CORE+WEB (straddles)
**Depends on:** #31 merged; #36 merged (to record `forced`). **Schedule after #32–#34 drain the WEB lane** (it edits `web/handlers.go` + `app.js`).
**Files:** `model/model.go` (a force-bypass path around `CanTransition`, or a `force` param on the transition check), `cmd/card.go` (`--force` flag on `card update`), `api/cards.go` (accept force; skip transition validation; mark the audit event `forced=true`), `web/handlers.go` (status endpoint honors a force param), `web/static/js/app.js` (hold **Alt** while dropping to force).
**Behavior:** A normally-illegal transition succeeds with `--force` / Alt-drop and is recorded as `forced` in the audit trail; without the override, transition rules still apply and teach on rejection. Available to anyone (not gated).
**Tests:** `cmd/card_test.go` — `--force` performs an otherwise-invalid transition; `store/audit_test.go` / `api` test — the event has `forced=true`; `web/handlers_test.go` — force param bypasses validation.
**Done:** PR `Closes #35`.

- [ ] Worktree `../kkullm-issue-35` (`issue-35-force-move`) off `main` (post-#34 and post-#36).
- [ ] Dispatch (read #35).
- [ ] Review, merge, remove worktree.

### Task 15 — #38 Keep the `/kkullm:cli` skill current  · lane DOCS · LAST
**Depends on:** #31, #35, #36 merged (its content describes them).
**Files:** `plugins/kkullm/skills/cli/SKILL.md`.
**Behavior:** Update the skill to reflect: `card update --blocked/--unblocked --reason`; that `blocked` is no longer a status and the new lifecycle; `--force` semantics; that actions are attributed to `--as` via the audit trail; re-confirm the "verbs are consistent (list/get/create/update only)" contract still holds.
**Tests:** none (skill doc). Cross-check every command against `kkullm agent-context` output from the merged binary.
**Done:** PR `Closes #38`.

- [ ] Worktree `../kkullm-issue-38` (`issue-38-skill-update`) off `main` (after #31/#35/#36).
- [ ] Dispatch (read #38); diff claims against `kkullm agent-context`.
- [ ] Review, merge, remove worktree.

---

## Self-review — spec coverage

| Roadmap issue | Task | Covered |
|---|---|---|
| #39 CI (GitHub Actions) | Task 0 | ✓ |
| #24 Archive button | Task 3 | ✓ |
| #25 Favicon | Task 2 | ✓ |
| #26 Edit title/body | Task 4 | ✓ |
| #27 Re-assign | Task 5 | ✓ |
| #28 Assets CRUD web | Task 6 | ✓ |
| #29 README | Task 1 | ✓ |
| #30 All-projects selector | Task 7 | ✓ |
| #31 blocked-as-flag (keystone) | Task 8 | ✓ |
| #32 web blocked badge + confirm | Task 9 | ✓ |
| #33 orchestrator blocked view | Task 10 | ✓ |
| #34 agent-view blocked scope | Task 13 | ✓ |
| #35 force-move | Task 14 | ✓ |
| #36 audit trail | Task 11 | ✓ |
| #37 actor identity | Task 12 | ✓ |
| #38 skill docs | Task 15 | ✓ |

**Dependency gates honored:** #39 (CI) before everything (Task 0 first); #30 before #31 (Task 7 before Task 8); #31 before #32/#33/#34/#35 (Task 8 before Tasks 9/10/13/14); #36 before #37 and before #34's formerly-assigned and #35's `forced` (Task 11 before Tasks 12/13/14); #38 last (Task 15). ✓

**Known risk flagged:** #31's migration is lossy for pre-existing `status='blocked'` rows (no recorded previous status → default `todo`); called out in Task 8. The repo's live `kkullm.db` (with WAL) will need re-migration or a `task dev-seed` reset after #31 merges.
