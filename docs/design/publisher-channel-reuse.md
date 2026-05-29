# Publisher Channel Reuse — an Optimization the Measurements Rejected

A recurring interview prompt on this project:

> "Opening a new AMQP channel for every publish is wasteful. Why not reuse one
> long-lived channel?"

I tried exactly that. The measurements said no. This document is the record.

## TL;DR

- The publisher originally opens a fresh AMQP channel per `POST /orders`
  publish, enables confirm mode, waits for the broker ack, and closes it.
- I refactored it to reuse **one** mutex-guarded confirm channel per publisher,
  expecting a throughput win from skipping per-message channel setup.
- Under load (50 concurrent users) it **regressed throughput 2.5–4×**, because a
  single channel + one mutex **serializes** every publish behind a synchronous
  broker-confirm round-trip. The per-message design pays setup cost but lets all
  in-flight requests publish in parallel — and that parallelism wins by a wide
  margin.
- I reverted to per-message channels. The lesson is the point: **measure the
  optimization against the real concurrency profile; don't assume.**

## 1. The change under test

Original (`publishOnce` / `PublishRaw`): each call does
`mq.Channel()` → `Confirm(false)` → `PublishWithContext` → wait on a
`NotifyPublish` confirm → `ch.Close()`.

Reused version: the `Publisher` holds one `*amqp.Channel` guarded by a
`sync.Mutex`. Every publish takes the lock, publishes on the shared channel,
waits for its confirm, releases the lock. A single confirm channel can only
track one in-flight publish safely (confirms are sequential), so the lock is
held across the whole publish-and-confirm.

That last sentence is the whole story: **the confirm wait happens inside the
lock.**

## 2. The experiment

Locust, 50 users × 3 min, `--spawn-rate 5`, against the full docker-compose
stack on one Mac, fresh MySQL/RabbitMQ volumes each run, 10 products × 100k
units. Same scenario the README's load test uses, shortened to 3 min per run.

| Build | Throughput | p50 | p95 | p99 | failures |
|---|---|---|---|---|---|
| per-message channel (baseline) | ~1,610 req/s | 46 ms | 83 ms | 120 ms | 0% |
| reused channel (run 1) | ~395 req/s | 110 ms | 200 ms | 370 ms | 0% |
| reused channel (run 2) | ~631 req/s | 72 ms | 120 ms | 180 ms | 0% |
| per-message restored | ~1,824 req/s | 41 ms | 71 ms | 110 ms | 0% |

The reused-channel runs are noisy (run 1 was contended by a concurrent image
build), but **both** sit far below the per-message baseline, and restoring
per-message channels recovered full throughput. The direction is unambiguous.

## 3. Why reuse loses here

`order-service` publishes one command per inbound `POST /orders`. Under 50
concurrent requests, the per-message design has up to 50 publishes in flight on
50 independent channels — the broker-confirm latency overlaps across requests.

The reused design funnels all 50 through one mutex. Request _N+1_ cannot publish
until request _N_ has received its confirm and released the lock. Throughput
collapses to roughly `1 / confirm_latency` instead of
`concurrency / confirm_latency`. The channel-open cost the optimization removed
is real but tiny next to the serialization it introduced.

This is the classic shape of a "local" optimization that ignores the
concurrency profile: it optimizes the single-call path and pessimizes the
contended one.

## 4. What I'd do if reuse were actually needed

A **channel pool** — N confirm channels, handed out round-robin, each with its
own mutex — keeps channel reuse without serializing all publishers behind one
lock. It's the correct shape if channel churn ever shows up as a real
bottleneck.

It is not implemented because per-message channels already sustain ~1,800 req/s
with p95 = 71 ms on this host, and nothing points at channel setup as the
limiter. Building the pool now would be optimizing a cost the measurements don't
show — the same trap, one level up. (See also
[`throughput-and-redis.md`](throughput-and-redis.md) for the same measure-first
stance applied to Redis.)

## 5. Interview talking points

| Prompt | Response |
|---|---|
| "Why open a channel per publish? Isn't that wasteful?" | "It is, slightly — but I measured the alternative. Reusing one confirm channel serializes publishes behind the confirm wait and cut throughput 2.5–4× under 50-way concurrency. Per-message channels publish in parallel and sustained ~1,800 req/s. The setup cost is real but dwarfed by the parallelism." |
| "So how would you reuse channels correctly?" | "A pool of N confirm channels, round-robin, one mutex each — reuse without a single serialization point. I didn't build it: per-message already does ~1,800 req/s here and channel setup isn't the bottleneck, so the pool would optimize a cost the data doesn't show." |
| "What did you learn?" | "Benchmark the optimization against the real concurrency profile before shipping it. A change that's obviously faster on one call was 4× slower under load. I reverted on the data." |
