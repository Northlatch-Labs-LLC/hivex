#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RESULT_DIR="$ROOT/bench/results"
TIMESTAMP="$(date -u +"%Y%m%dT%H%M%SZ")"
RUN_ID="${TIMESTAMP}-$$"
OUT_FILE="$RESULT_DIR/$RUN_ID.json"
HIVEX_BIN="${HIVEX_BENCH_HIVEX_BINARY:-$ROOT/hivex}"
BENCH_BIN="${hivebotBENCH_BIN:-$ROOT/hivexbench}"
PROBE="${hivebotBENCH_PROBE:-all}"

mkdir -p "$RESULT_DIR"

if [[ ! -x "$HIVEX_BIN" ]]; then
  go build -o "$HIVEX_BIN" ./cmd/hivex
fi

go build -o "$BENCH_BIN" ./cmd/hivexbench

args=(--probe "$PROBE" --out json)
if [[ -n "${hivebotBENCH_ITERS:-}" ]]; then
  args+=(--iters "$hivebotBENCH_ITERS")
fi

HIVEX_BENCH_HIVEX_BINARY="$HIVEX_BIN" "$BENCH_BIN" "${args[@]}" > "$OUT_FILE"
echo "$OUT_FILE"
