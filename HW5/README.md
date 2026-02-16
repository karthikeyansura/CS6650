# Product Service API

A thread-safe REST API for product management, built in Go with Gin, containerized with Docker, and deployed to AWS ECS Fargate using Terraform infrastructure-as-code.

## Project Structure

```
HW5/
├── src/
│   ├── main.go              # Product API server
│   ├── Dockerfile           # Multi-stage container build
│   ├── go.mod
│   └── go.sum
├── terraform/
│   ├── main.tf              # Infrastructure orchestrator and ECR lifecycle
│   ├── provider.tf          # AWS and Docker provider configurations
│   ├── variables.tf         # Environment and resource parameter definitions
│   ├── outputs.tf           # Exposed resource attributes
│   └── modules/
│       ├── ecr/             # ECR repository provisioning
│       ├── ecs/             # Fargate task definitions and service orchestration
│       ├── logging/         # CloudWatch centralized logging configuration
│       └── network/         # VPC networking and security group ingress/egress rules
├── locust/
│   ├── locustfile.py        # Distributed load testing scenarios (HttpUser/FastHttpUser)
│   └── docker-compose.yml   # Master-Worker container orchestration for Locust
├── postman/
│   ├── Collection.json      # API endpoint test suite
│   ├── Local.env.json       # Localhost/Docker environment variables
│   └── AWS.env.json         # Remote Fargate environment variables
├── docs/                    # Screenshots
├── api.yaml                 # OpenAPI specification
└── README.md                # Project documentation
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
```

### 2. Docker Deployment

```bash
cd src
docker build -t product-service .
docker run -p 8080:8080 --name product-service product-service
```

### 3. Cloud Deployment

```bash
cd terraform
terraform init
terraform apply -auto-approve
```

Terraform provisions the infrastructure stack and publishes the container image to Amazon ECR. The following AWS resources are created:

| Resource | Name |
|----------|------|
| ECR Repository | `product-service` |
| ECS Cluster | `product-service-cluster` |
| ECS Service | `product-service` |
| Security Group | `product-service-sg` (TCP 8080) |
| CloudWatch Logs | `/ecs/product-service` |

### 4. Retrieve Public IP Address

```bash
aws ec2 describe-network-interfaces \
  --network-interface-ids $(
    aws ecs describe-tasks \
      --cluster $(terraform output -raw ecs_cluster_name) \
      --tasks $(
        aws ecs list-tasks \
          --cluster $(terraform output -raw ecs_cluster_name) \
          --service-name $(terraform output -raw ecs_service_name) \
          --query 'taskArns[0]' --output text
      ) \
      --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
      --output text
  ) \
  --query 'NetworkInterfaces[0].Association.PublicIp' \
  --output text
```

Alternatively, navigate to the AWS Management Console and locate the public IP under ECS → Clusters → Tasks → Networking.

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

### Response Codes

| Code | Method | Condition |
|------|--------|-----------|
| 200  | GET    | Product found |
| 200  | GET    | Service operational |
| 204  | POST   | Product created or updated |
| 400  | GET/POST | Invalid product ID |
| 400  | POST   | Missing required fields |
| 400  | POST   | Path and payload `product_id` mismatch |
| 404  | GET    | Product not found |

## Postman

The `postman/` directory contains the API collection and environment configurations:

- `Collection.json`
- `Local.env.json`
- `AWS.env.json`

Use the **Local** environment for local development (`go run` or Docker) and **AWS** for the deployed Fargate service.

## Load Testing

Load testing is orchestrated with Locust using Docker Compose in a distributed master–worker setup.

```bash
cd locust

# Baseline run (1 worker)
docker compose up --scale worker=1

# Scaled run (4 workers)
docker compose up --scale worker=4 --force-recreate
```

Open `http://localhost:8089`.
Select the desired user class, set the target host, configure user count and ramp-up rate, and start the test.

### Results

| Experiment  | Client | Workers | Users | RPS | Median (ms) | p95 (ms) |
|-------------|--------|---------|-------|-----|-------------|----------|
| Baseline    | HttpUser | 1 | 50 | 154 | 19 | 25 |
| 4 Workers   | HttpUser | 4 | 50 | 158 | 19 | 23 |
| Fast Client | FastHttpUser | 1 | 50 | 157 | 18 | 30 |
| High Load   | HttpUser | 4 | 200 | 616 | 18 | 25 |

### Analysis

**Baseline vs 4 Workers:** Throughput remains effectively unchanged. At 50 concurrent users, a single Locust worker is sufficient to saturate the service. Client-side scaling does not materially impact results, indicating the load generator is not the limiting factor.

**HttpUser vs FastHttpUser:** Throughput is comparable. Although FastHttpUser leverages a lower-overhead C-based HTTP client (geventhttpclient), end-to-end latency is dominated by network round-trip time to Fargate rather than client execution overhead. The performance ceiling is therefore server/network bound.

**High Load:** Throughput scales near-linearly, with median latency remaining stable. This suggests the service maintains headroom at this concurrency level and does not exhibit saturation behavior within the tested range.

**GET vs POST:** Latency profiles are similar. The `sync.RWMutex` permits concurrent reads (`RLock`) while serializing writes (`Lock`), and no observable contention appears at the tested throughput. For read-heavy workloads, this concurrency model remains efficient.

## Design and Infrastructure Reflections

### Endpoint Semantics and Design Rationale
Although `api.yaml` defines a `404 Not Found` response for `POST /products/:id/details`, the `Instructions.md` mandates a product creation support. Therefore, the endpoint was implemented with idempotent upsert semantics: non-existent IDs result in creation, while existing IDs trigger updates. Consequently, `404 Not Found` is not returned for syntactically valid product identifiers.

### Scalable Backend Architecture
The `api.yaml` specification describes a system with distinct domains: Products, Shopping Carts, Warehouse, and Payments. To scale this beyond a single monolithic server, the system could be evolved into a microservices-based architecture.

* **Service Decomposition:** Separate the application into four independently deployable services (Product, Cart, Warehouse, Payment). This enables workload-specific scaling strategies.
* **Data Management Strategy:**
    * **Product Service:** Employ a read-through caching layer (e.g., Redis) in front of a relational database. Since product details change rarely but are read frequently, caching significantly reduces database load.
    * **Shopping Cart Service:** Utilize a high-throughput NoSQL datastore (e.g., DynamoDB or Redis) optimized for rapid session writes and temporary state.
    * **Warehouse & Payment Services:** Use ACID-compliant relational databases (e.g., PostgreSQL) to ensure inventory and financial consistency.
* **Asynchronous Communication:** Adopt event-driven integration using a message broker (e.g., RabbitMQ or AWS SQS) to decouple service interactions and improve fault isolation.
### Declarative Infrastructure with Terraform
Terraform defines infrastructure as state rather than procedure. The configuration expresses the intended system topology, and Terraform reconciles it against the current environment to compute and apply the required changes. This model ensures idempotent execution, automatic drift detection, and predictable change planning through explicit execution plans.

### Infrastructure Implementation
* **State Management:** Infrastructure state is persisted in `terraform.tfstate`, allowing safe, incremental changes and resource tracking.
* **Networking:** The system utilizes the default AWS VPC and subnets, with a dedicated security group permitting inbound TCP traffic on port 8080.