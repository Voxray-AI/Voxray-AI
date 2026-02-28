#!/bin/bash

# Fix Go formatting and optionally run golangci-lint --fix.
# Run from repo root or from scripts/.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo "Running go fmt..."
go fmt ./...

if command -v golangci-lint &>/dev/null; then
  echo "Running golangci-lint --fix..."
  golangci-lint run --fix ./...
else
  echo "golangci-lint not installed; skipping. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
fi

echo "Done."
