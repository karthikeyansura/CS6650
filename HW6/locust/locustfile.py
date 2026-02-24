# Locust load tests for the Product Search API using FastHttpUser.
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

# Baseline test with 5 concurrent users for 2 minutes to measure normal load behavior.
class SearchBaselineUser(FastHttpUser):
    wait_time = between(0.5, 1.5)

    @task(8)
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
    def search_no_match(self):
        with self.client.get(
                "/products/search?q=xyznonexistent",
                name="/products/search (no match)",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected: {resp.status_code}")

    @task(1)
    def health_check(self):
        with self.client.get("/health", name="/health", catch_response=True) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Health check failed: {resp.status_code}")


# Stress test with 20 concurrent users for 3 minutes to push CPU and observe ECS limits.
class SearchBreakingPointUser(FastHttpUser):
    wait_time = between(0.05, 0.2)

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
    def search_missing_query(self):
        with self.client.get(
                "/products/search",
                name="/products/search (missing q)",
                catch_response=True,
        ) as resp:
            if resp.status_code == 400:
                resp.success()
            else:
                resp.failure(f"Expected 400, got {resp.status_code}")


# Scaled load test with 50 concurrent users for 5 minutes against the ALB endpoint to validate horizontal scaling.
class SearchScaledUser(FastHttpUser):
    wait_time = between(0.05, 0.2)

    @task(8)
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
    def search_no_match(self):
        with self.client.get(
                "/products/search?q=xyznonexistent",
                name="/products/search (no match)",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected: {resp.status_code}")

    @task(1)
    def health_check(self):
        with self.client.get("/health", name="/health", catch_response=True) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Health check failed: {resp.status_code}")


# Spike test with 50+ concurrent users and minimal wait time to simulate sudden traffic surge.
class SearchSpikeUser(FastHttpUser):
    wait_time = between(0.01, 0.1)

    @task
    def search_product(self):
        term = random.choice(SEARCH_TERMS)
        with self.client.get(
                f"/products/search?q={term}",
                name="/products/search (spike)",
                catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Search failed: {resp.status_code}")