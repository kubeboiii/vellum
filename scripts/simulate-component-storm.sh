#!/usr/bin/env bash
# Phase 3 acceptance demo: fire N signals at one component_id over D
# seconds, then verify:
#   - Mongo `signals` has ~N raw documents for that component_id
#   - Postgres `work_items` has 1-3 rows (depending on window boundaries)
#   - reduction ratio (signals / work_items) ≥ 60×  (the spec target)
#
# This is NOT the full failure simulator (Phase 6); it's the smallest
# script needed to prove Phase 3's acceptance criterion from the PRD §7
# Goal G2.
#
# Prereqs:
#   - docker compose stack is up (postgres, mongo, redis healthy)
#   - migrations applied: migrate -path backend/migrations -database "$DATABASE_URL" up
#   - backend running on :8080
#
# Usage:
#   ./scripts/simulate-component-storm.sh
#   COMPONENT=RDBMS_X N=400 DURATION=8 ./scripts/simulate-component-storm.sh

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

COMPONENT="${COMPONENT:-CACHE_CLUSTER_$(date +%s)}"
N="${N:-200}"
DURATION="${DURATION:-8}"
TARGET="${TARGET:-http://localhost:8080}"

# Sleep duration between requests in seconds (float). 200 / 8 = 25/s
# means 40ms between sends. We use python to compute the float.
GAP="$(python3 -c "print(${DURATION} / ${N})")"

echo ">>> Storm config:"
echo "    component_id  = ${COMPONENT}"
echo "    signal count  = ${N}"
echo "    duration      = ${DURATION}s  (~$(python3 -c "print(round(${N}/${DURATION},1))")/s)"
echo "    target        = ${TARGET}"
echo

# Confirm backend is up.
if ! curl -sf "${TARGET}/health" >/dev/null; then
  echo "ERROR: backend not reachable at ${TARGET}/health" >&2
  exit 1
fi

echo ">>> Firing ${N} signals..."
ok=0
fail=0
start=$(date +%s)
for i in $(seq 1 "$N"); do
  body=$(cat <<JSON
{"component_id":"${COMPONENT}","component_type":"CACHE","severity":"P0","source":"storm","payload":{"i":${i}}}
JSON
)
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
    -H 'Content-Type: application/json' \
    "${TARGET}/v1/signals" \
    -d "$body" || echo "000")
  if [[ "$code" == "202" ]]; then
    ok=$((ok+1))
  else
    fail=$((fail+1))
  fi
  sleep "$GAP"
done
end=$(date +%s)

echo "    accepted (202): $ok"
echo "    failed (non-202): $fail"
echo "    elapsed wall:    $((end-start))s"

# Give the workers a beat to drain whatever's still in the queue.
echo
echo ">>> Waiting 2s for queue drain..."
sleep 2

# Read counts from each sink.
echo
echo ">>> Sink counts for component_id=${COMPONENT}:"

MONGO_COUNT=$(docker exec ims-mongo mongosh --quiet -u ims -p ims \
  --authenticationDatabase admin ims \
  --eval "db.signals.countDocuments({component_id:'${COMPONENT}'})")
echo "    mongo.signals      = ${MONGO_COUNT}"

PG_WI_COUNT=$(docker exec ims-postgres psql -U ims -d ims -tA -c \
  "SELECT COUNT(*) FROM work_items WHERE component_id='${COMPONENT}';")
echo "    pg.work_items      = ${PG_WI_COUNT}"

# signal_count column sums — should equal MONGO_COUNT (one bump per signal).
PG_SIGNAL_SUM=$(docker exec ims-postgres psql -U ims -d ims -tA -c \
  "SELECT COALESCE(SUM(signal_count),0) FROM work_items WHERE component_id='${COMPONENT}';")
echo "    pg.sum(signal_count) = ${PG_SIGNAL_SUM}"

TS_COUNT=$(docker exec ims-postgres psql -U ims -d ims -tA -c \
  "SELECT COUNT(*) FROM signal_metrics WHERE work_item_id IN
     (SELECT id FROM work_items WHERE component_id='${COMPONENT}');")
echo "    timescale.signal_metrics = ${TS_COUNT}"

echo
echo ">>> Pass/fail check:"
python3 - <<PY
mongo_n   = int("${MONGO_COUNT}")
pg_n      = int("${PG_WI_COUNT}")
pg_sum    = int("${PG_SIGNAL_SUM}")
ts_n      = int("${TS_COUNT}")
fired     = int("${ok}")
ratio     = mongo_n / pg_n if pg_n else 0
print(f"    fired (202):           {fired}")
print(f"    raw signals in Mongo:  {mongo_n}")
print(f"    work_items in Postgres:{pg_n}")
print(f"    sum(signal_count):     {pg_sum}")
print(f"    rows in Timescale:     {ts_n}")
print(f"    reduction ratio:       {ratio:.1f}x")
print()
ok = (
    pg_n >= 1 and pg_n <= 3 and       # 1-3 work items (window boundaries)
    mongo_n >= int(fired * 0.99) and   # ~all raw signals saved (allow 1% slack)
    ratio >= 60.0                      # G2 reduction target
)
print("    PASS" if ok else "    FAIL")
import sys
sys.exit(0 if ok else 1)
PY
