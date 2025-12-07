# Release Notes

## v0.3.1 - 2025-08-18

### 🎉 New Features

-   **Conversation Continuity**: Implemented conversation continuity for web searches, allowing for more context-aware interactions.
-   **Web Search Tool**: Added a web search tool and server functionality to the API.
-   **Structured Logging**: Integrated `slog` for structured JSON logging with dynamic log level control and thread-safe initialization.
-   **Verbosity Control**: Added verbosity parameter (low/medium/high) for controlling response detail level.
-   **Reasoning Effort Levels**: Support for four effort levels (minimal/low/medium/high) with effort-based timeouts (90s/3min/5min/10min).
-   **Enhanced Prompt System**: Comprehensive prompt template with parameter optimization recommendations and conversation continuity best practices.
-   **Configuration**: The application now loads configuration from environment variables, providing more flexibility.
-   **CLI**: Added a Go CLI for interacting with the API.
-   **Default Model**: Set `gpt-5-mini` as the default model for all interactions.

### 🔧 Improvements

-   **Prompt Engineering**: Refined and enhanced web search prompts for clarity, efficiency, and cost-effectiveness.
-   **Code Refactoring**: Simplified web search prompt construction and refactored the MCP server to use new library features, improving code quality and maintainability.

### 📚 Documentation

-   Added and updated release notes.
