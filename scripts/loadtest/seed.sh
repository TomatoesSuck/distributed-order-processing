#!/usr/bin/env bash
# Seed 10 products (8001..8010) with 100,000 units each for load testing.
# Uses the inventory-service HTTP API so the call works regardless of where
# the service runs (local docker-compose, ECS, etc).

set -euo pipefail

INVENTORY_URL="${INVENTORY_URL:-http://localhost:8082}"
STOCK="${STOCK:-100000}"

echo "Seeding products 8001..8010 with ${STOCK} units each at ${INVENTORY_URL}..."

for pid in $(seq 8001 8010); do
  # POST first — succeeds for new product, harmless 4xx if it already exists.
  curl -s -o /dev/null -X POST "${INVENTORY_URL}/inventory" \
    -H 'Content-Type: application/json' \
    -d "{\"product_id\":${pid},\"available_qty\":${STOCK}}" || true
  # PUT to force-set the qty (reset for repeated runs).
  curl -sf -o /dev/null -X PUT "${INVENTORY_URL}/inventory/${pid}" \
    -H 'Content-Type: application/json' \
    -d "{\"available_qty\":${STOCK}}"
  echo "  product ${pid}: ${STOCK} units"
done

echo "Seeding done."
