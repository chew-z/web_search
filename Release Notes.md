# Release Notes

## v0.4.0 - October 21, 2026

### 💥 Breaking Changes

- **Authentication Is Self-Contained:**
  - `GEMINI_AUTH_SECRET_KEY` is no longer read. Set `ANSWER_AUTH_SECRET_KEY` instead — at least 32 bytes
    when OAuth is enabled. Nothing is shared with the GeminiMCP service any more, and the production
    secret was rotated as part of the change.
  - The static token pair is now issuer `answer-mcp` with audience `answer-mcp-user`, replacing
    `gemini-mcp` / `gemini-mcp-user`. Tokens minted for the old pair are rejected with 401; regenerate them
    with `answer mcp -generate-token`.
  - Every bearer token now needs an `exp` claim, and the `user_id` and `role` claims are gone: the
    authenticated username travels as a request-context value and shards the OpenAI prompt cache per user.

### 🎉 New Features

- **OAuth 2.1 Connector Authentication:**
  - A vendored, standard-library-only authorization server (`internal/oauth`) brings RFC 9728
    protected-resource metadata, RFC 8414 authorization-server metadata, RFC 7591 dynamic registration with
    client-ID metadata documents, the authorization-code flow with PKCE S256, rotating refresh-token
    families, a PBKDF2-SHA256 browser login, and a rate limiter.
  - claude.ai, Claude Desktop, and Claude mobile connectors log in with an empty client ID and secret:
    the 401 challenge carries `resource_metadata`, which is what starts the browser flow.
  - Configured through `ANSWER_MCP_OAUTH`, `ANSWER_MCP_ISSUER`, `ANSWER_MCP_RESOURCE`,
    `ANSWER_MCP_LOGIN_USER`, `ANSWER_MCP_LOGIN_PASSWORD_HASH`, and the two TTL variables. Scope `answer`,
    realm `answer-mcp`. State is in-memory, so a restart eventually asks connectors to log in again.
- **Operator Endpoints:**
  - Unauthenticated `/health` for liveness probes and `/capabilities` for server info, the tool list, and
    the auth mode — both mounted outside the guard.
- **Credential Command-Line Helpers:**
  - `answer mcp -generate-token [-token-username <user>] [-token-ttl <duration>]` mints a static bearer
    token (default lifetime 744h).
  - `answer mcp -hash-password` produces the PBKDF2-SHA256 hash for `ANSWER_MCP_LOGIN_PASSWORD_HASH`,
    reading the password silently from a TTY.
- **Tool Annotations:**
  - `gpt_websearch` now advertises a title plus `readOnlyHint: true`, `destructiveHint: false`,
    `idempotentHint: true`, and `openWorldHint: true`. Without them the library emitted the MCP spec
    defaults, so clients were told a read-only web search is destructive and gated it behind approval
    prompts.
- **Multi-Provider Support:**
  - Choose between OpenAI (`openai`, default) and DeepSeek (`deepseek`) backends via the `PROVIDER` environment variable
    or the `-provider` flag (CLI and MCP modes).
  - Each provider brings its own endpoint, API key env var (`OPENAI_API_KEY` / `DEEPSEEK_API_KEY`), web-search tool type,
    default model (`gpt-6-luna` / `deepseek-flash`), and MCP guidance prompt. Provider registry lives in `provider.go`.
- **Reasoning Effort Levels:**
  - Effort selection expanded to `none`, `low`, `medium`, `high`, `xhigh` with matching timeout defaults (90s/3/5/10/15 min).

### 🔧 Improvements

- **mcp-go v0.56.0 → v1.3.0:**
  - Brings the stateless 2026-07-28 protocol core while legacy initialize handshakes still negotiate, so
    existing clients are unaffected. Both transports were verified live after the bump.
