.PHONY: help build test test-coverage test-race run fmt vet lint clean docker-build docker-up docker-down

# Default target
help:
	@echo "Available targets:"
	@echo "  make build          - Compile binary"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Generate HTML coverage report"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make run            - Start load balancer locally"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-up      - Start docker-compose demo"
	@echo "  make docker-down    - Stop docker-compose demo"

# Build binary
build:
	@echo "Building load-balancer..."
	@go build -o build/load-balancer ./cmd/
	@echo "Binary created at build/load-balancer"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -cover $(shell go list ./... | grep -v /scripts)

# Generate coverage report
test-coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out $(shell go list ./... | grep -v /scripts)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | grep total:

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race -v $(shell go list ./... | grep -v /scripts)

# Run the application
run: build
	@echo "Starting load balancer..."
	@./build/load-balancer

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w . 2>/dev/null || echo "Note: install goimports with: go install golang.org/x/tools/cmd/goimports@latest"

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet $(shell go list ./... | grep -v /scripts)

# Run linter
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run --exclude-dirs=scripts || echo "Note: install golangci-lint from https://golangci-lint.run/usage/install/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf build/
	@rm -f coverage.out coverage.html
# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t load-balancer:latest .
	@echo "Docker image built: load-balancer:latest"

# Start docker-compose demo environment
docker-up:
	@echo "Starting docker-compose demo..."
	@docker-compose up -d
	@echo "Demo environment running. Access load balancer at http://localhost:8080"
	@echo "Use 'make docker-down' to stop"

# Stop docker-compose demo environment
docker-down:
	@echo "Stopping docker-compose demo..."
	@docker-compose down
	@echo "Demo environment stopped"
	@docker build -t load-balancer:latest .
	@echo "Docker image built: load-balancer:latest"
