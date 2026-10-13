---
trigger: glob
globs: "**/*.go"
---

# Go version and dependency rule

The target is the `go` directive in `go.mod`; `GOLANG.md` states the version and the skill to use.

- Reach for the standard library first.
- Justify every new direct dependency: what it replaces, its size, and its maintenance cost.
- Run `task tidy` after every dependency change.
- Never bump the `go` directive or add a `toolchain` line unless asked.
