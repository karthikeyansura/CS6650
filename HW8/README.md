# HW8: Welcome to the Data Layer!

Shopping Cart API with persistent storage using **Amazon RDS MySQL** (STEP I) and **Amazon DynamoDB** (STEP II), deployed on **ECS Fargate** behind an **ALB** using **Terraform**.

## Project Structure

```
HW8/
├── src/
│   ├── main.go              # Shopping cart API (MySQL + DynamoDB backends)
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── terraform/
│   ├── main.tf              # Root module composing all infrastructure
│   ├── provider.tf
│   ├── variables.tf
│   ├── terraform.tfvars
│   ├── outputs.tf
│   └── modules/
│       ├── network/          # VPC, subnets, security groups (ECS, ALB, RDS)
│       ├── ecr/              # ECR repository
│       ├── ecs/              # ECS cluster, task definition, service
│       ├── alb/              # Application Load Balancer
│       ├── autoscaling/      # ECS auto scaling
│       ├── logging/          # CloudWatch log group
│       ├── rds/              # RDS MySQL 8.0 (db.t3.micro)
│       ├── dynamodb/         # DynamoDB table (PAY_PER_REQUEST)
│       └── iam/              # ECS task role with DynamoDB permissions
├── locust/
│   ├── locustfile.py         # Load test definitions
│   └── docker-compose.yml
├── test/
│   ├── test_performance.py   # 150-operation performance test
│   ├── test_consistency.py   # DynamoDB eventual consistency test
│   └── combine_results.py    # STEP III comparison analysis
├── postman/
│   └── Collection.json       # Postman collection for manual testing
└── README.md
```

## Architecture

```
Client → ALB → ECS Fargate (Go API) → RDS MySQL / DynamoDB
                                    ↕
                              CloudWatch Logs
```

The Go application uses an environment variable `DB_MODE` (`mysql` or `dynamodb`) to switch between backends. Both backends expose the exact same REST API.

## API Endpoints

| Method | Path                           | Description         |
|--------|--------------------------------|---------------------|
| POST   | /shopping-carts                | Create a new cart   |
| GET    | /shopping-carts/{id}           | Retrieve cart       |
| POST   | /shopping-carts/{id}/items     | Add items to cart   |
| GET    | /health                        | Health check        |

## Database Schema Design

### MySQL (STEP I)

**Two normalized tables:**

- `shopping_carts`: id (PK, AUTO_INCREMENT), customer_id (indexed), created_at
- `cart_items`: id (PK), cart_id (FK → shopping_carts, indexed), product_id, quantity, UNIQUE(cart_id, product_id)

**Design rationale:** Normalized schema enforces data integrity via foreign keys. The unique constraint on `(cart_id, product_id)` enables efficient `INSERT ... ON DUPLICATE KEY UPDATE` upserts. InnoDB's row-level locking handles concurrent cart modifications.

**Indexes:** `idx_customer` on customer_id for purchase history queries; `idx_cart` on cart_id for item retrieval; unique key `uq_cart_product` for fast upserts.

### DynamoDB (STEP II)

**Single table design:**

- Partition key: `cart_id` (UUID string for even distribution)
- Attributes: `customer_id`, `items` (list of maps), `created_at`

**Design rationale:** Items are embedded in the cart document (single-table design) because shopping cart access patterns are simple: create, read by ID, and update items. UUID partition keys ensure even distribution with no hot partitions, even at millions of carts. No secondary indexes needed since we only access by cart_id.

## Quick Start

### Prerequisites
- AWS CLI configured
- Terraform installed
- Docker running
- Go 1.25+
- Python 3 with `requests` package

### STEP I: Deploy with MySQL

```bash
cd terraform/

# Ensure db_mode = "mysql" in terraform.tfvars
# Set a strong db_password

terraform init
terraform apply -auto-approve

# Note the ALB DNS from output
export ALB_URL=$(terraform output -raw alb_dns_name)
```

### Run MySQL Performance Test

```bash
cd ../test/
python3 test_performance.py http://$ALB_URL mysql
# Produces: mysql_test_results.json
```

### STEP II: Switch to DynamoDB

```bash
cd ../terraform/

# Edit terraform.tfvars: change db_mode = "dynamodb"
terraform apply -auto-approve

# Wait for ECS service to stabilize (~2 min)
```

### Run DynamoDB Performance Test

```bash
cd ../test/
python3 test_performance.py http://$ALB_URL dynamodb
# Produces: dynamodb_test_results.json

# Run consistency test
python3 test_consistency.py http://$ALB_URL
```

### STEP III: Compare Results

```bash
cd ../test/
python3 combine_results.py
# Produces: combined_results.json + prints comparison tables
```

### Load Testing with Locust

```bash
cd ../locust/
TARGET_HOST=http://$ALB_URL docker compose up
# Open http://localhost:8089
```

### Cleanup

```bash
cd ../terraform/
terraform destroy -auto-approve
```
