#!/bin/sh

# Ensure Go tools are in PATH
export PATH="/usr/local/go/bin:$PATH"

# Required for encoding/json/v2 + encoding/json/jsontext imports (Go 1.26);
# golangci-lint loads packages and would otherwise fail the build constraint.
export GOEXPERIMENT=jsonv2

# Set cache directories (use existing HOME or fallback)
if [ -z "$HOME" ]; then
    export HOME="$PWD"
fi
export GOLANGCI_LINT_CACHE="$HOME/Library/Caches/golangci-lint"
export GOCACHE="$HOME/.cache/go-build"

# Run linter using PATH
golangci-lint run --fix ./...
