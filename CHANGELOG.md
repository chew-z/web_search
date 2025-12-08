# Release Notes

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