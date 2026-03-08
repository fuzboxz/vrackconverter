# vrackconverter Makefile
# Build automation for vrackconverter

# Variables
BINARY_NAME=vrackconverter
CLI_BINARY_NAME=vrackconverter-cli
CMD_DIR=./cmd/vrackconverter-gui
CLI_CMD_DIR=./cmd/vrackconverter
BUILD_DIR=./build
VERSION?=dev
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
BUILD_INFO=$(GOOS)$(GOARCH)@$(VERSION) $(GIT_COMMIT)
LDFLAGS:=-ldflags "-X main.Version=$(VERSION) -X 'main.BuildInfo=$(BUILD_INFO)'"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Default target (builds both GUI and CLI since tests need CLI)
.PHONY: all
all: fmt vet build-cli test build

# Build for current platform (GUI is default)
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) GUI (version $(VERSION))..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BINARY_NAME)"

# Build CLI for current platform
.PHONY: build-cli
build-cli:
	@echo "Building $(CLI_BINARY_NAME) CLI (version $(VERSION))..."
	$(GOBUILD) $(LDFLAGS) -o $(CLI_BINARY_NAME) $(CLI_CMD_DIR)
	@echo "Build complete: $(CLI_BINARY_NAME)"

# Run the GUI application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Package macOS app bundle
.PHONY: package
package: build
	@echo "Packaging $(BINARY_NAME).app..."
	@./scripts/macos-package.sh $(BINARY_NAME) $(VERSION)

# Build for all platforms
.PHONY: build-all
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building CLI linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-cli-linux-amd64 $(CLI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-cli-linux-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-cli-linux-amd64
	@echo "Building CLI linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-cli-linux-arm64 $(CLI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-cli-linux-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-cli-linux-arm64
	@echo "Building CLI darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-cli-darwin-amd64 $(CLI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-cli-darwin-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-cli-darwin-amd64
	@echo "Building CLI darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-cli-darwin-arm64 $(CLI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-cli-darwin-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-cli-darwin-arm64
	@echo "Building CLI windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-cli-windows-amd64.exe $(CLI_CMD_DIR)
	cd $(BUILD_DIR) && zip -q vrackconverter-cli-windows-amd64.zip vrackconverter-cli-windows-amd64.exe
	@echo "Building CLI windows/arm64..."
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-cli-windows-arm64.exe $(CLI_CMD_DIR)
	cd $(BUILD_DIR) && zip -q vrackconverter-cli-windows-arm64.zip vrackconverter-cli-windows-arm64.exe
	@echo "Building GUI darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-darwin-arm64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-darwin-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-darwin-arm64
	@echo "Building GUI darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-darwin-amd64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-darwin-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-darwin-amd64
	@echo "Building GUI linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-linux-amd64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-linux-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-linux-amd64
	@echo "Building GUI windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-windows-amd64.exe $(CMD_DIR)
	cd $(BUILD_DIR) && zip -q vrackconverter-windows-amd64.zip vrackconverter-windows-amd64.exe
	@echo "All builds complete in $(BUILD_DIR)/"

# Run tests (shows only failed tests by default)
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -race ./...
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

# Install binary to GOPATH/bin or /usr/local/bin (GUI is default)
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $$GOPATH/bin/$(BINARY_NAME) $(CMD_DIR) || \
		$(GOBUILD) $(LDFLAGS) -o /usr/local/bin/$(BINARY_NAME) $(CMD_DIR)
	@echo "Installed $(BINARY_NAME)"

# Install CLI binary to GOPATH/bin or /usr/local/bin
.PHONY: install-cli
install-cli:
	@echo "Installing $(CLI_BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $$GOPATH/bin/$(CLI_BINARY_NAME) $(CLI_CMD_DIR) || \
		$(GOBUILD) $(LDFLAGS) -o /usr/local/bin/$(CLI_BINARY_NAME) $(CLI_CMD_DIR)
	@echo "Installed $(CLI_BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(CLI_BINARY_NAME)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Show version info
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "GUI Binary: $(BINARY_NAME)"
	@echo "CLI Binary: $(CLI_BINARY_NAME)"
	@echo "Go version: $$($(GOCMD) version)"

# Generate SHA256 checksums for build artifacts
.PHONY: checksums
checksums: build-all
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
	@echo "  build         Build GUI for current platform"
	@echo "  build-cli     Build CLI for current platform"
	@echo "  build-all     Build GUI and CLI for all platforms (linux, darwin, windows)"
	@echo "  package       Package macOS .app bundle (GUI with icon)"
	@echo "  run           Build and run the GUI application"
	@echo "  test          Run tests (shows only failures)"
	@echo "  fmt           Format Go code"
	@echo "  vet           Run go vet"
	@echo "  install       Install GUI to $$GOPATH/bin or /usr/local/bin"
	@echo "  install-cli   Install CLI to $$GOPATH/bin or /usr/local/bin"
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
	@echo "  make build-cli"
	@echo "  make package"
	@echo "  make run"
	@echo "  VERSION=1.0.0 make build"
	@echo "  make build-all"
	@echo "  make test"
