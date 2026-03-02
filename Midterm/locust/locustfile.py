# Locust load tests for crash & recovery experiment.
# Test classes are selectable via the --class-picker flag in Locust UI.
import random
from locust import FastHttpUser, task, between

# Search terms used across all test scenarios
SEARCH_TERMS = [
    "Alpha", "Beta", "Gamma", "Delta", "Epsilon",
    "Electronics", "Books", "Home", "Garden", "Sports",
    "Clothing", "Toys", "Automotive", "Health", "Food",
    "Product", "Zeta", "Eta", "Theta", "Iota", "Kappa",
]


# Standard load test with 20 users hitting inventory-enriched endpoints.
# Used for all three phases: none, circuit-breaker, bulkhead.
class InventoryLoadUser(FastHttpUser):
    wait_time = between(0.1, 0.5)

    @task(7)
    def search_with_inventory(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
                f"/products/search/inventory?q={term}",
                name="/products/search/inventory",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Search failed: {resp.status_code}")

    @task(2)
    def get_product_inventory(self):
        product_id = random.randint(1, 1000)
        with self.client.get(
                f"/products/{product_id}/inventory",
                name="/products/:id/inventory",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Get failed: {resp.status_code}")

    @task(1)
    def health_check(self):
        with self.client.get("/health", name="/health", catch_response=True) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Health check failed: {resp.status_code}")


# Stress test with 50 users and minimal wait time for bulkhead demonstration.
class StressInventoryUser(FastHttpUser):
    wait_time = between(0.01, 0.1)

    @task(8)
    def search_with_inventory(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
                f"/products/search/inventory?q={term}",
                name="/products/search/inventory",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Search failed: {resp.status_code}")

    @task(2)
    def get_product_inventory(self):
        product_id = random.randint(1, 1000)
        with self.client.get(
                f"/products/{product_id}/inventory",
                name="/products/:id/inventory",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Get failed: {resp.status_code}")


# Baseline: search without inventory dependency for comparison.
class BaselineSearchUser(FastHttpUser):
    wait_time = between(0.1, 0.5)

    @task(9)
    def search_product(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
                f"/products/search?q={term}",
                name="/products/search",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Search failed: {resp.status_code}")

    @task(1)
    def health_check(self):
        with self.client.get("/health", name="/health", catch_response=True) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Health check failed: {resp.status_code}")