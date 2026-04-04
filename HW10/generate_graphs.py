"""
Generate distribution graphs from Locust results.

Usage:
    pip install matplotlib numpy
    python3 generate_graphs.py
    python3 generate_graphs.py --results-dir results --output-dir graphs
"""

import argparse
import csv
import json
import os

import matplotlib.pyplot as plt
import numpy as np


CONFIGS = ["lf_w5r1", "lf_w1r5", "lf_w3r3", "ll"]
RATIOS = ["w01", "w10", "w50", "w90"]
RATIO_LABELS = {"w01": "1%W", "w10": "10%W", "w50": "50%W", "w90": "90%W"}
CONFIG_LABELS = {
    "lf_w5r1": "W5R1",
    "lf_w1r5": "W1R5",
    "lf_w3r3": "W3R3",
    "ll": "Leaderless",
}


def parse_requests_csv(filepath):
    with open(filepath, "r") as f:
        reader = csv.DictReader(f)
        get_row, set_row = None, None
        for row in reader:
            if row["Type"] == "GET":
                get_row = row
            elif row["Type"] == "POST":
                set_row = row
    return get_row, set_row


def plot_latency_bar_chart(config, op_type, results_dir, output_dir):
    fig, ax = plt.subplots(figsize=(8, 4.5))
    x = np.arange(len(RATIOS))
    width = 0.22

    p50s, p95s, p99s, maxs = [], [], [], []
    for ratio in RATIOS:
        filepath = os.path.join(results_dir, config, f"{ratio}_requests.csv")
        get_row, set_row = parse_requests_csv(filepath)
        row = get_row if op_type == "read" else set_row
        if row is None:
            p50s.append(0); p95s.append(0); p99s.append(0); maxs.append(0)
            continue
        p50s.append(float(row["50%"]))
        p95s.append(float(row["95%"]))
        p99s.append(float(row["99%"]))
        maxs.append(float(row["100%"]))

    bars1 = ax.bar(x - 1.5*width, p50s, width, label="p50", color="#2196F3")
    bars2 = ax.bar(x - 0.5*width, p95s, width, label="p95", color="#FF9800")
    bars3 = ax.bar(x + 0.5*width, p99s, width, label="p99", color="#F44336")
    bars4 = ax.bar(x + 1.5*width, maxs, width, label="max (tail)", color="#9C27B0", alpha=0.7)

    for bars in [bars1, bars2, bars3, bars4]:
        for bar in bars:
            h = bar.get_height()
            if h > 0:
                ax.annotate(f"{h:.0f}", xy=(bar.get_x() + bar.get_width()/2, h),
                            xytext=(0, 2), textcoords="offset points",
                            ha="center", va="bottom", fontsize=6)

    op_label = "Read" if op_type == "read" else "Write"
    ax.set_title(f"{CONFIG_LABELS[config]}: {op_label} Latency Distribution (ms)")
    ax.set_xlabel("Write Ratio")
    ax.set_ylabel("Latency (ms)")
    ax.set_xticks(x)
    ax.set_xticklabels([RATIO_LABELS[r] for r in RATIOS])
    ax.legend(fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    plt.tight_layout()
    out = os.path.join(output_dir, f"{config}_{op_type}_latency_dist.png")
    plt.savefig(out, dpi=150)
    plt.close()
    print(f"  {out}")


def plot_rw_interval_histogram(config, results_dir, output_dir):
    fig, axes = plt.subplots(2, 2, figsize=(10, 7))
    axes = axes.flatten()

    for i, ratio in enumerate(RATIOS):
        filepath = os.path.join(results_dir, config, f"{ratio}_metrics.json")
        with open(filepath) as f:
            data = json.load(f)
        intervals = data.get("rw_intervals_ms_sample", [])
        ax = axes[i]
        if len(intervals) > 0:
            ax.hist(intervals, bins=40, color="#4CAF50", edgecolor="black", alpha=0.7)
            ax.axvline(np.median(intervals), color="red", linestyle="--",
                       label=f"median={np.median(intervals):.0f}ms")
            ax.legend(fontsize=8)
        else:
            ax.text(0.5, 0.5, "No data", transform=ax.transAxes, ha="center")
        stale = data.get("stale_reads", 0)
        total = data.get("total_versioned_reads", 0)
        ax.set_title(f"{RATIO_LABELS[ratio]} (stale: {stale}/{total})", fontsize=9)
        ax.set_xlabel("Interval since last write (ms)", fontsize=8)
        ax.set_ylabel("Count", fontsize=8)

    fig.suptitle(f"{CONFIG_LABELS[config]}: Read-After-Write Interval Distribution")
    plt.tight_layout()
    out = os.path.join(output_dir, f"{config}_rw_interval_dist.png")
    plt.savefig(out, dpi=150)
    plt.close()
    print(f"  {out}")


def plot_stale_reads_summary(results_dir, output_dir):
    fig, ax = plt.subplots(figsize=(8, 4.5))
    x = np.arange(len(RATIOS))
    width = 0.2
    for i, config in enumerate(CONFIGS):
        stales = []
        for ratio in RATIOS:
            with open(os.path.join(results_dir, config, f"{ratio}_metrics.json")) as f:
                stales.append(json.load(f).get("stale_reads", 0))
        ax.bar(x + i*width, stales, width, label=CONFIG_LABELS[config])
    ax.set_title("Stale Reads by Configuration and Write Ratio")
    ax.set_xlabel("Write Ratio")
    ax.set_ylabel("Stale Read Count")
    ax.set_xticks(x + 1.5*width)
    ax.set_xticklabels([RATIO_LABELS[r] for r in RATIOS])
    ax.legend(fontsize=8)
    ax.grid(axis="y", alpha=0.3)
    plt.tight_layout()
    out = os.path.join(output_dir, "stale_reads_summary.png")
    plt.savefig(out, dpi=150)
    plt.close()
    print(f"  {out}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--results-dir", default="results")
    parser.add_argument("--output-dir", default="graphs")
    args = parser.parse_args()
    os.makedirs(args.output_dir, exist_ok=True)

    print("Generating graphs...")
    for config in CONFIGS:
        plot_latency_bar_chart(config, "read", args.results_dir, args.output_dir)
        plot_latency_bar_chart(config, "write", args.results_dir, args.output_dir)
        plot_rw_interval_histogram(config, args.results_dir, args.output_dir)
    plot_stale_reads_summary(args.results_dir, args.output_dir)
    print(f"\nDone. Output: {args.output_dir}/")


if __name__ == "__main__":
    main()