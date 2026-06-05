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

echo "=== Quick benchmarks (single run) ==="

go test ./tests/benchmarks/... \
    -bench=. \
    -benchmem \
    -benchtime=1s \
    -count=1 \
    -run=^$ | tee bench_quick.txt

echo ""
echo "Results saved to: bench_quick.txt"
