#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

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
