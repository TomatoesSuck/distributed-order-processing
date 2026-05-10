#!/usr/bin/env bash
set -euo pipefail

ORDER_URL="${ORDER_URL:-http://localhost:8081}"
INVENTORY_URL="${INVENTORY_URL:-http://localhost:8082}"
PRODUCT_ID="${PRODUCT_ID:-2001}"
USER_ID="${USER_ID:-42}"
STOCK=5
QUANTITY=10
WAIT_SECS=5

echo "=== test-insufficient-stock ==="

echo "[1] seeding product ${PRODUCT_ID} qty=${STOCK}..."
curl -s -o /dev/null -X POST "${INVENTORY_URL}/inventory" \
  -H 'Content-Type: application/json' \
  -d "{\"product_id\":${PRODUCT_ID},\"available_qty\":${STOCK}}" || true
curl -sf -o /dev/null -X PUT "${INVENTORY_URL}/inventory/${PRODUCT_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"available_qty\":${STOCK}}"

echo "[2] placing order qty=${QUANTITY} (exceeds stock)..."
ORDER_RESP=$(curl -sf -X POST "${ORDER_URL}/orders" \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":${USER_ID},\"product_id\":${PRODUCT_ID},\"quantity\":${QUANTITY},\"total_amount\":99.99}")
ORDER_ID=$(echo "${ORDER_RESP}" | jq -r .order_id)
echo "    order_id=${ORDER_ID}"

echo "[3] waiting ${WAIT_SECS}s for saga..."
sleep "${WAIT_SECS}"

ORDER_JSON=$(curl -sf "${ORDER_URL}/orders/${ORDER_ID}")
STATUS=$(echo "${ORDER_JSON}" | jq -r .status)
echo "    order status=${STATUS}"

if [ "${STATUS}" = "FAILED" ]; then
  echo "PASS: order ${ORDER_ID} status=FAILED (insufficient stock, no compensation needed)"
  exit 0
fi

echo "FAIL: expected FAILED, got ${STATUS}"
exit 1
