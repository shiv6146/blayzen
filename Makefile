# Blayzen Voice Agent Framework Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Binary names
BINARY_NAME=blayzen
BINARY_UNIX=$(BINARY_NAME)_unix

# Build info
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Directories
BUILD_DIR=build
DIST_DIR=dist
TEST_DIR=test
COVERAGE_DIR=coverage

# Docker
DOCKER_IMAGE=blayzen:$(VERSION)

.PHONY: help
help: ## Display this help screen
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: fmt
fmt: ## Format Go code
	$(GOFMT) ./...

.PHONY: vet
vet: ## Run go vet
	$(GOVET) ./...

.PHONY: lint
lint: ## Run golangci-lint
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2)
	golangci-lint run

##@ Build

.PHONY: build
build: deps ## Build the binary
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/blayzen

.PHONY: build-all
build-all: ## Build for all platforms
	@mkdir -p $(DIST_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/blayzen
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/blayzen
	# Darwin AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/blayzen
	# Darwin ARM64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/blayzen
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/blayzen

.PHONY: build-examples
build-examples: ## Build example applications
	@mkdir -p $(BUILD_DIR)/examples
	$(GOBUILD) -o $(BUILD_DIR)/examples/simple-echo-bot ./examples/simple-echo-bot
	$(GOBUILD) -o $(BUILD_DIR)/examples/advanced-voice-bot ./examples/advanced-voice-bot
	$(GOBUILD) -o $(BUILD_DIR)/examples/json-config-bot ./examples/advanced-voice-bot

##@ Test

.PHONY: test
test: ## Run all tests (unit and e2e)
	@mkdir -p $(TEST_DIR)
	@echo "Running unit tests..."
	$(GOTEST) -v ./test/... -timeout 30s
	@echo "Running E2E tests..."
	$(GOTEST) -v -tags=e2e ./test/e2e/... -timeout 300s

.PHONY: test-race
test-race: ## Run tests with race detector
	@mkdir -p $(TEST_DIR)
	$(GOTEST) -v -race ./... -timeout 60s

.PHONY: test-cover
test-cover: ## Run tests with coverage
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

.PHONY: test-integration
test-integration: ## Run integration tests
	@mkdir -p $(TEST_DIR)
	$(GOTEST) -v -tags=integration ./test/integration/... -timeout 120s

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@mkdir -p $(TEST_DIR)
	$(GOTEST) -v -tags=e2e ./test/e2e/... -timeout 300s

.PHONY: benchmark
benchmark: ## Run benchmarks
	@mkdir -p $(TEST_DIR)
	$(GOTEST) -v -bench=. -benchmem ./... -timeout 120s | tee $(TEST_DIR)/benchmark.txt

##@ Quality

.PHONY: check
check: fmt vet lint test ## Run all quality checks

.PHONY: security
security: ## Run security scan
	@which gosec > /dev/null || (echo "Installing gosec..." && $(GOGET) github.com/securecodewarrior/gosec/v2/cmd/gosec)
	gosec ./...

.PHONY: vuln
vuln: ## Check for vulnerabilities
	@which govulncheck > /dev/null || (echo "Installing govulncheck..." && $(GOGET) golang.org/x/vuln/cmd/govulncheck)
	govulncheck ./...

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

.PHONY: docker-run
docker-run: ## Run Docker container
	docker run -p 8080:8080 $(DOCKER_IMAGE)

.PHONY: docker-push
docker-push: ## Push Docker image
	docker push $(DOCKER_IMAGE)

##@ Clean

.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -rf $(TEST_DIR)
	rm -rf $(COVERAGE_DIR)
	rm -f coverage.out

.PHONY: clean-cache
clean-cache: ## Clean Go module cache
	$(GOMOD) clean -modcache

##@ Run

.PHONY: run-simple
run-simple: ## Run simple echo bot
	$(GOBUILD) -o $(BUILD_DIR)/simple-echo-bot ./examples/simple-echo-bot
	EXOTEL_WS_URL=ws://localhost:8080/ws $(BUILD_DIR)/simple-echo-bot

.PHONY: run-advanced
run-advanced: ## Run advanced voice bot
	$(GOBUILD) -o $(BUILD_DIR)/advanced-voice-bot ./examples/advanced-voice-bot
	EXOTEL_WS_URL=ws://localhost:8080/ws OPENAI_API_KEY=$(OPENAI_API_KEY) $(BUILD_DIR)/advanced-voice-bot -model gpt-3.5-turbo

.PHONY: run-json
run-json: ## Run JSON config bot
	$(GOBUILD) -o $(BUILD_DIR)/json-config-bot ./examples/advanced-voice-bot
	EXOTEL_WS_URL=ws://localhost:8080/ws OPENAI_API_KEY=$(OPENAI_API_KEY) $(BUILD_DIR)/json-config-bot -config examples/agent-configs/simple-assistant.json

##@ Mock Server

.PHONY: run-mock-server
run-mock-server: ## Run mock Exotel server for testing
	$(GOBUILD) -o $(BUILD_DIR)/mock-server ./test/mock-server
	$(BUILD_DIR)/mock-server -port 8080

##@ Documentation

.PHONY: docs
docs: ## Generate documentation
	@which godoc > /dev/null || (echo "Installing godoc..." && $(GOGET) golang.org/x/tools/cmd/godoc)
	godoc -http=:6060

.PHONY: docs-gen
docs-gen: ## Generate documentation from code
	@which swag > /dev/null || (echo "Installing swag..." && $(GOGET) github.com/swaggo/swag/cmd/swag)
	swag init -g cmd/blayzen/main.go

##@ Release

.PHONY: release
release: clean check test-cover build-all ## Prepare release
	@echo "Release ready in $(DIST_DIR)"
	@ls -la $(DIST_DIR)

.PHONY: version
version: ## Show version info
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"

# Default target
.DEFAULT_GOAL := help