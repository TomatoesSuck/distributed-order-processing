# Outbox and the Dual-Write Question

A recurring interview prompt on this project:

> "You commit the order to MySQL and then publish to RabbitMQ. What happens if
> the process dies between the two? Isn't that a lost message?"

This document is the long answer.

## TL;DR

- The orchestrator does two writes that are **not in one transaction**: it
  commits a DB state change, then publishes an AMQP command. A crash in the gap
  loses the publish.
- This project **tolerates** that gap deliberately, not accidentally: every
  consumer is idempotent, and `RecoverInProgressSagas` re-publishes the
  step-appropriate command for any non-terminal saga every 30s. The lost
  publish is recovered on the next tick.
- The **transactional outbox** is the cleaner industrial answer — atomic with
  the state write, no recovery-poll latency. It is not implemented here because
  the recovery loop already closes the correctness gap and the workload does not
  need sub-30s recovery. This is the "designed, with a known upgrade path"
  position.

## 1. Where the dual write happens

In `services/order-service/internal/service/saga_orchestrator.go`:

- `StartSaga`: `sagaRepo.Create(...)` (DB commit) → `pub.Publish(ReserveInventoryCmd)`.
- `onInventoryReserved`: `CommitInventoryReserved(...)` (DB) → `pub.Publish(ProcessPaymentCmd)`.
- `onPaymentProcessed`: `CommitPaymentProcessed(...)` (DB) → `pub.Publish(ReleaseInventoryCmd)` on the compensation branch.

In each, the DB transaction commits first; the publish is a separate network
call. The two are not atomic. The same shape exists on the worker side
(inventory/payment commit their side-effect, then publish their event).

## 2. The failure and why it is bounded

If the process crashes (or the broker is briefly unreachable) **after** the DB
commit but **before** the publish confirm, the saga's persisted state has
advanced but no downstream command/event was emitted. Without mitigation the
saga would stall forever.

Three existing mechanisms bound this:

- **Persisted saga state.** `saga_states.current_step` records exactly where the
  saga is, committed atomically with the side-effect. After a crash the truth is
  on disk.
- **Recovery re-publish.** `RecoverInProgressSagas` runs at startup and every
  30s, lists sagas in `IN_PROGRESS` / `COMPENSATING`, and re-publishes the
  command for the current step. A publish lost to a crash is re-emitted.
- **Consumer idempotency.** `inventory_logs (order_id, action)` and
  `processed_events (message_id)` make a re-publish a no-op if the original
  *did* land. So re-publishing blindly is always safe.

Net: the gap costs at most ~30s of extra latency for an unlucky saga, never a
lost order. Correctness holds; only worst-case recovery time is affected.

## 3. What the transactional outbox would change

The outbox pattern removes the gap at the source:

1. In the SAME DB transaction that writes the saga state, insert a row into an
   `outbox` table describing the message to send (exchange, routing key,
   payload, headers).
2. A relay process polls `outbox` for unsent rows, publishes each to RabbitMQ,
   and marks it sent (or deletes it) — at least once.
3. Because the state write and the outbox write commit together, there is no
   window where state advanced but the intent to publish was lost.

The relay still publishes at-least-once, so consumer idempotency is still
required — the outbox moves the guarantee from "recovery eventually re-derives
the command" to "the command's intent was durably recorded the instant the
state changed."

## 4. Why it is not implemented here

- **The recovery loop already provides the correctness guarantee.** The outbox
  would improve *recovery latency* (immediate relay vs. ≤30s poll), not
  correctness. No requirement in this project asks for sub-30s recovery.
- **Cost.** An outbox adds a table per writing service, a relay process (or a
  CDC pipeline like Debezium), and its own monitoring — operational surface this
  workload does not justify.
- **The re-publish is cheap and safe.** Idempotent consumers make the recovery
  loop's blind re-sends free, so the simpler design is not paying a correctness
  tax for its simplicity.

The defensible position: **the gap is real, it is bounded by recovery +
idempotency, and the outbox is the right next step if recovery latency ever
becomes a requirement.**

## 5. Interview talking points

| Prompt | Response |
|---|---|
| "DB commit then publish — lost message on crash?" | "Yes, that gap exists. I bound it with persisted saga state + a 30s recovery loop that re-publishes the current step, and idempotent consumers that make re-sends free. Worst case is ~30s extra latency, never a lost order." |
| "Why not a transactional outbox?" | "It's the cleaner fix and I'd reach for it if we needed sub-30s recovery. Here the recovery loop already gives the correctness guarantee; the outbox would only cut recovery latency, at the cost of an outbox table + relay per service. Not justified by this workload." |
| "Isn't the recovery loop just a worse outbox?" | "It trades immediacy for simplicity. The outbox records publish intent atomically; the recovery loop re-derives it from persisted saga state on a timer. Same at-least-once + idempotency contract; different latency/operational tradeoff." |
| "What stops a re-publish from double-processing?" | "Consumer-side idempotency: `(order_id, action)` for inventory, `processed_events(message_id)` for payment/order. A duplicate is a no-op." |
