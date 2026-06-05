#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Unset sitemon env vars so benchmarks are not affected by .env
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
OUTPUT="$RESULTS_DIR/bench_quick_${TIMESTAMP}.txt"

echo "=== Quick benchmarks (single run) ==="

go test ./tests/benchmarks/... \
    -bench=. \
    -benchmem \
    -benchtime=1s \
    -count=1 \
    -run=^$ | tee "$OUTPUT"

echo ""
echo "Results saved to: $OUTPUT"
