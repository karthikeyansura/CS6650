# Integration tests against a running cluster.
import json
import os
import random
import threading
import time

import requests


LEADER = os.getenv("LEADER_URL", "http://localhost:8081")
FOLLOWERS = os.getenv(
    "FOLLOWER_URLS",
    "http://localhost:8082,http://localhost:8083,http://localhost:8084,http://localhost:8085",
).split(",")
LEADERLESS = os.getenv(
    "LEADERLESS_URLS",
    "http://localhost:8091,http://localhost:8092,http://localhost:8093,http://localhost:8094,http://localhost:8095",
).split(",")


def _wait_for(urls, timeout=45):
    """Wait until all node health endpoints return 200."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        ok = True
        for u in urls:
            try:
                r = requests.get(f"{u}/health", timeout=1)
                if r.status_code != 200:
                    ok = False
            except Exception:
                ok = False
        if ok:
            return
        time.sleep(0.5)
    raise RuntimeError(f"cluster not healthy within {timeout}s: {urls}")


def _json(path, obj):
    """Write JSON test artifacts to disk."""
    with open(path, "w", encoding="utf-8") as f:
        json.dump(obj, f, indent=2)


def test_leader_consistency_after_ack():
    """Validate consistent reads after leader write acknowledgment."""
    _wait_for([LEADER] + FOLLOWERS)

    key = f"leader-ack-{int(time.time()*1000)}"
    value = "v1"

    r = requests.post(f"{LEADER}/set", json={"key": key, "value": value}, timeout=10)
    assert r.status_code == 201

    leader_read = requests.get(f"{LEADER}/get", params={"key": key}, timeout=5)
    follower_read = requests.get(f"{random.choice(FOLLOWERS)}/get", params={"key": key}, timeout=5)

    assert leader_read.status_code == 200
    assert follower_read.status_code == 200
    assert leader_read.json()["value"] == value
    assert follower_read.json()["value"] == value


def test_leader_local_read_shows_inconsistency_window():
    """Observe local follower staleness during replication window."""
    _wait_for([LEADER] + FOLLOWERS)

    key = f"leader-window-{int(time.time()*1000)}"
    observations = []

    def writer():
        requests.post(f"{LEADER}/set", json={"key": key, "value": "fresh"}, timeout=15)

    t = threading.Thread(target=writer)
    t.start()

    stale_detected = False
    deadline = time.time() + 2.5
    while time.time() < deadline:
        for f in FOLLOWERS:
            r = requests.get(f"{f}/local_read", params={"key": key}, timeout=2)
            observations.append({"node": f, "status": r.status_code, "time": time.time()})
            if r.status_code == 404:
                stale_detected = True
                break
        if stale_detected:
            break
        time.sleep(0.02)

    t.join()
    _json("leader_local_read_window.json", observations)
    assert stale_detected


def test_leaderless_inconsistency_window_and_eventual_consistency():
    """Observe leaderless staleness window followed by eventual consistency."""
    _wait_for(LEADERLESS)

    key = f"leaderless-window-{int(time.time()*1000)}"
    coordinator = random.choice(LEADERLESS)
    others = [n for n in LEADERLESS if n != coordinator]

    stale_seen = False
    observed = []

    def writer():
        requests.post(f"{coordinator}/set", json={"key": key, "value": "v"}, timeout=15)

    t = threading.Thread(target=writer)
    t.start()

    deadline = time.time() + 2.5
    while time.time() < deadline:
        target = random.choice(others)
        r = requests.get(f"{target}/get", params={"key": key}, timeout=2)
        observed.append({"node": target, "status": r.status_code, "time": time.time()})
        if r.status_code == 404:
            stale_seen = True
            break
        time.sleep(0.02)

    t.join()

    rc = requests.get(f"{coordinator}/get", params={"key": key}, timeout=5)
    ro = requests.get(f"{random.choice(others)}/get", params={"key": key}, timeout=5)

    _json("leaderless_window_results.json", {
        "coordinator": coordinator,
        "stale_seen_before_ack": stale_seen,
        "eventual_read_coordinator": rc.status_code,
        "eventual_read_other": ro.status_code,
        "observations": observed,
    })

    assert stale_seen
    assert rc.status_code == 200
    assert ro.status_code == 200
    assert rc.json()["value"] == "v"
    assert ro.json()["value"] == "v"
