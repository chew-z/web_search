# Repository Guidelines

This document helps contributors build, test, and extend the Answer CLI/MCP server. Keep changes focused, idiomatic, and well‑documented.

## Project Structure & Module Organization
- `main.go`: CLI entry; starts MCP (`stdio`/`http`).
- `api.go`, `mcp_server.go`, `config.go`, `errors.go`, `prompts.go`, `transport.go`: core logic and transports.
- `bin/`: compiled binaries (e.g., `bin/answer`).
- `tests/`: auxiliary scripts (e.g., `tests/test_logging.py`).
- `.env` / `.env.example`: runtime configuration (never commit secrets).

## Build, Test, and Development Commands
- Build: `go build -o bin/answer .` — compile the CLI/MCP server.
- Test (Go): `go test ./...` or `./run_test.sh` — run package tests.
- Lint: `./run_lint.sh` — runs `golangci-lint` if installed.
- Format: `./run_format.sh` or `go fmt ./...` — apply `gofmt`.
- Run CLI: `./bin/answer -q "Latest AI developments" -model gpt-5-mini`.
- MCP (stdio): `./bin/answer mcp -t stdio`.
- MCP (HTTP): `./bin/answer mcp -t http -port 8080`.

## Coding Style & Naming Conventions
- Go style, formatted with `gofmt`; tabs for indentation; wrap long lines reasonably.
- Packages: lower case; files: lower case with optional underscores.
- Names: exported `CamelCase`, unexported `camelCase`; use idioms (`ID`, `URL`).
- Errors: wrap with context; prefer `errors.Is/As`; define sentinel errors in `errors.go`.

## Testing Guidelines
- Go tests co-located as `*_test.go`; name `TestXxx`; keep tests hermetic (no network).
- Run tests: `go test ./...`.
- MCP logging utility: build first, then `python3 tests/test_logging.py`.

## Commit & Pull Request Guidelines
- Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`; optional scope (e.g., `feat(websearch): ...`).
- PRs include: concise description, motivation/linked issue(s), usage examples (commands), behavior changes, and screenshots/logs when relevant. Keep diffs focused; update README if flags/usage change.

## Security & Configuration Tips
- Require `OPENAI_API_KEY` via environment or `.env`; never log or commit secrets.
- Update deps via `go get -u`; verify `go.sum` diffs are minimal.
- In HTTP mode, validate inputs and avoid binding to public interfaces by default in development.

