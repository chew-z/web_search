# mcp-go v0.37 → v0.52 — Improvement Report for the Answer MCP Server

**Date:** 2026-05-05
**Project:** Answer (`github.com/chew-z/web_search`)
**Current dependency:** `github.com/mark3labs/mcp-go v0.52.0` (already bumped in `go.mod`)
**Server shape:** single tool (`gpt_websearch`), HTTP/SSE + stdio transports, JWT/HS256 auth, single-instance deployment, blocking 10-minute upstream calls to OpenAI.

---

## Executive summary

The dependency jump covers 15 minor releases. Most new features are not load-bearing for *this* server. After filtering against our actual constraints — single tool, no real OAuth, single instance, and **no client support yet for the MCP `tasks/*` capability in Claude Code or Claude Desktop** — the recommended scope shrinks to ~6 small, low-risk changes.

The headline feature of the upgrade (async **task support**, v0.44) is **not currently usable**: registering `gpt_websearch` via `AddTaskTool` makes it async-only, and Claude Code/Desktop/mcp-inspector do not implement `tasks/list`, `tasks/get`, `tasks/result`, or `tasks/cancel`. A non-task-aware client calling such a tool gets JSON-RPC error `-32601` ("does not support synchronous execution"). Defer until clients catch up.

---

## Free wins already shipped by being on v0.52

These are bug fixes that landed between v0.37 and v0.52 and now apply automatically:

| Version | Fix |
|---------|-----|
| v0.38, v0.42, v0.47 | stdio transport race conditions for concurrent tool calls |
| v0.44 | drain pending notifications before writing responses (no lost notifications) |
| v0.44 | honor custom session ID generator |
| v0.46 | cancel in-flight request contexts on `notifications/cancelled` — `gpt_websearch`'s OpenAI call now gets clean `context.Canceled` propagation |
| v0.47 | HTTP/2 hang fix; goroutine-leak fixes in client transport on context cancellation |
| v0.48 | session-ID cleanup on GET connection close (memory leak); 202 Accepted for empty pings/sampling |
| v0.51 | stop retrying on session-terminated in `listenForever`; transport-agnostic `Handle` entry point |

No code changes required to benefit.

---

## Tier 1 — Recommended changes (small, safe, real value)

| # | Change | Source release | Why for *this* server |
|---|--------|---------------|------------------------|
| 1 | `server.WithRecovery()` in `NewMCPServer` options | pre-v0.38 (already in API) | Panic in the tool handler (e.g., nil deref while parsing OpenAI response) currently terminates the process. Trivial safety net. |
| 2 | `server.WithInputSchemaValidation()` (SEP-1303) | **v0.50** | We already declare `mcp.Enum("minimal","low","medium","high")` on `reasoning_effort` and `mcp.Required()` on `query`. Without this option the framework does **not** enforce them — bad values reach OpenAI. With it, the client gets a proper `IsError` result with the validation message. Sync tools (our case) get full validation. |
| 3 | `mcp.WithSchemaAdditionalProperties(false)` on the `gpt_websearch` tool definition | **v0.44** | Cleanly rejects unknown fields like typos (`{"query":"X","extar":true}`). Pairs naturally with #2. |
| 4 | `server.WithWebsiteURL("https://github.com/chew-z/web_search")` | **v0.45** | Surfaces the project URL in `serverInfo.websiteURL` — discoverability in client UIs. |
| 5 | `server.WithTitle("GPT Web Search")` | **v0.43 / v0.45** | Human-readable display title separate from the technical `serverName`. Better UX in clients that distinguish them. |
| 6 | `mcpLogAdapter` implementing `util.Logger` + `server.WithLogger(...)` on `StreamableHTTPServer` | **v0.49 area** | Routes mcp-go's internal HTTP transport logs through our existing slog setup — uniform JSON output across our code and library code. |

**Estimated total diff:** ~20 lines across `mcp_server.go`, `transport.go`, and a small adapter in `logging.go`.

**Test additions:** schema-declaration tests that fetch `tools/list` and assert the enum and `additionalProperties:false` constraints are present in the inputSchema.

---

## Tier 2 — Worth considering, slightly larger

