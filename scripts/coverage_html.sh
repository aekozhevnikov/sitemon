#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Running tests with coverage ==="
go test -race -coverprofile=coverage.out -coverpkg=./internal/... ./tests/unit/...

echo ""
echo "=== Coverage summary ==="
go tool cover -func=coverage.out

echo ""
echo "=== Generating HTML report ==="
go tool cover -html=coverage.out -o coverage.html

echo ""
echo "=== Opening in browser ==="
open coverage.html
