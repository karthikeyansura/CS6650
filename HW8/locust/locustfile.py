"""
HW8 Locust load test for Shopping Cart API.
Tests create_cart, add_items, and get_cart operations.
Works identically for both MySQL and DynamoDB backends.

Usage:
    cd locust/
    TARGET_HOST=http://<ALB_DNS> docker compose up --scale locust-worker=2
    Open http://localhost:8089
"""

import random
from locust import HttpUser, task, between


class ShoppingCartUser(HttpUser):
    """Simulates a customer using the shopping cart."""

    wait_time = between(0.5, 2.0)
    cart_ids = []

    def on_start(self):
        """Create a cart when user spawns."""
        self._create_cart()

    def _create_cart(self):
        customer_id = random.randint(1, 100_000)
        with self.client.post(
            "/shopping-carts",
            json={"customer_id": customer_id},
            catch_response=True,
            name="POST /shopping-carts"
        ) as resp:
            if resp.status_code == 201:
                cart_id = resp.json().get("shopping_cart_id")
                if cart_id is not None:
                    self.cart_ids.append(cart_id)
                resp.success()
            else:
                resp.failure(f"Create cart failed: {resp.status_code}")

    @task(3)
    def create_cart(self):
        """Create a new shopping cart."""
        self._create_cart()

    @task(5)
    def add_item_to_cart(self):
        """Add a random product to a random cart."""
        if not self.cart_ids:
            self._create_cart()
            return

        cart_id = random.choice(self.cart_ids)
        product_id = random.randint(1, 10_000)
        quantity = random.randint(1, 5)

        with self.client.post(
            f"/shopping-carts/{cart_id}/items",
            json={"product_id": product_id, "quantity": quantity},
            catch_response=True,
            name="POST /shopping-carts/{id}/items"
        ) as resp:
            if resp.status_code == 204:
                resp.success()
            elif resp.status_code == 404:
                # Cart might have been removed; create a new one
                resp.success()
                self._create_cart()
            else:
                resp.failure(f"Add item failed: {resp.status_code}")

    @task(5)
    def get_cart(self):
        """Retrieve an existing cart."""
        if not self.cart_ids:
            self._create_cart()
            return

        cart_id = random.choice(self.cart_ids)
        with self.client.get(
            f"/shopping-carts/{cart_id}",
            catch_response=True,
            name="GET /shopping-carts/{id}"
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            elif resp.status_code == 404:
                resp.success()
            else:
                resp.failure(f"Get cart failed: {resp.status_code}")

    @task(1)
    def health_check(self):
        """Quick health probe."""
        self.client.get("/health", name="GET /health")
