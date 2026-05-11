#!/usr/bin/env bash
# Phase 2 acceptance: 10K signals/sec for 60s, p99 < 50ms.
#
# Why this script bumps VELLUM_RATE_LIMIT_RPS: vegeta runs on one host so every
# request shares one source IP (127.0.0.1). The production default of 1000
# req/sec per source is intentional (FR-1.6 — protects against a single
# chatty agent), but for a single-host load test we need to lift it or
# we'd just be benchmarking the rate limiter. The limiter is still in the
# loop — we just configure it so localhost can be a "fleet" for this test.
#
# Usage:
#   ./scripts/load-test.sh              # uses defaults
#   RATE=15000 DURATION=30s ./scripts/load-test.sh
#
# Acceptance:
#   success rate ≥ 99% (HTTP 2xx)
#   p99 latency ≤ 50 ms
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Tunables.
RATE="${RATE:-10000}"          # requests per second
DURATION="${DURATION:-60s}"
TARGET="${TARGET:-http://localhost:8080}"
REPORT_DIR="${REPORT_DIR:-${ROOT}/.loadtest}"

mkdir -p "$REPORT_DIR"

if ! command -v vegeta >/dev/null 2>&1; then
  echo "vegeta not found. Install with: brew install vegeta" >&2
  exit 1
fi

# Build a one-line target list. POST /v1/signals with a static body —
# vegeta sends the same payload for every request, which is fine because
# the backend's hot path doesn't care about payload contents.
TARGETS="$REPORT_DIR/targets.txt"
BODY="$REPORT_DIR/body.json"
cat > "$BODY" <<'JSON'
{"component_id":"RDBMS_PRIMARY_01","component_type":"RDBMS","severity":"P0","source":"vegeta","payload":{"err":"benchmark"}}
JSON
cat > "$TARGETS" <<EOF
POST $TARGET/v1/signals
Content-Type: application/json
@$BODY
EOF

echo ">>> Vegeta attack: rate=${RATE}/s duration=${DURATION} target=${TARGET}"
echo ">>> (Make sure backend is running with VELLUM_RATE_LIMIT_RPS >= ${RATE})"

RESULTS="$REPORT_DIR/results.bin"
vegeta attack \
  -targets="$TARGETS" \
  -rate="$RATE" \
  -duration="$DURATION" \
  -timeout=2s \
  -max-workers=512 \
  > "$RESULTS"

echo
echo ">>> Text report:"
vegeta report -type=text "$RESULTS" | tee "$REPORT_DIR/report.txt"

echo
echo ">>> JSON report saved to $REPORT_DIR/report.json"
vegeta report -type=json "$RESULTS" > "$REPORT_DIR/report.json"

echo
echo ">>> Pass/fail check:"
python3 - <<PY
import json, sys
r = json.load(open("$REPORT_DIR/report.json"))
success = r["success"]                   # 0..1 fraction
p99_ns  = r["latencies"]["99th"]
p99_ms  = p99_ns / 1e6
rate    = r["rate"]
duration_s = r["duration"] / 1e9
print(f"  effective rate: {rate:.0f} req/s")
print(f"  duration:       {duration_s:.1f} s")
print(f"  success:        {success*100:.2f}%")
print(f"  p99 latency:    {p99_ms:.2f} ms")
ok = success >= 0.99 and p99_ms <= 50.0
print()
print("  PASS" if ok else "  FAIL")
sys.exit(0 if ok else 1)
PY
