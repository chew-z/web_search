---
title: web_search architecture
date: 2026-10-13
status: active
tags: [architecture]
---

# web_search architecture

> How the system is built, as verified against the code.
> Current health lives in [STATUS.md](STATUS.md); unresolved work in [OPEN_ITEMS.md](OPEN_ITEMS.md).

## What the system does

`web_search` (built as `bin/answer`) wraps the OpenAI-style Responses API to give a caller a web-searching deep-reasoning model.
It has two front ends over one core:
a one-shot CLI (`answer -q "..."`) and an MCP server (`answer mcp`) that exposes a `gpt_websearch` tool to MCP clients such as Claude Code and Claude Desktop.
The upstream is one of two providers — OpenAI's `gpt-6-luna` or DeepSeek — chosen by flag or environment variable.

The only outbound network call is a POST to the selected provider's `/responses` endpoint, built at api.go:88 and sent at api.go:95.
Responses are capped at 10 MB (api.go:18).

## Components

Everything is `package main` at the repository root except the vendored `internal/oauth`
authorization server; the split below is by concern, not by package.

| File | Reach | What it owns |
| --- | --- | --- |
| `main.go` | CLI + MCP entry | `main` routes on `os.Args[1] == "mcp"` (main.go:19); `runCLI`; `runMCPMode` (main.go:34) and the credential commands. |
| `api.go` | shared | `CallAPI` (api.go:56), the one outbound HTTP call; `ExtractAnswer` (api.go:166); `HandleWebSearch` (api.go:223). |
| `provider.go` | shared | The `provider` struct (provider.go:22) and `providerRegistry` (provider.go:33); lookups at provider.go:66, provider.go:80. |
| `config.go` | shared | `loadEnvConfig` (config.go:155), `parseMCPConfig` (config.go:228), OAuth load and validation (config.go:262-315). |
| `models.go` | shared | `effortRegistry` (models.go:20), `verbosityLevels` (models.go:29), `modelRegistry` (models.go:83), plus the schema helpers. |
| `prompts.go` | MCP only | The two guidance prompts: `webSearchPrompt` (prompts.go:4) and `deepseekWebSearchPrompt` (prompts.go:124). |
| `mcp_server.go` | MCP only | `NewMCPServer` (mcp_server.go:26), the tool/resource/prompt registrations and handlers, `logToClient`. |
| `transport.go` | MCP only | `RunHTTPTransport`, `buildHTTPHandler`: guarded MCP catch-all, `/health`, `/capabilities`, OAuth mount, timeouts. |
| `auth.go` | MCP HTTP only | Token minting (auth.go:36-71), `parseBearerToken` (auth.go:86), guard `protectMCP` (auth.go:131), `actorFrom` (auth.go:191). |
| `mcp_cli.go` | MCP only | Credential helpers: `runGenerateToken` (mcp_cli.go:55), `runHashPassword` (mcp_cli.go:71), `readPassword` seam (mcp_cli.go:21). |
| `internal/oauth/` | MCP HTTP only | Vendored OAuth 2.1 authorization server (Pushover copy); entry `Mount` (mount.go:9), design under Transport and auth. |
| `logging.go` | both | A `slog` JSON logger on stderr, initialised by `initLogger` (logging.go:19), plus the `Debug`/`Info`/`Warn`/`Error` wrappers. |
| `errors.go` | both | `ErrNoAPIKey`, `ErrInvalidProvider`, `APIError`, and the CLI-only `exitError` / `fail`. |

## Data flow

### CLI

1. `runCLI` (main.go:249) does an advisory `loadEnvConfig("")` (main.go:254) purely to seed
   flag defaults, ignoring its error.
2. `parseCLIArgs` (main.go:179) defines the flags, calls `flag.Parse`, and resolves the
   question from `-q`, `-question`, or the positional argument (main.go:229).
3. Effort and verbosity are normalised to `medium` when unrecognised
   (main.go:209-210, config.go:210, config.go:218).
4. Timeout precedence: the `-timeout` flag, else `TIMEOUT`, else
   `getTimeoutForEffort(effort)` (main.go:199, main.go:211-213).
5. The authoritative `loadEnvConfig(args.provider)` (main.go:258) validates the provider and
   requires its API key; failure exits with code 2.
6. `resolveEndpointAndModel` (main.go:153) applies provider defaults when `-base` / `-model`
   were not supplied.
7. The prompt cache key is resolved only when the provider supports continuity (main.go:273).
8. `CallAPI` (api.go:56) applies `context.WithTimeout` (api.go:85), attaches the web-search
   tool when enabled (api.go:74), and sets `Authorization: Bearer` (api.go:93).
9. Output: `-show-all` prints the raw response as indented JSON (main.go:293); otherwise
   `ExtractAnswer` (api.go:166) strips the narration and prints the answer.
   An empty answer exits with code 3.