| # | Change | Source release | Tradeoff |
|---|--------|---------------|----------|
| 7 | Declare an explicit `outputSchema` for `gpt_websearch` + `server.WithOutputSchemaValidation()` | **v0.51** | We already return `{answer, response_id, model, …}` as `StructuredContent`. With a schema, structured-aware clients can render it natively, and the contract is enforced at runtime. ~30 min of work. |
| 8 | Use `mcp.NewToolResultJSON(...)` for the API-error path | **v0.40** | Cleaner than the current text-wrapped JSON. Cosmetic improvement. |
| 9 | Tool result annotations (`audience`, `priority`, `lastModified`) on returned content | **v0.41, v0.44** | Lets clients prioritize / cache results. Minor UX polish. |

---

## Tier 3 — Defer or skip for this server

| Feature | Source | Why not now |
|---------|--------|-------------|
| Task support (`AddTaskTool`, `WithTaskCapabilities`, `WithMaxConcurrentTasks`, `WithTaskHooks`, `WithTaskSupport`) | v0.44, v0.46, v0.47 | **Blocks Claude Code / Claude Desktop / mcp-inspector** — they do not implement `tasks/*`. Async-only registration breaks all current clients. Revisit when major clients add task-protocol support. |
| OAuth Protected Resource Metadata (RFC 9728), RFC 7591, RFC 8707 | v0.45, v0.48, v0.50, v0.51 | We use static HS256 JWT, not real OAuth. |
| `StatelessGeneratingSessionIdManager`, `SessionIdManagerResolver` | v0.43.1, v0.43 | Multi-instance LB deployments. Single-instance today. |
| Per-session tools / resources / resource templates | v0.42, v0.43 | Multi-tenant patterns. Not our shape. |
| Elicitation (URL mode and dialog) | v0.40, v0.44 | Server-asks-user round trips; client support uneven. |
| Auto-completion handlers | v0.44 | Mostly free-form `query`; little to complete. |
| `WithDisableStreaming` | v0.42 | We rely on streaming for SSE notifications. |
| `WithIcons` | v0.44 | Cosmetic; only useful if we have an icon. |
| TLS for streamable-HTTP | v0.39 | Reverse proxy (nginx) handles TLS today. |
| Iterator-based client methods, `CommandTransport`, `LoggingTransport`, `SchemaCache` | v0.50, v0.51 | Client-side or out-of-scope for a server. |
| Tool Search / deferred tool loading | v0.44 | Designed for servers with many tools; we have one. |
| Roots, Sampling improvements | v0.40, v0.42 | Filesystem roots and server-initiated LLM calls — neither applies. |

---

## Discovered constraints worth preserving in project notes

These were learned the hard way during the failed first attempt and are worth recording:

1. **`AddTaskTool` is strictly async-only.** Sync calls return JSON-RPC `-32601`. `mcp.WithTaskSupport(mcp.TaskSupportOptional)` on the tool definition does not enable a sync fallback when registered via `AddTaskTool`.
2. **`WithInputSchemaValidation` does not fire for task-augmented calls** in v0.52 — only for sync `tools/call`. This matters only if you're using tasks; for our recommended Tier 1 (sync), validation works fully.
3. **Task goroutine context is tied to the HTTP transport lifecycle.** Without a long-lived `GET /` SSE listening connection, the task is cancelled the moment the POST `tools/call` response is sent (observed: 41–262 µs). Tests must establish a parallel GET listener before they POST. Real clients normally do this; today's Claude Code client does not speak the task protocol regardless.
4. **`server.WithLogger` is a `StreamableHTTPOption`, not a `ServerOption`.** Apply it to `NewStreamableHTTPServer`, not to `NewMCPServer`.
5. **`util.Logger` interface (pinned v0.52):** only `Infof(format, v ...any)` and `Errorf(format, v ...any)`. Compile-time interface guard recommended.

---

## Suggested implementation order

1. **Single small commit** landing Tier 1 #1–#5 with their schema-declaration tests (~15 lines of code, ~30 lines of tests).
2. **Separate small commit** landing Tier 1 #6 (logger adapter + `WithLogger` wiring).
3. *(Optional follow-up)* Tier 2 #7 — output schema declaration + validation. Decide based on how downstream consumers want to read the result.

No new dependencies. No breaking changes for clients. No async or transport-shape changes.

---

## Sources

- mcp-go GitHub releases v0.38.0–v0.52.0 (fetched via `gh api repos/mark3labs/mcp-go/releases`)
- godoc for `github.com/mark3labs/mcp-go/server` and `github.com/mark3labs/mcp-go/util` (pinned v0.52.0)
- MCP specification 2025-06-18 (tasks utilities) — confirmed clients must opt in to `tasks.requests.tools.call` capability
- Empirical testing in this repo's prior implementation attempt (now reverted)
