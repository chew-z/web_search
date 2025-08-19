# TODO.md

This document outlines the tasks identified during the review of the project, based on findings by Codex.

## 1) Tests

- **Add Go tests:** The repository currently lacks Go tests.
    - Implement unit tests for key functions and components, including:
        - `ExtractAnswer` (concatenating `output_text` segments, empty responses)
        - `validateEffort`/`validateVerbosity` (valid enums, defaults)
        - `getTimeoutForEffort` (exact durations)
        - `CallAPI` (success, error handling, request shape)
        - `HandleWebSearch` (missing query, success path, timeout)
        - `loadEnvConfig` (missing key, parsing booleans/durations)
    - Ensure tests are hermetic using `httptest.NewServer` for HTTP calls and `t.Setenv` for environment variables (e.g., API keys).
    - Integrate `run_test.sh` into CI to ensure tests are run automatically.


    
    

## 3) Module Path Sanity

- **Canonical Module Path:** The current module path `module Answer` is not canonical and prevents `go get`/`go install`.
    - Choose a canonical, lowercase VCS-backed module path (e.g., `github.com/<your-org>/answer`).
    - Update the `module` directive in `go.mod` accordingly.
    - Ensure the directory name and module path casing are aligned (prefer lowercase `answer`).
    - After changing the module path, run `go mod tidy` and validate builds (`go build -o bin/answer .`).
- **Go Version Alignment:**
    - Align the `go` directive in `go.mod` to a released Go version (e.g., `go 1.23`) and update `README.md`/Docker references accordingly.
