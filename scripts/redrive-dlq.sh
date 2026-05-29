#!/usr/bin/env bash
# Redrive (re-queue) all messages from a dead-letter queue back to the saga
# command/event exchange under their original routing key, via the RabbitMQ
# HTTP management API. Local default targets the docker-compose broker.
#
# Usage:   bash scripts/redrive-dlq.sh <dlq-name> <target-exchange>
# Example: bash scripts/redrive-dlq.sh inventory.commands.dlq saga.commands
#
# Env overrides (for AWS / Amazon MQ): RABBITMQ_MGMT_URL, RABBITMQ_USER,
# RABBITMQ_PASS, RABBITMQ_VHOST.
set -euo pipefail

DLQ="${1:?usage: redrive-dlq.sh <dlq-name> <target-exchange>}"
EXCHANGE="${2:?usage: redrive-dlq.sh <dlq-name> <target-exchange>}"
MGMT="${RABBITMQ_MGMT_URL:-http://localhost:15672}"
RUSER="${RABBITMQ_USER:-guest}"
PASS="${RABBITMQ_PASS:-guest}"
VHOST="${RABBITMQ_VHOST:-%2F}"

auth=(-u "$RUSER:$PASS")
moved=0

while :; do
  batch="$(curl -fsS "${auth[@]}" -H 'content-type: application/json' \
    -d "{\"count\":50,\"ackmode\":\"ack_requeue_false\",\"encoding\":\"auto\"}" \
    "$MGMT/api/queues/$VHOST/$DLQ/get")"

  n="$(echo "$batch" | jq 'length')"
  [ "$n" -eq 0 ] && break

  echo "$batch" | jq -c '.[]' | while read -r msg; do
    rk="$(echo "$msg" | jq -r '.routing_key')"
    payload="$(echo "$msg" | jq -r '.payload')"
    curl -fsS "${auth[@]}" -H 'content-type: application/json' \
      -d "$(jq -nc --arg rk "$rk" --arg p "$payload" \
            '{properties:{delivery_mode:2},routing_key:$rk,payload:$p,payload_encoding:"string"}')" \
      "$MGMT/api/exchanges/$VHOST/$EXCHANGE/publish" >/dev/null
  done

  moved=$((moved + n))
  echo "redriven $moved so far..."
done

echo "PASS: redriven $moved message(s) from $DLQ -> $EXCHANGE"
