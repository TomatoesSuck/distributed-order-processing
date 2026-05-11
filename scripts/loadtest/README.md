# Locust load test

Drives `POST /orders` against a running order-service stack at a constant 50
concurrent users for 5 minutes.

## Prereqs

- The full stack must be up (order, inventory, payment, MySQL × 3, RabbitMQ).
  From the repo root:
  ```bash
  make up
  ```
- Python 3.10+ and Locust:
  ```bash
  pip install 'locust>=2.31'
  ```

## Run

```bash
# 1) Seed the inventory pool (10 products × 100,000 units each).
bash scripts/loadtest/seed.sh

# 2) Run Locust headless for the full 5-minute shape.
locust -f scripts/loadtest/locustfile.py \
       --host http://localhost:8081 \
       --headless \
       --print-stats \
       --html scripts/loadtest/report.html
```

Locust exits when the shape (`ConstantLoadShape`) returns `None` after 5 min.
Open `scripts/loadtest/report.html` for the full chart (RPS, p50/p95/p99,
error rate).

## Tunables

- `STOCK=200000 bash scripts/loadtest/seed.sh` — adjust seeded quantity.
- `INVENTORY_URL=http://other:8082 bash scripts/loadtest/seed.sh` — seed a
  remote inventory-service.
- Edit `TARGET_USERS` / `DURATION_SECONDS` in `locustfile.py` for other shapes.

## What's measured

Each `POST /orders` returns the moment the order row + saga state are
persisted and the first command is published — *not* when the saga completes.
The Locust report therefore measures synchronous HTTP throughput of the order
service. Saga completion happens asynchronously via RabbitMQ; assertions
about end-to-end consistency live in `tests/integration/`.
