# TODO.md

This document outlines the tasks identified during the review of the project, based on findings by Codex.

## 1) Tests

- **Go tests implemented and verified:** Comprehensive unit tests have been added for key functions and components, including:
    - `ExtractAnswer` (concatenating `output_text` segments, empty responses, nil API response)
    - `validateEffort`/`validateVerbosity` (valid enums, defaults)
    - `getTimeoutForEffort` (exact durations)
    - `CallAPI` (success, error handling for malformed JSON/non-2xx HTTP errors, timeout, request shape with/without web search tools, missing API key)
    - `loadEnvConfig` (missing key, parsing booleans/durations, reading other environment variables)
    - `parseMCPConfig` (defaults and non-defaults)
    - Basic health route test for `mcp_server.go` (verifying server setup).

    All tests are hermetic, use `httptest.NewServer` for HTTP calls and `t.Setenv` for environment variables, and follow idiomatic Go practices including table-driven tests and parallel execution where safe. The tests have been verified and are passing.


    
    

## 3) Module Path Sanity

- **Canonical Module Path:** The current module path `module Answer` is not canonical and prevents `go get`/`go install`.
    - Choose a canonical, lowercase VCS-backed module path (e.g., `github.com/<your-org>/answer`).
    - Update the `module` directive in `go.mod` accordingly.
    - Ensure the directory name and module path casing are aligned (prefer lowercase `answer`).
    - After changing the module path, run `go mod tidy` and validate builds (`go build -o bin/answer .`).
- **Go Version Alignment:**
    - Align the `go` directive in `go.mod` to a released Go version (e.g., `go 1.23`) and update `README.md`/Docker references accordingly.
