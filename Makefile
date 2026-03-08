# vrackconverter Makefile
# Build automation for vrackconverter

# Variables
BINARY_NAME=vrackconverter
GUI_BINARY_NAME=vrackconverter-gui
CMD_DIR=./cmd/vrackconverter
GUI_CMD_DIR=./cmd/vrackconverter-gui
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

# Default target
.PHONY: all
all: fmt vet test build

# Build for current platform
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) (version $(VERSION))..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BINARY_NAME)"

# Build GUI for current platform
.PHONY: build-gui
build-gui:
	@echo "Building $(GUI_BINARY_NAME) (version $(VERSION))..."
	$(GOBUILD) $(LDFLAGS) -o $(GUI_BINARY_NAME) $(GUI_CMD_DIR)
	@echo "Build complete: $(GUI_BINARY_NAME)"

# Run the GUI application
.PHONY: gui-run
gui-run: build-gui
	@echo "Running $(GUI_BINARY_NAME)..."
	./$(GUI_BINARY_NAME)

# Build for all platforms
.PHONY: build-all
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@echo "Building CLI linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-linux-amd64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-linux-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-linux-amd64
	@echo "Building CLI linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-linux-arm64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-linux-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-linux-arm64
	@echo "Building CLI darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-darwin-amd64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-darwin-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-darwin-amd64
	@echo "Building CLI darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-darwin-arm64 $(CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-darwin-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-darwin-arm64
	@echo "Building CLI windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-windows-amd64.exe $(CMD_DIR)
	cd $(BUILD_DIR) && zip -q vrackconverter-windows-amd64.zip vrackconverter-windows-amd64.exe
	@echo "Building CLI windows/arm64..."
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-windows-arm64.exe $(CMD_DIR)
	cd $(BUILD_DIR) && zip -q vrackconverter-windows-arm64.zip vrackconverter-windows-arm64.exe
	@echo "Building GUI darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-gui-darwin-arm64 $(GUI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-gui-darwin-arm64.tar.gz -C $(BUILD_DIR) vrackconverter-gui-darwin-arm64
	@echo "Building GUI darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-gui-darwin-amd64 $(GUI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-gui-darwin-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-gui-darwin-amd64
	@echo "Building GUI linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-gui-linux-amd64 $(GUI_CMD_DIR)
	tar -czf $(BUILD_DIR)/vrackconverter-gui-linux-amd64.tar.gz -C $(BUILD_DIR) vrackconverter-gui-linux-amd64
	@echo "Building GUI windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/vrackconverter-gui-windows-amd64.exe $(GUI_CMD_DIR)
	cd $(BUILD_DIR) && zip -q vrackconverter-gui-windows-amd64.zip vrackconverter-gui-windows-amd64.exe
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

# Install binary to GOPATH/bin or /usr/local/bin
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $$GOPATH/bin/$(BINARY_NAME) $(CMD_DIR) || \
		$(GOBUILD) $(LDFLAGS) -o /usr/local/bin/$(BINARY_NAME) $(CMD_DIR)
	@echo "Installed $(BINARY_NAME)"

# Install GUI binary to GOPATH/bin or /usr/local/bin
.PHONY: install-gui
install-gui:
	@echo "Installing $(GUI_BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $$GOPATH/bin/$(GUI_BINARY_NAME) $(GUI_CMD_DIR) || \
		$(GOBUILD) $(LDFLAGS) -o /usr/local/bin/$(GUI_BINARY_NAME) $(GUI_CMD_DIR)
	@echo "Installed $(GUI_BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(GUI_BINARY_NAME)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Show version info
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "CLI Binary: $(BINARY_NAME)"
	@echo "GUI Binary: $(GUI_BINARY_NAME)"
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
	@echo "  build         Build CLI for current platform"
	@echo "  build-gui     Build GUI for current platform"
	@echo "  build-all     Build CLI and GUI for all platforms (linux, darwin, windows)"
	@echo "  gui-run       Build and run the GUI application"
	@echo "  test          Run tests (shows only failures)"
	@echo "  fmt           Format Go code"
	@echo "  vet           Run go vet"
	@echo "  install       Install CLI to $$GOPATH/bin or /usr/local/bin"
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
	@echo "  make build-gui"
	@echo "  make gui-run"
	@echo "  VERSION=1.0.0 make build"
	@echo "  make build-all"
	@echo "  make test"
