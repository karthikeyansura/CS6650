# Crash & Recovery: Cascading Failure with Circuit Breaker and Bulkhead

A crash and recovery experiment extending the Product Search Service with a dependent Inventory microservice, chaos engineering, and resilience patterns (fail-fast, circuit breaker, bulkhead). Built in Go with Gin, containerized with Docker, and load tested with Locust.

## Project Structure

```
Midterm/
├── product/
│   ├── main.go              # Product API, inventory enrichment and resilience patterns
│   ├── Dockerfile           # Multi-stage container build
│   ├── go.mod
│   └── go.sum
├── inventory/
│   ├── main.go              # Inventory microservice with chaos engineering toggles
│   ├── Dockerfile           # Multi-stage container build
│   ├── go.mod
│   └── go.sum
├── locust/
│   ├── locustfile.py        # Load test scenarios (FastHttpUser)
│   └── docker-compose.yml   # Master-Worker container orchestration
├── postman/
│   ├── Collection.json      # API endpoint test suite
│   └── Local.env.json       # Localhost environment variables
├── docs/                    # Screenshots, report, and presentation slides
├── docker-compose.yml       # Multi-service orchestration
└── README.md
```

## Prerequisites

- Go 1.23+
- Docker Desktop (running)

## Getting Started

### 1. Generate dependency checksum files

```bash
cd product && go mod tidy && cd ..
cd inventory && go mod tidy && cd ..
```

### 2. Docker Compose

The `RESILIENCE_MODE` environment variable controls which resilience patterns are active:

```bash
# Phase 1: No protection (demonstrates cascading failure)
RESILIENCE_MODE=none docker compose up --build

# Phase 2: Fail-fast and circuit breaker (demonstrates graceful degradation)
RESILIENCE_MODE=circuit-breaker docker compose up --build

# Phase 3: Fail-fast, circuit breaker, and bulkhead (full resilience)
RESILIENCE_MODE=bulkhead docker compose up --build
```

Docker Compose starts four services:

| Service | Port | Description |
|---------|------|-------------|
| product | 8080 | Product API with inventory enrichment |
| inventory | 8081 | Inventory service with chaos toggles |
| locust-master | 8089 | Load test UI |
| locust-worker | — | 2 Locust workers |

### 3. Teardown

```bash
docker compose down
```

## Architecture

```
[Locust :8089] --> [Product Service :8080] --> [Inventory Service :8081]
```

The Product Service adds `/products/search/inventory` and `/products/:id/inventory` endpoints that enrich product data with live stock information from the Inventory Service. The Inventory Service supports chaos engineering via `/chaos/:mode`.

Three resilience modes are available via `RESILIENCE_MODE`:

| Mode | Timeout | Circuit Breaker | Bulkhead | Behavior on Failure |
|------|---------|-----------------|----------|---------------------|
| `none` | 30s | No | No | Requests hang, cascading failure |
| `circuit-breaker` | 500ms | Yes (5 failures → open, 10s timeout, 3 successes → close) | No | Fast degradation, returns product without inventory |
| `bulkhead` | 500ms | Yes | Yes (10 concurrent slots) | CB + bounded concurrency |

## API Endpoints

### Product Service (:8080)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/products/search?q=term` | Search catalog (no inventory) |
| GET | `/products/search/inventory?q=term` | Search with inventory enrichment |
| GET | `/products/:id` | Get product by ID |
| GET | `/products/:id/inventory` | Get product with inventory |
| POST | `/products/:id/details` | Create/update product |
| GET | `/metrics` | Circuit breaker and bulkhead stats |

### Inventory Service (:8081)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/inventory/:id` | Get stock for product |
| GET | `/health` | Health check |
| POST | `/chaos/off` | Normal operation |
| POST | `/chaos/slow?delay_ms=3000` | Inject latency per request |
| POST | `/chaos/error` | Return 500 for all requests |
| POST | `/chaos/partial?delay_ms=2000` | 50% error, 50% slow |
| GET | `/chaos` | Check current chaos mode |

