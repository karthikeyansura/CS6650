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
                name="/products/[id]/details (data seeding)",
            )

    @task(6)
    def get_product(self):
        pid = random.randint(1, 20)
        with self.client.get(f"/products/{pid}", name="/products/[id]", catch_response=True) as response:
            if response.status_code in [200, 404]:
                response.success()
            else:
                response.failure(f"Unexpected GET error: {response.status_code}")

    @task(2)
    def post_product(self):
        pid = random.randint(1, 100)
        with self.client.post(
                f"/products/{pid}/details",
                json=make_product(pid),
                name="/products/[id]/details",
                catch_response=True
        ) as response:
            if response.status_code == 204:
                response.success()
            else:
                response.failure(f"Unexpected POST error: {response.status_code}")

    @task(1)
    def get_invalid_id(self):
        with self.client.get("/products/abc", name="/products/[id] (invalid ID)", catch_response=True) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400 for invalid ID, got {response.status_code}")

    @task(1)
    def post_missing_fields(self):
        pid = random.randint(101, 150)
        invalid_payload = make_product(pid)
        del invalid_payload["sku"]

        with self.client.post(
                f"/products/{pid}/details",
                json=invalid_payload,
                name="/products/[id]/details (missing field(s))",
                catch_response=True
        ) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400 for missing field(s), got {response.status_code}")

    @task(1)
    def post_mismatched_id(self):
        mismatched_payload = make_product(888)

        with self.client.post(
                "/products/999/details",
                json=mismatched_payload,
                name="/products/[id]/details (mismatched ID)",
                catch_response=True
        ) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400 for mismatched ID, got {response.status_code}")

# FastHttpUser built on geventhttpclient (C-based) for higher throughput
class ProductFastUser(FastHttpUser):
    wait_time = between(0.1, 0.5)

    def on_start(self):
        for i in range(1, 11):
            self.client.post(
                f"/products/{i}/details",
                headers={"Content-Type": "application/json"},
                data=json.dumps(make_product(i)),
                name="/products/[id]/details (data seeding)",
            )

    @task(6)
    def get_product(self):
        pid = random.randint(1, 20)
        with self.client.get(f"/products/{pid}", name="/products/[id]", catch_response=True) as response:
            if response.status_code in [200, 404]:
                response.success()
            else:
                response.failure(f"Unexpected GET error: {response.status_code}")

    @task(2)
    def post_product(self):
        pid = random.randint(1, 100)
        with self.client.post(
                f"/products/{pid}/details",
                headers={"Content-Type": "application/json"},
                data=json.dumps(make_product(pid)),
                name="/products/[id]/details",
                catch_response=True
        ) as response:
            if response.status_code == 204:
                response.success()
            else:
                response.failure(f"Unexpected POST error: {response.status_code}")

    @task(1)
    def get_invalid_id(self):
        with self.client.get("/products/abc", name="/products/[id] (invalid ID)", catch_response=True) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400 for invalid ID, got {response.status_code}")

    @task(1)
    def post_missing_fields(self):
        pid = random.randint(101, 150)
        invalid_payload = make_product(pid)
        del invalid_payload["sku"]

        with self.client.post(
                f"/products/{pid}/details",
                headers={"Content-Type": "application/json"},
                data=json.dumps(invalid_payload),
                name="/products/[id]/details (missing field(s))",
                catch_response=True
        ) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400 for missing field(s), got {response.status_code}")

    @task(1)
    def post_mismatched_id(self):
        mismatched_payload = make_product(888)

        with self.client.post(
                "/products/999/details",
                headers={"Content-Type": "application/json"},
                data=json.dumps(mismatched_payload),
                name="/products/[id]/details (mismatched ID)",
                catch_response=True
        ) as response:
            if response.status_code == 400:
                response.success()
            else:
                response.failure(f"Expected 400 for mismatched ID, got {response.status_code}")