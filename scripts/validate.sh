#!/usr/bin/env sh
set -eu

# validate.sh — repeatable local validation for the heindall API slice.
# Usage: ./scripts/validate.sh
#
# Runs: unit tests, benchmark comparisons, allocation checks, and lightweight
# deployment-compliance checks against the local compose definition.

API_DIR="$(cd "$(dirname "$0")/../apps/api" && pwd)"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== heindall local validation ==="
echo ""

# 1. Unit tests
echo "[1/5] Running unit tests..."
cd "$API_DIR"
go test ./... -count=1 || { echo "FAIL: unit tests"; exit 1; }
echo "      PASS"
echo ""

# 2. Benchmark the handler path (fast path) and check zero allocation regressions.
echo "[2/5] Benchmarking raw handler path..."
cd "$API_DIR"
go test -bench='BenchmarkFraudScoreRawHandler$' -benchmem -benchtime=500ms ./internal/router/ \
  | tee /tmp/heindall-bench-handler.txt
echo ""

go test -bench='BenchmarkReadRequestBody(PooledFastPath|ExactSize)$' -benchmem -benchtime=500ms ./internal/router/ \
  | tee /tmp/heindall-bench-body-read.txt
echo ""

# 3. Benchmark the ready handler.
echo "[3/5] Benchmarking ready handler..."
cd "$API_DIR"
go test -bench='BenchmarkReadyRawHandler$' -benchmem -benchtime=500ms ./internal/router/ \
  | tee /tmp/heindall-bench-ready.txt
echo ""

# 4. Allocation check (ensure the handler is not regressing).
echo "[4/5] Checking allocation sanity..."
BODY_FAST_ALLOCS=$(grep 'BenchmarkReadRequestBodyPooledFastPath' /tmp/heindall-bench-body-read.txt | awk '{print $(NF-1)}')
BODY_EXACT_ALLOCS=$(grep 'BenchmarkReadRequestBodyExactSize' /tmp/heindall-bench-body-read.txt | awk '{print $(NF-1)}')
HANDLER_ALLOCS=$(grep 'BenchmarkFraudScoreRawHandler-' /tmp/heindall-bench-handler.txt | awk '{print $(NF-1)}')

if [ -z "$BODY_FAST_ALLOCS" ] || [ -z "$HANDLER_ALLOCS" ]; then
  echo "WARN: could not parse allocation counts; benchmarks may need more time."
else
  echo "      Body read allocs/op (pooled): $BODY_FAST_ALLOCS"
  if [ -n "$BODY_EXACT_ALLOCS" ]; then
    echo "      Body read allocs/op (httptest): $BODY_EXACT_ALLOCS"
  fi
  echo "      Handler allocs/op            : $HANDLER_ALLOCS"
  if [ "$BODY_FAST_ALLOCS" -gt 2 ]; then
    echo "FAIL: pooled body-read fast path regressed above 2 allocs/op"
    exit 1
  fi
fi

echo ""
echo "[5/5] Checking compose-level challenge compliance..."
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

grep -q '^  lb:$' "$COMPOSE_FILE" || { echo "FAIL: missing lb service"; exit 1; }
grep -q '^  api1:$' "$COMPOSE_FILE" || { echo "FAIL: missing api1 service"; exit 1; }
grep -q '^  api2:$' "$COMPOSE_FILE" || { echo "FAIL: missing api2 service"; exit 1; }
grep -q '"9999:9999"' "$COMPOSE_FILE" || { echo "FAIL: missing public port 9999 mapping"; exit 1; }
grep -q 'driver: bridge' "$COMPOSE_FILE" || { echo "FAIL: compose network is not bridge"; exit 1; }
grep -q 'GOMEMLIMIT: "120MiB"' "$COMPOSE_FILE" || { echo "FAIL: missing GOMEMLIMIT"; exit 1; }
grep -q 'GOGC:' "$COMPOSE_FILE" || { echo "FAIL: missing GOGC env wiring"; exit 1; }

echo ""
echo "=== validation complete ==="
