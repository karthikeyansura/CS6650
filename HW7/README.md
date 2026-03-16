# Event-Driven Order Processing with SNS/SQS and Lambda

Event-driven order processing system demonstrating synchronous vs asynchronous architectures using AWS SNS, SQS, ECS Fargate, and Lambda.

## Project Structure

```
HW7/
├── src/
│   ├── receiver/              # Order Receiver: /orders/sync, /orders/async, /health
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   ├── processor/             # Order Processor: SQS poller with configurable workers
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   └── lambda/                # Lambda Processor (Part III): SNS-triggered
│       ├── main.go
│       ├── go.mod
│       ├── go.sum
│       └── function.zip       # Built Lambda deployment package
├── terraform/
│   ├── main.tf                # Root module composing all infrastructure
│   ├── provider.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── modules/
│       ├── vpc/               # VPC (10.0.0.0/16), subnets, NAT, IGW, security groups
│       ├── alb/               # Application Load Balancer + target group
│       ├── ecr/               # ECR repositories (receiver + processor)
│       ├── ecs/               # ECS cluster, task definitions, services
│       ├── iam/               # IAM roles (receiver, processor, lambda)
│       ├── logging/           # CloudWatch log groups
│       ├── sns_sqs/           # SNS topic, SQS queue, subscription
│       └── lambda/            # Lambda function + SNS subscription
├── locust/
│   ├── locustfile.py          # Load test scenarios (4 classes)
│   └── docker-compose.yml
├── scripts/
│   ├── build_lambda.sh        # Cross-compile Lambda for provided.al2
│   └── update_workers.sh      # Change processor worker count without rebuild
└── docs/                      # Screenshots
```

## Prerequisites

- Go 1.23+
- Docker Desktop (running)
- Terraform CLI
- AWS CLI (configured with credentials)

## Deployment

### 1. Initialize Go modules

```bash
cd src/receiver  && go mod tidy && cd ../..
cd src/processor && go mod tidy && cd ../..
cd src/lambda    && go mod tidy && cd ../..
```

### 2. Deploy Part II infrastructure

```bash
cd terraform
terraform init
terraform apply -auto-approve
```

Provisions: custom VPC (10.0.0.0/16) with public subnets (10.0.1.0/24, 10.0.2.0/24) for the ALB and private subnets (10.0.10.0/24, 10.0.11.0/24) for ECS tasks routed through a NAT gateway, ALB, 2 ECR repos, 2 ECS Fargate services (receiver + processor), SNS topic (`order-processing-events`), SQS queue (`order-processing-queue`, visibility timeout 60s, long-poll 20s), IAM roles, and CloudWatch log groups. Docker images are built and pushed to ECR automatically.

### 3. Get ALB endpoint

```bash
export ALB=$(terraform output -raw alb_dns_name)
```

### 4. Verify

```bash
curl http://$ALB/health
# {"status":"ok"}
```

## Architecture

```
Sync Path:
  Client --> ALB --> Receiver --> Payment (3s semaphore) --> 200 OK

Async Path:
  Client --> ALB --> Receiver --> SNS --> 202 Accepted
                                  |
                                  v
                                 SQS --> Processor (configurable workers, 3s each)

Lambda Path (Part III):
  Client --> ALB --> Receiver --> SNS --> Lambda (3s per order, no SQS)
```

## API Endpoints

| Method | Path | Description | Response |
|--------|------|-------------|----------|
| GET | `/health` | Health check | 200 `{"status":"ok"}` |
| POST | `/orders/sync` | Synchronous order (blocks 3s) | 200 with order details |
| POST | `/orders/async` | Async order (publishes to SNS) | 202 Accepted |

## Load Testing

### Start Locust

```bash
cd locust
docker compose up --scale worker=1
```

Open http://localhost:8089. Select test class from the class picker, set host to `http://<ALB_DNS>`.

### Test Classes

| Class | Purpose | Config |
|-------|---------|--------|
| `SyncNormalUser` | Phase 1: normal operations | 5 users, spawn rate 1/s, 30s |
| `SyncFlashUser` | Phase 1: flash sale stress | 20 users, spawn rate 10/s, 60s |
| `AsyncFlashUser` | Phase 3: async flash sale | 20 users, spawn rate 10/s, 60s |
| `WorkerScalingUser` | Phase 5: worker scaling | 20 users, spawn rate 10/s, 60s |

### Worker Scaling (Phase 5)

Between `WorkerScalingUser` runs, update the processor's goroutine count:

```bash
./scripts/update_workers.sh 5
./scripts/update_workers.sh 20
./scripts/update_workers.sh 100
```

Wait ~2 minutes after each update for the ECS rolling deployment to complete before starting the next Locust test. Purge the SQS queue between runs:

```bash
aws sqs purge-queue --queue-url $(cd terraform && terraform output -raw sqs_queue_url)
```

## Part III: Lambda

### Build and deploy

```bash
./scripts/build_lambda.sh

cd terraform
terraform apply -var deploy_lambda=true -auto-approve
```

**Warning:** With Lambda deployed, the SNS topic fans out to both SQS and Lambda. Do not run Locust while Lambda is subscribed. Send only 5-10 manual orders via curl:

```bash
for i in $(seq 1 10); do
  curl -s -X POST http://$ALB/orders/async \
    -H "Content-Type: application/json" \
    -d "{\"customer_id\": $i, \"items\": [{\"product_id\": $i, \"name\": \"Test-Product-$i\", \"quantity\": 1, \"price\": 9.99}]}"
  echo ""
  sleep 1
done
```

Observe cold starts in CloudWatch: Log groups > `/aws/lambda/order-service-order-processor`. Look for REPORT lines with `Init Duration` (cold) vs without (warm).

### Undeploy Lambda

```bash
cd terraform
terraform apply -var deploy_lambda=false -auto-approve
```

## Key Results

| Test | Requests | RPS | Median | p95 | Failures |
|------|----------|-----|--------|-----|----------|
| Sync normal (5 users, 30s) | 9 | 0.4 | 12,000 ms | 15,000 ms | 0% |
| Sync flash (20 users, 60s) | 19 | 0.4 | 30,000 ms | 56,000 ms | 0% |
| Async flash (1 worker, 20 users, 60s) | 3,376 | 56 | 47 ms | 64 ms | 0% |
| Async (5 workers) | 3,299 | 56.5 | 49 ms | 70 ms | 0% |
| Async (20 workers) | 3,357 | 57.4 | 48 ms | 69 ms | 0% |
| Async (100 workers) | 3,287 | 54.6 | 51 ms | 110 ms | 0% |

Async accepted **178x more orders** than sync (3,376 vs 19). Minimum workers to prevent queue buildup at 56 orders/s: ceil(56 x 3) = 168 goroutines.

Lambda cold start: 85.55 ms init duration (2.8% overhead on 3s processing). Cost: $0/month under 267K orders (free tier) vs $17/month for ECS.

## Teardown

```bash
cd terraform
terraform destroy -auto-approve
```