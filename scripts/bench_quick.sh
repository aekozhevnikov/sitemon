#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Quick benchmarks (single run) ==="

go test ./tests/benchmarks/... \
    -bench=. \
    -benchmem \
    -benchtime=1s \
    -count=1 \
    -run=^$ | tee bench_quick.txt

echo ""
echo "Results saved to: bench_quick.txt"