- **One Authentication Guard:**
  - A single HTTP-layer guard validates one HS256 parse — signing method pinned against algorithm
    confusion, 60 seconds of clock leeway, non-empty username — against the static and OAuth token
    profiles.
  - Failures return 401 with a `WWW-Authenticate` challenge carrying `realm`, `error`, and (with OAuth on)
    `resource_metadata` and `scope`; a valid token with the wrong scope gets 403 `insufficient_scope`.
  - Request bodies are capped at 4 MiB and OPTIONS preflight passes unauthenticated, per the CORS spec.
- **Strict Configuration and Startup Validation:**
  - `ANSWER_MCP_*` booleans and durations are parsed strictly, and a malformed value stops startup with the
    variable name and the offending value.
  - With OAuth on, startup validates the issuer and resource as absolute URLs, the login user, the
    password-hash format, the secret length, and the TTL ordering. `MCPConfigParams` collapsed into
    `MCPConfig`.
- **Go 1.27:**
  - The `go` directive moved to 1.27.0 and `encoding/json/v2` is used directly, so `GOEXPERIMENT=jsonv2` is
    gone from the Taskfile and the run wrappers; the `go fix` modernizers were applied across the tree.
- **Cleaner Answers:**
  - Answer extraction now strips intermediate narration messages emitted before/between tool calls
    (observed with DeepSeek), returning only the final answer text.
- **Continuity Scoping:**
  - `previous_response_id` and `prompt_cache_key` are only exposed/used for providers that support conversation continuity (OpenAI); DeepSeek is stateless.
- **Provider Flag Ordering:**
  - `-provider deepseek` now works even when `OPENAI_API_KEY` is unset (credentials are resolved after flag parsing).

### 📚 Documentation

- **Role Docs:**
  - `docs/ARCHITECTURE.md` (design), `docs/STATUS.md` (health and recent changes), and
    `docs/OPEN_ITEMS.md` (defects, debt, pending decisions) are kept current against the code, with
    `file:line` citations re-verified.
  - `docs/OPERATIONS.md` is the deployment runbook: the karma systemd unit, the nginx blocks shipped as
    `deploy/nginx-answer.conf`, discovery paths, verification curls, connector setup, and secret rotation.
- Updated README, AGENTS.md, and CLAUDE.md for multi-provider configuration, model guidance, OAuth, and the
  `task`-based build and gate workflow (`task check`).

## v0.3.5 - December 08, 2025

### 🎉 New Features

- **Enhanced API Client and Response Handling:**
  - Introduced a more robust HTTP client with configurable timeouts and connection pooling to enhance API call reliability.
  - Limited response body size to prevent excessive memory usage and processing large, potentially malformed responses.
  - Refined logic for extracting text from API responses to correctly concatenate multiple text segments into a single, coherent answer.
- **Structured Logging with Dynamic Level Control:**
  - Introduced structured logging using the `slog` package to provide better visibility into the application's behavior.
  - Centralized logging logic, allowing for dynamic control of log levels, specifically enabling debug logging when the verbose flag is set.
  - Enhanced observability by providing consistent, machine-readable log output.
- **Conversation Continuity and API Improvements:**
  - Added comprehensive tests for the API client and response handling, significantly improving stability and reliability.
  - Implemented conversation continuity via previous response ID, enabling seamless follow-up interactions.
- **Model Name Updates:**
  - Updated model name references from GPT-5 to GPT-5.1 across documentation and prompt configuration files
    to reflect expected future model designations, ensuring consistency for complex reasoning tasks.

### 🔧 Improvements

- **Dependency and Language Updates:**
  - Updated the minimum required Go version to 1.25.5 to ensure compatibility with newer language features and address known issues.
  - Bumped the `mcp-go` dependency to v0.43.2 to incorporate recent improvements and fixes from the library.
- **Repository Maintenance:**
  - Updated `.gitignore` file to include repomix-output.xml and other generated files, preventing accidental commits and keeping the repository clean.
- **Configuration and Versioning:**
  - Updated server version metadata and prepared for new configuration options related to logging verbosity and effort levels.

### 📚 Documentation

- Updated project release notes and documentation.

### ⚙️ Internal Changes

- Refactored model name constants and references throughout the codebase for consistency.
- Enhanced configuration loading and validation logic.