## Postman

The `postman/` directory contains the API collection and environment configuration:

- `Collection.json`: All endpoints including inventory enrichment, chaos engineering, and resilience metrics
- `Local.env.json`: 
  - Set `productUrl` to `http://localhost:8080`
  - Set `inventoryUrl` to `http://localhost:8081`

## Load Testing

Open `http://localhost:8089` after `docker compose up`. Select the desired user class from the class picker, set the target host to `http://product:8080`, configure user count and ramp-up rate, and start the test.

### Test Classes

| Class | Purpose | Users | Wait Time |
|-------|---------|-------|-----------|
| InventoryLoadUser | Standard load with inventory calls | 20 | 100–500ms |
| StressInventoryUser | High-concurrency bulkhead demo | 50 | 10–100ms |
| BaselineSearchUser | Baseline search (no inventory calls) | 20 | 100–500ms |

### Injecting Chaos During a Test

While Locust is running, use Postman (select `Local` environment):

1. **Chaos - SLOW (3s delay)**: injects 3-second latency on every inventory call
2. **Chaos - ERROR (500s)**: switches to total outage
3. **Chaos - OFF (Normal)**: restores normal operation
4. **Resilience Metrics**: returns circuit breaker and bulkhead stats
5. **Chaos - Check Status**: returns current chaos mode

## Experiment

### Phase 1: No Protection (`RESILIENCE_MODE=none`)

The Product Service relies on a standard 30-second HTTP timeout. When the Inventory Service experiences latency, threads block, leading to an immediate collapse in throughput.

**Steps:**
1. `RESILIENCE_MODE=none docker compose up --build -d`
2. Locust → select `InventoryLoadUser` → 20 users, ramp 5/s, host `http://product:8080`
3. Run for 1 minute to establish a performance baseline.
4. Postman → **Chaos - SLOW (3s delay)**
5. Observe the cascading failure (latency spike, throughput drop).
6. Stop Locust → `docker compose down`

### Phase 2: Circuit Breaker (`RESILIENCE_MODE=circuit-breaker`)

The HTTP timeout is reduced to 500ms (fail-fast). After 5 consecutive timeouts, the circuit breaker opens. Subsequent requests return immediately with degraded data (`inventory_source: "circuit-open"`), allowing the core product API to remain functional.

**Steps:**
1. `RESILIENCE_MODE=circuit-breaker docker compose up --build -d`
2. Locust → select `InventoryLoadUser` → 20 users, ramp 5/s, host `http://product:8080`
3. Run for 1 minute to establish a performance baseline.
4. Postman → **Chaos - SLOW (3s delay)**
5. Wait ~30s → observe degraded but fast responses.
6. Postman → **Resilience Metrics** → verify circuit breaker state is "open".
7. Postman → **Chaos - OFF (Normal)**
8. Wait ~15s → observe recovery (circuit closes, `inventory_source` back to "live").
9. Check product service logs for `[CIRCUIT BREAKER]` transitions.
10. Stop Locust → `docker compose down`

### Phase 3: Bulkhead (`RESILIENCE_MODE=bulkhead`)

Similar to Phase 2, but adds a bounded concurrency pool (10 slots) to restrict in-flight inventory calls. This prevents severe dependency slowness from exhausting all available server goroutines before the circuit even has a chance to open.

**Steps:**
1. `RESILIENCE_MODE=bulkhead docker compose up --build -d`
2. Locust → select `StressInventoryUser` → 50 users, ramp 10/s, host `http://product:8080`
3. Run for 1 minute to establish a performance baseline.
4. Postman → **Chaos - SLOW (3s delay)**
5. Postman → **Resilience Metrics** → verify bulkhead rejection stats.
6. Observe bounded degradation in Locust.
7. Postman → **Chaos - OFF (Normal)**
8. Stop Locust → `docker compose down`