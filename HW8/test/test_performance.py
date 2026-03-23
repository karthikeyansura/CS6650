#!/usr/bin/env python3
"""
Performance Test: 150 operations (50 create, 50 add items, 50 get cart).
Usage:
    python3 test_performance.py <ALB_URL> <mysql|dynamodb>

Outputs:
    mysql_test_results.json    or
    dynamodb_test_results.json
"""

import sys
import json
import time
import requests
from datetime import datetime, timezone

def run_test(base_url: str, db_mode: str):
    base_url = base_url.rstrip("/")
    results = []
    cart_ids = []

    print(f"\nPerformance Test — {db_mode.upper()} mode")
    print(f"Target: {base_url}\n")

    # Phase 1: Create 50 carts
    print("[1/3] Creating 50 shopping carts...")
    for i in range(50):
        payload = {"customer_id": 1000 + i}
        start = time.time()
        try:
            resp = requests.post(f"{base_url}/shopping-carts", json=payload, timeout=10)
            elapsed = (time.time() - start) * 1000  # ms
            success = resp.status_code == 201
            if success:
                body = resp.json()
                cart_ids.append(body.get("shopping_cart_id"))
            results.append({
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": success,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
        except Exception as e:
            results.append({
                "operation": "create_cart",
                "response_time": 0.0,
                "success": False,
                "status_code": 0,
                "error": str(e),
                "timestamp": datetime.now(timezone.utc).isoformat()
            })

    if not cart_ids:
        print("Fatal: Failed to create any carts. Aborting test.")
        sys.exit(1)

    # Phase 2: Add items to the 50 carts
    print("[2/3] Adding items to carts...")
    for cart_id in cart_ids:
        payload = {"product_id": 5001, "quantity": 2}
        start = time.time()
        try:
            resp = requests.post(f"{base_url}/shopping-carts/{cart_id}/items", json=payload, timeout=10)
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 204,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
        except Exception as e:
            results.append({
                "operation": "add_items",
                "response_time": 0.0,
                "success": False,
                "status_code": 0,
                "error": str(e),
                "timestamp": datetime.now(timezone.utc).isoformat()
            })

    # Phase 3: Get the 50 carts
    print("[3/3] Retrieving carts...")
    for cart_id in cart_ids:
        start = time.time()
        try:
            resp = requests.get(f"{base_url}/shopping-carts/{cart_id}", timeout=10)
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 200,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).isoformat()
            })
        except Exception as e:
            results.append({
                "operation": "get_cart",
                "response_time": 0.0,
                "success": False,
                "status_code": 0,
                "error": str(e),
                "timestamp": datetime.now(timezone.utc).isoformat()
            })

    output_file = f"{db_mode.lower()}_test_results.json"
    with open(output_file, "w") as f:
        json.dump(results, f, indent=2)

    total = len(results)
    successes = sum(1 for r in results if r["success"])
    times = [r["response_time"] for r in results if r["success"]]

    print(f"\n{'Total operations':<17}: {total}")
    print(f"{'Successful':<17}: {successes} ({100*successes/total:.1f}%)")
    if times:
        times.sort()
        avg = sum(times) / len(times)
        p50 = times[len(times) // 2]
        p95 = times[int(len(times) * 0.95)]
        p99 = times[int(len(times) * 0.99)]
        print(f"{'Avg response time':<17}: {avg:.2f} ms")
        print(f"{'P50':<17}: {p50:.2f} ms")
        print(f"{'P95':<17}: {p95:.2f} ms")
        print(f"{'P99':<17}: {p99:.2f} ms")

    print("")
    for op in ["create_cart", "add_items", "get_cart"]:
        op_times = [r["response_time"] for r in results if r["operation"] == op and r["success"]]
        if op_times:
            op_avg = sum(op_times)/len(op_times)
            print(f"{op:<17}: avg={op_avg:.2f} ms (n={len(op_times)})")

    print(f"\nResults saved to: {output_file}\n")

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python3 test_performance.py <ALB_URL> <mysql|dynamodb>")
        sys.exit(1)

    url = sys.argv[1]
    mode = sys.argv[2].lower()
    if mode not in ["mysql", "dynamodb"]:
        print("Mode must be 'mysql' or 'dynamodb'")
        sys.exit(1)

    run_test(url, mode)