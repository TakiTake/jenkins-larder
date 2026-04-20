# Jenkins Larder - Makefile for common development tasks

.PHONY: build test test-unit test-integration test-contract clean run docker-build docker-run setup lint fmt

# Build binary
build:
	go build -o bin/larder ./cmd/larder

# Run all tests
test: test-unit test-integration test-contract

# Run unit tests
test-unit:
	go test -v ./tests/unit/...

# Run integration tests
test-integration:
	go test -v -tags=integration ./tests/integration/...

# Run contract tests
test-contract:
	go test -v ./tests/contract/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf /tmp/jenkins-larder-test-*

# Run locally
run: build
	CONFIG_PATH=config/test.yaml ./bin/larder

# Build Docker image
docker-build:
	docker build -t jenkins-larder:latest .

# Run Docker container
docker-run:
	docker run -p 8080:8080 -p 8081:8081 -p 9090:9090 \
		-v $(PWD)/config:/etc/jenkins-larder \
		jenkins-larder:latest

# Run with Docker Compose (TODO: create docker-compose.yaml)
docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

# Set up git hooks and dev tools
setup:
	git config core.hooksPath .githooks

# Linting
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate mocks (if needed)
generate:
	go generate ./...

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build the binary"
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-contract      - Run contract tests"
	@echo "  clean              - Clean build artifacts"
	@echo "  run                - Run locally"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run Docker container"
	@echo "  lint               - Run linter"
	@echo "  fmt                - Format code"
	@echo "  deps               - Download dependencies"
