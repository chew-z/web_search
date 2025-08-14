# CLAUDE.md

LLM guidance for working with Answer - dual-mode Go application (CLI + MCP server) for OpenAI web search.

## Build Commands
- `go build -o bin/answer .`
- `./run_format.sh` - format code 
- `./run_lint.sh` - lint (golangci-lint)
- `./run_test.sh` - run tests
- `./test_timeouts.sh` - timeout behavior testing

## Environment
Required: `OPENAI_API_KEY`
Optional: `MODEL`, `EFFORT`, `SHOW_ALL`, `TIMEOUT`, `QUESTION`
Uses godotenv for `.env` loading

## Architecture
**Dual-mode design**: CLI or MCP server based on args
- CLI: `./bin/answer "query"`
- MCP: `./bin/answer mcp [options]`

**Key files**:
- `main.go` - entry point, mode routing
- `mcp_server.go` - MCP server implementation (commit b6c3478 enhanced prompts)
- `api.go` - OpenAI API integration  
- `config.go` - environment config, timeouts
- `transport.go` - stdio/HTTP transports
- `errors.go` - error handling

## Recent Changes (commit b6c3478)
**Enhanced MCP Prompt System**: Replaced basic `web_search` with `intelligent_web_search`
- **Name**: `intelligent_web_search` (was `web_search`)
- **Argument**: `user_question` (was `topic`)
- **System Message**: Comprehensive LLM instructions for cost-effective tool usage

**Model Selection Logic**:
- `gpt-5-nano`: Simple facts, definitions, summaries
- `gpt-5-mini`: Research, comparisons, specific topics
- `gpt-5`: Complex analysis, coding, reasoning

**Reasoning Effort**:
- `low`: 3min timeout, factual queries
- `medium`: 5min timeout, synthesis tasks
- `high`: 10min timeout, complex analysis

**Search Strategy**: Single/sequential/parallel approaches based on query complexity

## MCP Implementation Details

**Tool**: `gpt_websearch` - web search using GPT models with:
- Model selection: gpt-5-nano/mini/full based on complexity
- Reasoning effort: low/medium/high with timeout mapping
- Query formulation: context-aware, detailed searches
- Strategy: single/sequential/parallel based on task

**Prompt Template**: `intelligent_web_search` (mcp_server.go:139-213) provides:
- Systematic LLM instructions for cost-effective tool usage
- Model selection guidelines by task complexity
- Search strategy optimization (single/sequential/parallel)
- Query formulation best practices

**Transports**: 
- STDIO: Claude Desktop integration
- HTTP/SSE: Web applications (port 8080)

## API Integration
- Endpoint: `https://api.openai.com/v1/responses`
- Tool type: `web_search_preview`
- Models: gpt-5, gpt-5-mini, gpt-5-nano
- Effort-based timeouts: 3/5/10 minutes

## Error Handling
- CLI: `fail()` function with exit codes (errors.go)
- MCP: Structured JSON responses
- API errors: Custom `APIError` type wrapping

## Configuration Priority
1. CLI flags
2. Environment variables  
3. Defaults (gpt-5, low effort, 3min timeout)

## Testing
- `integration_test.go` - core API functions
- `test_timeouts.sh` - timeout behavior
- Environment-aware skipping for missing API keys

## Development Workflow
1. Set `OPENAI_API_KEY`
2. Make changes
3. `./run_format.sh && ./run_lint.sh && ./run_test.sh`
4. `go build -o bin/answer .`
5. Test CLI: `./bin/answer "query"`
6. Test MCP: `./bin/answer mcp -t stdio`

## Dependencies
- `github.com/mark3labs/mcp-go` v0.37.0 - MCP protocol
- `github.com/joho/godotenv` v1.5.1 - environment loading