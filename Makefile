APP_NAME = inventory-api
IMAGE_NAME = inventory-tracker
PWD = $(shell pwd)

.SHELL := bash
.PHONY: help build up down logs migrate-dev migrate-test test setup clean ci-setup ci-test ci-local dev-setup

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk "BEGIN {FS = \":.*?## \"}; {printf \"%-20s %s\\n\", \$$1, \$$2}"

# Docker commands
build: ## Build the application
	docker compose build

up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

logs: ## Show application logs
	docker compose logs -f app

migrate-dev: ## Run migrations for development
	docker compose exec app soda migrate -e development

migrate-test: ## Run migrations for test
	docker compose exec app soda migrate -e test

test: ## Run tests
	docker compose exec app bash ./scripts/run-tests.sh

setup: ## Setup databases and run migrations
	docker compose exec app bash ./scripts/setup-db.sh

clean: ## Clean up containers and volumes
	docker compose down -v
	docker system prune -f

# CI commands
ci-setup: ## Setup for CI environment
	bash ./scripts/setup-db-ci.sh

ci-test: ## Run tests in CI environment
	bash ./scripts/run-tests-ci.sh

ci-local: ## Simulate CI locally
	bash ./scripts/setup-local-ci.sh

# Development workflow
dev-setup: build up setup ## Complete development setup
	@echo "Development environment is ready!"
	@echo "App: http://localhost:3000"
	@echo "pgAdmin: http://localhost:5050"


# build:
# 	go build -o $(APP_NAME) ./api/main.go

# run:
# 	go run ./api/main.go

# test:
# 	go test ./...

# docker-build:
# 	docker build -t $(IMAGE_NAME) .

# docker-run:
# 	docker-compose up --build

# docker-test:
# 	MSYS_NO_PATHCONV=1 docker run --rm \
#  	-v "$(PWD)":/app \
#  	-w /app \
#  	-e DATABASE_URL=postgres://postgres:example@host.docker.internal:5432/inventory?sslmode=disable \
#  	golang:1.24 \
#  	go test ./...
