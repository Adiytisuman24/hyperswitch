.PHONY: help build test run clean fmt lint docker-build docker-run

# Variables
BINARY_NAME=hyperswitch-go-ext
DOCKER_IMAGE=hyperswitch-go-extensions:latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) cmd/server/main.go
	@echo "Build complete: bin/$(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	go test -v -cover ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

run: ## Run the application
	@echo "Starting $(BINARY_NAME)..."
	go run cmd/server/main.go

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run
	@echo "Lint complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies downloaded"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8081:8081 --env-file .env $(DOCKER_IMAGE)

dev: ## Run in development mode with auto-reload
	@echo "Starting development server..."
	air

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed"

generate-docs: ## Generate API documentation
	@echo "Generating API documentation..."
	@echo "API docs available at: docs/API.md"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Default target
all: clean fmt test build