### MCP

1. `runMCPMode` (main.go:34) parses flags from `os.Args[2:]`; `-transport` overrides `-t`.
   The one-shot credential commands `-generate-token` and `-hash-password` run before any
   provider or API-key loading and exit (main.go:74-82).
2. Provider and API key come from `loadEnvConfig` (main.go:85); the JWT secret from
   `ANSWER_AUTH_SECRET_KEY` (main.go:111) and the OAuth settings from `loadOAuthEnv`
   (main.go:115, config.go:262). `Validate` (config.go:302) refuses startup on a missing
   secret, a secret shorter than 32 bytes with OAuth on, or an invalid OAuth derivation.
3. `parseMCPConfig` (config.go:228) fills the remaining defaults, then `NewMCPServer`
   (main.go:125) builds the server and `runTransport` (main.go:133) starts it.

Per tool call: `webSearchHandler` (mcp_server.go:137) → `HandleWebSearch` (api.go:223) →
`CallAPI` → `ExtractAnswer` → a `WebSearchResult` returned as structured-only content
(mcp_server.go:202).

`HandleWebSearch` re-validates effort and verbosity (api.go:242) and derives the timeout
from the effort alone (api.go:247), so MCP calls cannot override the timeout per call.
For continuity providers the cache key is the caller's `prompt_cache_key`, else
`serverName:userID` from the JWT context, else `serverName` (api.go:251);
for stateless providers `previous_response_id` is cleared (api.go:255).
An empty query returns a `Success: false` result with a nil error (api.go:224).

## Providers

The registry (provider.go:33) is the single place a backend is described.

| Provider | API key env | Base URL | Web-search tool | Default model | Continuity |
| --- | --- | --- | --- | --- | --- |
| `openai` (default) | `OPENAI_API_KEY` | `https://api.openai.com/v1/responses` | `web_search` | `gpt-6-luna` | yes |
| `deepseek` | `DEEPSEEK_API_KEY` | `https://api.deepseek.com/responses` | `web_search` | `deepseek-flash` | no |

Both rows were confirmed against the live providers on 2026-10-13, through the deployed
server at `karma.rrj.pl/answer/mcp`: each accepted `web_search` as the tool type,
`reasoning.effort: none`, and `text.verbosity`,
and the OpenAI continuity path round-tripped a real `previous_response_id`
together with a caller-supplied `prompt_cache_key`.
The two differ sharply in search freshness, which matters for a tool whose purpose is current facts:
asked for the current Go release, `gpt-6-luna` answered 1.27.2 with a go.dev citation,
while `deepseek-flash` twice answered 1.26.3 "as of May 2026" — months stale.
Prefer the `openai` provider for version and freshness lookups.

`SupportsContinuity` gates three things: computing a prompt cache key (main.go:273,
api.go:252), dropping `previous_response_id` (api.go:255), and advertising those two
parameters in the tool schema at all (mcp_server.go:110).
`Models` feeds the `models://list` resource, `Prompt` feeds the MCP prompt, and
`DefaultModel` feeds the tool-schema default.

## MCP surface

Server name `gpt-websearch-mcp`, version `0.3.5` (config.go:25, config.go:27), with logging, tools,
resources, prompts, recovery, and input/output schema validation enabled (mcp_server.go:28).

- **Tool `gpt_websearch`** (registered mcp_server.go:45, defined mcp_server.go:88).
  Parameters: `query` (required), `model` (defaults to the provider model),
  `reasoning_effort` (enum from `effortRegistry`), `verbosity` (`low`/`medium`/`high`),
  `web_search` (bool, default true), and — OpenAI only — `previous_response_id` and
  `prompt_cache_key`.
  The output schema is derived from `WebSearchResult` (api.go:301).
  Annotations are set explicitly (mcp_server.go:126-130) — read-only, non-destructive,
  idempotent, open-world — because mcp-go otherwise emits the MCP spec defaults, which
  advertise the tool as destructive and make clients gate it behind approval prompts.
- **Resources**: `server://info` (mcp_server.go:48) and `models://list` (mcp_server.go:59).
- **Prompt `web_search`** (mcp_server.go:70): one required argument, `user_question`,
  returning the provider's guidance prompt wrapped around that question (mcp_server.go:292).

## Transport and auth

**stdio** is the default (`-t stdio`) and never authenticates: `RunStdioTransport` calls
`server.ServeStdio` (transport.go:23).

**HTTP** (`-t http`) runs a streamable-HTTP server on `-host:-port`, default
`127.0.0.1:8080` (transport.go:52).
Server timeouts are read 30s, write 15m (matching the `xhigh` budget), idle 120s
(transport.go:16-20).
`-heartbeat` sets the SSE heartbeat interval, default 30s, `0` disables it (main.go:46).
`ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` (default `true`) toggles mcp-go's DNS-rebinding
protection, which is off because production runs behind a reverse proxy (main.go:93,
transport.go:87).

