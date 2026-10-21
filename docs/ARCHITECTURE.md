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

Everything is `package main` at the repository root; the split below is by concern, not by package.

| File | Reach | What it owns |
| --- | --- | --- |
| `main.go` | CLI + MCP entry | `main` routes on `os.Args[1] == "mcp"` (main.go:19); `runCLI`, CLI flags; `runMCPMode`, `runTransport`, `requireAuthSecret`. |
| `api.go` | shared | `CallAPI` (api.go:56), the one outbound HTTP call; `ExtractAnswer` (api.go:166); `HandleWebSearch` (api.go:223). |
| `provider.go` | shared | The `provider` struct (provider.go:22) and `providerRegistry` (provider.go:33); lookups at provider.go:66, provider.go:80. |
| `config.go` | shared | Constants and wire structs; `loadEnvConfig` (config.go:125), `validateEffort` (config.go:180), `parseMCPConfig` (config.go:216). |
| `models.go` | shared | `effortRegistry` (models.go:20), `verbosityLevels` (models.go:29), `modelRegistry` (models.go:83), plus the schema helpers. |
| `prompts.go` | MCP only | The two guidance prompts: `webSearchPrompt` (prompts.go:4) and `deepseekWebSearchPrompt` (prompts.go:124). |
| `mcp_server.go` | MCP only | `NewMCPServer` (mcp_server.go:26), the tool/resource/prompt registrations and handlers, `logToClient`. |
| `transport.go` | MCP only | `RunStdioTransport` (transport.go:19), `RunHTTPTransport` (transport.go:40), HTTP server timeouts. |
| `auth.go` | MCP HTTP only | `validateJWT` (auth.go:51), `newAuthHTTPMiddleware` (auth.go:78), `getUserInfo` (auth.go:132). |
| `logging.go` | both | A `slog` JSON logger on stderr, initialised by `initLogger` (logging.go:19), plus the `Debug`/`Info`/`Warn`/`Error` wrappers. |
| `errors.go` | both | `ErrNoAPIKey`, `ErrInvalidProvider`, `APIError`, and the CLI-only `exitError` / `fail`. |

## Data flow

### CLI

1. `runCLI` (main.go:238) does an advisory `loadEnvConfig("")` (main.go:243) purely to seed
   flag defaults, ignoring its error.
2. `parseCLIArgs` (main.go:169) defines the flags, calls `flag.Parse`, and resolves the
   question from `-q`, `-question`, or the positional argument (main.go:219).
3. Effort and verbosity are normalised to `medium` when unrecognised
   (main.go:199, config.go:180, config.go:188).
4. Timeout precedence: the `-timeout` flag, else `TIMEOUT`, else
   `getTimeoutForEffort(effort)` (main.go:201).
5. The authoritative `loadEnvConfig(args.provider)` (main.go:248) validates the provider and
   requires its API key; failure exits with code 2.
6. `resolveEndpointAndModel` (main.go:143) applies provider defaults when `-base` / `-model`
   were not supplied.
7. The prompt cache key is resolved only when the provider supports continuity (main.go:263).
8. `CallAPI` (api.go:56) applies `context.WithTimeout` (api.go:85), attaches the web-search
   tool when enabled (api.go:74), and sets `Authorization: Bearer` (api.go:93).
9. Output: `-show-all` prints the raw response as indented JSON (main.go:283); otherwise
   `ExtractAnswer` (api.go:166) strips the narration and prints the answer.
   An empty answer exits with code 3.

### MCP

1. `runMCPMode` (main.go:35) parses flags from `os.Args[2:]`; `-transport` overrides `-t`
   (main.go:64).
2. Provider and API key come from `loadEnvConfig` (main.go:72); the JWT secret from
   `requireAuthSecret` (main.go:79).
3. `parseMCPConfig` (config.go:216) fills the remaining defaults, then `NewMCPServer`
   (main.go:104) builds the server and `runTransport` (main.go:113) starts it.

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

`SupportsContinuity` gates three things: computing a prompt cache key (main.go:263,
api.go:252), dropping `previous_response_id` (api.go:255), and advertising those two
parameters in the tool schema at all (mcp_server.go:110).
`Models` feeds the `models://list` resource, `Prompt` feeds the MCP prompt, and
`DefaultModel` feeds the tool-schema default.

## MCP surface

Server name `gpt-websearch-mcp`, version `0.3.5` (config.go:22, config.go:24), with logging, tools,
resources, prompts, recovery, and input/output schema validation enabled (mcp_server.go:28).

