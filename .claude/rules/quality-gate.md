---
paths:
  - "**/*.go"
---

# Run the Go quality gate

Run `task check` before reporting work done.
A red gate is not done.

`task check` runs `fmt:check`, `vet`, `lint:ci`, and `test:short`.
Do not weaken `.golangci.yml` or skip tests to get green.
If a check cannot run, say which check failed and why.
