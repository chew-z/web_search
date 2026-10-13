#!/bin/sh
set -eu

# Format Go files: delegates to Task once; runs gofmt directly when called from a task.
cd "$(dirname "$0")"
# Ensure Go tools are in PATH. Appended, not prepended, so a Go already on
# PATH (Homebrew, asdf, a newer toolchain) still wins.
export PATH="${PATH}:/usr/local/go/bin"

if [ -z "${RUN_WRAPPER_IN_TASK:-}" ] && command -v task >/dev/null 2>&1; then
  RUN_WRAPPER_IN_TASK=1 exec task fmt
fi
exec gofmt -w .
