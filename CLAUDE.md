# CLAUDE.md

LLM guidance for working with Answer - dual-mode Go application (CLI + MCP server) for web search via OpenAI and DeepSeek Responses APIs.

## Build Commands

**Requires `GOEXPERIMENT=jsonv2`** (Go 1.26): the code imports `encoding/json/v2`
and `encoding/json/jsontext`, gated behind this experiment. Without it, builds
fail with `build constraints exclude all Go files in .../encoding/json/v2`. The
flag is centralized in `Taskfile.yml` (`env:` block) and exported by
`run_test.sh` / `run_lint.sh`.

-   `task build` - build (preferred; sets the flag) → `bin/answer`
-   `task test` / `task lint` / `task fmt` - test, lint, format with the flag
-   `task run -- "query"` - run the CLI
-   Ad-hoc: `GOEXPERIMENT=jsonv2 go build -o bin/answer .`
-   `./run_format.sh` - format code (gofmt only; no flag needed)
-   `./run_lint.sh` / `./run_test.sh` - lint / test (export the flag themselves)
-   gopls can't read the Taskfile; for editor diagnostics only, optionally
    `go env -w GOEXPERIMENT=jsonv2` (not needed for repo build/test/lint).

Module path: `github.com/chew-z/web_search` (was `Answer`; `go install` binary is `web_search`).

## Environment

Provider selection: `PROVIDER` (`openai` default, `deepseek`); `-provider` flag
overrides the env var in both CLI and MCP modes.

Required: `OPENAI_API_KEY` (provider `openai`) or `DEEPSEEK_API_KEY` (provider `deepseek`)
Optional: `MODEL`, `EFFORT`, `SHOW_ALL`, `TIMEOUT`, `QUESTION`,
`ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION` (default `true` — disables mcp-go's
DNS rebinding protection; set `false` only for local-only dev without a reverse proxy)
Uses godotenv for `.env` loading

## Architecture

**Dual-mode design**: CLI or MCP server based on args

-   CLI: `./bin/answer "query"`
-   MCP: `./bin/answer mcp [options]`

**Key files**:

-   `main.go` - entry point, mode routing
-   `mcp_server.go` - MCP server implementation (provider-aware tool schema, models resource, prompts)
-   `api.go` - Responses API integration (provider-specific tool type; narration stripping in `ExtractAnswer`)
-   `provider.go` - provider registry: endpoint, key env, tool type, model list, prompt per backend
-   `config.go` - environment config, timeouts
-   `models.go` - effort/model registries
-   `prompts.go` - per-provider MCP guidance prompts
-   `transport.go` - stdio/HTTP transports
-   `logging.go` - structured slog logging, startup banner
-   `errors.go` - error handling

## Historical Notes (commit b6c3478, pre-provider era)

**Enhanced MCP Prompt System**: Replaced basic `web_search` with `intelligent_web_search`

-   **Name**: `intelligent_web_search` (was `web_search`)
-   **Argument**: `user_question` (was `topic`)
-   **System Message**: Comprehensive LLM instructions for cost-effective tool usage

**Model Selection Logic**:

-   `gpt-5-nano`: Simple facts, definitions, summaries
-   `gpt-5-mini`: Research, comparisons, specific topics
-   `gpt-5.1`: Complex analysis, coding, reasoning

**Reasoning Effort**:

-   `low`: 3min timeout, factual queries
-   `medium`: 5min timeout, synthesis tasks
-   `high`: 10min timeout, complex analysis

**Search Strategy**: Single/sequential/parallel approaches based on query complexity

## MCP Implementation Details

**Tool**: `gpt_websearch` - web search with the configured provider:

-   Provider registry in `provider.go` (endpoint, key env, tool type, models, prompt)
-   Model selection per provider (OpenAI: gpt-5.4 family; DeepSeek: deepseek-v4-flash)
-   Reasoning effort: none/low/medium/high/xhigh with timeout mapping (models.go)
-   Continuity args (`previous_response_id`, `prompt_cache_key`) exposed only for OpenAI

**Prompt**: per-provider `intelligent_web_search` guidance in `prompts.go`, served via the MCP prompts resource.

**Transports**:

-   STDIO: Claude Desktop integration
-   HTTP/SSE: Web applications (port 8080)

## API Integration

Both backends speak the OpenAI Responses API shape; differences live in `provider.go`:

|                | `openai`                              | `deepseek`                              |
| -------------- | ------------------------------------- | --------------------------------------- |
| Endpoint       | `https://api.openai.com/v1/responses` | `https://api.deepseek.com/responses`    |
| Tool type      | `web_search_preview`                  | `web_search` (server-side execution)    |
| Models         | gpt-5.4-nano / gpt-5.4-mini / gpt-5.4 | deepseek-v4-flash (v4-pro pending)      |
| Continuity     | `previous_response_id`, `prompt_cache_key` | Stateless — both params dropped   |

-   Effort-based timeouts: 90s/3/5/10/15 minutes (none/low/medium/high/xhigh)
-   `text.verbosity` must never be sent empty (DeepSeek rejects `""` with 400)

## Error Handling

-   CLI: `fail()` function with exit codes (errors.go)
-   MCP: Structured JSON responses
-   API errors: Custom `APIError` type wrapping

## Configuration Priority

1. CLI flags
2. Environment variables
3. Defaults (provider's default model — gpt-5.4-mini / deepseek-v4-flash; medium effort, 5min timeout)

## Testing

-   `integration_test.go` - core API functions
-   `test_timeouts.sh` - timeout behavior
-   Environment-aware skipping for missing API keys

## Development Workflow

1. Set the API key for your provider (`OPENAI_API_KEY`, or `DEEPSEEK_API_KEY` with `PROVIDER=deepseek`)
2. Make changes
3. `./run_format.sh && ./run_lint.sh && ./run_test.sh`
4. `go build -o bin/answer .`
5. Test CLI: `./bin/answer "query"`
6. Test MCP: `./bin/answer mcp -t stdio`

## Dependencies

-   `github.com/mark3labs/mcp-go` v0.56.0 - MCP protocol (v0.56 added DNS
    rebinding protection on streamable HTTP; disabled by default via
    `ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION=true` for reverse-proxy deployments)
-   `github.com/joho/godotenv` v1.5.1 - environment loading
