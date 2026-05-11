"""Locust load test for the order-service HTTP API.

Single user behavior: POST /orders with a random product from a pre-seeded pool of 10.
Shape: hold 50 concurrent users for 5 minutes, then stop.

Run from the project root:

    locust -f scripts/loadtest/locustfile.py --host http://localhost:8081 \
        --headless --print-stats --html scripts/loadtest/report.html

The --host overrides the default; point it at the order-service exposed port
(8081 with the bundled docker-compose stack).
"""

import random

from locust import HttpUser, LoadTestShape, constant, task

# Must match the product ids created by scripts/loadtest/seed.sh.
PRODUCT_IDS = list(range(8001, 8011))


class OrderUser(HttpUser):
    # No think time; we want sustained pressure.
    wait_time = constant(0)

    @task
    def place_order(self):
        body = {
            "user_id": random.randint(1, 10_000),
            "product_id": random.choice(PRODUCT_IDS),
            "quantity": 1,
            # 10.00 USD per unit; aligns with seed.sh's mental model.
            "total_amount": 10.0,
        }
        # name= groups all POSTs together in the Locust report.
        self.client.post("/orders", json=body, name="POST /orders")


class ConstantLoadShape(LoadTestShape):
    """Hold 50 users for 5 minutes, then stop."""

    DURATION_SECONDS = 5 * 60
    TARGET_USERS = 50
    SPAWN_RATE = 50  # ramp instantly so we measure steady-state, not warm-up

    def tick(self):
        if self.get_run_time() >= self.DURATION_SECONDS:
            return None
        return (self.TARGET_USERS, self.SPAWN_RATE)
