#!/bin/sh
set -eu

# Run golangci-lint: delegates to Task once; runs it directly when called from a task.
cd "$(dirname "$0")"
# Ensure Go tools are in PATH. Appended, not prepended, so a Go already on
# PATH (Homebrew, asdf, a newer toolchain) still wins.
export PATH="${PATH}:/usr/local/go/bin"

if [ -z "${HOME:-}" ]; then export HOME="$PWD"; fi
if [ "$(uname -s)" = "Darwin" ]; then
  export GOLANGCI_LINT_CACHE="${HOME}/Library/Caches/golangci-lint"
else
  export GOLANGCI_LINT_CACHE="${XDG_CACHE_HOME:-${HOME}/.cache}/golangci-lint"
fi
export GOCACHE="${XDG_CACHE_HOME:-${HOME}/.cache}/go-build"

if [ -z "${RUN_WRAPPER_IN_TASK:-}" ] && command -v task >/dev/null 2>&1; then
  RUN_WRAPPER_IN_TASK=1 exec task lint
fi
exec golangci-lint run --fix ./...
