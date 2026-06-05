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

OUTPUT="bench_results.txt"
PREVIOUS="bench_results_prev.txt"

echo "=== Running benchmarks ==="
echo "Date: $(date)"
echo ""

# Run benchmarks 3 times for statistical significance
go test ./tests/benchmarks/... \
    -bench=. \
    -benchmem \
    -benchtime=3s \
    -count=3 \
    -run=^$ | tee "$OUTPUT"

echo ""
echo "=== Statistical summary (benchstat) ==="
if command -v benchstat &> /dev/null; then
    benchstat "$OUTPUT"
else
    echo "benchstat not found. Install: go install golang.org/x/perf/cmd/benchstat@latest"
fi

# Compare with previous run if available
if [ -f "$PREVIOUS" ]; then
    echo ""
    echo "=== Comparison with previous run ==="
    benchstat "$PREVIOUS" "$OUTPUT"
fi

# Save current as previous for next run
cp "$OUTPUT" "$PREVIOUS"

echo ""
echo "Results saved to: $OUTPUT"
