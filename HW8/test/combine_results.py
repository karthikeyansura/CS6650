#!/usr/bin/env python3
"""
Combines mysql_test_results.json and dynamodb_test_results.json
into combined_results.json.
Also prints comparison tables in both terminal and markdown formats.
"""

import json
import os
import sys
import statistics

def load_results(filepath):
    with open(filepath) as f:
        return json.load(f)

def compute_stats(results):
    times = sorted([r["response_time"] for r in results if r["success"]])
    if not times:
        return {}
    n = len(times)
    return {
        "count": n,
        "success_rate": 100 * n / len(results),
        "avg": round(statistics.mean(times), 2),
        "p50": round(times[n // 2], 2),
        "p95": round(times[int(n * 0.95)], 2),
        "p99": round(times[int(n * 0.99)], 2),
        "min": round(min(times), 2),
        "max": round(max(times), 2),
    }

def main():
    mysql_file = "mysql_test_results.json"
    dynamo_file = "dynamodb_test_results.json"

    if not os.path.exists(mysql_file):
        print(f"Error: {mysql_file} not found. Run the MySQL test first.")
        sys.exit(1)
    if not os.path.exists(dynamo_file):
        print(f"Error: {dynamo_file} not found. Run the DynamoDB test first.")
        sys.exit(1)

    mysql_data = load_results(mysql_file)
    dynamo_data = load_results(dynamo_file)

    if len(mysql_data) != 150 or len(dynamo_data) != 150:
        print(f"Warning: Expected 150 operations each. Found MySQL:{len(mysql_data)}, DynamoDB:{len(dynamo_data)}")

    ms = compute_stats(mysql_data)
    ds = compute_stats(dynamo_data)

    def winner(metric_key, lower_is_better=True):
        if metric_key not in ms or metric_key not in ds:
            return "N/A", 0
        if ms[metric_key] == ds[metric_key]:
            return "Tie", 0
        if lower_is_better:
            return ("MySQL", ds[metric_key] - ms[metric_key]) if ms[metric_key] < ds[metric_key] else ("DynamoDB", ms[metric_key] - ds[metric_key])
        else:
            return ("MySQL", ms[metric_key] - ds[metric_key]) if ms[metric_key] > ds[metric_key] else ("DynamoDB", ds[metric_key] - ms[metric_key])

    print("\nDATABASE PERFORMANCE COMPARISON")
    header = f"{'Metric':<28} {'MySQL':>10} {'DynamoDB':>10} {'Winner':>10} {'Margin':>10}"
    print(header)
    for label, key, is_lower_better in [
        ("Avg Response Time (ms)", "avg", True),
        ("P50 Response Time (ms)", "p50", True),
        ("P95 Response Time (ms)", "p95", True),
        ("P99 Response Time (ms)", "p99", True),
        ("Success Rate (%)", "success_rate", False),
    ]:
        w, margin = winner(key, is_lower_better)
        print(f"{label:<28} {ms.get(key, 'N/A'):>10} {ds.get(key, 'N/A'):>10} {w:>10} {margin:>9.2f}")
    print(f"{'Total Operations':<28} {len(mysql_data):>10} {len(dynamo_data):>10}")

    print("\nOPERATION-SPECIFIC BREAKDOWN")
    header2 = f"{'Operation':<15} {'MySQL Avg':>12} {'DynamoDB Avg':>14} {'Faster By':>20}"
    print(header2)
    for op in ["create_cart", "add_items", "get_cart"]:
        m_op = [r["response_time"] for r in mysql_data if r["operation"] == op and r["success"]]
        d_op = [r["response_time"] for r in dynamo_data if r["operation"] == op and r["success"]]

        m_avg = sum(m_op)/len(m_op) if m_op else 0
        d_avg = sum(d_op)/len(d_op) if d_op else 0

        if m_avg == 0 or d_avg == 0:
            diff = "N/A"
        elif m_avg < d_avg:
            diff = f"MySQL by {d_avg - m_avg:.2f} ms"
        elif d_avg < m_avg:
            diff = f"DynamoDB by {m_avg - d_avg:.2f} ms"
        else:
            diff = "Tie"

        print(f"{op.upper():<15} {m_avg:>12.2f} {d_avg:>14.2f} {diff:>20}")

    combined = {
        "metadata": {
            "mysql_operations": len(mysql_data),
            "dynamodb_operations": len(dynamo_data)
        },
        "mysql_stats": ms,
        "dynamodb_stats": ds,
        "mysql_raw": mysql_data,
        "dynamodb_raw": dynamo_data
    }

    with open("combined_results.json", "w") as f:
        json.dump(combined, f, indent=2)

    print("\nCombined data saved to: combined_results.json\n")

if __name__ == "__main__":
    main()