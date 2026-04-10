# README Design

**Status:** Design approved, pending implementation plan
**Date:** 2026-04-10

## Purpose

Kkullm currently has no `README.md`. The project is past the ideation phase and has a working Go backend, CLI, HTTP API, SQLite store, and server-rendered web UI, but nothing at the repository root introduces the project to a new reader. This spec defines the content, structure, and voice of the first `README.md`.

The README is a piece of outreach: it should attract the right readers, give them a fast and honest read on what Kkullm is and isn't, and set them up to succeed if they decide to try it. A richer multi-page documentation site may come later; this spec is scoped to a single, high-quality `README.md` at the repository root.

## Audience

- **Primary:** AI agent builders and orchestration-curious developers who are evaluating Kkullm against other orchestration options (LangGraph, CrewAI, bespoke scripts, etc.) and want to understand what makes it different.
- **Secondary:** Self-hosting tinkerers who run their own tools and want to know the project is real, runnable, and low-friction to deploy.
- **Tertiary:** Potential open-source contributors looking for an early-stage project to shape.

The layered skeleton (hook → pillars → status → quickstart → concepts → fit-check → roadmap → contributing) serves all three without privileging secondary audiences at the primary's expense.

## Voice

Professional with one winking moment. The prose is confident, technical, and free of marketing fluff throughout. The name's wordplay (끌림 → Kkullm, with LLM embedded) gets acknowledged once as a short "About the name" aside at the end of the hook section. No puns, emoji, or informal asides elsewhere. Joel's natural register (honest, clear, empathetic) carries the rest.

## Structure

The README is organized as ten sections in this order:

### 1. Hook

