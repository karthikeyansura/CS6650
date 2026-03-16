"""
Locust load tests: Sync vs Async Order Processing.

Test classes are selectable via the --class-picker flag in Locust UI.

Usage:
  Phase 1 (Sync Normal):    5 users,  spawn rate 1/s,  30 seconds
  Phase 1 (Sync Flash):     20 users, spawn rate 10/s, 60 seconds
  Phase 3 (Async Flash):    20 users, spawn rate 10/s, 60 seconds
  Phase 5 (Worker Scaling): 20 users, spawn rate 10/s, 60 seconds (repeat per worker count)
"""

import random
from locust import HttpUser, task, between


def generate_order():
    """Generate a random order payload."""
    num_items = random.randint(1, 5)
    items = [
        {
            "product_id": random.randint(1, 10000),
            "name": f"Product-{random.randint(1, 100)}",
            "quantity": random.randint(1, 3),
            "price": round(random.uniform(9.99, 199.99), 2),
        }
        for _ in range(num_items)
    ]
    return {"customer_id": random.randint(1, 1000), "items": items}


class SyncNormalUser(HttpUser):
    """Phase 1: Normal operations.
    Config: 5 users, spawn rate 1/s, run 30 seconds.
    Expected: 100% success rate, ~0.33 orders/sec throughput.
    """

    wait_time = between(0.1, 0.5)

    @task
    def place_sync_order(self):
        payload = generate_order()
        with self.client.post(
            "/orders/sync",
            json=payload,
            name="/orders/sync",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Sync order failed: {resp.status_code}")

    @task(0)
    def health_check(self):
        self.client.get("/health", name="/health")


class SyncFlashUser(HttpUser):
    """Phase 1: Flash sale with synchronous processing.
    Config: 20 users, spawn rate 10/s, run 60 seconds.
    Purpose: Demonstrates severe latency degradation and Head-of-Line
    blocking under heavy load.
    """

    wait_time = between(0.1, 0.5)

    @task
    def place_sync_order(self):
        payload = generate_order()
        with self.client.post(
            "/orders/sync",
            json=payload,
            name="/orders/sync (flash)",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Sync flash failed: {resp.status_code}")


class AsyncFlashUser(HttpUser):
    """Phase 3: Flash sale with async processing.
    Config: 20 users, spawn rate 10/s, run 60 seconds.
    Expected: 100% acceptance rate, sub-100ms response times.
    """

    wait_time = between(0.1, 0.5)

    @task
    def place_async_order(self):
        payload = generate_order()
        with self.client.post(
            "/orders/async",
            json=payload,
            name="/orders/async (flash)",
            catch_response=True,
        ) as resp:
            if resp.status_code == 202:
                resp.success()
            else:
                resp.failure(f"Async order failed: {resp.status_code}")


class WorkerScalingUser(HttpUser):
    """Phase 5: Worker scaling tests.
    Config: 20 users, spawn rate 10/s, run 60 seconds.
    Run this test multiple times while changing WORKER_COUNT (1, 5, 20, 100)
    on the processor task between runs.
    Monitor SQS ApproximateNumberOfMessagesVisible in CloudWatch.
    """

    wait_time = between(0.1, 0.5)

    @task
    def place_async_order(self):
        payload = generate_order()
        with self.client.post(
            "/orders/async",
            json=payload,
            name="/orders/async (scaling)",
            catch_response=True,
        ) as resp:
            if resp.status_code == 202:
                resp.success()
            else:
                resp.failure(f"Async order failed: {resp.status_code}")
