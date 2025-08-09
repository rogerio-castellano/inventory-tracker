APP_NAME = inventory-api
IMAGE_NAME = inventory-tracker
PWD = $(shell pwd)

.SHELL := bash
.PHONY: help build-go test fast-test lint docs 

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk "BEGIN {FS = \":.*?## \"}; {printf \"%-20s %s\\n\", \$$1, \$$2}"

# Go targets
bg:build-go
build-go:
	@start=$$(date +%s); \
	go build ./api/main.go; \
	end=$$(date +%s); \
	echo "$$((end - start)) seconds to build"

test: ## Run all tests
	go test ./...

t:fast-test
fast-test: ## Run unit tests
	go test ./internal/tests/handlers_integrated_test_suite

lint:
	golangci-lint run

docs:
	swag init -g api/main.go --output api/docs

.PHONY: docker-build-dev build up down logs migrate-dev setup clean ci-setup ci-test ci-local dev-setup

# Docker targets
b:docker-build-dev
docker-build-dev:
	@start=$$(date +%s); \
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o inventory-api ./api/main.go; \
	docker compose -f docker-compose.yml -f docker-compose-fast.yml build; \
	end=$$(date +%s); \
	echo "Build completed in $$((end - start))s (local + container)"
	docker-compose down api
	docker-compose -f docker-compose.yml -f docker-compose-fast.yml up -d api

bd: build
build: ## Build the application
	docker compose build
	$(MAKE) down
	$(MAKE) up

up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

logs: ## Show application logs
	docker compose logs -f api

migrate-dev: ## Run migrations for development
	docker compose exec api soda migrate -e development

setup: ## Setup databases and run migrations
	docker compose exec api bash ./scripts/setup-db.sh

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
