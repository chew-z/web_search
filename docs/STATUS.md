---
title: web_search status
date: 2026-10-13
status: active
tags: [status]
---

# Status

> Where the project stands today.
> Design lives in [ARCHITECTURE.md](ARCHITECTURE.md); unresolved work in [OPEN_ITEMS.md](OPEN_ITEMS.md).

## Toolchain

| What | Version | Notes |
| --- | --- | --- |
| `go` directive | 1.27.0 (go.mod) | The source of truth, not the installed toolchain. |
| Installed Go | go1.27.2 darwin/arm64 | Matches the directive. |
| golangci-lint | 2.14.0 | Built with go1.27.2; invoked through `task lint:ci`. |
| go-task | 3.54.0 | `/opt/homebrew/bin/task`. |
| markdownlint-cli2 | 0.23.3 (markdownlint 0.41.1) | Invoked through `task lint:md`. |

The directive is a floor, not a preference.
`encoding/json/v2` and `encoding/json/jsontext` are GA in Go 1.27, and Go 1.27's
`stdversion` vet check — on by default under `go test` — fails the build when json/v2
symbols are used from a module whose directive is older.
That is why the module moved from 1.26.0 to 1.27.0 on 2026-10-13, and why
`GOEXPERIMENT=jsonv2` is gone from the Taskfile and the `run_*.sh` wrappers.

## Health

- `task check` — green on 2026-10-21: `fmt:check`, `vet`, `lint:ci` (0 issues), `test:short`;
  `go test -race ./...` green the same day.
- `task lint:md` — green on 2026-10-21.
- Tests live beside the code as `*_test.go` and run without network access:
  `api_test.go`, `auth_test.go`, `config_test.go`, `deepseek_probe_test.go`,
  `logging_test.go`, `main_test.go`, `mcp_cli_test.go`, `mcp_server_test.go`,
  `models_test.go`, `oauth_e2e_test.go`, plus the vendored suite under `internal/oauth/`.
