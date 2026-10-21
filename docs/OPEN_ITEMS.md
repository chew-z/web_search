---
title: web_search open items
date: 2026-10-13
status: active
tags: [open-items]
---

# Open items

> Everything still unresolved: verified defects, debt, and decisions waiting on a human.
> Current state lives in [STATUS.md](STATUS.md).
> Resolved items move to docs/retired/ or are deleted; this file lists only what is open.

## Decisions waiting on a human

- **Is the `gopls-lsp` plugin intentionally off?** `.claude/settings.json` disables
  `gopls-lsp@claude-plugins-official` while the scaffold enables it for Go projects, and
  this project ships an `LSP.md` describing the gopls workflow. Flip it back on, or record
  why it is off. Owner: project owner.
- **`run_release.sh` is stale — delete or repair?** It references `chew-z/GeminiMCP`,
  builds a `mcp-gemini` binary, and hardcodes `v0.1.0`; its own header comment admits it
  was copy-pasted and says fixing it is out of scope. `task release` supersedes it.
  Owner: project owner.
- **Which changelog is canonical?** `CHANGELOG.md` and `Release Notes.md` restate the same
  release history, both still list the multi-provider work as "Unreleased", and the
  version in code is pinned at `0.3.5` (config.go:24).
  Pick one file to maintain, or describe the release process in a rule.
  Owner: project owner.
- **Two proposed rules are waiting on a yes or no.** Both were drafted but deliberately not
  written — rules are only added on approval — so say the word and they land.
  - `release.md`, scoped to `CHANGELOG.md`, `Release Notes.md`, and `config.go`: the version
    lives in `serverVersion` (config.go:24) and in the changelog heading, both of which have
    to move together; five GitHub releases exist (v0.1.0 … v0.3.5, tag plus notes, no binary
    asset) with no documented procedure, and the one script that claims to do it is stale.
  - `commits.md`, scoped to `**`: Conventional Commits, which every commit in the log
    already follows (`feat(transport): …`, `chore: …`). Recovered from `AGENTS.old.md`.

## Defects

- **The README documents three HTTP endpoints that do not exist** — README.md:184-189
  lists `GET /` (API documentation), `GET /health`, and `POST /message`, but the HTTP
  transport mounts the MCP streamable handler at `/` and nothing else
  (transport.go:26, transport.go:68). There is no health route and no `/message` route;
  a JSON-RPC 2.0 payload goes to the MCP endpoint itself. Verified 2026-10-13.
- **`Claims.Role` is populated but unreachable** — `auth.go:32` declares it and
  `auth.go:124` fills it from the token, but `getUserInfo` (auth.go:132) returns only the
  user id and username, so nothing can read the role. Verified 2026-10-13.

## Debt

- **2026-10-13 — no tagged release since v0.3.5 (2025-12-08)** while the code has moved
  through multi-provider support, json/v2 migration, and mcp-go v1.3.0.
