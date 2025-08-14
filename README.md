# Answer - GPT Web Search CLI & MCP Server

A Go application that provides intelligent web search capabilities using OpenAI's GPT models. Works as both a CLI tool and MCP (Model Context Protocol) server with cost-effective model selection.

## Features

- 🔍 **Intelligent Web Search**: Uses OpenAI's GPT models (gpt-5, gpt-5-mini, gpt-5-nano) with web search capabilities
- 🎯 **Cost-Effective**: Automatic model selection based on query complexity for optimal cost/performance
- 🚀 **Dual Mode**: CLI tool and MCP server with stdio/HTTP transports
- ⚙️ **Smart Configuration**: Effort-based timeouts (3/5/10 minutes) and environment-driven setup
- 🧠 **Enhanced MCP Prompts**: Intelligent prompt templates guide optimal tool usage
- 🔐 **Secure**: Environment-based API key management

## Installation

### Prerequisites

- Go 1.24.0 or later
- OpenAI API key with web search preview access

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd Answer

# Install dependencies
go mod download

# Build the binary
go build -o bin/answer .

# Or install globally
go install .
```

## Configuration

### Environment Variables

Create a `.env` file in the project root:

```env
OPENAI_API_KEY=your-api-key-here
MODEL=gpt-5-mini         # Optional: gpt-5-mini (default), gpt-5, gpt-5-nano
EFFORT=low               # Optional: reasoning effort (low/medium/high)
SHOW_ALL=false           # Optional: show raw JSON
QUESTION=                # Optional: default question
```

**Model Selection Guidelines**:
- `gpt-5-nano`: Simple facts, definitions, quick lookups
- `gpt-5-mini`: Research tasks, comparisons, specific topics  
- `gpt-5`: Complex analysis, coding questions, reasoning tasks

**Effort-Based Timeouts**: `low` = 3 minutes, `medium` = 5 minutes, `high` = 10 minutes.

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
./bin/answer -q "Explain quantum computing" -model gpt-5-mini -effort high

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

#### HTTP/SSE Transport (for Web Integration)

```bash
# Start HTTP server on default port 8080
./bin/answer mcp -t http

# Custom port
./bin/answer mcp -t http -port 3000

# With verbose logging
./bin/answer mcp -t http -verbose
```

**Endpoints:**
- `GET /` - API documentation
- `GET /health` - Health check
- `GET /sse` - Server-Sent Events for MCP protocol
- `GET /message` - Message handling endpoint

## MCP Server Features

### Tool: `gpt_websearch`
Performs intelligent web searches with cost-effective model selection:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | - | The search query or question |
| `model` | string | No | `gpt-5-mini` | GPT model: gpt-5-mini, gpt-5, or gpt-5-nano |
| `reasoning_effort` | string | No | `low` | Effort level:<br>`low` = 3 minutes<br>`medium` = 5 minutes<br>`high` = 10 minutes |

### Prompt: `web_search`
Enhanced prompt template that guides Claude Desktop to:
- Analyze user questions in conversation context
- Select cost-effective models based on complexity
- Choose appropriate reasoning effort levels
- Use single, sequential, or parallel search strategies

### Example Response

```json
{
  "success": true,
  "answer": "The complete answer to your query...",
  "query": "original query",
  "model": "gpt-5-mini",
  "effort": "low",
  "timeout_used": "3m0s"
}
```

## Command-Line Reference

### CLI Mode
```
answer [options] [question]

Options:
  -q, -question    Question to ask (required, can also use positional argument)
  -model          Model: gpt-5-mini (default), gpt-5, gpt-5-nano
  -effort         Reasoning effort: low (3min), medium (5min), high (10min timeout)
  -timeout        Request timeout (overrides effort-based defaults)
  -show-all       Show raw JSON response
  -base           API endpoint URL
```

### MCP Server Mode
```
answer mcp [options]

Options:
  -t, --transport  Transport type: stdio or http (default: stdio)
  -port           HTTP server port (default: 8080)
  -base           API endpoint URL
  -verbose        Enable verbose logging
```

## Examples

### CLI Examples

```bash
# Quick question
./bin/answer "What's the weather in San Francisco?"

# Research query with high effort
./bin/answer -q "Latest breakthroughs in quantum computing 2024" -effort high

# Using gpt-5-mini for research tasks
./bin/answer -q "Explain the theory of relativity" -model gpt-5-mini

# Debug mode with raw output
./bin/answer -q "Test query" -show-all
```

### MCP Integration Examples

**JavaScript SSE Client:**

```javascript
const eventSource = new EventSource('http://localhost:8080/sse');

eventSource.onmessage = (event) => {
  console.log('Received:', JSON.parse(event.data));
};

// Send request via fetch to message endpoint
fetch('http://localhost:8080/message', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    jsonrpc: '2.0',
    method: 'tools/call',
    params: {
      name: 'gpt_websearch',
      arguments: {
        query: 'Latest AI news',
        model: 'gpt-5-mini',
        reasoning_effort: 'medium'  // 5-minute timeout
      }
    },
    id: 1
  })
});
```

## Docker Support

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o answer .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/answer /usr/local/bin/answer
ENV OPENAI_API_KEY=""
EXPOSE 8080
CMD ["answer", "mcp", "-t", "http"]
```

Build and run:
```bash
docker build -t answer .
docker run -p 8080:8080 -e OPENAI_API_KEY=your-key answer
```

## Development

### Project Structure
```
Answer/
├── main.go              # Main entry point with CLI and MCP modes
├── config.go           # Configuration structures and helpers
├── errors.go           # Error definitions
├── go.mod              # Go module definition
├── go.sum              # Dependency checksums
├── .env                # Environment variables (not committed)
├── bin/
│   └── answer         # Compiled binary
├── AGENTS.md          # Development guidelines
├── MCP_SERVER.md      # Detailed MCP documentation
└── README.md          # This file
```

### Building
```bash
go build -o bin/answer .
```

### Testing
```bash
go test ./...
```

### Formatting
```bash
go fmt ./...
```

## Troubleshooting

### Common Issues

1. **"OPENAI_API_KEY is not set"**
   - Set the environment variable or create a `.env` file

2. **"API error: status=401"**
   - Verify your API key is valid and has web search preview access

3. **"no output_text found in response"**
   - The model might not support web search
   - Try using a different model

4. **MCP client connection issues**
   - For stdio: Check that the binary path is correct
   - For HTTP: Verify the port is not in use

### Debug Mode

```bash
# CLI debug
./bin/answer -q "test" -show-all

# MCP debug
./bin/answer mcp -t stdio -verbose
./bin/answer mcp -t http -verbose
```

## License

See LICENSE file for details.

## Contributing

Please see [AGENTS.md](AGENTS.md) for development guidelines and contribution rules.

## Links

- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [OpenAI API Documentation](https://platform.openai.com/docs)
- [Detailed MCP Server Documentation](MCP_SERVER.md)
