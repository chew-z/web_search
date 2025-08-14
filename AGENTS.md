# Repository Guidelines

This repository hosts a small Go CLI that sends a prompt to an API and prints the extracted text response. Use this guide to contribute safely and consistently.

## Project Structure & Module Organization
- `main.go`: Entry point; builds the request, performs HTTP call, prints output.
- `go.mod`: Module name `Answer` and Go toolchain version.
- `bin/`: Compiled binaries (e.g., `bin/answer`). Do not commit artifacts.
- `.gemini/`: Local tool settings (non-essential for builds).

## Build, Test, and Development Commands
- Build: `go build -o bin/answer .` — builds the CLI.
- Run (positional or `-q`): `go run . "Your question"` or `go run . -q "Your question"`.
- Lint/Format: `go fmt ./... && go vet ./...` — apply formatting and basic static checks.
- Tests (if present): `go test ./...` — run unit tests across packages.

## Coding Style & Naming Conventions
- Formatting: Always run `go fmt ./...` before pushing; commit formatted code only.
- Naming: Follow Go conventions — exported identifiers in `CamelCase`, unexported in `camelCase`; package names are short, lowercase, no underscores.
- Errors: Use concise `fail(code, msg, args...)` for fatal errors; prefer wrapped errors elsewhere (`fmt.Errorf("...: %w", err)`) when adding functions.
- Flags: Keep flags short and descriptive; document defaults in help text.

## Testing Guidelines
- Framework: Standard library `testing`. Name tests `*_test.go` near code under test.
- Coverage: Aim for meaningful coverage of parsing and response handling; avoid live network calls.
- Techniques: Use `httptest.Server` and pass a custom `-base` URL to simulate API responses.

## Commit & Pull Request Guidelines
- Commits: Imperative subject line (max ~72 chars). Group related changes. Example: `feat: add -timeout flag with default`.
- PRs: Include a clear description, rationale, and testing notes; reference issues (`Closes #123`). Add screenshots or sample outputs when relevant.

## Security & Configuration Tips
- Secrets: Do not commit or log tokens. Set `OPENAI_API_KEY` in your environment.
- Timeouts: Keep sensible HTTP timeouts; prefer per-request context if expanding functionality.
