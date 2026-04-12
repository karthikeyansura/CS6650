REGION       := us-west-2
PROJECT      := album-store
CLUSTER      := $(PROJECT)-cluster

.PHONY: tidy build-api build-worker docker-api docker-worker push deploy smoke submit leaderboard locust tf-init tf-plan tf-apply

# ── Go ──────────────────────────────────────────────────────────────

tidy:
	go mod tidy

build-api:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/api ./cmd/api

build-worker:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/worker ./cmd/worker

# ── Docker ──────────────────────────────────────────────────────────

docker-api:
	docker build -t $(PROJECT)-api:latest -f deployments/docker/Dockerfile.api .

docker-worker:
	docker build -t $(PROJECT)-worker:latest -f deployments/docker/Dockerfile.worker .

# ── ECR push ────────────────────────────────────────────────────────

push:
	chmod +x scripts/build_push.sh
	./scripts/build_push.sh

# ── Deploy ──────────────────────────────────────────────────────────

deploy:
	chmod +x scripts/deploy.sh
	./scripts/deploy.sh

force-deploy:
	aws ecs update-service --cluster $(CLUSTER) --service $(PROJECT)-api --force-new-deployment --region $(REGION) --no-cli-pager
	aws ecs update-service --cluster $(CLUSTER) --service $(PROJECT)-worker --force-new-deployment --region $(REGION) --no-cli-pager

# ── Testing ─────────────────────────────────────────────────────────

smoke:
	@echo "Usage: make smoke BASE_URL=http://your-alb-dns"
	chmod +x scripts/smoke_test.sh
	./scripts/smoke_test.sh $(BASE_URL)

submit:
	@echo "Usage: make submit EMAIL=you@northeastern.edu NICKNAME=yourname BASE_URL=http://your-alb-dns"
	chmod +x scripts/submit.sh
	./scripts/submit.sh $(EMAIL) $(NICKNAME) $(BASE_URL)

leaderboard:
	chmod +x scripts/leaderboard.sh
	./scripts/leaderboard.sh

locust:
	@echo "Usage: make locust BASE_URL=http://your-alb-dns"
	cd locust && locust -f locustfile.py --host=$(BASE_URL)

# ── Terraform ───────────────────────────────────────────────────────

tf-init:
	cd terraform && terraform init

tf-plan:
	cd terraform && terraform plan

tf-apply:
	cd terraform && terraform apply
