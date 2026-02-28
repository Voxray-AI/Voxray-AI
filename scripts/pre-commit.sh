#!/bin/bash

# Go pre-commit checks: format and vet.
# Run from repo root or from scripts/; exits 1 on failure.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "Running pre-commit checks..."

# Format check: list files that would change
echo "Checking code formatting..."
UNFORMATTED=$(gofmt -l . 2>/dev/null | grep -v '^$' || true)
if [ -n "$UNFORMATTED" ]; then
  echo -e "${RED}Code formatting issues found. The following files need 'go fmt':${NC}"
  echo "$UNFORMATTED"
  echo "Run: ./scripts/fix-lint.sh  or  go fmt ./..."
  exit 1
fi

# Vet
echo "Running go vet..."
if ! go vet ./...; then
  echo -e "${RED}go vet failed.${NC}"
  exit 1
fi

echo -e "${GREEN}All pre-commit checks passed.${NC}"
