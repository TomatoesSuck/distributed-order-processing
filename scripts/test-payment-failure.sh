#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "${SCRIPT_DIR}/.." && pwd )"
COMPOSE_DIR="${PROJECT_ROOT}/deploy"

ORDER_URL="${ORDER_URL:-http://localhost:8081}"
INVENTORY_URL="${INVENTORY_URL:-http://localhost:8082}"
PRODUCT_ID="${PRODUCT_ID:-2002}"
USER_ID="${USER_ID:-43}"
STOCK=10
QUANTITY=2
RESTART_WAIT=8
SAGA_WAIT=10

echo "=== test-payment-failure ==="

restore_payment() {
  echo "[cleanup] restoring payment-service with PAYMENT_FAILURE_RATE=0.0..."
  ( cd "${COMPOSE_DIR}" && PAYMENT_FAILURE_RATE=0.0 docker compose up -d --force-recreate payment-service > /dev/null )
}
trap restore_payment EXIT

echo "[1] restarting payment-service with PAYMENT_FAILURE_RATE=1.0..."
( cd "${COMPOSE_DIR}" && PAYMENT_FAILURE_RATE=1.0 docker compose up -d --force-recreate payment-service > /dev/null )
echo "    waiting ${RESTART_WAIT}s for payment-service to come up..."
sleep "${RESTART_WAIT}"

echo "[2] seeding product ${PRODUCT_ID} qty=${STOCK}..."
curl -s -o /dev/null -X POST "${INVENTORY_URL}/inventory" \
  -H 'Content-Type: application/json' \
  -d "{\"product_id\":${PRODUCT_ID},\"available_qty\":${STOCK}}" || true
curl -sf -o /dev/null -X PUT "${INVENTORY_URL}/inventory/${PRODUCT_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"available_qty\":${STOCK}}"
INV_BEFORE=$(curl -sf "${INVENTORY_URL}/inventory/${PRODUCT_ID}" | jq -r .available_qty)
echo "    available_qty before=${INV_BEFORE}"

echo "[3] placing order qty=${QUANTITY}..."
ORDER_RESP=$(curl -sf -X POST "${ORDER_URL}/orders" \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":${USER_ID},\"product_id\":${PRODUCT_ID},\"quantity\":${QUANTITY},\"total_amount\":50.00}")
ORDER_ID=$(echo "${ORDER_RESP}" | jq -r .order_id)
echo "    order_id=${ORDER_ID}"

echo "[4] waiting ${SAGA_WAIT}s for compensation..."
sleep "${SAGA_WAIT}"

STATUS=$(curl -sf "${ORDER_URL}/orders/${ORDER_ID}" | jq -r .status)
INV_AFTER=$(curl -sf "${INVENTORY_URL}/inventory/${PRODUCT_ID}" | jq -r .available_qty)
echo "    order status=${STATUS}  available_qty after=${INV_AFTER}"

if [ "${STATUS}" != "COMPENSATED" ]; then
  echo "FAIL: expected order status COMPENSATED, got ${STATUS}"
  exit 1
fi
if [ "${INV_AFTER}" != "${INV_BEFORE}" ]; then
  echo "FAIL: inventory not restored: before=${INV_BEFORE} after=${INV_AFTER}"
  exit 1
fi

echo "PASS: order ${ORDER_ID} compensated, inventory restored to ${INV_AFTER}"
