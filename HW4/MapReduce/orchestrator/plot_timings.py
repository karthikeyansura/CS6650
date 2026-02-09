import json
import argparse
import matplotlib.pyplot as plt
import numpy as np

def plot_single_run(timings, output_file="mapreduce_timing.png"):
    """Generates a breakdown chart for a single execution."""
    phases = ["Split"]
    times = [timings["split"]]

    # Add individual map times
    for i, t in enumerate(timings["map_individual"]):
        phases.append(f"Map-{i}")
        times.append(t)

    phases.append("Reduce")
    times.append(timings["reduce"])

    plt.figure(figsize=(10, 6))
    bars = plt.barh(phases, times, color="#4c72b0")

    plt.xlabel("Time (seconds)")
    plt.title(f"MapReduce Execution Breakdown (Total: {timings['total']:.2f}s)")

    for bar in bars:
        width = bar.get_width()
        plt.text(width, bar.get_y() + bar.get_height()/2,
                 f'{width:.2f}s', ha='left', va='center')

    plt.tight_layout()
    plt.savefig(output_file)
    print(f"Chart saved to {output_file}")

def plot_comparison(files, labels, output_file="mapreduce_comparison.png"):
    """Generates a comparison chart between multiple execution runs."""
    metrics = {
        "Split": [],
        "Map (Total)": [],
        "Reduce": [],
        "Total": []
    }

    for filename in files:
        with open(filename) as f:
            data = json.load(f)
            metrics["Split"].append(data["split"])
            metrics["Map (Total)"].append(data["map_total"])
            metrics["Reduce"].append(data["reduce"])
            metrics["Total"].append(data["total"])

    x = np.arange(len(files))
    width = 0.2
    fig, ax = plt.subplots(figsize=(10, 6))

    for i, (metric, values) in enumerate(metrics.items()):
        offset = (i - 1.5) * width
        rects = ax.bar(x + offset, values, width, label=metric)

        # Label bars
        for rect in rects:
            height = rect.get_height()
            ax.annotate(f'{height:.1f}',
                        xy=(rect.get_x() + rect.get_width() / 2, height),
                        xytext=(0, 3),
                        textcoords="offset points",
                        ha='center', va='bottom', fontsize=8)

    ax.set_ylabel("Time (seconds)")
    ax.set_title("Performance Comparison")
    ax.set_xticks(x)
    ax.set_xticklabels(labels)
    ax.legend()

    plt.tight_layout()
    plt.savefig(output_file)
    print(f"Comparison chart saved to {output_file}")

def main():
    parser = argparse.ArgumentParser(description="Plot MapReduce timing metrics.")
    parser.add_argument("--files", nargs="+", default=["timings.json"], help="List of JSON timing files to plot")
    parser.add_argument("--labels", nargs="+", help="Labels for the comparison chart (one per file)")

    args = parser.parse_args()

    if len(args.files) == 1:
        with open(args.files[0]) as f:
            plot_single_run(json.load(f))
    else:
        labels = args.labels if args.labels else [f"Run {i+1}" for i in range(len(args.files))]
        plot_comparison(args.files, labels)

if __name__ == "__main__":
    main()