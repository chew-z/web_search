# Release Notes

## v0.3.1 - August 19, 2025

### 🎉 New Features
- **Enhanced Web Search Capabilities:**
  - Added intelligent web search control for more precise results.
  - Implemented conversation continuity for web searches, allowing for more natural and continuous interactions.
  - Structured web search results for better readability and integration.
- **Improved API Client and Conversation Continuity:**
  - Introduced comprehensive tests for the API client and response handling, significantly improving stability and reliability.
  - Added conversation continuity via previous response ID, enabling seamless follow-up interactions.
- **Configurable HTTP Transport:**
  - Now allows configuring HTTP host and port for greater flexibility in deployment.
  - Updated HTTP transport to use POST for messages.
- **Centralized Logging:**
  - Centralized logging to MCP clients and enabled client logging for better monitoring and debugging.

### 🔧 Improvements
- **CLI and Prompt Enhancements:**
  - Default effort for CLI commands is now set to medium, with improved timeout logic for a smoother user experience.
  - Refined web search prompts for clarity and efficiency.
- **MCP Performance:**
  - Improved conversation continuity and overall performance within the MCP server.

### 📚 Documentation
- Updated project documentation and ignore files for better clarity and maintainability.
- Added release notes for v0.1.0.

### ⚙️ Internal Changes
- Refactored MCP server to leverage new `mcp-go` library features, simplifying transport and enhancing API usage.
- Simplified web search prompt construction and moved web search prompts to a dedicated file for better organization.