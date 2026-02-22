# Product Search Service with Horizontal Scaling

A thread-safe REST API for product management and search, built in Go with Gin, containerized with Docker, and deployed to AWS ECS Fargate with ALB and Auto Scaling using Terraform infrastructure-as-code.

## Project Structure

```
HW6/
├── src/
│   ├── main.go              # Product API + Search endpoint (100K products)
│   ├── Dockerfile           # Multi-stage container build
│   ├── go.mod
│   └── go.sum
├── terraform/
│   ├── main.tf              # Infrastructure orchestrator
│   ├── provider.tf          # AWS and Docker provider configurations
│   ├── variables.tf         # Environment and resource parameter definitions
│   ├── outputs.tf           # Exposed resource attributes (ALB DNS, cluster/service names)
│   └── modules/
│       ├── ecr/             # ECR repository provisioning
│       ├── ecs/             # Fargate task definitions and service orchestration
│       ├── logging/         # CloudWatch centralized logging configuration
│       ├── network/         # VPC networking and security group ingress/egress rules
│       ├── alb/             # Application Load Balancer and Target Group
│       └── autoscaling/     # CPU-based Auto Scaling policy
├── locust/
│   ├── locustfile.py        # Distributed load testing scenarios (FastHttpUser)
│   └── docker-compose.yml   # Master-Worker container orchestration for Locust
├── postman/
│   ├── Collection.json      # API endpoint test suite
│   ├── Local.env.json       # Localhost/Docker environment variables
│   └── AWS.env.json         # Remote ALB environment variables
├── docs/                    # Screenshots and report
├── api.yaml                 # OpenAPI specification
└── README.md
```

## Prerequisites

- Go 1.25+
- Docker Desktop (running)
- Terraform CLI
- AWS CLI (configured)
- Postman (for API testing)

## Getting Started

### 1. Local Development

```bash
cd src
go mod tidy
go run .
# Server starts on http://localhost:8080
# Loads 100,000 products into search catalog at startup
```

### 2. Docker Deployment

```bash
cd src
docker build -t product-service .
docker run -p 8080:8080 --name product-service product-service
```

### 3. Cloud Deployment

Configure instance count in `terraform/variables.tf`:
- **Single Instance:** `ecs_count = 1`, `autoscaling_min = 1`, `autoscaling_max = 1`
- **Horizontal Scaling:** `ecs_count = 2`, `autoscaling_min = 2`, `autoscaling_max = 4`

```bash
cd terraform
terraform init
terraform apply -auto-approve
```

Terraform provisions the following AWS resources:

| Resource | Name |
|----------|------|
| ECR Repository | `product-service` |
| ECS Cluster | `product-service-cluster` |
| ECS Service | `product-service` |
| ALB | `product-service-alb` |
| Target Group | `product-service-tg` (IP, HTTP:8080, /health) |
| Auto Scaling | CPU target tracking at 70% (min 2, max 4) |
| Security Groups | `product-service-ecs-sg`, `product-service-alb-sg` |
| CloudWatch Logs | `/ecs/product-service` |

### 4. Retrieve ALB DNS

```bash
terraform output -raw alb_dns_name
```

### 5. Teardown

```bash
cd terraform
terraform destroy -auto-approve
```

## API Endpoints

### Health Check
```
GET /health → 200
```
```json
{"status": "ok"}
```

### Product Search
```
GET /products/search?q=Electronics → 200
```
```json
{
  "products": [...],
  "total_found": 10,
  "search_time": "151.875µs"
}
```
Searches 100,000 products (checks exactly 100 per request), returns max 20 results.

### Create Product
```
POST /products/:id/details → 204 No Content
```
```json
{
  "product_id": 1,
  "sku": "ABC-123-XYZ",
  "manufacturer": "Acme Corporation",
  "category_id": 456,
  "weight": 1250,
  "some_other_id": 789
}
```

### Get Product
```
GET /products/:id → 200
```
Returns the product JSON, or 404 if not found.

### Search - No Match
```
GET /products/search?q=xyznonexistent → 200
```
```json
{"products": [], "total_found": 0, "search_time": "113.542µs"}
```

### Error: Not Found
```
GET /products/999 → 404
```
```json
{
  "error": "NOT_FOUND",
  "message": "Product not found",
  "details": "No product exists with ID 999"
}
```

### Error: Invalid ID
```
GET /products/abc → 400
```
```json
{
  "error": "INVALID_INPUT",
  "message": "Invalid product ID",
  "details": "Product ID must be a positive integer"
}
```

### Error: Missing Search Query
```
GET /products/search → 400
```
```json
{
  "error": "INVALID_INPUT",
  "message": "Missing search query",
  "details": "Query parameter 'q' is required"
}
```

### Error: Missing Required Fields
```
POST /products/2/details → 400
```
```json
{
  "error": "INVALID_INPUT",
  "message": "Missing or invalid required fields",
  "details": "sku is required and cannot be empty"
}
```

### Error: ID Mismatch
```
POST /products/3/details → 400
Body: {"product_id": 99, ...}
```
```json
{
  "error": "INVALID_INPUT",
  "message": "Product ID mismatch",
  "details": "Path product ID does not match body product_id"
}
```

### Response Codes

| Code | Method | Condition |
|------|--------|-----------|
| 200  | GET    | Product found / Search results |
| 200  | GET    | Service operational |
| 204  | POST   | Product created or updated |
| 400  | GET/POST | Invalid product ID |
| 400  | GET    | Missing search query parameter |
| 400  | POST   | Missing required fields |
| 400  | POST   | Path and payload `product_id` mismatch |
| 404  | GET    | Product not found |

## Postman

The `postman/` directory contains the API collection and environment configurations:

- `Collection.json` — All endpoints including search
- `Local.env.json` — Set `baseUrl` to `http://localhost:8080`
- `AWS.env.json` — Set `baseUrl` to `http://<ALB_DNS>`

## Load Testing

Load testing is orchestrated with Locust using Docker Compose in a distributed master-worker setup.

```bash
cd locust

# Part II: Single instance tests (1 worker)
docker compose up --scale worker=1

# Part III: Horizontal scaling tests (4 workers)
docker compose up --scale worker=4 --force-recreate
```

Open `http://localhost:8089`. Select the desired user class from the class picker, set the target host to the ALB DNS, configure user count and ramp-up rate, and start the test.

### Available Test Classes

| Class | Purpose | Users | Duration |
|-------|---------|-------|----------|
| SearchBaselineUser | Baseline CPU measurement | 5 | 2 min |
| SearchBreakingPointUser | Stress single instance | 20 | 3 min |
| SearchScaledUser | Stress with horizontal scaling | 50 | 5 min |
| SearchSpikeUser | Sudden traffic burst | 50-100 | 3 min |

## Infrastructure

### Scaling Configuration

Edit `terraform/variables.tf` to adjust:

| Variable | Default | Description |
|----------|---------|-------------|
| `ecs_count` | 2 | Initial task count |
| `autoscaling_min` | 2 | Minimum tasks |
| `autoscaling_max` | 4 | Maximum tasks |
| `cpu_target_value` | 70 | CPU % threshold for scaling |
| `scale_out_cooldown` | 300 | Seconds between scale-out events |
| `scale_in_cooldown` | 300 | Seconds between scale-in events |

### Vertical Scaling Experiment

To test with doubled CPU (512 units), edit `terraform/modules/ecs/variables.tf`:
```hcl
variable "cpu"    { default = "512" }
variable "memory" { default = "1024" }
```
Apply with `terraform apply -auto-approve`.