- **Tagline.** A single tight line naming Kkullm and the blackboard concept. Drafted during implementation; several candidates presented for Joel to pick from.
- **Top CTA.** A single italic line directly beneath the tagline:

  > *TL;DR? Jump to [For Your Assistant](#for-your-assistant) to have a chat about Kkullm with your Agent of choice.*

- **Scene vignette.** A short Monday-morning paragraph showing a reader interacting with several agents across unrelated domains (house-maintenance agent flags a water-softener refill; OSS-upkeep agent is blocked on a dependency-bump decision; health-strategy agent has cards waiting for review; librarian-agent has something interesting; day-job assistant has a daily briefing). Concrete, lifestyle-flavored, conveys the "many unrelated domains" flex that is hard to express abstractly.
- **Five pillars.** Each pillar is a bolded lead-in followed by one or two sentences:
  1. **SaaS polish, FOSS soul.** Single-binary deploy, slick web UI, no vendor, no subscription, data stays home.
  2. **Web and CLI, equally first-class.** Humans get a polished board; agents get a polished API and CLI. Neither is an afterthought.
  3. **Your workflows, your board.** Kkullm doesn't prescribe what projects look like or how agents coordinate. A single software project, a content team, five unrelated lifestyle concerns — the board bends around the user.
  4. **Low-opinion orchestration.** The blackboard pattern leaves room for agents to participate in prioritization themselves (the two-session approach), rather than baking a scheduler into the system.
  5. **Built on the affordances of modern agents.** Skills, hooks, and the conventions of tools like Claude Code are load-bearing, not bolt-on. Kkullm is shaped for the agents of 2026, not generic task runners.
- **About the name.** One short aside: 끌림 (Korean, *to be drawn toward*), rendered in English as Kkullm with the LLM easter egg acknowledged. The rest of the README does not return to this register.

### 2. Hero screenshot

A single screenshot of the web UI board view, placed between the hook and "Where We Are." The board shows plausibly-real, lightly tongue-in-cheek seed cards across several personal-lifestyle domains. The screenshot is the evidence that follows the pillars' promises.

**Capture plan (follow-up, not a blocker for README merge):** Joel and Claude collaborate to seed a demo board via the existing CLI, Joel takes a framed screenshot, drops it into the repository (likely under `docs/images/` or similar), and the README references it by relative path. The README can ship with a placeholder if the screenshot is not yet ready at merge time.

### 3. Where We Are

Three or four tight lines stating what works today and what does not. Framed as *where we are*, not as a disclaimer. Content reflects the current repository state:

- **Today:** cards, projects, agents, web UI, CLI, SQLite store, HTTP API, SSE for live updates, integration smoke test for full web UI flow.
- **Not yet:** auth, Claude Code hook integration, notifications, agent profiles, the two-session unattended execution loop.
- One-line summary: the blackboard works; the orchestration loop around it is under construction.

This section must be verified against the repository at implementation time in case new capabilities land between now and then.

### 4. Quickstart

Grounded in what actually works today. Two code blocks:

- **Install and run.** `go install github.com/joelhelbling/kkullm@latest`, then `kkullm serve`, then open `http://localhost:8080`. Mentions pure-Go modernc SQLite (no CGO required) as a one-line callout.
- **CLI round-trip.** A short example showing how to create a project, create a card, and list cards as an agent, using the `--as` flag or `KKULLM_AGENT` environment variable. Concrete commands that a reader can paste and run.

Implementation note: commands and flags must be verified against `cmd/` before writing, so the quickstart matches the real CLI surface.

### 5. Concepts

The mental model a reader needs to use Kkullm well, in prose rather than a feature list:

- **Cards** as the central unit of work, with the full status lifecycle: `considering` → `todo` → `in_flight` → `completed` → `done`, plus `tabled` and `blocked`.
- **The blackboard pattern.** Agents pull cards relevant to them rather than being pushed tasks by a central scheduler. This is the load-bearing design idea.
- **Card relationships.** `blocked_by`, `belongs_to`, `interested_in` — each with one-line explanation.
- **Agents and projects** as first-class entities on the board.
- **The two-session unattended execution pattern** as a design idea: an agent pulls actionable cards, prioritizes, composes a prompt, terminates, and the relaunched agent executes. Clearly marked as a design idea not yet wired up.

### 6. Is This For You?

Two short lists, placed post-Concepts so readers self-select from understanding, not from marketing.

- **Kkullm might be for you if…** you want to orchestrate multiple agents across unrelated domains; you're comfortable self-hosting; you like the blackboard pattern's "agents pull, humans don't push" stance; you want to keep the door open to swooping in and pairing interactively.
- **Kkullm probably isn't for you if…** you want a managed SaaS; you need enterprise auth/SSO today; you want a prescriptive methodology (Scrum, GTD, etc.) baked in; you need a battle-tested production system right now.

### 7. Roadmap

Three short groupings, kept tight and clearly marked as current thinking:

- **Near-term:** auth, Claude Code hook integration, agent profiles, two-session unattended loop.
- **Medium-term:** notifications, richer card relationships, multi-user considerations.
- **Longer-term / under consideration:** items Joel wants to flag as being thought about but not committed to.

The exact items in each group are finalized at implementation time in consultation with Joel.

### 8. Contributing

Brief. The project is early; issues and discussions are welcome; the design record lives at `docs/superpowers/specs/`; the stack is Go plus SQLite. No contributor license agreement or elaborate process — low friction appropriate to the stage.

### 9. License

MIT. The README's License section is a short paragraph naming the license and pointing at the `LICENSE` file at the repository root.

**Follow-up (blocker for README merge):** A `LICENSE` file containing the standard MIT license text, with Joel Helbling as the copyright holder and 2026 as the copyright year, must be added to the repository root as part of the same change set as the README.

### 10. For Your Assistant

The dense technical primer, placed dead last after License. Seven parts in order:

1. **Directive paragraph.** Addressed to the AI assistant, not the human. Frames the assistant's job: the reader is likely being helped to decide whether Kkullm fits, or to understand how it works. What follows is a dense technical primer; use it alongside whatever context the assistant already has about the reader.
2. **Raw URL in a code fence, by itself**, so GitHub's copy button grabs it cleanly:

   ```
   https://raw.githubusercontent.com/joelhelbling/kkullm/main/README.md
   ```

3. **Architecture at a glance.** One paragraph: Go, single binary, modernc SQLite (pure Go, no CGO), HTTP API, server-rendered web UI, CLI client, all one process. Stateless request handlers; SSE for live board updates.
4. **Data model essentials.** Prose summary of the card schema, the status lifecycle, the three relationship types, and how projects and agents relate to cards. Dense, no bullet lists.
5. **Design decisions with rationale.** Why blackboard over push-scheduling. Why the two-session unattended execution pattern. Why low-opinion rather than a prescriptive workflow. Why SQLite for v1. Why Go. Each decision a sentence or two, leading with the *why*.
6. **Deliberate non-goals.** Not a managed SaaS. No built-in auth in v1. No prescriptive methodology. Not a replacement for Jira-style project management tools for humans-only teams. Not trying to be a general-purpose task runner or workflow engine.
7. **Decision axes.** A short paragraph naming the questions that usually matter when someone is weighing Kkullm: How many concurrent agentic projects does the reader have? How much do they value low-opinion flexibility versus guided workflow? Are they comfortable self-hosting? Do they want to stay in the loop as a pair-programmer, or fully delegate? This gives the assistant a frame for the conversation it is about to have.

This section is deliberately the densest part of the README. Its audience (an AI reader ingesting the document in one go) handles structure effortlessly.

## Length target

- Hook through "Is This For You?" should fit within what a first-visit reader is willing to absorb: roughly 500–700 words of flowing content, plus the screenshot and code blocks.
- Roadmap and Contributing stay tight — a few sentences each.
- License is a short paragraph.
- "For Your Assistant" can be as dense and long as needed; expect it to be a meaningful fraction of total length.

## Constraints and principles

- **No marketing fluff.** The primary audience is allergic to it. Confident, technical, specific.
- **Honesty about maturity.** "Where We Are" goes before Quickstart. No overselling.
- **Reality-grounded quickstart.** Commands verified against `cmd/` before writing.
- **No emoji.** Consistent with Joel's preferences and the project voice.
- **The README is the artifact.** No companion docs created as part of this work; future multi-page documentation is out of scope.

## Follow-ups tracked alongside this work

1. **Add `LICENSE` file** with MIT text, 2026, Joel Helbling. Blocker for README merge.
2. **Hero screenshot capture.** Seed a lightly tongue-in-cheek demo board via CLI, capture a framed screenshot, commit to the repository, reference from README. Can ship README with placeholder if needed.
3. **Update `CLAUDE.md`** — the "No code yet — only design documents in `docs/ideation/`" line is stale and contradicts the README's "Where We Are." Correct it to reflect the current state of the repository. Discovered during brainstorming; worth fixing as part of the same change set to avoid contradiction.

## What this spec does not cover

- Multi-page documentation site design.
- Website, landing page, or marketing copy beyond the README.
- Logo, favicon, or other branding assets.
- Contribution guidelines beyond a short README section (no `CONTRIBUTING.md`).
- Code of conduct document.
- GitHub issue or pull request templates.

These are all legitimate future work items but out of scope for the initial README.
