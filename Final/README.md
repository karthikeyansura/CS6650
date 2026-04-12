# ChaosArena v1 -- Album Store

CS6650 Final: production-grade Album Store REST API with async photo processing pipeline.

## Architecture

```
Client -> ALB (us-west-2) -> ECS Fargate API (4+ tasks)
                                |
                                +-> DynamoDB (albums, album_counters, photos)
                                +-> S3 (photo blob storage)
                                +-> SQS (async job queue)
                                        |
                                        v
                              ECS Fargate Worker (6+ tasks)
                                |
                                +-> DynamoDB (status updates)
                                +-> S3 (URL generation)
```

**Key design decisions:**
- DynamoDB atomic `UpdateItem` for per-album seq allocation (concurrency safe)
- S3 multipart upload manager for streaming large files without full RAM buffering
- SQS Standard Queue with DLQ for durable async processing
- Conditional DynamoDB writes in worker to prevent zombie records on delete race
- Parallel S3 + DynamoDB deletion to stay within 5 second budget
- VPC endpoints for S3 and DynamoDB to bypass NAT gateway

## Quick Start

### Prerequisites

- Go 1.26+
- Docker
- Terraform >= 1.6
- AWS CLI configured with us-west-2 credentials

### 1. Bootstrap Infrastructure

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars: set api_image and worker_image to placeholder values first
terraform init
terraform apply   # creates VPC, ECR, DynamoDB, S3, SQS, ALB, ECS
```

### 2. Build and Push Images

```bash
# from project root
make push
```

### 3. Update Terraform with ECR Image URIs

Edit `terraform/terraform.tfvars` with the ECR URIs from `make push` output, then:

```bash
make tf-apply
```

### 4. Smoke Test

```bash
ALB_DNS=$(cd terraform && terraform output -raw alb_dns_name)
make smoke BASE_URL="http://${ALB_DNS}"
```

### 5. Submit to ChaosArena

```bash
make submit EMAIL=sura.s@northeastern.edu NICKNAME=karthik BASE_URL="http://${ALB_DNS}"
```

### 6. Check Leaderboard

```bash
make leaderboard
```

### 7. Load Test Locally

```bash
pip install -r locust/requirements.txt
make locust BASE_URL="http://${ALB_DNS}"
```

## Project Structure

```
album-store/
  cmd/api/main.go           API server entrypoint
  cmd/worker/main.go        SQS consumer worker entrypoint
  internal/
    config/                  Environment variable loading
    model/                   Domain models and DTOs
    handler/                 HTTP handlers (all 7 endpoints)
    middleware/              Logging and panic recovery
    store/                   DynamoDB operations
    blob/                    S3 upload/delete operations
    queue/                   SQS producer/consumer
  deployments/docker/        Dockerfiles (multi-stage)
  terraform/                 Modular IaC (11 modules)
  locust/                    Load testing
  scripts/                   Build, deploy, smoke test, submit
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Health check |
| PUT | /albums/:album_id | Create/update album (idempotent) |
| GET | /albums/:album_id | Get album |
| GET | /albums | List all albums |
| POST | /albums/:album_id/photos | Upload photo (async, returns 202) |
| GET | /albums/:album_id/photos/:photo_id | Get photo status |
| DELETE | /albums/:album_id/photos/:photo_id | Delete photo (hard delete) |
