# Locust workload with inline staleness and interval tracking.
import json
import os
import random
import threading
import time
from locust import HttpUser, task, between, events


MODE = os.getenv("MODE", "leader-follower")
LEADER = os.getenv("LEADER_HOST", "http://host.docker.internal:8081")
NODES = [n.strip() for n in os.getenv(
    "NODE_HOSTS",
    "http://host.docker.internal:8081,http://host.docker.internal:8082,http://host.docker.internal:8083,http://host.docker.internal:8084,http://host.docker.internal:8085",
).split(",") if n.strip()]
WRITE_PERCENT = int(os.getenv("WRITE_PERCENT", "10"))
HOT_KEYS = int(os.getenv("HOT_KEYS", "20"))
COLD_KEYS = int(os.getenv("COLD_KEYS", "1000"))
HOT_PROB = float(os.getenv("HOT_PROB", "0.85"))
METRICS_FILE = os.getenv("METRICS_FILE", "/mnt/locust/locust_consistency_metrics.json")

_lock = threading.Lock()
_latest = {}  # key -> (version, write_ts)
_stale_reads = 0
_total_versioned_reads = 0
_read_after_write_intervals = []


def choose_key():
    if random.random() < HOT_PROB:
        return f"hot-{random.randint(0, HOT_KEYS-1)}"
    return f"cold-{random.randint(0, COLD_KEYS-1)}"


@events.quitting.add_listener
def on_quitting(**kwargs):
    with _lock:
        output = {
            "mode": MODE,
            "write_percent": WRITE_PERCENT,
            "stale_reads": _stale_reads,
            "total_versioned_reads": _total_versioned_reads,
            "observed_keys": len(_latest),
            "rw_intervals_count": len(_read_after_write_intervals),
            "rw_intervals_ms_sample": [round(v * 1000, 2) for v in _read_after_write_intervals[:500]],
        }
    try:
        with open(METRICS_FILE, "w", encoding="utf-8") as f:
            json.dump(output, f, indent=2)
    except Exception as e:
        print(f"Failed to write metrics: {e}")


class KVUser(HttpUser):
    wait_time = between(0.001, 0.03)

    @task
    def mixed_rw(self):
        global _stale_reads, _total_versioned_reads
        key = choose_key()

        if random.randint(1, 100) <= WRITE_PERCENT:
            value = f"v-{int(time.time() * 1000)}-{random.randint(1, 999999)}"
            target = LEADER if MODE == "leader-follower" else random.choice(NODES)
            with self.client.post(
                    f"{target}/set",
                    json={"key": key, "value": value},
                    name="set",
                    catch_response=True,
            ) as resp:
                if resp.status_code != 201:
                    resp.failure(f"set failed status={resp.status_code}")
                else:
                    resp.success()
                    try:
                        payload = resp.json()
                        version = payload.get("version")
                        if version is not None:
                            with _lock:
                                _latest[key] = (version, time.time())
                    except Exception:
                        pass
        else:
            target = random.choice(NODES)
            with self.client.get(
                    f"{target}/get",
                    params={"key": key},
                    name="get",
                    catch_response=True,
            ) as resp:
                if resp.status_code not in (200, 404):
                    resp.failure(f"get failed status={resp.status_code}")
                else:
                    resp.success()
                    if resp.status_code == 200:
                        try:
                            payload = resp.json()
                            got_version = payload.get("version")
                            if got_version is not None:
                                now = time.time()
                                with _lock:
                                    if key in _latest:
                                        expected_version, write_ts = _latest[key]
                                        _total_versioned_reads += 1
                                        if got_version < expected_version:
                                            _stale_reads += 1
                                        _read_after_write_intervals.append(now - write_ts)
                        except Exception:
                            pass