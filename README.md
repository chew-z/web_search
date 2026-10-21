# Answer - Deep-Reasoning Web Search CLI & MCP Server

A Go application that provides intelligent web search capabilities using deep-reasoning models via the Responses API — OpenAI GPT or DeepSeek.
Works as both a CLI tool and an MCP (Model Context Protocol) server with configurable reasoning effort.

## Features

- 🔍 **Intelligent Web Search**: OpenAI GPT (gpt-6-luna) or DeepSeek (deepseek-flash → DeepSeek-V4.1-Flash) with web search capabilities
- 🔀 **Multi-Provider**: Switch backends with `PROVIDER` env or `-provider` flag
- 🎯 **Cost-Effective**: Configurable reasoning effort based on query complexity for optimal cost/performance
- 🚀 **Dual Mode**: CLI tool and MCP server with stdio/HTTP transports
- ⚙️ **Smart Configuration**: Effort-based timeouts (90s/3/5/10/15 minutes) and environment-driven setup
- 🧠 **Enhanced MCP Prompts**: Provider-specific prompt templates guide optimal tool usage
- 🔄 **Conversation Continuity**: Response IDs enable follow-up questions with maintained context (OpenAI provider)
- 🔐 **Secure**: Environment-based API key management

## Installation

### Prerequisites

- Go 1.27.0 or later
- An OpenAI API key (provider `openai`) or a DeepSeek API key (provider `deepseek`)

### JSON v2

This project uses `encoding/json/v2` + `encoding/json/jsontext`, which are generally
available as of Go 1.27 — no build experiment, no `GOEXPERIMENT` flag, and nothing
extra to configure in your editor.

Two v2 defaults are worth knowing when reading the wire structs: nil slices and maps
marshal as `[]` and `{}` rather than `null`, and object names match case-sensitively.

The recommended workflow goes through [go-task](https://taskfile.dev), which defines
every command in one place:

```bash
go mod download
task build
task test
task lint
task fmt
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/chew-z/web_search
cd web_search

# Install dependencies
go mod download

# Build the binary
task build
# …or: go build -o bin/answer .

# Or install globally (the installed binary is named web_search)
go install github.com/chew-z/web_search@latest
```

## Configuration

### Providers

Answer can talk to two Responses API backends, selected with the `PROVIDER`
environment variable or the `-provider` flag.
Precedence is flag, then environment, then `.env`, then the default:
`.env` only fills in variables the process does not already have,
so a `PROVIDER` set by systemd, a shell `export`, or an MCP client's `env` block
beats the one in `.env`.

| Provider   | API key env        | Default model    | Notes                                                                     |
| ---------- | ------------------ | ---------------- | ------------------------------------------------------------------------- |
| `openai`   | `OPENAI_API_KEY`   | `gpt-6-luna`     | Default. Supports conversation continuity (`previous_response_id`).        |
<!-- markdownlint-disable-next-line MD013 -->
| `deepseek` | `DEEPSEEK_API_KEY` | `deepseek-flash` | Stateless: `previous_response_id`/`prompt_cache_key` are not sent. Web search runs server-side. `deepseek-flash` is DeepSeek's rolling alias — it resolves to DeepSeek-V4.1-Flash today. |

The base URL, web-search tool type, model defaults, and the MCP guidance
prompt all adapt to the selected provider; `-base` still overrides the
endpoint explicitly.

### Environment Variables

Create a `.env` file in the project root:

```env
OPENAI_API_KEY=your-api-key-here
PROVIDER=openai          # Optional: openai (default) or deepseek
DEEPSEEK_API_KEY=        # Required when PROVIDER=deepseek
MODEL=gpt-6-luna         # Optional: provider default (gpt-6-luna / deepseek-flash)
EFFORT=low               # Optional: reasoning effort (low/medium/high, default: medium)
SHOW_ALL=false           # Optional: show raw JSON
QUESTION=                # Optional: default question
```

**Available Model** (OpenAI provider):

- `gpt-6-luna`: Factual lookups, research, comparisons, complex analysis, coding questions, and reasoning tasks

DeepSeek provider currently offers a single model, `deepseek-flash` — the
version-less alias that DeepSeek repoints at each new flash release
(DeepSeek-V4.1-Flash at the time of writing); tune `EFFORT` instead of the model.

**Effort-Based Timeouts**: `none` = 90 seconds, `low` = 3 minutes, `medium` = 5 minutes, `high` = 10 minutes, `xhigh` = 15 minutes.
Tip: Use `low` for quicker answers when speed matters.

## Usage

### CLI Mode

Use Answer as a command-line tool for direct web searches:

```bash
# Simple query with positional argument
./bin/answer "Who won the 2024 Super Bowl?"

# Using the -q flag
./bin/answer -q "Latest AI developments"

# Error if no query provided
./bin/answer  # Error: please provide a question to ask

# With custom model and effort
./bin/answer -q "Explain quantum computing" -model gpt-6-luna -effort high

# Show raw JSON response
./bin/answer -q "Test query" -show-all

# Custom timeout
./bin/answer -q "Complex analysis" -timeout 120s
```

### MCP Server Mode

Run Answer as an MCP server for integration with AI assistants:

#### STDIO Transport (for Claude Desktop)

```bash
# Start MCP server in stdio mode (default)
./bin/answer mcp

# Or explicitly specify stdio
./bin/answer mcp -t stdio
./bin/answer mcp --transport stdio
```

**Claude Desktop Configuration:**

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
    "mcpServers": {
        "gpt-websearch": {
            "command": "/path/to/Answer/bin/answer",
            "args": ["mcp", "-t", "stdio"],
            "env": {
                "OPENAI_API_KEY": "your-api-key"
            }
        }
    }
}
```

To use DeepSeek instead, add `"PROVIDER": "deepseek"` and set `DEEPSEEK_API_KEY` in the `env` block.

#### HTTP Transport (for Web Integration)

```bash
# Start HTTP server on default port 8080
./bin/answer mcp -t http

