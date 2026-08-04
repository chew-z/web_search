# Answer - Deep-Reasoning Web Search CLI & MCP Server

A Go application that provides intelligent web search capabilities using deep-reasoning models via the Responses API — OpenAI GPT or DeepSeek. Works as both a CLI tool and an MCP (Model Context Protocol) server with cost-effective model selection.

## Features

-   🔍 **Intelligent Web Search**: OpenAI GPT (gpt-5.4 family) or DeepSeek (deepseek-v4-flash) with web search capabilities
-   🔀 **Multi-Provider**: Switch backends with `PROVIDER` env or `-provider` flag
-   🎯 **Cost-Effective**: Automatic model selection based on query complexity for optimal cost/performance
-   🚀 **Dual Mode**: CLI tool and MCP server with stdio/HTTP transports
-   ⚙️ **Smart Configuration**: Effort-based timeouts (3/5/10 minutes) and environment-driven setup
-   🧠 **Enhanced MCP Prompts**: Provider-specific prompt templates guide optimal tool usage
-   🔄 **Conversation Continuity**: Response IDs enable follow-up questions with maintained context (OpenAI provider)
-   🔐 **Secure**: Environment-based API key management

## Installation

### Prerequisites

-   Go 1.26.0 or later
-   An OpenAI API key (provider `openai`) or a DeepSeek API key (provider `deepseek`)

### JSON v2 / Go toolchain

This project uses `encoding/json/v2` + `encoding/json/jsontext`, which on Go 1.26
are gated behind a build experiment. **Every `go` command must run with
`GOEXPERIMENT=jsonv2`** or the build fails with
`build constraints exclude all Go files in .../encoding/json/v2`.

