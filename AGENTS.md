# Repository Guidelines

## Project Structure & Modules
- `main.go`: CLI entry; starts MCP (`stdio`/`http`).
- `api.go`, `mcp_server.go`, `config.go`, `errors.go`, `prompts.go`, `transport.go`: core logic and server transport.
- `bin/`: compiled binaries (e.g., `bin/answer`).
- `tests/`: auxiliary scripts (e.g., `tests/test_logging.py`).
- `.env`/`.env.example`: runtime configuration (never commit secrets).

## Build, Test, Run
- Build: `go build -o bin/answer .` — compile the CLI/MCP server.
- Test (Go): `go test ./...` or `./run_test.sh` — run package tests.
- Lint: `./run_lint.sh` — runs `golangci-lint` if installed.
- Format: `./run_format.sh` or `go fmt ./...` — apply `gofmt`.
- CLI example: `./bin/answer -q "Latest AI developments" -model gpt-5-mini`.
- MCP (stdio): `./bin/answer mcp -t stdio`.
- MCP (HTTP): `./bin/answer mcp -t http -port 8080`.

## Coding Style & Naming
- Follow standard Go conventions. Use `gofmt` and keep imports organized.
- Indentation: tabs (default `gofmt`). Line length: be reasonable; wrap for readability.
- Packages: lower case, no spaces (`websearch`, `config`).
- Files: lower case with optional underscores (`mcp_server.go`).
- Names: exported `CamelCase`, unexported `camelCase`. Avoid abbreviations unless idiomatic (`ID`, `URL`).
- Errors: wrap with context; prefer `errors.Is/As` and sentinel errors in `errors.go`.

## Testing Guidelines
- Go tests co-located as `*_test.go`. Name tests `TestXxx` and keep them hermetic.
- Python utility: `tests/test_logging.py` exercises MCP logging over stdio.
  - Example: `python3 tests/test_logging.py` (requires built `bin/answer`).
- Aim for coverage on public behavior and edge cases; avoid external network in unit tests.

## Commit & PR Guidelines
- Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`. Scope optional, e.g., `feat(websearch): add conversation continuity`.
- PRs must include: concise description, motivation/linked issue, usage examples (commands), and any behavior changes.
- Add screenshots or logs when relevant (e.g., MCP responses). Keep diffs focused; update README if flags/usage change.

## Security & Config Tips
- Required: `OPENAI_API_KEY` via environment or `.env`. Do not log secrets.
- Prefer dependency updates via `go get -u` and verify `go.sum` changes are minimal.
- For HTTP mode, validate inputs and avoid binding to public interfaces by default in development.
