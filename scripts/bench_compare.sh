#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ $# -lt 2 ]; then
    echo "Usage: $0 <old.txt> <new.txt>"
    echo ""
    echo "Example:"
    echo "  $0 bench_results_prev.txt bench_results.txt"
    exit 1
fi

OLD="$1"
NEW="$2"

if [ ! -f "$OLD" ]; then
    echo "File not found: $OLD"
    exit 1
fi

if [ ! -f "$NEW" ]; then
    echo "File not found: $NEW"
    exit 1
fi

echo "=== Comparing benchmarks ==="
echo "OLD: $OLD"
echo "NEW: $NEW"
echo ""

if command -v benchstat &> /dev/null; then
    benchstat "$OLD" "$NEW"
else
    echo "benchstat not found. Install: go install golang.org/x/perf/cmd/benchstat@latest"
fi
