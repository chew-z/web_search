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

- `task check` — green on 2026-10-13: `fmt:check`, `vet`, `lint:ci` (0 issues), `test:short`.
- `task lint:md` — green on 2026-10-13.
- Tests live beside the code as `*_test.go` and run without network access:
  `api_test.go`, `auth_test.go`, `config_test.go`, `deepseek_probe_test.go`,
  `logging_test.go`, `main_test.go`, `mcp_server_test.go`, `models_test.go`.
- Production runs commit `f76d4fc` since 2026-10-13: the `answer.service` systemd unit on
  the VPS executes `bin/answer mcp -port 8086 -transport http`, served behind a reverse
  proxy at `karma.rrj.pl/answer/mcp` (`.mcp.json`), with
  `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` left at its default.
  The provider comes from the unit's `Environment=` block, not from `.env`
  (see the precedence note in [ARCHITECTURE.md](ARCHITECTURE.md)); it was switched from
  `deepseek` to `openai` the same day and both were verified live.
  Deployment is a manual build-and-copy; nothing in this repository automates it.
- Releases: the last tagged release is v0.3.5 (2025-12-08, per `CHANGELOG.md`), and the
  version string in code is still `0.3.5` (config.go:24). No release since.

## Recent changes

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
