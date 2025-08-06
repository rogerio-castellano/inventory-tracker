APP_NAME = inventory-api
IMAGE_NAME = inventory-tracker
PWD = $(shell pwd)

.SHELL := bash
.PHONY: help build up down logs migrate-dev migrate-test test setup clean ci-setup ci-test ci-local dev-setup

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk "BEGIN {FS = \":.*?## \"}; {printf \"%-20s %s\\n\", \$$1, \$$2}"

# Docker commands
build: ## Build the application
	 $(MAKE) down
	docker compose build
	$(MAKE) up
	
up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

logs: ## Show application logs
	docker compose logs -f api

migrate-dev: ## Run migrations for inventory
	docker compose exec api soda migrate -e inventory

test: ## Run tests
	go test ./...

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
