# Contributing to RackConverter

Thank you for your interest in contributing! This document covers how to build, test, and contribute to RackConverter.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/fuzboxz/vrackconverter.git
cd vrackconverter

# Build
make build

# Run tests
make test

# Convert a patch
./vrackconverter input.vcv -o output.vcv
```

## Prerequisites

- Go 1.23 or later
- Git
- Make (optional, but recommended)

## Building

### Build for Current Platform

```bash
make build
```

The binary will be created as `./vrackconverter`.

### Build for All Platforms

```bash
make build-all
```

This creates binaries for:
- linux-amd64, linux-arm64
- darwin-amd64, darwin-arm64
- windows-amd64, windows-arm64

### Install to System

```bash
make install
```

Installs to `$GOPATH/bin` or `/usr/local/bin`.

## Testing

### Run All Tests

```bash
make test
```

Tests run with the race detector enabled.

### Run Specific Test

```bash
# Run only converter tests
go test -v ./internal/converter/

# Run only e2e tests
go test -v ./test/

# Run a specific test
go test -v ./test/ -run TestE2E_MorningstarlingRegression
```

## Code Style

### Format Code

```bash
make fmt
```

### Check Code

```bash
make vet
```

## Development Workflow

1. **Create a branch** for your changes
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following the coding standards below

3. **Run tests** - tests must pass before committing
   ```bash
   make test
   ```

4. **Format and vet** your code
   ```bash
   make fmt && make vet
   ```

5. **Commit** your changes with a clear message
   ```bash
   git commit -m "Add feature: description of changes"
   ```

6. **Push** and create a pull request

## Coding Standards

### Go Conventions

Follow standard Go conventions from [Effective Go](https://go.dev/doc/effective_go).

### Error Handling

```go
// DO: Wrap errors with context
return fmt.Errorf("failed to read input file: %w", err)

// DON'T: Discard errors silently
if err != nil {
    return
}
```

### Testing

- Add tests for new functionality
- Test files: `*_test.go` in the same package
- Use `t.Run()` for subtests with different cases

## Project Structure

```
vrackconverter/
├── cmd/vrackconverter/    # CLI entry point
├── internal/
│   ├── converter/         # Core conversion logic
│   └── patch/             # Data structures
├── test/                  # E2E tests and test fixtures
├── Makefile
├── README.md              # User documentation
├── CONTRIBUTING.md        # This file
└── CLAUDE.md              # AI assistant context
```

## What We're Looking For

- Bug fixes
- Performance improvements
- Additional test cases (especially real-world patches)
- Documentation improvements
- Cross-platform compatibility fixes

## Reporting Issues

When reporting bugs, please include:

1. **Input file** (if possible/safe to share)
2. **Expected behavior**
3. **Actual behavior**
4. **Platform** (OS, architecture)
5. **RackConverter version** (`./vrackconverter -V`)

## License

By contributing, you agree that your contributions will be licensed under the BSD-3-Clause license.
