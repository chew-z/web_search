# GOLANG.md

Go conventions for `web_search`.
The house rules below hold across projects;
the `Stack` and `Project conventions` sections are this project's own.

## Go version

Go 1.27 — the `go` directive in `go.mod` is the source of truth.
Language features, `encoding/json/v2`, `go fix` modernization and version discipline
live in the `golang-modern` skill; use it rather than restating them here.

## Taskfile

Every Go project has a `Taskfile.yml`; it is the one place commands are defined.

- If the project has none, add one. If it no longer covers the project —
  a new binary, a code-generation step, a deploy or release target — update it
  in the same change.
- Expected tasks: `default` (lists tasks), `build`, `test`, `lint`, `fmt`,
  and `check` — the gate that runs at least fmt/vet, lint and test.
- The gate rule is `quality-gate.md` in the tool's rules folder (`.claude/rules/` or `.qoder/rules/`).

## Build output

All binaries go into `bin/`, which is gitignored.
Build through the Taskfile, or `go build -o bin/<name> <pkg>` — never into the repo root.

## House rules

- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`; match with `errors.Is` / `errors.As`.
- `ctx context.Context` is the first parameter of every function that does I/O.
- Keep `main` thin: exit with `os.Exit` from `main` or a `run()` wrapper;
  never `log.Fatal` or `log.Panic`.
- No `io/ioutil`; use `os.ReadFile`, `os.WriteFile` and friends.
- Never write `interface{}`; write `any`.
- Files are `snake_case.go`, named by concern (`config.go`, `errors.go`, `logging.go`);
  packages are single lowercase words.
- Layout follows size: a small tool stays a flat `package main` at the root;
  a service with real subsystems uses `cmd/<name>/main.go` plus `internal/<pkg>/`.
- Tests sit beside the code as `*_test.go`: stdlib `testing`, table-driven with `t.Run`,
  `t.Helper()` in helpers, `t.TempDir()` / `t.Setenv()` for state.
  Use testify only where the project already does.
- Lint with `golangci-lint` and format with `gofmt`; both clean before done.

## Stack

This project is a flat `package main` at the root; every file is listed in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#components).

- `github.com/mark3labs/mcp-go` — the MCP server (`server`) and MCP types (`mcp`).
  Version 0.56.0.
- `github.com/golang-jwt/jwt/v5` — HS256 bearer-token validation for the HTTP transport.
- `github.com/joho/godotenv/autoload` — blank import in `main.go` that loads `.env`
  before `main` runs.
- `encoding/json/v2` + `encoding/json/jsontext` — all wire encoding, GA since Go 1.27.
  Note the v2 defaults: nil slices marshal as `[]`, and names match case-sensitively.

No web framework, no database driver, and no charting library: the CLI is `flag`,
the transport is `net/http` plus mcp-go, and there is no persistence.

## Project conventions

- **One provider registry.** A backend is described entirely by the `provider` struct
  in `provider.go` — base URL, key env var, web-search tool type, default model,
  continuity support, prompt. To add a backend, add a registry entry; do not branch on
  the provider name anywhere else.
- **The binary is `answer`.** The module path is `github.com/chew-z/web_search`, so
  `go install` produces `web_search` — build through the Taskfile to get `answer`.
- **`verbosity` is never sent empty.** DeepSeek rejects an empty `text.verbosity` with a
  400, so the field is set from the validated verbosity level on every request.
- **Environment variables are CLI-only, except two.** `MODEL`, `EFFORT`, `TIMEOUT`,
  `QUESTION`, `SHOW_ALL`, and `PROMPT_CACHE_KEY` are read only by the CLI path;
  MCP callers pass the equivalent tool parameters. Only `PROVIDER` and the API key
  are read in both modes, plus `GEMINI_AUTH_SECRET_KEY` and
  `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` in MCP mode.
- **Errors have two exit paths.** CLI failures return an `exitError` that `main` maps to
  a process exit code; MCP failures return structured tool errors. Neither path calls
  `log.Fatal` or `log.Panic`.
- **Logging goes to stderr** through `log/slog` (`logging.go`), never stdout — stdout
  carries the CLI's answer and, in stdio mode, the MCP protocol itself.

### Project-only additions to the house rules

- Never write `interface{}`; write `any` (also a `go fix` rewrite).
- The four `run_*.sh` wrappers are scaffold-owned and gitignored: they delegate to
  `task` when it is installed and fall back to the raw tool otherwise. Put project
  commands in `Taskfile.yml`, not in those scripts.
