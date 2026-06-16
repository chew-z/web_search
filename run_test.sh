#!/bin/sh

export PATH="/usr/local/go/bin:$PATH"

# Required for encoding/json/v2 + encoding/json/jsontext imports (Go 1.26).
export GOEXPERIMENT=jsonv2

# Run tests
go test -v ./...
