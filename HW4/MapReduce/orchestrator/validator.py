import json
import re
import sys
from collections import Counter
import boto3

# Configuration
BUCKET = "cs6650-mapreduce-wordcount"
INPUT_KEY = "hamlet.txt"
RESULT_KEY = "final_result.json"

def get_local_word_counts(text):
    """
    Parses text and counts words using logic consistent with the Mapper service.
    """
    # Regex splits on any character that is not a letter or number
    words = re.findall(r'[a-zA-Z0-9]+', text.lower())
    return Counter(words)

def verify_results():
    s3 = boto3.client("s3", region_name="us-east-1")

    # Fetch Input Data
    print(f"Fetching input: s3://{BUCKET}/{INPUT_KEY}")
    try:
        input_obj = s3.get_object(Bucket=BUCKET, Key=INPUT_KEY)
        input_text = input_obj["Body"].read().decode("utf-8")
    except Exception as e:
        print(f"Failed to fetch input file: {e}")
        sys.exit(1)

    # Perform Local Counting
    print("Performing local word count...")
    local_counts = get_local_word_counts(input_text)
    local_total = sum(local_counts.values())

    # Fetch MapReduce Result
    print(f"Fetching result: s3://{BUCKET}/{RESULT_KEY}")
    try:
        result_obj = s3.get_object(Bucket=BUCKET, Key=RESULT_KEY)
        mr_data = json.loads(result_obj["Body"].read().decode("utf-8"))
        mr_counts = mr_data.get("all_counts", mr_data)
    except Exception as e:
        print(f"Failed to fetch result file: {e}")
        sys.exit(1)

    mr_total = sum(mr_counts.values())

    # Comparison
    print("\nResults")
    print(f"Local Count:     {local_total} words, {len(local_counts)} unique")
    print(f"MapReduce Count: {mr_total} words, {len(mr_counts)} unique")

    mismatches = []

    # Check for discrepancies
    for word, local_count in local_counts.items():
        mr_count = mr_counts.get(word, 0)
        if local_count != mr_count:
            mismatches.append(f"Word '{word}': Local={local_count}, MR={mr_count}")

    # Check for extra words in MR result
    for word in mr_counts:
        if word not in local_counts:
            mismatches.append(f"Word '{word}': Extra in MR result")

    if not mismatches:
        print("Status: PASS - Results match")
    else:
        print(f"Status: FAIL - {len(mismatches)} mismatches found")
        for error in mismatches[:10]:
            print(f"  - {error}")
        if len(mismatches) > 10:
            print(f"  ... and {len(mismatches) - 10} more.")

if __name__ == "__main__":
    verify_results()