- **Tool `gpt_websearch`** (registered mcp_server.go:45, defined mcp_server.go:88).
  Parameters: `query` (required), `model` (defaults to the provider model),
  `reasoning_effort` (enum from `effortRegistry`), `verbosity` (`low`/`medium`/`high`),
  `web_search` (bool, default true), and — OpenAI only — `previous_response_id` and
  `prompt_cache_key`.
  The output schema is derived from `WebSearchResult` (api.go:301).
- **Resources**: `server://info` (mcp_server.go:48) and `models://list` (mcp_server.go:59).
- **Prompt `web_search`** (mcp_server.go:70): one required argument, `user_question`,
  returning the provider's guidance prompt wrapped around that question (mcp_server.go:292).

## Transport and auth

**stdio** is the default (`-t stdio`) and never authenticates: `RunStdioTransport` calls
`server.ServeStdio` (transport.go:19).

**HTTP** (`-t http`) runs a streamable-HTTP server on `-host:-port`, default
`127.0.0.1:8080` (transport.go:40).
Server timeouts are read 30s, write 15m (matching the `xhigh` budget), idle 120s
(transport.go:12).
`-heartbeat` sets the SSE heartbeat interval, default 30s, `0` disables it (main.go:47).
`ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` (default `true`) toggles mcp-go's DNS-rebinding
protection, which is off because production runs behind a reverse proxy (main.go:85,
transport.go:52).

JWT auth is opt-in via `-auth-enabled` and is HTTP-only (transport.go:62).
`GEMINI_AUTH_SECRET_KEY` is then mandatory and the process refuses to start without it
(main.go:133).
`validateJWT` pins HS256, expects issuer `gemini-mcp` and audience `gemini-mcp-user`, and
allows 60s of clock skew (auth.go:51); tokens are interchangeable with GeminiMCP.
Failures return 401 with a machine-readable detail code: `missing_token`, `invalid_token`,
`expired_token`, `token_not_valid_yet`, `malformed_token`, or `invalid_signature`
(auth.go:112).
On success the claims land in the request context (auth.go:121) and feed both the per-user
cache key and handler logging.

## Configuration

`.env` is loaded into the process environment before `main` by the `godotenv` autoload
import (main.go:13).

### Environment variables

| Variable | Read for | Mode | Default |
| --- | --- | --- | --- |
| `PROVIDER` | `openai` or `deepseek` (config.go:128) | both | `openai` |
| `OPENAI_API_KEY` / `DEEPSEEK_API_KEY` | the active provider's key (config.go:156) | both | required |
| `QUESTION` | default for `-q` (config.go:137) | CLI | — |
| `MODEL` | default for `-model` (config.go:138) | CLI | provider model |
| `EFFORT` | default for `-effort` (config.go:139) | CLI | `medium` |
| `SHOW_ALL` | default for `-show-all` (config.go:142) | CLI | `false` |
| `TIMEOUT` | default for `-timeout` (config.go:149) | CLI | effort-derived |
| `PROMPT_CACHE_KEY` | default for `-cache-key` (main.go:190) | CLI | — |
| `GEMINI_AUTH_SECRET_KEY` | HTTP JWT secret (main.go:134) | MCP | required with `-auth-enabled` |
| `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` | DNS-rebinding protection (main.go:86) | MCP | `true` (protection off) |

MCP mode ignores `MODEL`, `EFFORT`, `TIMEOUT`, `QUESTION`, `SHOW_ALL`, and
`PROMPT_CACHE_KEY` — they are read only by the CLI path.
Set them per call through the tool parameters instead.

### CLI flags

`-q` / `-question` (or a positional argument) supply the question; `-provider`, `-base`,
`-model`, `-effort`, `-verbosity`, `-web-search`, `-timeout`, `-show-all`, and `-cache-key`
are defined in `parseCLIArgs` (main.go:169).

### MCP flags

`-t` / `-transport`, `-port`, `-host`, `-base`, `-provider`, `-verbose`, `-auth-enabled`,
and `-heartbeat` (main.go:36).

### Effort and timeout

| Effort | Timeout | Notes |
| --- | --- | --- |
| `none` | 90s | no internal reasoning |
| `low` | 3m | |
| `medium` | 5m | default |
| `high` | 10m | |
| `xhigh` | 15m | matches the HTTP server write timeout |

An unrecognised effort deliberately resolves to the `low` timeout (config.go:173) but to
the `medium` label (config.go:184); the asymmetry is documented at config.go:165.

### Precedence

Flags beat environment variables, which beat `.env`, which beats defaults (main.go:143).
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
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | HS256 validation of the HTTP bearer token (auth.go:11). |
| `github.com/joho/godotenv` | v1.5.1 | Blank autoload import that reads `.env` (main.go:13). |

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