- Production runs commit `7bc8225` since 2026-10-21, with OAuth on: the `answer.service`
  systemd unit executes `bin/answer mcp -port 8086 -transport http` from the repo checkout
  at `/home/debian/Go/src/Answer`, behind nginx at `karma.rrj.pl/answer/mcp` with the
  three blocks from `deploy/nginx-answer.conf` (the shared GET-denying alternation no
  longer includes `answer`). `ANSWER_MCP_OAUTH=true` also switched the JWT guard on —
  before this deploy the endpoint carried a secret but ran unauthenticated.
  Verified live on 2026-10-21: 401 challenge with `resource_metadata`, both discovery
  documents, `/health`, `/capabilities`, the full register → login → token → MCP →
  refresh flow, static-JWT interoperability (on the `gemini-mcp` pair that commit still
  used), and bogus-token rejection. Provider and keys come from the unit's `Environment=`
  block, not from `.env` (see the precedence note in [ARCHITECTURE.md](ARCHITECTURE.md)).
  Deployment is manual: push, then on the VPS `git pull`, `task deps`,
  `task build`, restart the unit; nothing in this repository automates it.
  Outstanding: the claude.ai connector login itself (browser, owner's account).
- **The GeminiMCP decouple below is committed but not on the box.** Production still runs
  `7bc8225` with `GEMINI_AUTH_SECRET_KEY` and the `gemini-mcp` static pair.
  Install the updated unit (`tmp/answer.service`: rotated secret,
  `ANSWER_AUTH_SECRET_KEY`) _before_ restarting on the new commit — the old unit sets a
  variable the new binary never reads, so startup fails validation.
  After the restart every previously issued token is dead (the secret and the static pair
  both changed): regenerate static tokens with `mcp -generate-token` and have connectors
  log in again.
- Releases: the last tagged release is v0.3.5 (2025-12-08, per `CHANGELOG.md`), and the
  version string in code is still `0.3.5` (config.go:27). No release since.

## Recent changes

### 2026-10-21 — decoupled from GeminiMCP

- `ANSWER_AUTH_SECRET_KEY` replaces `GEMINI_AUTH_SECRET_KEY` in `main.go`, `config.go`,
  `mcp_cli.go`, the staged unit, and the docs; the production secret was rotated at the same
  time (`openssl rand -hex 32`), so the rename and the rotation land together.
- The static JWT pair is now `answer-mcp` / `answer-mcp-user` (auth.go:17) instead of the
  borrowed `gemini-mcp` / `gemini-mcp-user`. Nothing here needs GeminiMCP's issuer any more,
  and a token minted for that service no longer opens this one.
- `Claims` lost `user_id` and `role` — the guard only ever read the username, and the role
  was unreachable (the OPEN_ITEMS defect this closes). `getUserInfo` / `identityOf` collapse
  into `actorFrom(ctx)` over an `actorKey{}` context value.
- Tests, `tmp/mktoken`, and the docs follow the new pair and variable.

### 2026-10-21 — OAuth 2.1 connector auth, mcp-go v1.3.0

- mcp-go upgraded v0.56.0 → v1.3.0 (matches Pushover; the stateless 2026-07-28 protocol
  core is supported but legacy initialize handshakes still negotiate, so clients are
  unaffected); both transports smoked live after the bump.
- `internal/oauth` vendored from Pushover (identical to the Alerts copy modulo naming):
  OAuth 2.1 authorization server with RFC 9728/8414 discovery, dynamic registration
  plus CIMD, PKCE S256, refresh families, PBKDF2 login, rate limiter, ceilinged store.
  Scope `answer`, realm `answer-mcp`.
- `auth.go` rebuilt on the sibling `authGuard` pattern: one HTTP-layer guard accepting the
  legacy static pair (`gemini-mcp`/`gemini-mcp-user`, GeminiMCP-compatible) beside the
  OAuth pair; 401 challenges carry `resource_metadata` so claude.ai connectors start the
  browser login. `exp` is now required on every bearer; a 4 MiB body cap was added.
- `ANSWER_MCP_*` env set, startup validation, and the `mcp -generate-token` /
  `mcp -hash-password` credential CLIs (x/term added).
- `/health` and `/capabilities` endpoints added (unauthenticated, Pushover parity).
- Full e2e test walks discovery → register → login → token → refresh → MCP call;
  mutation checks confirm the scope and challenge assertions bite.
- Deploy artifacts installed on karma and verified live the same day:
  `deploy/nginx-answer.conf` (three blocks, `answer` removed from the shared
  alternation and backend map) and the unit's `ANSWER_MCP_*` block;
  [OPERATIONS.md](OPERATIONS.md) carries the runbook. `MCPConfigParams` collapsed
  into `MCPConfig`; the README endpoint list was corrected.

### 2026-10-13 — model configuration refresh

- OpenAI collapsed to the single model `gpt-6-luna`, replacing the `gpt-5.4-nano` / `gpt-5.4-mini` / `gpt-5.4` ladder.
- `gpt-5.4-nano` is deprecated with an April 1, 2027 shutdown.
- The OpenAI Responses API web search tool type moved from `web_search_preview` to `web_search` to match the model's documented tool list;
  confirmed live the same day — a `gpt-6-luna` call with search on returned a cited answer, where the old name would have failed with a 400.
- The retired `deepseek-v4-flash` alias was replaced by the rolling alias `deepseek-flash` throughout the docs and the DeepSeek probe file.
- The `deepseek-v4-pro` probe was dropped.

### 2026-10-13 — re-initialized from the Go scaffold, modernized to Go 1.27

- `go.mod` directive 1.26.0 → 1.27.0; `GOEXPERIMENT=jsonv2` removed from `Taskfile.yml`
  and the three `run_*.sh` wrappers; the wrappers now delegate to `task`.
- `go fix` applied: `interface{}` → `any`, redundant per-iteration loop-variable copies
  dropped, `errors.AsType`, `slices.Contains`, and `bytes.SplitSeq` adopted.
- `Taskfile.yml` rebuilt on the house task set — `build`, `run`, `test`, `test:short`,
  `cover`, `fmt`, `fmt:check`, `vet`, `lint`, `lint:ci`, `lint:md`, `tidy`, `deps:update`,
  `check`, `clean`, `release` — with the binary at `bin/answer` and no `GOEXPERIMENT`.
- `.gitignore` reworked: `.claude/*` + `!.claude/rules/` and `.qoder/*` + `!.qoder/rules/`
  so the rules travel with the repo; `AGENTS.md`, `GOLANG.md`, and `*_test.go` un-ignored;
  `*.scaffold`, `.task/`, `coverage.out`, `LSP.md`, and `run_*.sh` added.
  Three test files that had been silently excluded by the `*_test.go` rule are now
  visible to git (see [OPEN_ITEMS.md](OPEN_ITEMS.md)).
- Gate config adopted and tracked: `.golangci.yml`, `.markdownlint.yaml`,
  `.markdownlint-cli2.jsonc`, `tools/markdownlint/`.
- 11 golangci-lint findings cleared across `api.go`, `main.go`, and three test files
  (errcheck, noctx, nolintlint, unparam, staticcheck).
- Role docs written (`docs/ARCHITECTURE.md`, `docs/STATUS.md`, `docs/OPEN_ITEMS.md`),
  `GOLANG.md` filled with this project's stack and conventions, and `POSTGRES.md` deleted —
  the project has no database.
