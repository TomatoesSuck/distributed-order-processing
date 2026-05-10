#!/usr/bin/env bash
set -euo pipefail

ORDER_URL="${ORDER_URL:-http://localhost:8081}"
INVENTORY_URL="${INVENTORY_URL:-http://localhost:8082}"
PRODUCT_ID="${PRODUCT_ID:-2004}"
USER_ID="${USER_ID:-45}"
STOCK=10
N=50
SAGA_WAIT=20

echo "=== test-concurrent-orders ==="

echo "[1] seeding product ${PRODUCT_ID} qty=${STOCK}..."
curl -s -o /dev/null -X POST "${INVENTORY_URL}/inventory" \
  -H 'Content-Type: application/json' \
  -d "{\"product_id\":${PRODUCT_ID},\"available_qty\":${STOCK}}" || true
curl -sf -o /dev/null -X PUT "${INVENTORY_URL}/inventory/${PRODUCT_ID}" \
  -H 'Content-Type: application/json' \
  -d "{\"available_qty\":${STOCK}}"

ORDERS_FILE=$(mktemp)
trap 'rm -f "${ORDERS_FILE}"' EXIT

post_one() {
  curl -sf -X POST "${ORDER_URL}/orders" \
    -H 'Content-Type: application/json' \
    -d "{\"user_id\":${USER_ID},\"product_id\":${PRODUCT_ID},\"quantity\":1,\"total_amount\":9.99}" \
    | jq -r .order_id
}
export -f post_one
export ORDER_URL PRODUCT_ID USER_ID

echo "[2] firing ${N} concurrent qty=1 orders..."
seq 1 "${N}" | xargs -n1 -P"${N}" -I{} bash -c 'post_one' > "${ORDERS_FILE}"
PLACED=$(grep -c . "${ORDERS_FILE}" || true)
echo "    placed ${PLACED} orders (file: ${ORDERS_FILE})"

if [ "${PLACED}" != "${N}" ]; then
  echo "FAIL: expected ${N} orders placed, got ${PLACED}"
  exit 1
fi

echo "[3] waiting ${SAGA_WAIT}s for sagas to settle..."
sleep "${SAGA_WAIT}"

CONFIRMED=0
FAILED=0
OTHER=0
while IFS= read -r OID; do
  [ -z "${OID}" ] && continue
  S=$(curl -sf "${ORDER_URL}/orders/${OID}" | jq -r .status)
  case "${S}" in
    CONFIRMED) CONFIRMED=$((CONFIRMED+1)) ;;
    FAILED)    FAILED=$((FAILED+1)) ;;
    *)         OTHER=$((OTHER+1)) ;;
  esac
done < "${ORDERS_FILE}"

AVAIL=$(curl -sf "${INVENTORY_URL}/inventory/${PRODUCT_ID}" | jq -r .available_qty)
EXPECTED_FAILED=$((N - STOCK))
echo "    confirmed=${CONFIRMED}  failed=${FAILED}  other=${OTHER}  available_qty=${AVAIL}"

if [ "${CONFIRMED}" = "${STOCK}" ] && [ "${FAILED}" = "${EXPECTED_FAILED}" ] && [ "${AVAIL}" = "0" ] && [ "${OTHER}" = "0" ]; then
  echo "PASS: exactly ${STOCK} CONFIRMED + ${EXPECTED_FAILED} FAILED + available_qty=0"
  exit 0
fi

echo "FAIL: expected ${STOCK} CONFIRMED + ${EXPECTED_FAILED} FAILED + available_qty=0, got C=${CONFIRMED} F=${FAILED} O=${OTHER} avail=${AVAIL}"
exit 1
