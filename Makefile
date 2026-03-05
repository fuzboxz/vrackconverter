# vrackconverter Makefile
# Build automation for vrackconverter

# Variables
BINARY_NAME=vrackconverter
CMD_DIR=./cmd/vrackconverter
BUILD_DIR=./build
VERSION?=dev
LDFLAGS:=-ldflags "-X main.Version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Default target
.PHONY: all
all: fmt vet test build

# Build for current platform
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) (version $(VERSION))..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BINARY_NAME)"

# Build for all platforms
.PHONY: build-all
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	@echo "Building linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	@echo "Building darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	@echo "Building darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	@echo "Building windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "All builds complete in $(BUILD_DIR)/"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.txt ./...
	@echo "Tests passed"

# Run tests with coverage report
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	$(GOCMD) tool coverage -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Code formatted"

# Run go vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet passed"

# Install binary to GOPATH/bin or /usr/local/bin
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $$GOPATH/bin/$(BINARY_NAME) $(CMD_DIR) || \
		$(GOBUILD) $(LDFLAGS) -o /usr/local/bin/$(BINARY_NAME) $(CMD_DIR)
	@echo "Installed $(BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.txt coverage.html
	@echo "Clean complete"

# Show version info
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Binary: $(BINARY_NAME)"
	@echo "Go version: $$($(GOCMD) version)"

# Generate SHA256 checksums for build artifacts
.PHONY: checksums
checksums: build-all
	@echo "Generating SHA256 checksums..."
	cd $(BUILD_DIR) && \
		shasum -a 256 $$(ls $(BINARY_NAME)-* 2>/dev/null || ls $(BINARY_NAME)-*.exe 2>/dev/null) > checksums.txt && \
		cat checksums.txt
	@echo "Checksums written to $(BUILD_DIR)/checksums.txt"

# Show help
.PHONY: help
help:
	@echo "vrackconverter Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Run fmt, vet, test, and build (default)"
	@echo "  build         Build for current platform"
	@echo "  build-all     Build for all platforms (linux, darwin, windows)"
	@echo "  test          Run tests with verbose output"
	@echo "  test-coverage Run tests and generate coverage report"
	@echo "  fmt           Format Go code"
	@echo "  vet           Run go vet"
	@echo "  install       Install to $$GOPATH/bin or /usr/local/bin"
	@echo "  clean         Remove build artifacts"
	@echo "  checksums     Generate SHA256 checksums for all builds"
	@echo "  version       Show version info"
	@echo "  help          Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION       Version string to inject (default: dev)"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  VERSION=1.0.0 make build"
	@echo "  make build-all"
	@echo "  make test"
