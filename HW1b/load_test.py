import requests
import time
import matplotlib.pyplot as plt
import numpy as np

# Your AWS Server URL
EC2_URL = "http://16.148.60.9:8080/albums"

def load_test(url, duration_seconds=30):
    response_times = []
    start_time = time.time()
    end_time = start_time + duration_seconds

    print(f"Starting load test for {duration_seconds} seconds...")
    print(f"Targeting: {url}")

    request_count = 0
    while time.time() < end_time:
        try:
            start_request = time.time()
            # Timeout set to 5s to catch hanging requests
            requests.get(url, timeout=5)
            end_request = time.time()

            # Convert to milliseconds
            response_time = (end_request - start_request) * 1000
            response_times.append(response_time)
            request_count += 1

            # Print progress every 10 requests
            if request_count % 10 == 0:
                print(f"Request {request_count}: {response_time:.2f}ms")

        except Exception as e:
            print(f"Request failed: {e}")

    return response_times

if __name__ == "__main__":
    # Run Test
    times = load_test(EC2_URL)

    if not times:
        print("No successful requests made. Check if server is running.")
        exit()

    print(f"\nTotal Requests: {len(times)}")
    print(f"Average Latency: {np.mean(times):.2f}ms")

    # Plotting
    plt.figure(figsize=(10, 8))

    # 1. Histogram
    plt.subplot(2, 1, 1)
    plt.hist(times, bins=20, color='skyblue', edgecolor='black')
    plt.title('Distribution of Response Times')
    plt.xlabel('Time (ms)')
    plt.ylabel('Count')

    # 2. Scatter Plot
    plt.subplot(2, 1, 2)
    plt.plot(times, 'o', color='orange', alpha=0.5)
    plt.title('Response Times over Sequence')
    plt.xlabel('Request Number')
    plt.ylabel('Time (ms)')

    plt.tight_layout()
    plt.show()
    