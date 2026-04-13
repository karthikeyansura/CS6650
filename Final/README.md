# ChaosArena Album Store

Album Store REST API with async photo processing pipeline.

## Architecture

```
Client -> ALB (us-west-2) -> ECS Fargate API (6 tasks, 2048 CPU / 4096 MB)
                                |
                                +-> DynamoDB (albums, album_counters, photos)
                                +-> S3 (photo blob storage, Transfer Acceleration)
                                +-> SQS (async fallback queue)
                                        |
                                        v
                              ECS Fargate Worker (12 tasks, 512 CPU / 1024 MB)
                                |
                                +-> DynamoDB (status updates)
                                +-> S3 (URL generation)
```

### Design Decisions

- **Early 202 flush with streaming S3 upload** reduces POST-to-202 latency to <50ms while the handler continues streaming to S3 in the same connection
- **Optimistic inline completion** bypasses SQS on the happy path; SQS serves as a durable fallback
- **DynamoDB atomic `UpdateItem`** on a dedicated counters table guarantees unique monotonic `seq` values under concurrent uploads
- **Conditional writes** with `attribute_exists` in `CompletePhoto` prevent zombie records when delete races with completion
- **Parallel S3 + DynamoDB deletion** via goroutines with `sync.WaitGroup` to stay within the 5 second budget
- **VPC Gateway Endpoints** for S3 and DynamoDB bypass NAT gateway latency and cost

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| PUT | `/albums/:album_id` | Create or update album (idempotent) |
| GET | `/albums/:album_id` | Retrieve album |
| GET | `/albums` | List all albums |
| POST | `/albums/:album_id/photos` | Upload photo (async, returns 202) |
| GET | `/albums/:album_id/photos/:photo_id` | Photo status and URL |
| DELETE | `/albums/:album_id/photos/:photo_id` | Hard delete photo and S3 object |

## Prerequisites

- Go 1.26+
- Docker
- Terraform >= 1.6
- AWS CLI configured with `us-west-2` credentials

## Usage

### Bootstrap Infrastructure

```bash
cd terraform
terraform init
terraform apply
```

### Build, Deploy, and Smoke Test

```bash
./scripts/deploy.sh
```

Runs the full pipeline: `terraform apply` -> Docker build (no-cache) -> ECR push -> ECS force-new-deployment -> task stabilization wait -> smoke test.

### Submit to ChaosArena

```bash
./scripts/submit.sh <email> <nickname> <alb-url>
```

Example:

```bash
./scripts/submit.sh sura.sa@northeastern.edu Karthikeyan http://album-store-alb-XXXXXXXXX.us-west-2.elb.amazonaws.com
```

### Smoke Test

```bash
./scripts/smoke_test.sh <alb-url>
```

Validates all 7 endpoints including photo upload, polling for completion, delete verification, and S3 URL accessibility.

### Check Leaderboard

```bash
./scripts/leaderboard.sh
```

### Load Test (Locust)

```bash
pip install -r locust/requirements.txt
locust -f locust/locustfile.py --host=<alb-url> --headless -u 50 -r 10 -t 60s
```

### Tear Down

```bash
cd terraform
terraform destroy -auto-approve
```

## Project Structure

```
Final/
  cmd/api/main.go             API server entrypoint with connection pooling
  cmd/worker/main.go          SQS consumer with configurable concurrency
  internal/
    config/                   Environment variable loader
    model/                    Domain models and DTOs
    handler/                  HTTP handlers (7 endpoints)
    middleware/               Request logging and panic recovery
    store/                    DynamoDB operations (CRUD, atomic counters)
    blob/                     S3 upload manager and delete operations
    queue/                    SQS producer and consumer
  Dockerfile.api              Multi-stage API build
  Dockerfile.worker           Multi-stage worker build
  terraform/                  Modular IaC (11 modules)
  locust/                     Load test suite approximating S11-S15
  scripts/
    deploy.sh                 Full deploy pipeline
    submit.sh                 ChaosArena submission
    smoke_test.sh             Endpoint validation
    leaderboard.sh            Leaderboard query
```
