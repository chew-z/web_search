# Release Notes

## v0.4.0 - 2026-10-21

### 💥 Breaking Changes

- **Auth secret renamed**: `GEMINI_AUTH_SECRET_KEY` is no longer read — set `ANSWER_AUTH_SECRET_KEY`
  (at least 32 bytes when OAuth is on). Auth is self-contained now; nothing is shared with GeminiMCP.
- **Static token pair changed**: issuer and audience are `answer-mcp` / `answer-mcp-user` instead of
  `gemini-mcp` / `gemini-mcp-user`, so tokens minted for the old pair are rejected with 401.
  Regenerate them with `answer mcp -generate-token`.
- **`exp` is required** on every bearer token; tokens minted without an expiry stop validating.
- **Claims slimmed**: the `user_id` and `role` claims are gone. The authenticated username travels as a
  request-context value (`actorFrom`) and shards the OpenAI prompt cache per user.

### 🎉 New Features

- **OAuth 2.1 Connector Auth**: a vendored, stdlib-only authorization server (`internal/oauth`) with
  RFC 9728 protected-resource metadata, RFC 8414 authorization-server metadata, RFC 7591 dynamic
  registration plus client-ID metadata documents, authorization-code flow with PKCE S256, rotating
  refresh-token families, a PBKDF2-SHA256 browser login, and a rate limiter. claude.ai and Claude
  Desktop connectors log in with an empty client ID and secret. Scope `answer`, realm `answer-mcp`,
  configured through `ANSWER_MCP_*`.
- **Operator Endpoints**: unauthenticated `/health` and `/capabilities` (server info, tool list, auth mode).
- **Credential CLIs**: `answer mcp -generate-token [-token-username -token-ttl]` mints a static bearer;
  `answer mcp -hash-password` produces the PBKDF2 hash for `ANSWER_MCP_LOGIN_PASSWORD_HASH` with a silent
  TTY read (`golang.org/x/term`).
- **Tool Annotations**: `gpt_websearch` advertises a title, `readOnlyHint`, `destructiveHint: false`,
  `idempotentHint`, and `openWorldHint`, so clients stop gating a read-only web search behind
  destructive-tool approval prompts.
- **Multi-Provider Support**: Choose between OpenAI (`openai`, default) and DeepSeek (`deepseek`)
  backends via the `PROVIDER` environment variable or the `-provider` flag (CLI and MCP modes).
  Each provider brings its own endpoint, API key env var (`OPENAI_API_KEY` / `DEEPSEEK_API_KEY`),
  web-search tool type, default model (`gpt-6-luna` / `deepseek-flash`), and MCP guidance prompt.
  Provider registry lives in `provider.go`.
- **Reasoning Effort Levels**: Effort selection expanded to `none`, `low`, `medium`, `high`, `xhigh` with matching timeout defaults (90s/3/5/10/15 min).

### 🔧 Improvements

- **mcp-go v0.56.0 → v1.3.0**: the stateless 2026-07-28 protocol core, with legacy initialize handshakes
  still negotiated so existing clients are unaffected.
- **One Auth Guard**: a single HTTP-layer guard validates one HS256 parse (signing method pinned, 60s clock
  leeway) against the static and OAuth token profiles; 401 challenges carry `resource_metadata` and `scope`
  (the RFC 9728 trigger that starts a connector login); request bodies are capped at 4 MiB; OPTIONS
  preflight passes unauthenticated.
- **Strict Configuration**: `ANSWER_MCP_*` booleans and durations parse strictly and startup validation
  names the offending variable; `MCPConfigParams` collapsed into `MCPConfig`.
- **Go 1.27**: the `go` directive moved to 1.27.0, `encoding/json/v2` is used directly (`GOEXPERIMENT` is
  gone), and the `go fix` modernizers were applied.
- **Cleaner Answers**: `ExtractAnswer` now strips intermediate narration messages emitted before/between tool calls (observed with DeepSeek),
  returning only the final answer text.
- **Continuity Scoping**: `previous_response_id` and `prompt_cache_key` are only exposed/used for providers that support conversation continuity (OpenAI);
  DeepSeek is stateless.
- **Provider Flag Ordering**: `-provider deepseek` now works even when `OPENAI_API_KEY` is unset (credentials are resolved after flag parsing).

### 📚 Documentation

- **Role Docs**: `docs/ARCHITECTURE.md`, `docs/STATUS.md`, and `docs/OPEN_ITEMS.md` carry the design, the
  health, and the debt; `docs/OPERATIONS.md` is the runbook for the karma unit, the nginx blocks
  (`deploy/nginx-answer.conf`), discovery paths, verification curls, and connector setup.
- Updated README, AGENTS.md, and CLAUDE.md for multi-provider configuration, model guidance, OAuth, and the
  `task`-based build and gate workflow (`task check`).

## v0.3.5 - 2025-12-08

### 🎉 New Features

- **Enhanced API Client**: Improved HTTP client with configurable timeouts and connection pooling for better reliability.
  Added response body size limiting and refined text extraction from API responses.
- **Structured Logging**: Introduced `slog`-based structured JSON logging with dynamic log level control, centralizing logging to MCP clients with thread-safe initialization.
- **Conversation Continuity**: Enhanced API client with conversation continuity via previous response ID, enabling seamless follow-up interactions.
- **Model Updates**: Updated model name references from GPT-5 to GPT-5.1 to reflect expected future model designations.

### 🔧 Improvements

- **Dependency Updates**: Updated minimum Go version to 1.25.5 and bumped `mcp-go` dependency to v0.43.2 for compatibility and latest features.
- **Gitignore Enhancements**: Added repomix and other generated files to `.gitignore` to keep repository clean.
- **Configuration**: Updated server version metadata and prepared for new configuration options related to logging verbosity and effort levels.

### 📚 Documentation

- Updated release notes and project documentation.
