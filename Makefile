.PHONY: build run test clean lint deps

# Build variables
BINARY_NAME=rag-server
BUILD_DIR=./build
MAIN_PATH=./cmd/server

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# Run the server
run:
	@echo "Running..."
	CGO_ENABLED=1 $(GORUN) $(MAIN_PATH)

# Run with specific config (usage: make run-config CONFIG=./configs/config.yaml)
CONFIG ?= ./configs/config.yaml
run-config:
	@echo "Running with config: $(CONFIG)"
	CGO_ENABLED=1 $(GORUN) $(MAIN_PATH) -config $(CONFIG)

# Run with Ollama (local, no API key needed)
run-ollama:
	@echo "Running with Ollama config (local)..."
	CGO_ENABLED=1 $(GORUN) $(MAIN_PATH) -config ./configs/config-ollama.yaml

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Generate (if needed)
generate:
	@echo "Running go generate..."
	$(GOCMD) generate ./...

# Create data directory
init-dirs:
	@echo "Creating directories..."
	mkdir -p data
	mkdir -p logs

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  run           - Run the server"
	@echo "  run-config    - Run with specific config (CONFIG=path)"
	@echo "  run-ollama    - Run with Ollama (local, no API key needed)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download dependencies"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  init-dirs     - Create necessary directories"
	@echo ""
	@echo "Quick start (local testing with Ollama):"
	@echo "  1. Install Ollama: https://ollama.ai"
	@echo "  2. Pull models: ollama pull nomic-embed-text && ollama pull llama3.2"
	@echo "  3. Run: make run-ollama"
	@echo "  4. Open: http://localhost:8080"