Bearer auth is opt-in via `-auth-enabled`, HTTP-only, and enforced at the HTTP layer before
any MCP handshake (transport.go:104-112).
The guard validates one HS256 JWT parse — `exp` required, 60s clock leeway, non-empty
username (auth.go:86) — against two token profiles (auth.go:117):

- the **static pair**: issuer `answer-mcp`, audience `answer-mcp-user`, no scope required;
  minted by `mcp -generate-token` (mcp_cli.go:55). Auth is fully self-contained — the
  former GeminiMCP compatibility (shared `GEMINI_AUTH_SECRET_KEY`, `gemini-mcp` pair,
  `user_id`/`role` claims) was removed on 2026-10-21;
- the **OAuth pair**: issuer `ANSWER_MCP_ISSUER`, audience `ANSWER_MCP_RESOURCE`, scope
  `answer` required; minted by the vendored OAuth 2.1 authorization server, mounted beside
  the MCP endpoint when `ANSWER_MCP_OAUTH=true` (transport.go:116) with RFC 9728/8414
  discovery, RFC 7591 dynamic registration plus CIMD, authorization-code flow with PKCE
  S256, a browser login against a PBKDF2-SHA256 hash, and rotating refresh tokens.
  Its state is in-memory, so a restart eventually requires a fresh login.

Failures return 401 with `WWW-Authenticate: Bearer realm="answer-mcp"` and an error code;
with OAuth enabled the challenge carries `resource_metadata` and `scope` (auth.go:176-182) —
the RFC 9728 trigger that starts claude.ai's connector login.
A valid token with the wrong scope gets 403 `insufficient_scope` (auth.go:149-154).
Request bodies are capped at 4 MiB (auth.go:20) and OPTIONS preflight passes unauthenticated
(auth.go:134-137).
On success the username lands in the request context under `actorKey{}` (auth.go:155), and
`actorFrom` (auth.go:191) reads it back for the per-user cache key (api.go:216) and the
handler log line (mcp_server.go:142); mcp-go derives the tool context from the request
context, so the identity reaches tool handlers.
`/health` and `/capabilities` serve unauthenticated operator metadata
(transport.go:113-114).

## Configuration

`.env` is loaded into the process environment before `main` by the `godotenv` autoload
import (main.go:13).

### Environment variables

| Variable | Read for | Mode | Default |
| --- | --- | --- | --- |
| `PROVIDER` | `openai` or `deepseek` (config.go:158) | both | `openai` |
| `OPENAI_API_KEY` / `DEEPSEEK_API_KEY` | the active provider's key (config.go:186) | both | required |
| `QUESTION` | default for `-q` (config.go:167) | CLI | — |
| `MODEL` | default for `-model` (config.go:168) | CLI | provider model |
| `EFFORT` | default for `-effort` (config.go:169) | CLI | `medium` |
| `SHOW_ALL` | default for `-show-all` (config.go:172) | CLI | `false` |
| `TIMEOUT` | default for `-timeout` (config.go:179) | CLI | effort-derived |
| `PROMPT_CACHE_KEY` | default for `-cache-key` (main.go:201) | CLI | — |
| `ANSWER_AUTH_SECRET_KEY` | HTTP JWT signing and validation secret (main.go:111) | MCP | required with `-auth-enabled`; ≥ 32 bytes with OAuth |
| `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` | DNS-rebinding protection (main.go:93) | MCP | `true` (protection off) |
| `ANSWER_MCP_OAUTH` | enable the OAuth 2.1 server; implies auth (config.go:262) | MCP | `false` |
| `ANSWER_MCP_ISSUER` | public OAuth issuer URL (config.go:274) | MCP | — |
| `ANSWER_MCP_RESOURCE` | public protected-resource URL (config.go:275) | MCP | — |
| `ANSWER_MCP_LOGIN_USER` | browser-login username (config.go:276) | MCP | — |
| `ANSWER_MCP_LOGIN_PASSWORD_HASH` | PBKDF2 hash from `mcp -hash-password` (config.go:277) | MCP | — |
| `ANSWER_MCP_ACCESS_TTL` | OAuth access-token lifetime (config.go:278) | MCP | `1h` |
| `ANSWER_MCP_REFRESH_TTL` | OAuth refresh-token lifetime (config.go:281) | MCP | `720h` |

MCP mode ignores `MODEL`, `EFFORT`, `TIMEOUT`, `QUESTION`, `SHOW_ALL`, and
`PROMPT_CACHE_KEY` — they are read only by the CLI path.
Set them per call through the tool parameters instead.

