"""
Locust load test for the Product API.
Supports both HttpUser and FastHttpUser via class picker.
"""
import json
import random
from locust import HttpUser, FastHttpUser, task, between

# Shared product payload generator
def make_product(pid):
    return {
        "product_id": pid,
        "sku": f"SKU-{pid:05d}",
        "manufacturer": f"Manufacturer-{pid % 10}",
        "category_id": (pid % 5) + 1,
        "weight": random.randint(100, 5000),
        "some_other_id": pid + 1000,
    }

# Standard HttpUser built on python-requests
class ProductUser(HttpUser):
    wait_time = between(0.1, 0.5)

    def on_start(self):
        for i in range(1, 11):
            self.client.post(
                f"/products/{i}/details",
                json=make_product(i),
                name="/products/[id]/details (seed)",
            )

    @task(3)
    def get_product(self):
        pid = random.randint(1, 20)
        self.client.get(f"/products/{pid}", name="/products/[id]")

    @task(1)
    def post_product(self):
        pid = random.randint(1, 100)
        self.client.post(
            f"/products/{pid}/details",
            json=make_product(pid),
            name="/products/[id]/details",
        )

# FastHttpUser built on geventhttpclient (C-based) for higher throughput
class ProductFastUser(FastHttpUser):
    wait_time = between(0.1, 0.5)

    def on_start(self):
        for i in range(1, 11):
            self.client.post(
                f"/products/{i}/details",
                headers={"Content-Type": "application/json"},
                data=json.dumps(make_product(i)),
                name="/products/[id]/details (seed)",
            )

    @task(3)
    def get_product(self):
        pid = random.randint(1, 20)
        self.client.get(f"/products/{pid}", name="/products/[id]")

    @task(1)
    def post_product(self):
        pid = random.randint(1, 100)
        self.client.post(
            f"/products/{pid}/details",
            headers={"Content-Type": "application/json"},
            data=json.dumps(make_product(pid)),
            name="/products/[id]/details",
        )
