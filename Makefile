# Makefile for aft-pipeline-tool

# Variables
BINARY_NAME=aft-pipeline-tool
MAIN_PACKAGE=.
BUILD_DIR=build
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT?=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X github.com/hacker65536/aft-pipeline-tool/cmd.Version=$(VERSION) -X github.com/hacker65536/aft-pipeline-tool/cmd.GitCommit=$(GIT_COMMIT) -X github.com/hacker65536/aft-pipeline-tool/cmd.BuildDate=$(BUILD_DATE)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Default target
.PHONY: all
all: clean deps test build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	# Temporarily commented out for beta version to speed up CI - only building for Mac ARM
	# GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	# GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	# GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	# GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run; \
	else \
		echo "golangci-lint not installed. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

# Check code quality
.PHONY: check
check: fmt vet lint test

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install the binary to GOPATH/bin
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) $(MAIN_PACKAGE)

# Uninstall the binary from GOPATH/bin
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	rm -f $(GOPATH)/bin/$(BINARY_NAME)

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Run with list command
.PHONY: run-list
run-list: build
	@echo "Running $(BINARY_NAME) list..."
	./$(BINARY_NAME) list

# Run with help
.PHONY: run-help
run-help: build
	@echo "Running $(BINARY_NAME) help..."
	./$(BINARY_NAME) --help

# Development setup
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint

# Update dependencies
.PHONY: update-deps
update-deps:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Security audit
.PHONY: audit
audit:
	@echo "Running security audit..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed. Install it with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Generate documentation
.PHONY: docs
docs:
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Starting godoc server at http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "godoc not installed. Install it with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Create release archive
.PHONY: release
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/releases
	@for binary in $(BUILD_DIR)/$(BINARY_NAME)-*; do \
		if [ -f "$$binary" ]; then \
			base=$$(basename "$$binary"); \
			if [[ "$$base" == *".exe" ]]; then \
				zip -j "$(BUILD_DIR)/releases/$${base%.exe}.zip" "$$binary" README.md; \
			else \
				tar -czf "$(BUILD_DIR)/releases/$$base.tar.gz" -C $(BUILD_DIR) "$$(basename "$$binary")" -C .. README.md; \
			fi; \
		fi; \
	done
	@echo "Release archives created in $(BUILD_DIR)/releases/"

# GoReleaser targets
.PHONY: goreleaser-check
goreleaser-check:
	@echo "Checking GoReleaser configuration..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser check; \
	else \
		echo "GoReleaser not installed. Install it from https://goreleaser.com/install/"; \
	fi

.PHONY: goreleaser-build
goreleaser-build:
	@echo "Building with GoReleaser (snapshot)..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser build --snapshot --clean; \
	else \
		echo "GoReleaser not installed. Install it from https://goreleaser.com/install/"; \
	fi

.PHONY: goreleaser-release
goreleaser-release:
	@echo "Creating release with GoReleaser..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --clean; \
	else \
		echo "GoReleaser not installed. Install it from https://goreleaser.com/install/"; \
	fi

.PHONY: goreleaser-release-dry
goreleaser-release-dry:
	@echo "Dry run release with GoReleaser..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --skip=publish --clean; \
	else \
		echo "GoReleaser not installed. Install it from https://goreleaser.com/install/"; \
	fi

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all                 - Clean, install deps, test, and build"
	@echo "  build               - Build the binary"
	@echo "  build-all           - Build for multiple platforms"
	@echo "  deps                - Install dependencies"
	@echo "  test                - Run tests"
	@echo "  test-coverage       - Run tests with coverage report"
	@echo "  bench               - Run benchmarks"
	@echo "  fmt                 - Format code"
	@echo "  lint                - Lint code (requires golangci-lint)"
	@echo "  vet                 - Vet code"
	@echo "  check               - Run fmt, vet, lint, and test"
	@echo "  clean               - Clean build artifacts"
	@echo "  install             - Install binary to GOPATH/bin"
	@echo "  uninstall           - Remove binary from GOPATH/bin"
	@echo "  run                 - Build and run the application"
	@echo "  run-list            - Build and run with list command"
	@echo "  run-help            - Build and run with help"
	@echo "  dev-setup           - Setup development environment"
	@echo "  update-deps         - Update dependencies"
	@echo "  audit               - Run security audit (requires govulncheck)"
	@echo "  docs                - Generate and serve documentation"
	@echo "  release             - Create release archives for all platforms"
	@echo "  goreleaser-check    - Check GoReleaser configuration"
	@echo "  goreleaser-build    - Build with GoReleaser (snapshot)"
	@echo "  goreleaser-release  - Create release with GoReleaser"
	@echo "  goreleaser-release-dry - Dry run release with GoReleaser"
	@echo "  help                - Show this help message"
