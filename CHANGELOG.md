# Release Notes

## Unreleased

### 🎉 New Features

- **Multi-Provider Support**: Choose between OpenAI (`openai`, default) and DeepSeek (`deepseek`) backends via the `PROVIDER` environment variable or the `-provider` flag (CLI and MCP modes). Each provider brings its own endpoint, API key env var (`OPENAI_API_KEY` / `DEEPSEEK_API_KEY`), web-search tool type, default model (`gpt-5.4-mini` / `deepseek-v4-flash`), and MCP guidance prompt. Provider registry lives in `provider.go`.
- **Reasoning Effort Levels**: Effort selection expanded to `none`, `low`, `medium`, `high`, `xhigh` with matching timeout defaults (90s/3/5/10/15 min).

### 🔧 Improvements

- **Cleaner Answers**: `ExtractAnswer` now strips intermediate narration messages emitted before/between tool calls (observed with DeepSeek), returning only the final answer text.
- **Continuity Scoping**: `previous_response_id` and `prompt_cache_key` are only exposed/used for providers that support conversation continuity (OpenAI); DeepSeek is stateless.
- **Provider Flag Ordering**: `-provider deepseek` now works even when `OPENAI_API_KEY` is unset (credentials are resolved after flag parsing).

### 📚 Documentation

- Updated README, AGENTS.md, and CLAUDE.md for multi-provider configuration, model guidance, and the refreshed build/test workflow (`task` + `GOEXPERIMENT=jsonv2`).

## v0.3.5 - 2025-12-08

### 🎉 New Features

- **Enhanced API Client**: Improved HTTP client with configurable timeouts and connection pooling for better reliability. Added response body size limiting and refined text extraction from API responses.
- **Structured Logging**: Introduced `slog`-based structured JSON logging with dynamic log level control, centralizing logging to MCP clients with thread-safe initialization.
- **Conversation Continuity**: Enhanced API client with conversation continuity via previous response ID, enabling seamless follow-up interactions.
- **Model Updates**: Updated model name references from GPT-5 to GPT-5.1 to reflect expected future model designations.

### 🔧 Improvements

- **Dependency Updates**: Updated minimum Go version to 1.25.5 and bumped `mcp-go` dependency to v0.43.2 for compatibility and latest features.
- **Gitignore Enhancements**: Added repomix and other generated files to `.gitignore` to keep repository clean.
- **Configuration**: Updated server version metadata and prepared for new configuration options related to logging verbosity and effort levels.

### 📚 Documentation

- Updated release notes and project documentation.