The flag is centralized in `Taskfile.yml` (an `env:` block), so the recommended
workflow is to use [go-task](https://taskfile.dev):

```bash
task build   # go build with GOEXPERIMENT=jsonv2
task test
task lint
task fmt
```

The `run_*.sh` scripts also export the flag. For ad-hoc commands, prefix them:
`GOEXPERIMENT=jsonv2 go test ./...`.

> **Editor / gopls note:** gopls cannot read the Taskfile env, so it will report
> json/v2 imports as build errors. Optional per-developer fix (persistent,
> machine-wide): `go env -w GOEXPERIMENT=jsonv2`. This is **not** required for
> repo build/test/lint (those go through `task`); it only affects in-editor
> diagnostics.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/chew-z/web_search
cd web_search

# Install dependencies
go mod download

# Build the binary (flag required — see "JSON v2 / Go toolchain" above)
task build
# …or: GOEXPERIMENT=jsonv2 go build -o bin/answer .

# Or install globally (external users must also enable the experiment):
GOEXPERIMENT=jsonv2 go install github.com/chew-z/web_search@latest
```

## Configuration

### Providers

Answer can talk to two Responses API backends, selected with the `PROVIDER`
environment variable or the `-provider` flag (flag wins):

| Provider   | API key env        | Default model    | Notes                                                                     |
| ---------- | ------------------ | ---------------- | ------------------------------------------------------------------------- |
| `openai`   | `OPENAI_API_KEY`   | `gpt-5.4-mini`   | Default. Supports conversation continuity (`previous_response_id`).        |
| `deepseek` | `DEEPSEEK_API_KEY` | `deepseek-v4-flash` | Stateless: `previous_response_id`/`prompt_cache_key` are not sent. Web search runs server-side. |

The base URL, web-search tool type, model defaults, and the MCP guidance
prompt all adapt to the selected provider; `-base` still overrides the
endpoint explicitly.

### Environment Variables

Create a `.env` file in the project root:

```env
OPENAI_API_KEY=your-api-key-here
PROVIDER=openai          # Optional: openai (default) or deepseek
DEEPSEEK_API_KEY=        # Required when PROVIDER=deepseek
MODEL=gpt-5.4-mini       # Optional: provider default (gpt-5.4-mini / deepseek-v4-flash)
EFFORT=low               # Optional: reasoning effort (low/medium/high, default: medium)
SHOW_ALL=false           # Optional: show raw JSON
QUESTION=                # Optional: default question
```

**Model Selection Guidelines** (OpenAI provider):

-   `gpt-5.4-nano`: Simple facts, definitions, quick lookups
-   `gpt-5.4-mini`: Research tasks, comparisons, specific topics
-   `gpt-5.4`: Complex analysis, coding questions, reasoning tasks

DeepSeek provider currently offers a single model, `deepseek-v4-flash`
(`deepseek-v4-pro` pending); tune `EFFORT` instead of the model.

**Effort-Based Timeouts**: `low` = 3 minutes, `medium` = 5 minutes, `high` = 10 minutes.
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
./bin/answer -q "Explain quantum computing" -model gpt-5.4-mini -effort high

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

-   `GET /` - API documentation
-   `GET /health` - Health check

-   `POST /message` - Message handling endpoint

**Note:** `POST /message` is for sending requests to the MCP server (JSON-RPC 2.0 payload).

## MCP Server Features

### Tool: `gpt_websearch`

Performs intelligent web searches with cost-effective model selection and (OpenAI provider) conversation continuity:

| Parameter              | Type    | Required | Default              | Description                                                                       |
| ---------------------- | ------- | -------- | -------------------- | --------------------------------------------------------------------------------- |
| `query`                | string  | Yes      | -                    | The search query or question                                                      |
| `model`                | string  | No       | provider default     | `gpt-5.4-mini` (openai) or `deepseek-v4-flash` (deepseek)                         |
| `reasoning_effort`     | string  | No       | `medium`             | Effort level:<br>`low` = 3 minutes<br>`medium` = 5 minutes<br>`high` = 10 minutes |
| `verbosity`            | string  | No       | `medium`             | Response verbosity: `low`, `medium`, or `high`                                    |
| `previous_response_id` | string  | No       | -                    | Previous response ID for conversation continuity (openai only; param absent for deepseek) |
| `prompt_cache_key`     | string  | No       | server default       | OpenAI prompt cache shard (openai only; param absent for deepseek)                |
| `web_search`           | boolean | No       | `true`               | Use web search (default: true)                                                    |

### Prompt: `web_search`

Enhanced prompt template that guides Claude Desktop to:

-   Analyze user questions in conversation context
-   Select cost-effective models based on complexity
-   Choose appropriate reasoning effort levels
-   Use single, sequential, or parallel search strategies
-   Remember and use response IDs for conversation continuity

### Example Response

```json
{
    "success": true,
    "answer": "The complete answer to your query...",
    "query": "original query",
    "model": "gpt-5.4-mini-0210-global",
    "effort": "low",
    "timeout_used": "3m0s",
    "id": "resp_68a24ac476a081a09c4c914ee8827c2b0f42d84e6960dd2d",
    "requested_model": "gpt-5.4-mini",
    "requested_effort": "low"
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
        "model": "gpt-5.4-mini",
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

```
answer [options] [question]

Options:
  -q, -question    Question to ask (required, can also use positional argument)
  -provider       API provider: openai (default) or deepseek (env PROVIDER)
  -model          Model (default: gpt-5.4-mini for openai, deepseek-v4-flash for deepseek; env MODEL)
  -effort         Reasoning effort: none (90s), low (3min), medium (5min), high (10min), xhigh (15min timeout) (default: medium)
  -verbosity      Response verbosity: low, medium, high (default: medium)
  -timeout        Request timeout (overrides effort-based defaults)
  -show-all       Show raw JSON response
  -base           API endpoint URL (default: provider-specific)
  -web-search     Use web search (default: true)
  -cache-key      OpenAI prompt_cache_key (openai provider only; env PROMPT_CACHE_KEY)
```

### MCP Server Mode

```
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

# Using gpt-5.4-mini for research tasks
./bin/answer -q "Explain the theory of relativity" -model gpt-5.4-mini

# DeepSeek backend
./bin/answer -provider deepseek -q "Latest EU regulations on AI"

# Debug mode with raw output
./bin/answer -q "Test query" -show-all
```

## Development

### Project Structure

```
Answer/
├── main.go              # Main entry point with CLI and MCP modes
├── api.go               # Responses API client and answer extraction
├── provider.go          # Provider registry (openai / deepseek)
├── mcp_server.go        # MCP tools, resources, prompts
├── prompts.go           # Per-provider MCP guidance prompts
├── models.go            # Effort and model registries
├── config.go            # Configuration structures and helpers
├── transport.go         # stdio/HTTP transports
├── auth.go              # JWT auth for HTTP transport
├── logging.go           # Structured logging
├── errors.go            # Error definitions
├── go.mod               # Go module definition
├── .env                 # Environment variables (not committed)
├── bin/
│   └── answer           # Compiled binary
├── AGENTS.md            # Development guidelines
└── README.md            # This file
```

### Building

```bash
task build   # preferred; or: GOEXPERIMENT=jsonv2 go build -o bin/answer .
```

### Testing

```bash
task test    # preferred; or: GOEXPERIMENT=jsonv2 go test ./...
```

### Formatting

```bash
go fmt ./...
```
