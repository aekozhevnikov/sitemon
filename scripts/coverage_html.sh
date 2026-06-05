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

RESULTS_DIR="results"
mkdir -p "$RESULTS_DIR"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
COVERAGE_FILE="$RESULTS_DIR/coverage_${TIMESTAMP}.out"
HTML_FILE="$RESULTS_DIR/coverage_${TIMESTAMP}.html"
LATEST_LINK="$RESULTS_DIR/coverage_latest.html"

echo "=== Running tests with coverage ==="
go test -race -coverprofile="$COVERAGE_FILE" -coverpkg=./internal/... ./tests/unit/...

echo ""
echo "=== Coverage summary ==="
go tool cover -func="$COVERAGE_FILE"

echo ""
echo "=== Generating HTML report ==="
go tool cover -html="$COVERAGE_FILE" -o "$HTML_FILE"

# Create symlink to latest report
ln -sf "$HTML_FILE" "$LATEST_LINK"

echo ""
echo "=== Results ==="
echo "Coverage data: $COVERAGE_FILE"
echo "HTML report:   $HTML_FILE"
echo "Latest link:   $LATEST_LINK"
echo ""
echo "=== Opening in browser ==="
open "$HTML_FILE"
