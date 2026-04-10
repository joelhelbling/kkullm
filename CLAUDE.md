# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kkullm (from Korean 끌림, "kkeullim" — to be drawn/pulled toward) is an agent orchestration system based on the classic blackboard pattern. The name embeds "LLM" intentionally. It combines concepts from kanban boards and Slack-like team chat to enable messaging between users and AI agents, and between agents themselves.

## Product Vision

Kkullm is planned as an open-source, self-hosted system, with a web UI for humans, and an API+CLI for AI agents.  It should be as self-contained as possible, and should leverage free and easy components such as SqlLite for storage.

## Core Concepts

- **Cards**: The central unit of work. Cards have a title, body, assignee(s), comments, project, tags, related cards, and a status lifecycle: `considering` → `todo` → `in_flight` → `completed` → `done` (also `tabled`, `blocked`)
- **Blackboard pattern**: Agents pull cards relevant to them rather than being pushed tasks
- **Agent profiles**: Agents have defined responsibilities/roles and capabilities within Kkullm
- **Two session modes**: Unattended (agent pulls cards, prioritizes, composes prompt, restarts to execute) and user-interactive (collaborative prompt composition)
- **Card relationships**: `blocked_by`, `belongs_to`, `interested_in`

## Planned Interfaces

- Simple CLI for efficient interaction
- Web UI for human users
- Notifications system
- Claude Code hook integration (pulls actionable cards on startup)

## Project Status

Early-stage implementation. Working today: Go backend, HTTP API, SQLite store (via `modernc.org/sqlite`, no CGO), server-rendered web UI with SSE live updates, and a Cobra-based CLI. Design documents live in `docs/ideation/`; specs and implementation plans live in `docs/superpowers/`.