### CLI flags

`-q` / `-question` (or a positional argument) supply the question; `-provider`, `-base`,
`-model`, `-effort`, `-verbosity`, `-web-search`, `-timeout`, `-show-all`, and `-cache-key`
are defined in `parseCLIArgs` (main.go:179).

### MCP flags

`-t` / `-transport`, `-port`, `-host`, `-base`, `-provider`, `-verbose`, `-auth-enabled`,
and `-heartbeat` (main.go:36), plus the one-shot credential commands `-generate-token` /
`-token-username` / `-token-ttl` and `-hash-password` (main.go:48-51), which print and exit
before any provider loading.

### Effort and timeout

| Effort | Timeout | Notes |
| --- | --- | --- |
| `none` | 90s | no internal reasoning |
| `low` | 3m | |
| `medium` | 5m | default |
| `high` | 10m | |
| `xhigh` | 15m | matches the HTTP server write timeout |

An unrecognised effort deliberately resolves to the `low` timeout (config.go:199-204) but to
the `medium` label (config.go:210-215); the asymmetry is documented at config.go:194-198.

### Precedence

Flags beat environment variables, which beat `.env`, which beats defaults (main.go:153-163).
`-provider` is authoritative in both modes.

`.env` sits at the bottom because the autoload import calls `godotenv.Load`, not `Overload`:
`Load` sets a key only when the process environment does not already carry it
(godotenv v1.5.1, `loadFile`, the `!currentEnv[key]` guard).
So a variable set by the launcher — a systemd `Environment=` line, a shell `export`,
an MCP client's `env` block — always wins over the same key in `.env`,
and editing `.env` changes nothing for a process whose unit already sets that key.
That is the trap behind a provider switch that "does not take":
on 2026-10-13 the production unit carried `PROVIDER=deepseek` in its `Environment=`,
so a restart after changing `.env` faithfully reloaded DeepSeek.
Change the unit, `daemon-reload`, and confirm with the binary's own first log line,
`"server starting" … "provider":"…"`, before trusting the switch.

## Wire types and storage

There is no database.
The request and response shapes live in the wire structs `requestBody` and `apiResponse`
(config.go), and the MCP-facing result in `WebSearchResult` (api.go:301).
They are serialised with `encoding/json/v2` and `encoding/json/jsontext` (main.go:5,
api.go:6, mcp_server.go:5), whose v2 semantics apply: case-sensitive name matching,
duplicate object names rejected, and nil slices marshalling as `[]`.
A non-empty `text.verbosity` is mandatory — DeepSeek rejects an empty string with a 400.

## Dependencies

| Module | Version | Used for |
| --- | --- | --- |
| `github.com/mark3labs/mcp-go` | v1.3.0 | The MCP server (`server`, main.go:14) and MCP types (`mcp`, api.go:15). |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | HS256 bearer-token signing and validation (auth.go). |
| `github.com/joho/godotenv` | v1.5.1 | Blank autoload import that reads `.env` (main.go:13). |
| `golang.org/x/term` | v0.47.0 | Silent password read for `mcp -hash-password` (mcp_cli.go:21). |

`google/jsonschema-go` (pulled in by `mcp.WithOutputSchema[WebSearchResult]`,
mcp_server.go:127), `google/uuid`, `santhosh-tekuri/jsonschema/v6`, `spf13/cast`,
`yosida95/uritemplate/v3`, and `golang.org/x/text` are indirect.

## Known constraints

Re-checked against mcp-go v1.3.0 on 2026-10-21; the list descends from the
v0.37→v0.52 migration report (retired in `docs/retired/`).

- The server exposes no task tools. The v0.x `AddTaskTool` behavior (async-only,
  `-32601` for non-task clients, schema validation skipped for task-augmented calls,
  goroutine context tied to the HTTP request lifecycle) changed in v1.x — the `-32601`
  path is gone from the server package — so re-check all of it before ever exposing tasks.
  Claude Code and Claude Desktop do not speak `tasks/*` anyway.
- `server.WithLogger` is a `ServerOption` taking `*slog.Logger` (server/logger.go:47);
  the streamable transport takes its own `WithStreamableHTTPLogger`
  (server/streamable_http.go:180), which is what this project uses (transport.go:56).
- The v0.x `util` package and its `Logger` adapter are gone in v1.x.
- v1.x speaks the stateless 2026-07-28 protocol core, but the initialize handshake
  still negotiates legacy versions up to 2025-11-25 (mcp/version.go), so existing
  clients are unaffected.

## Naming gotcha

The module path is `github.com/chew-z/web_search` but the local binary is `bin/answer`, so
`go install github.com/chew-z/web_search@latest` produces a `web_search` binary.
Build through the Taskfile (`task build`) to get `answer`.
