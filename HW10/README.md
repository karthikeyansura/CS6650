# Distributed Databases using Replication

Replicated in-memory key-value store in Go with Leader-Follower and Leaderless architectures. N=5 nodes, configurable W/R strategies, Docker Compose deployment, Locust load testing.

## Architectures

**Leader-Follower** (N=5): `lf1` (leader) + `lf2-lf5` (followers), ports `8081-8085`.
- W=5 R=1: synchronous full replication, local leader reads
- W=1 R=5: async replication, all-node reads with max-version
- W=3 R=3: quorum writes and reads with overlap guarantee

**Leaderless** (N=5, W=N, R=1): `ll1-ll5`, ports `8091-8095`. Any node coordinates writes.

## API

| Endpoint | Method | Description |
|---|---|---|
| `/set` | POST | Write key-value pair, returns 201 |
| `/get?key=` | GET | Read key, returns 200 or 404 |
| `/local_read?key=` | GET | Node-local read (test hook) |
| `/health` | GET | Node status, mode, strategy |

## Delay Model

- Leader/coordinator sleeps **200ms** after each replication send
- Follower sleeps **100ms** before ACKing replication
- Follower sleeps **50ms** for leader-triggered internal reads

## Usage

### Start cluster

```bash
STRATEGY=W5R1 docker compose up --build -d
STRATEGY=W1R5 docker compose up --build -d
STRATEGY=W3R3 docker compose up --build -d
docker compose --profile leaderless up --build -d
```

### Run tests

```bash
go test ./...
pip install -r requirements.txt
pytest -v test/test_consistency.py
```

### Load testing

```bash
cd locust
MODE=leader-follower \
LEADER_HOST=http://host.docker.internal:8081 \
NODE_HOSTS=http://host.docker.internal:8081,http://host.docker.internal:8082,http://host.docker.internal:8083,http://host.docker.internal:8084,http://host.docker.internal:8085 \
WRITE_PERCENT=10 \
docker compose up
```

Open http://localhost:8089. 50 users, ramp 10/s, 120 seconds.

### Generate graphs

```bash
pip install matplotlib numpy
python3 generate_graphs.py
```

### Postman

Import `postman/Collection.json` with `postman/Local.env.json`.

## File Structure

```
HW10/
  cmd/server/
    main.go              # single binary, leader-follower + leaderless modes
    server_test.go       # httptest-based leader consistency test
  internal/kv/
    store.go             # RWMutex-protected map with Set/SetIfNewer/Get
    store_test.go        # store unit tests
  locust/
    locustfile.py        # Locust workload with inline staleness tracking
    docker-compose.yml   # single-process Locust container
  postman/
    Collection.json      # Distributed KV Store API collection
    Local.env.json       # localhost environment
  test/
    test_consistency.py  # pytest: leader + leaderless consistency tests
  results/
    lf_w5r1/             # W5R1 load test results (4 ratios)
    lf_w1r5/             # W1R5 load test results (4 ratios)
    lf_w3r3/             # W3R3 load test results (4 ratios)
    ll/                  # Leaderless load test results (4 ratios)
    leader_local_read_window.json
    leaderless_window_results.json
  docs/                  # screenshots and report
  graphs/                # generated distribution graphs
  Dockerfile
  docker-compose.yml
  go.mod
  requirements.txt
  generate_graphs.py
```