# Custom port
./bin/answer mcp -t http -port 3000

# With verbose logging
./bin/answer mcp -t http -verbose
```

**Endpoints:**

- `POST` on any path (conventionally `/mcp`) — the MCP streamable HTTP endpoint (JSON-RPC 2.0);
  the handler routes by method, so prefix-forwarding proxies work unchanged.
- `GET /health` — health check (unauthenticated).
- `GET /capabilities` — server, tool, and authentication metadata (unauthenticated).
- With `ANSWER_MCP_OAUTH=true`, additionally: `GET /authorize`, `GET`/`POST /login`,
  `POST /token`, `POST /register`, and the RFC 9728 / RFC 8414 `/.well-known/...`
  discovery documents.

**Authentication (HTTP transport):** opt-in with `-auth-enabled`; requests then need
`Authorization: Bearer <JWT>` (HS256, signed with `GEMINI_AUTH_SECRET_KEY`).
Two token profiles are accepted: static tokens minted with
`./bin/answer mcp -generate-token` (interchangeable with GeminiMCP), and OAuth 2.1 access
tokens from the browser login that claude.ai connectors start automatically from the 401
challenge. Deployment, connector setup, and verification runbook:
[docs/OPERATIONS.md](docs/OPERATIONS.md).

## MCP Server Features

### Tool: `gpt_websearch`

Performs intelligent web searches with configurable reasoning effort and (OpenAI provider) conversation continuity:

| Parameter              | Type    | Required | Default              | Description                                                                       |
| ---------------------- | ------- | -------- | -------------------- | --------------------------------------------------------------------------------- |
| `query`                | string  | Yes      | -                    | The search query or question                                                      |
| `model`                | string  | No       | provider default     | `gpt-6-luna` (openai) or `deepseek-flash` (deepseek)                               |
<!-- markdownlint-disable-next-line MD013 -->
| `reasoning_effort`     | string  | No       | `medium`             | Effort level:<br>`none` = 90 seconds<br>`low` = 3 minutes<br>`medium` = 5 minutes<br>`high` = 10 minutes<br>`xhigh` = 15 minutes |
| `verbosity`            | string  | No       | `medium`             | Response verbosity: `low`, `medium`, or `high`                                    |
| `previous_response_id` | string  | No       | -                  | Previous response ID for conversation continuity (openai only; param absent for deepseek) |
| `prompt_cache_key`     | string  | No       | server default       | OpenAI prompt cache shard (openai only; param absent for deepseek)                |
| `web_search`           | boolean | No       | `true`               | Use web search (default: true)                                                    |

### Prompt: `web_search`

Enhanced prompt template that guides Claude Desktop to:

- Analyze user questions in conversation context
- Choose cost-effective search parameters based on complexity
- Choose appropriate reasoning effort levels
- Use single, sequential, or parallel search strategies
- Remember and use response IDs for conversation continuity

### Example Response

```json
{
    "success": true,
    "answer": "The complete answer to your query...",
    "query": "original query",
    "model": "gpt-6-luna",
    "effort": "low",
    "timeout_used": "3m0s",
    "id": "resp_68a24ac476a081a09c4c914ee8827c2b0f42d84e6960dd2d",
    "requested_model": "gpt-6-luna",
    "requested_effort": "low",
    "web_search_used": true
}
```

### Conversation Continuity (OpenAI provider)

The MCP server supports conversation continuity through response IDs when the
`openai` provider is active (DeepSeek's API is stateless). Each search response
includes an `id` field that can be used in follow-up queries to maintain context:

**Initial Query:**

```json
{
    "name": "gpt_websearch",
    "arguments": {
        "query": "Tell me about Luxembourg City",
        "model": "gpt-6-luna",
        "reasoning_effort": "medium"
    }
}
```

**Response includes ID:**

```json
{
    "id": "resp_68a24ac476a081a09c4c914ee8827c2b0f42d84e6960dd2d",
    "answer": "Luxembourg City is the capital..."
    // ... other fields
}
```

**Follow-up Query with Context:**

```json
{
    "name": "gpt_websearch",
    "arguments": {
        "query": "What are the main tourist attractions there?",
        "previous_response_id": "resp_68a24ac476a081a09c4c914ee8827c2b0f42d84e6960dd2d",
        "reasoning_effort": "low"
    }
}
```

The AI assistant will automatically remember context from the previous search and provide more relevant answers for follow-up questions.

## Command-Line Reference

### CLI Mode

```text
answer [options] [question]

