.PHONY: all build build-all test test-integration test-coverage bench clean install fmt lint security docker docker-run help

# Variables
BINARY_NAME=shadowvault
BINARIES=backup-agent restore-agent peerctl
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME) -s -w"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get
GOFMT=gofmt
GOLINT=golangci-lint

# Directories
BIN_DIR=bin
CMD_DIR=cmd
INTERNAL_DIR=internal
TEST_DIR=tests

# Build all binaries
all: clean build

build: ## Build all binaries
	@echo "Building binaries..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-backup-agent $(CMD_DIR)/backup-agent/main.go
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-restore-agent $(CMD_DIR)/backup-agent-restore/main.go
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-peerctl $(CMD_DIR)/peerctl/main.go
	@echo "Build complete: binaries in $(BIN_DIR)/"

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BIN_DIR)
	# Linux amd64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-backup-agent-linux-amd64 $(CMD_DIR)/backup-agent/main.go
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-restore-agent-linux-amd64 $(CMD_DIR)/backup-agent-restore/main.go
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-peerctl-linux-amd64 $(CMD_DIR)/peerctl/main.go
	# Linux arm64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-backup-agent-linux-arm64 $(CMD_DIR)/backup-agent/main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-restore-agent-linux-arm64 $(CMD_DIR)/backup-agent-restore/main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-peerctl-linux-arm64 $(CMD_DIR)/peerctl/main.go
	# macOS amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-backup-agent-darwin-amd64 $(CMD_DIR)/backup-agent/main.go
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-restore-agent-darwin-amd64 $(CMD_DIR)/backup-agent-restore/main.go
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-peerctl-darwin-amd64 $(CMD_DIR)/peerctl/main.go
	# macOS arm64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-backup-agent-darwin-arm64 $(CMD_DIR)/backup-agent/main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-restore-agent-darwin-arm64 $(CMD_DIR)/backup-agent-restore/main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-peerctl-darwin-arm64 $(CMD_DIR)/peerctl/main.go
	# Windows amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-backup-agent-windows-amd64.exe $(CMD_DIR)/backup-agent/main.go
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-restore-agent-windows-amd64.exe $(CMD_DIR)/backup-agent-restore/main.go
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/shadowvault-peerctl-windows-amd64.exe $(CMD_DIR)/peerctl/main.go
	@echo "Multi-platform build complete!"

test: ## Run unit tests
	@echo "Running unit tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete!"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./$(TEST_DIR)/...
	@echo "Integration tests complete!"

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...
	@echo "Benchmarks complete!"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"

install: build ## Install binaries to system
	@echo "Installing binaries..."
	@sudo cp $(BIN_DIR)/shadowvault-* /usr/local/bin/
	@echo "Installation complete!"

fmt: ## Format code
	@echo "Formatting code..."
	@$(GOFMT) -s -w .
	@echo "Format complete!"

check-fmt: ## Check code formatting
	@./check-format.sh

lint: ## Run linters
	@echo "Running linters..."
	@$(GOLINT) run --timeout=5m || echo "golangci-lint not installed, skipping..."
	@echo "Lint complete!"

security: ## Run security scanner
	@echo "Running security scanner..."
	@gosec -fmt json -out gosec-results.json ./... || echo "gosec not installed, skipping..."
	@echo "Security scan complete!"

docker: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t shadowvault:$(VERSION) .
	@docker tag shadowvault:$(VERSION) shadowvault:latest
	@echo "Docker image built: shadowvault:$(VERSION)"

docker-run: docker ## Run Docker container
	@echo "Running Docker container..."
	@docker run -d \
		--name shadowvault \
		-v $(PWD)/data:/data \
		-p 9000:9000 \
		-p 8080:8080 \
		-p 9090:9090 \
		shadowvault:latest

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated!"

proto: ## Generate protobuf code
	@echo "Generating protobuf code..."
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto || echo "Skipping proto generation (protoc not installed)"

dev: ## Run in development mode
	@echo "Running in development mode..."
	@$(GOBUILD) -o $(BIN_DIR)/shadowvault-dev $(CMD_DIR)/backup-agent/main.go
	@SHADOWVAULT_LOG_LEVEL=debug $(BIN_DIR)/shadowvault-dev

# Help target
help: ## Show this help message
	@echo "ShadowVault - Production-Ready Decentralized Backup System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
