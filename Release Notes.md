# Release Notes

## Unreleased

### 🎉 New Features

- **Multi-Provider Support:**
    -   Choose between OpenAI (`openai`, default) and DeepSeek (`deepseek`) backends via the `PROVIDER` environment variable or the `-provider` flag (CLI and MCP modes).
    -   Each provider brings its own endpoint, API key env var (`OPENAI_API_KEY` / `DEEPSEEK_API_KEY`), web-search tool type, default model (`gpt-5.4-mini` / `deepseek-v4-flash`), and MCP guidance prompt. Provider registry lives in `provider.go`.
- **Reasoning Effort Levels:**
    -   Effort selection expanded to `none`, `low`, `medium`, `high`, `xhigh` with matching timeout defaults (90s/3/5/10/15 min).

### 🔧 Improvements

- **Cleaner Answers:**
    -   Answer extraction now strips intermediate narration messages emitted before/between tool calls (observed with DeepSeek), returning only the final answer text.
- **Continuity Scoping:**
    -   `previous_response_id` and `prompt_cache_key` are only exposed/used for providers that support conversation continuity (OpenAI); DeepSeek is stateless.
- **Provider Flag Ordering:**
    -   `-provider deepseek` now works even when `OPENAI_API_KEY` is unset (credentials are resolved after flag parsing).

### 📚 Documentation

- Updated README, AGENTS.md, and CLAUDE.md for multi-provider configuration, model guidance, and the refreshed build/test workflow (`task` + `GOEXPERIMENT=jsonv2`).

## v0.3.5 - December 08, 2025

### 🎉 New Features

- **Enhanced API Client and Response Handling:**
    -   Introduced a more robust HTTP client with configurable timeouts and connection pooling to enhance API call reliability.
    -   Limited response body size to prevent excessive memory usage and processing large, potentially malformed responses.
    -   Refined logic for extracting text from API responses to correctly concatenate multiple text segments into a single, coherent answer.
- **Structured Logging with Dynamic Level Control:**
    -   Introduced structured logging using the `slog` package to provide better visibility into the application's behavior.
    -   Centralized logging logic, allowing for dynamic control of log levels, specifically enabling debug logging when the verbose flag is set.
    -   Enhanced observability by providing consistent, machine-readable log output.
- **Conversation Continuity and API Improvements:**
    -   Added comprehensive tests for the API client and response handling, significantly improving stability and reliability.
    -   Implemented conversation continuity via previous response ID, enabling seamless follow-up interactions.
- **Model Name Updates:**
    -   Updated model name references from GPT-5 to GPT-5.1 across documentation and prompt configuration files to reflect expected future model designations, ensuring consistency for complex reasoning tasks.

### 🔧 Improvements

- **Dependency and Language Updates:**
    -   Updated the minimum required Go version to 1.25.5 to ensure compatibility with newer language features and address known issues.
    -   Bumped the `mcp-go` dependency to v0.43.2 to incorporate recent improvements and fixes from the library.
- **Repository Maintenance:**
    -   Updated `.gitignore` file to include repomix-output.xml and other generated files, preventing accidental commits and keeping the repository clean.
- **Configuration and Versioning:**
    -   Updated server version metadata and prepared for new configuration options related to logging verbosity and effort levels.

### 📚 Documentation

-   Updated project release notes and documentation.

### ⚙️ Internal Changes

-   Refactored model name constants and references throughout the codebase for consistency.
-   Enhanced configuration loading and validation logic.