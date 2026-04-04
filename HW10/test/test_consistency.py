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


# Health endpoint

def test_health_endpoint():
    """Leader /health returns 200 with expected metadata."""
    _wait_for([LEADER])
    r = requests.get(f"{LEADER}/health", timeout=5)
    assert r.status_code == 200, f"expected 200, got {r.status_code}"
    body = r.json()
    assert body["status"] == "ok"
    assert body["mode"] == "leader-follower"


# API contract tests

def test_set_empty_key_returns_400():
    """POST /set with empty key must be rejected."""
    _wait_for([LEADER])
    r = requests.post(f"{LEADER}/set", json={"key": "", "value": "x"}, timeout=5)
    assert r.status_code == 400, f"expected 400 for empty key, got {r.status_code}"


def test_get_empty_key_returns_400():
    """GET /get without a key parameter must be rejected."""
    _wait_for([LEADER])
    r = requests.get(f"{LEADER}/get", timeout=5)
    assert r.status_code == 400, f"expected 400 for missing key, got {r.status_code}"


def test_set_wrong_method_returns_405():
    """GET on /set must return 405 Method Not Allowed."""
    _wait_for([LEADER])
    r = requests.get(f"{LEADER}/set", timeout=5)
    assert r.status_code == 405, f"expected 405, got {r.status_code}"


def test_get_wrong_method_returns_405():
    """POST on /get must return 405 Method Not Allowed."""
    _wait_for([LEADER])
    r = requests.post(f"{LEADER}/get", json={"key": "k"}, timeout=5)
    assert r.status_code == 405, f"expected 405, got {r.status_code}"


def test_set_malformed_json_returns_400():
    """POST /set with non-JSON body must be rejected."""
    _wait_for([LEADER])
    r = requests.post(
        f"{LEADER}/set",
        data="not json",
        headers={"Content-Type": "application/json"},
        timeout=5,
    )
    assert r.status_code == 400, f"expected 400 for malformed json, got {r.status_code}"


def test_get_missing_key_returns_404():
    """GET /get for a key that was never written must return 404."""
    _wait_for([LEADER])
    r = requests.get(
        f"{LEADER}/get",
        params={"key": f"nonexistent-{int(time.time()*1000)}"},
        timeout=5,
    )
    assert r.status_code == 404, f"expected 404 for missing key, got {r.status_code}"


# Leader-Follower consistency

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
    """Observe local follower staleness during replication window.

    Retries up to 10 attempts to handle scheduling variance.
    Passes if at least one attempt catches a 404 on local_read.
    """
    _wait_for([LEADER] + FOLLOWERS)

    stale_detected = False
    all_observations = []
    attempts = 10

    for attempt in range(attempts):
        key = f"leader-window-{int(time.time()*1000)}-{attempt}"
        observations = []

        def writer():
            requests.post(
                f"{LEADER}/set",
                json={"key": key, "value": "fresh"},
                timeout=15,
            )

        t = threading.Thread(target=writer)
        t.start()

        deadline = time.time() + 2.5
        while time.time() < deadline:
            for f in FOLLOWERS:
                r = requests.get(f"{f}/local_read", params={"key": key}, timeout=2)
                observations.append(
                    {"node": f, "status": r.status_code, "time": time.time()}
                )
                if r.status_code == 404:
                    stale_detected = True
                    break
            if stale_detected:
                break
            time.sleep(0.02)

        t.join()
        all_observations.extend(observations)

        if stale_detected:
            break

    _json("leader_local_read_window.json", all_observations)
    assert stale_detected, (
        f"Failed to detect local_read staleness across {attempts} attempts. "
        f"Collected {len(all_observations)} observations, none returned 404."
    )


# Leaderless consistency

def test_leaderless_inconsistency_window_and_eventual_consistency():
    """Observe leaderless staleness window followed by eventual consistency.

    Retries up to 10 attempts to detect a stale 404 during the
    write coordinator's sequential fanout.
    """
    _wait_for(LEADERLESS)

    stale_seen = False
    all_observed = []
    last_coordinator = None
    last_others = None
    last_key = None
    attempts = 10

    for attempt in range(attempts):
        key = f"leaderless-window-{int(time.time()*1000)}-{attempt}"
        coordinator = random.choice(LEADERLESS)
        others = [n for n in LEADERLESS if n != coordinator]
        observed = []

        def writer():
            requests.post(
                f"{coordinator}/set",
                json={"key": key, "value": "v"},
                timeout=15,
            )

        t = threading.Thread(target=writer)
        t.start()

        deadline = time.time() + 2.5
        while time.time() < deadline:
            target = random.choice(others)
            r = requests.get(f"{target}/get", params={"key": key}, timeout=2)
            observed.append(
                {"node": target, "status": r.status_code, "time": time.time()}
            )
            if r.status_code == 404:
                stale_seen = True
                break
            time.sleep(0.02)

        t.join()
        all_observed.extend(observed)
        last_coordinator = coordinator
        last_others = others
        last_key = key

        if stale_seen:
            break

    rc = requests.get(
        f"{last_coordinator}/get", params={"key": last_key}, timeout=5
    )
    ro = requests.get(
        f"{random.choice(last_others)}/get", params={"key": last_key}, timeout=5
    )

    _json(
        "leaderless_window_results.json",
        {
            "coordinator": last_coordinator,
            "stale_seen_before_ack": stale_seen,
            "eventual_read_coordinator": rc.status_code,
            "eventual_read_other": ro.status_code,
            "observations": all_observed,
        },
    )

    assert stale_seen, (
        f"Failed to detect leaderless staleness across {attempts} attempts. "
        f"Collected {len(all_observed)} observations, none returned 404."
    )
    assert rc.status_code == 200, (
        f"coordinator should be consistent after ack, got {rc.status_code}"
    )
    assert ro.status_code == 200, (
        f"other node should be consistent after ack, got {ro.status_code}"
    )
    assert rc.json()["value"] == "v"
    assert ro.json()["value"] == "v"