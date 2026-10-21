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

- **`Claims.Role` is populated but unreachable** — `auth.go:44` declares it and
  `identityOf` (auth.go:180) fills it from the token, but `getUserInfo` (auth.go:215)
  returns only the user id and username, so nothing can read the role. Verified
  2026-10-13, re-verified against the guard rework 2026-10-21.

## Debt

- **2026-10-13 — no tagged release since v0.3.5 (2025-12-08)** while the code has moved
  through multi-provider support, json/v2 migration, and mcp-go v1.3.0.
- **2026-10-21 — OAuth state is in-memory** (`oauth.NewStore(nil)`, transport.go:116):
  a restart drops registered clients and refresh tokens, so connectors need a fresh
  browser login once their access token expires, and an issued access token stays valid
  until its `exp` no matter what — keep `ANSWER_MCP_ACCESS_TTL` short. The same trade-off
  is accepted in Pushover and Alerts; see OPERATIONS.md troubleshooting.
- **2026-10-21 — the karma deployment of the OAuth port is pending** (PLAN.md Phase 8):
  unit `Environment=` additions, the atomic nginx edit (three new blocks plus removing
  `answer` from the shared GET-denying alternation), the live verification curls, and the
  claude.ai connector login. Until then production still runs the pre-OAuth build.
