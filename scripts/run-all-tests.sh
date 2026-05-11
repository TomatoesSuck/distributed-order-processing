#!/usr/bin/env bash
# Run unit tests for every service + the integration suite. Exit 1 if any fail.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

step() {
  printf '\n=== %s ===\n' "$1"
}

step "unit: order-service"
( cd "${ROOT}/services/order-service" && go test -cover ./... )

step "unit: inventory-service"
( cd "${ROOT}/services/inventory-service" && go test -cover ./... )

step "unit: payment-service"
( cd "${ROOT}/services/payment-service" && go test -cover ./... )

step "integration"
( cd "${ROOT}/tests/integration" && go test -tags=integration -timeout=10m ./... )

step "ALL TESTS PASSED"
