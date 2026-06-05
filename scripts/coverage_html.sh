#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Unset sitemon env vars so tests use defaults and explicit test values only.
unset SITEMON_SERVER_ADDR
unset SITEMON_CHECK_INTERVAL
unset SITEMON_TIMEOUT
unset SITEMON_STORAGE_PATH
unset SITEMON_TELEGRAM_BOT_TOKEN
unset SITEMON_TELEGRAM_CHAT_ID
unset SITEMON_SITES

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
