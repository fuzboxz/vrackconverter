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
	@mkdir -p $(BUILD_DIR)/vrackconverter-linux-amd64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-linux-amd64/$(BINARY_NAME) $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-linux-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-linux-amd64
	@echo "Building linux/arm64..."
	@mkdir -p $(BUILD_DIR)/vrackconverter-linux-arm64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-linux-arm64/$(BINARY_NAME) $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-linux-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-linux-arm64
	@echo "Building darwin/amd64..."
	@mkdir -p $(BUILD_DIR)/vrackconverter-darwin-amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-darwin-amd64/$(BINARY_NAME) $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-darwin-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-darwin-amd64
	@echo "Building darwin/arm64..."
	@mkdir -p $(BUILD_DIR)/vrackconverter-darwin-arm64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-darwin-arm64/$(BINARY_NAME) $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-darwin-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-darwin-arm64
	@echo "Building windows/amd64..."
	@mkdir -p $(BUILD_DIR)/vrackconverter-windows-amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-windows-amd64/$(BINARY_NAME).exe $(CMD_DIR)
	cd $(BUILD_DIR)/vrackconverter-windows-amd64 && zip -q ../vrackconverter-windows-amd64.zip $(BINARY_NAME).exe
	@echo "Building windows/arm64..."
	@mkdir -p $(BUILD_DIR)/vrackconverter-windows-arm64
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-windows-arm64/$(BINARY_NAME).exe $(CMD_DIR)
	cd $(BUILD_DIR)/vrackconverter-windows-arm64 && zip -q ../vrackconverter-windows-arm64.zip $(BINARY_NAME).exe
	@echo "All builds complete in $(BUILD_DIR)/"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...
	@echo "Tests passed"

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
	@echo "Clean complete"

# Clean intermediate build directories (keep archives)
.PHONY: clean-intermediate
clean-intermediate:
	@echo "Cleaning intermediate build directories..."
	for dir in $(BUILD_DIR)/vrackconverter-linux-* $(BUILD_DIR)/vrackconverter-darwin-* $(BUILD_DIR)/vrackconverter-windows-*; do \
		if [ -d "$$dir" ]; then \
			rm -rf "$$dir"; \
		fi; \
	done
	@echo "Intermediate directories cleaned"

# Show version info
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Binary: $(BINARY_NAME)"
	@echo "Go version: $$($(GOCMD) version)"

# Generate SHA256 checksums for build artifacts
.PHONY: checksums
checksums: build-all clean-intermediate
	@echo "Generating SHA256 checksums..."
	cd $(BUILD_DIR) && shasum -a 256 *.tar.gz *.zip > checksums.txt
	@cat $(BUILD_DIR)/checksums.txt
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
