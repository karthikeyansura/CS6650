#!/usr/bin/env python3
"""
Eventual Consistency Investigation for DynamoDB.
Tests read-after-write behavior and measures consistency delays.

Usage:
    python3 test_consistency.py <ALB_URL>
"""

import sys
import time
import json
import requests
from datetime import datetime, timezone

def test_read_after_write(base_url, trials=20):
    """Create a cart, then immediately read it back. Measure staleness."""
    results = []
    stale_count = 0

    print(f"\n[Test 1] Read-After-Write Consistency ({trials} trials)")

    for i in range(trials):
        # Create cart
        resp = requests.post(f"{base_url}/shopping-carts",
                             json={"customer_id": 9000 + i}, timeout=10)
        if resp.status_code != 201:
            print(f"Trial {i+1}: create failed ({resp.status_code})")
            continue
        cart_id = resp.json()["shopping_cart_id"]

        # Immediately read it back
        start = time.time()
        get_resp = requests.get(f"{base_url}/shopping-carts/{cart_id}", timeout=10)
        read_time = (time.time() - start) * 1000

        if get_resp.status_code == 200:
            results.append({"trial": i+1, "found": True, "read_time_ms": round(read_time, 2)})
        elif get_resp.status_code == 404:
            stale_count += 1
            results.append({"trial": i+1, "found": False, "read_time_ms": round(read_time, 2)})
            print(f"Trial {i+1}: STALE READ (404 Not Found)")
        else:
            print(f"Trial {i+1}: unexpected read error ({get_resp.status_code})")

    avg_read = sum(r["read_time_ms"] for r in results) / len(results) if results else 0
    print(f"Result: {stale_count} stale reads out of {len(results)} successful creates.")
    print(f"Average read latency: {avg_read:.2f} ms")
    return {"stale_reads": stale_count, "total_trials": len(results), "avg_read_ms": avg_read}

def test_add_then_read(base_url, trials=20):
    """Add an item to an existing cart, immediately read, check if item is there."""
    print(f"\n[Test 2] Add-Then-Read Consistency ({trials} trials)")

    # Setup: Create one cart
    resp = requests.post(f"{base_url}/shopping-carts", json={"customer_id": 9999}, timeout=10)
    if resp.status_code != 201:
        print("Setup failed: could not create cart")
        return {}
    cart_id = resp.json()["shopping_cart_id"]

    stale_count = 0
    for i in range(trials):
        product_id = 1000 + i
        # Add item
        add_resp = requests.post(f"{base_url}/shopping-carts/{cart_id}/items",
                                 json={"product_id": product_id, "quantity": 1}, timeout=10)
        if add_resp.status_code != 204:
            print(f"Trial {i+1}: add failed ({add_resp.status_code})")
            continue

        # Immediately read
        get_resp = requests.get(f"{base_url}/shopping-carts/{cart_id}", timeout=10)
        if get_resp.status_code == 200:
            items = get_resp.json().get("items", [])
            # Check if our product_id is in the list
            found = any(item.get("product_id") == product_id for item in items)
            if not found:
                stale_count += 1
                print(f"Trial {i+1}: STALE READ (Item {product_id} not in cart yet)")
        else:
            print(f"Trial {i+1}: read failed ({get_resp.status_code})")

    print(f"Result: {stale_count} stale item reads out of {trials} attempts.")
    return {"stale_reads": stale_count, "total_trials": trials}

def test_rapid_updates(base_url, updates=30):
    """Blast a single cart with multiple additions, then read."""
    print(f"\n[Test 3] Rapid Updates Consistency (1 cart, {updates} items)")

    # Setup
    resp = requests.post(f"{base_url}/shopping-carts", json={"customer_id": 8888}, timeout=10)
    cart_id = resp.json()["shopping_cart_id"]

    # Rapidly add different products
    for i in range(updates):
        requests.post(f"{base_url}/shopping-carts/{cart_id}/items",
                      json={"product_id": 6000 + i, "quantity": 1}, timeout=10)

    # Small delay then read
    time.sleep(0.5)
    get_resp = requests.get(f"{base_url}/shopping-carts/{cart_id}", timeout=10)
    if get_resp.status_code == 200:
        items = get_resp.json().get("items", [])
        print(f"Expected items: {updates}")
        print(f"Actual items:   {len(items)}")
        print(f"All present:    {len(items) == updates}")
        return {"expected": updates, "actual": len(items), "all_present": len(items) == updates}
    return {"error": get_resp.status_code}

def main():
    if len(sys.argv) != 2:
        print("Usage: python3 test_consistency.py <ALB_URL>")
        sys.exit(1)

    base_url = sys.argv[1].rstrip("/")
    print("\nDynamoDB Eventual Consistency Investigation")
    print(f"Target: {base_url}")

    r1 = test_read_after_write(base_url)
    r2 = test_add_then_read(base_url)
    r3 = test_rapid_updates(base_url)

    report = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "read_after_write": r1,
        "add_then_read": r2,
        "rapid_updates": r3
    }

    with open("consistency_test_results.json", "w") as f:
        json.dump(report, f, indent=2)
    print("\nResults saved to: consistency_test_results.json\n")

if __name__ == "__main__":
    main()