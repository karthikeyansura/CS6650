import requests
import time
import json
import sys

# Configuration
BUCKET = "cs6650-mapreduce-wordcount"
KEY = "hamlet.txt"

# Service Endpoints
SPLITTER = "http://44.204.243.184:8080"
MAPPERS = [
    "http://54.209.43.138:8080",
    "http://13.223.234.27:8080",
    "http://44.199.213.207:8080"
]
REDUCER = "http://44.192.120.241:8080"

def call_service(name, url, timeout=120):
    """
    Makes an HTTP GET request to the specified URL and returns the JSON response
    along with the elapsed time. Exits the program on failure.
    """
    print(f"[{name}] Requesting: {url}")
    start_time = time.time()

    try:
        response = requests.get(url, timeout=timeout)
        elapsed_time = time.time() - start_time

        response.raise_for_status()
        data = response.json()

        print(f"[{name}] Success: {response.status_code} ({elapsed_time:.3f}s)")
        return data, elapsed_time

    except requests.exceptions.RequestException as e:
        elapsed_time = time.time() - start_time
        print(f"[{name}] Failed after {elapsed_time:.3f}s: {e}")
        sys.exit(1)

def run_pipeline():
    timings = {}
    pipeline_start = time.time()

    # Phase 1: Split
    # The splitter divides the input file into chunks based on the number of mappers
    print("\nPhase 1: Split")
    split_url = f"{SPLITTER}/split?bucket={BUCKET}&key={KEY}&n={len(MAPPERS)}"
    split_data, split_time = call_service("SPLITTER", split_url)

    timings["split"] = split_time
    chunks = split_data.get("chunks", [])
    print(f"Chunks created: {chunks}")

    # Phase 2: Map
    # Each chunk is processed by a mapper in a round-robin fashion
    print("\nPhase 2: Map")
    map_results = []
    map_times = []

    for i, chunk in enumerate(chunks):
        mapper_url = MAPPERS[i % len(MAPPERS)]
        map_url = f"{mapper_url}/map?bucket={BUCKET}&key={chunk}&id={i}"

        map_data, map_time = call_service(f"MAPPER-{i}", map_url)

        map_results.append(map_data.get("result"))
        map_times.append(map_time)

        total_words = map_data.get("total_words", 0)
        unique_words = map_data.get("unique_words", 0)
        print(f"Mapper-{i} stats: {total_words} words, {unique_words} unique")

    timings["map_individual"] = map_times
    timings["map_total"] = sum(map_times)

    # Phase 3: Reduce
    # The reducer aggregates all intermediate map results
    print("\nPhase 3: Reduce")
    keys_param = ",".join(map_results)
    reduce_url = f"{REDUCER}/reduce?bucket={BUCKET}&keys={keys_param}"

    reduce_data, reduce_time = call_service("REDUCER", reduce_url)
    timings["reduce"] = reduce_time

    # Pipeline Completion
    total_duration = time.time() - pipeline_start
    timings["total"] = total_duration

    # Output Summary
    print("\nResults")
    print(f"Total Unique Words: {reduce_data.get('total_unique_words')}")
    print(f"Total Words:        {reduce_data.get('total_words')}")
    print(f"Output Location:    s3://{BUCKET}/{reduce_data.get('result')}")

    print("\nTiming Analysis")
    print(f"Split Phase:   {timings['split']:.3f}s")
    print(f"Map Phase:     {timings['map_total']:.3f}s (cumulative)")
    print(f"Reduce Phase:  {timings['reduce']:.3f}s")
    print(f"Total time: {total_duration:.3f}s")

    if "top_25" in reduce_data:
        print("\nMost Frequent Words")
        for item in reduce_data["top_25"][:5]:
            print(f"{item['word']}: {item['count']}")

    # Save metrics
    with open("timings.json", "w") as f:
        json.dump(timings, f, indent=4)
    print("\nMetrics saved to timings.json")

if __name__ == "__main__":
    run_pipeline()