Options:
  -q, -question    Question to ask (required, can also use positional argument)
  -provider       API provider: openai (default) or deepseek (env PROVIDER)
  -model          Model (default: gpt-6-luna for openai, deepseek-flash for deepseek; env MODEL)
  -effort         Reasoning effort: none (90s), low (3min), medium (5min), high (10min), xhigh (15min timeout) (default: medium)
  -verbosity      Response verbosity: low, medium, high (default: medium)
  -timeout        Request timeout (overrides effort-based defaults)
  -show-all       Show raw JSON response
  -base           API endpoint URL (default: provider-specific)
  -web-search     Use web search (default: true)
  -cache-key      OpenAI prompt_cache_key (openai provider only; env PROMPT_CACHE_KEY)
```

### MCP Server Mode

```text
answer mcp [options]

Options:
  -t, --transport  Transport type: stdio or http (default: stdio)
  -provider       API provider: openai (default) or deepseek (env PROVIDER)
  -port           HTTP server port (default: 8080)
  -host           HTTP server host (default: 127.0.0.1)
  -base           API endpoint URL (default: provider-specific)
  -verbose        Enable verbose logging
```

## Examples

### CLI Examples

```bash
# Quick question
./bin/answer "What's the weather in San Francisco?"

# Research query with high effort
./bin/answer -q "Latest breakthroughs in quantum computing" -effort high

# Using gpt-6-luna for research tasks
./bin/answer -q "Explain the theory of relativity" -model gpt-6-luna

# DeepSeek backend
./bin/answer -provider deepseek -q "Latest EU regulations on AI"

# Debug mode with raw output
./bin/answer -q "Test query" -show-all
```

## Development

### Project Structure

```text
Answer/
├── main.go              # Main entry point with CLI and MCP modes
├── api.go               # Responses API client and answer extraction
├── provider.go          # Provider registry (openai / deepseek)
├── mcp_server.go        # MCP tools, resources, prompts
├── prompts.go           # Per-provider MCP guidance prompts
├── models.go            # Effort and model registries
├── config.go            # Configuration structures and helpers
├── transport.go         # stdio/HTTP transports, /health, /capabilities, OAuth mount
├── auth.go              # JWT guard: static + OAuth token pairs
├── mcp_cli.go           # Credential CLIs: -generate-token, -hash-password
├── logging.go           # Structured logging
├── errors.go            # Error definitions
├── internal/
│   └── oauth/           # Vendored OAuth 2.1 authorization server
├── deploy/
│   └── nginx-answer.conf # karma nginx blocks for OAuth + MCP
├── go.mod               # Go module definition
├── .env                 # Environment variables (not committed)
├── bin/
│   └── answer           # Compiled binary
├── AGENTS.md            # Development guidelines
└── README.md            # This file
```

### Building

```bash
task build   # preferred; or: go build -o bin/answer .
```

### Testing

```bash
task test    # preferred; or: go test ./...
```

### Formatting

```bash
go fmt ./...
```
