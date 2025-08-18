# gofs - Makefile

##@ Project Configuration
# ------------------------------------------------------------------------------
PROJECT_NAME := gofs
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | awk '{print $$3}')

# Binary and build configuration
BINARY_NAME := $(PROJECT_NAME)
BUILD_DIR := ./build

# Platform detection
HOST_OS := $(shell go env GOOS)
HOST_ARCH := $(shell go env GOARCH)

# Go build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.goVersion=$(GO_VERSION)"

# Build targets for different platforms
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

# Docker configuration for local development
DOCKER_IMAGE := $(PROJECT_NAME)
DOCKER_PLATFORMS := linux/amd64,linux/arm64

# Terminal colors for output formatting
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
PURPLE := \033[0;35m
CYAN := \033[0;36m
NC := \033[0m

##@ Dependencies Management
# ------------------------------------------------------------------------------
# Function to check and install Go tools
# Usage: $(call ensure-go-tool,tool-name,install-command)
define ensure-go-tool
	@if ! command -v $(1) >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing $(1)...$(NC)"; \
		$(2); \
	fi
endef

# Function to check and install external tools via script
# Usage: $(call ensure-external-tool,tool-name,install-script)
define ensure-external-tool
	@if ! command -v $(1) >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing $(1)...$(NC)"; \
		$(2); \
	fi
endef

# Dependency installation commands
GOIMPORTS_INSTALL := go install golang.org/x/tools/cmd/goimports@latest
GOLANGCI_LINT_INSTALL := curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest
GOSEC_INSTALL := go install github.com/securego/gosec/v2/cmd/gosec@latest

.PHONY: install-deps
install-deps: ## Install all development dependencies
	@echo "$(BLUE)Installing development dependencies...$(NC)"
	$(call ensure-go-tool,goimports,$(GOIMPORTS_INSTALL))
	$(call ensure-external-tool,golangci-lint,$(GOLANGCI_LINT_INSTALL))
	$(call ensure-go-tool,gosec,$(GOSEC_INSTALL))
	@echo "$(GREEN)All dependencies installed$(NC)"

.PHONY: check-deps
check-deps: ## Check if all required dependencies are installed
	@echo "$(BLUE)Checking development dependencies...$(NC)"
	@missing_deps=""; \
	for tool in goimports golangci-lint gosec; do \
		if ! command -v $$tool >/dev/null 2>&1; then \
			missing_deps="$$missing_deps $$tool"; \
		else \
			echo "$(GREEN)✓ $$tool is installed$(NC)"; \
		fi; \
	done; \
	if [ -n "$$missing_deps" ]; then \
		echo "$(RED)✗ Missing dependencies:$$missing_deps$(NC)"; \
		echo "$(YELLOW)Run 'make install-deps' to install missing dependencies$(NC)"; \
		exit 1; \
	else \
		echo "$(GREEN)All dependencies are installed$(NC)"; \
	fi

##@ General
.PHONY: help
help: ## Display available commands
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
.PHONY: fmt
fmt: ## Format Go code with goimports
	@echo "$(BLUE)Formatting Go code...$(NC)"
	$(call ensure-go-tool,goimports,$(GOIMPORTS_INSTALL))
	@goimports -w -local $(PROJECT_NAME) .
	@echo "$(GREEN)Code formatting completed$(NC)"

.PHONY: lint
lint: ## Run golangci-lint code analysis
	@echo "$(BLUE)Running code analysis...$(NC)"
	$(call ensure-external-tool,golangci-lint,$(GOLANGCI_LINT_INSTALL))
	golangci-lint run
	@echo "$(GREEN)Code analysis completed$(NC)"

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix
	@echo "$(BLUE)Running code analysis with auto-fix...$(NC)"
	$(call ensure-external-tool,golangci-lint,$(GOLANGCI_LINT_INSTALL))
	golangci-lint run --fix
	@echo "$(GREEN)Code analysis and fixes completed$(NC)"

.PHONY: sec
sec: ## Run security analysis with gosec
	@echo "$(BLUE)Running security analysis...$(NC)"
	$(call ensure-go-tool,gosec,$(GOSEC_INSTALL))
	@mkdir -p $(BUILD_DIR)
	gosec -fmt sarif -out $(BUILD_DIR)/gosec-report.sarif -no-fail ./...
	gosec -fmt text -no-fail ./...
	@echo "$(GREEN)Security analysis completed$(NC)"
	@echo "$(CYAN)Report saved to: $(BUILD_DIR)/gosec-report.sarif$(NC)"

##@ Build
.PHONY: build
build: ## Build binary for current platform
	@echo "$(BLUE)Building Go binary for $(HOST_OS)/$(HOST_ARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=$(HOST_OS) GOARCH=$(HOST_ARCH) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)$(if $(filter windows,$(HOST_OS)),.exe,) ./cmd/gofs
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)$(if $(filter windows,$(HOST_OS)),.exe,)$(NC)"

.PHONY: build-linux
build-linux: ## Build binary for Linux (amd64)
	@echo "$(BLUE)Building Go binary for linux/amd64...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/gofs
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64$(NC)"

.PHONY: build-linux-arm
build-linux-arm: ## Build binary for Linux (arm64)
	@echo "$(BLUE)Building Go binary for linux/arm64...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/gofs
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64$(NC)"

.PHONY: build-darwin
build-darwin: ## Build binary for macOS (amd64)
	@echo "$(BLUE)Building Go binary for darwin/amd64...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/gofs
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64$(NC)"

.PHONY: build-darwin-arm
build-darwin-arm: ## Build binary for macOS (arm64/M1)
	@echo "$(BLUE)Building Go binary for darwin/arm64...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/gofs
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64$(NC)"

.PHONY: build-windows
build-windows: ## Build binary for Windows (amd64)
	@echo "$(BLUE)Building Go binary for windows/amd64...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/gofs
	@echo "$(GREEN)Build completed: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe$(NC)"

.PHONY: build-all
build-all: ## Build binaries for all supported platforms
	@echo "$(BLUE)Building Go binaries for all platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "$(CYAN)Building for $$os/$$arch...$(NC)"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext ./cmd/gofs; \
		if [ $$? -eq 0 ]; then \
			echo "$(GREEN)✓ Built $(BUILD_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext$(NC)"; \
		else \
			echo "$(RED)✗ Failed to build for $$os/$$arch$(NC)"; \
		fi; \
	done
	@echo "$(GREEN)All builds completed!$(NC)"
	@echo "$(BLUE)Built binaries:$(NC)"
	@ls -la $(BUILD_DIR)/

##@ Testing
.PHONY: test
test: ## Run all tests
	@echo "$(BLUE)Running tests...$(NC)"
	@go test -v ./...
	@echo "$(GREEN)All tests passed!$(NC)"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go test -v -coverprofile=$(BUILD_DIR)/coverage.out ./...
	@go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "$(GREEN)Coverage report generated: $(BUILD_DIR)/coverage.html$(NC)"
	@go tool cover -func=$(BUILD_DIR)/coverage.out | tail -1

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "$(BLUE)Running tests with race detection...$(NC)"
	@go test -race -v ./...
	@echo "$(GREEN)Race detection tests passed!$(NC)"

.PHONY: test-bench
test-bench: ## Run benchmark tests
	@echo "$(BLUE)Running benchmark tests...$(NC)"
	@go test -bench=. -benchmem ./...
	@echo "$(GREEN)Benchmark tests completed!$(NC)"

##@ Quality Assurance
.PHONY: check
check: fmt lint sec test ## Run complete quality checks (format + lint + security + test)
	@echo "$(GREEN)All quality checks passed!$(NC)"

##@ Setup
.PHONY: install-hooks
install-hooks: ## Install Git pre-commit hooks
	@echo "$(BLUE)Installing Git hooks...$(NC)"
	@./scripts/install-hooks.sh
	@echo "$(GREEN)Git hooks installation completed$(NC)"

##@ Cleanup
.PHONY: clean
clean: ## Clean build artifacts and test outputs
	@echo "$(BLUE)Cleaning build artifacts and test outputs...$(NC)"
	@rm -rf $(BUILD_DIR) gofs dist
	@echo "$(GREEN)Cleanup completed$(NC)"

##@ Docker
.PHONY: docker-build
docker-build: ## Build Docker image for local development
	@echo "$(BLUE)Building Docker image $(DOCKER_IMAGE):$(VERSION)...$(NC)"
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--tag $(DOCKER_IMAGE):$(VERSION) \
		--tag $(DOCKER_IMAGE):latest \
		.
	@echo "$(GREEN)Docker image built successfully$(NC)"

.PHONY: docker-build-multiarch
docker-build-multiarch: ## Build multi-architecture Docker image for local testing
	@echo "$(BLUE)Building multi-architecture Docker image...$(NC)"
	@if ! docker buildx ls | grep -q "multiarch-builder"; then \
		echo "$(YELLOW)Creating buildx builder instance...$(NC)"; \
		docker buildx create --name multiarch-builder --driver docker-container --bootstrap --use; \
	else \
		docker buildx use multiarch-builder; \
	fi
	@docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--tag $(DOCKER_IMAGE):$(VERSION) \
		--tag $(DOCKER_IMAGE):latest \
		--load \
		.
	@echo "$(GREEN)Multi-architecture Docker image built successfully$(NC)"

.PHONY: docker-run
docker-run: ## Run Docker container locally for testing
	@echo "$(BLUE)Running Docker container...$(NC)"
	@docker run --rm -it \
		-p 8000:8000 \
		-v $(PWD):/data:ro \
		$(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker container stopped$(NC)"

.PHONY: docker-clean
docker-clean: ## Clean local Docker images and build cache
	@echo "$(BLUE)Cleaning Docker images and build cache...$(NC)"
	@docker images $(DOCKER_IMAGE) -q | xargs -r docker rmi -f
	@docker buildx prune -f
	@echo "$(GREEN)Docker cleanup completed$(NC)"

.DEFAULT_GOAL := help
