# Makefile Reference

## Overview

The Makefile provides build automation for vrackconverter, including building, testing, formatting, and cross-platform compilation.

## Usage

```bash
make [target]
```

## Targets

### Default Target

- **`all`** - Runs `fmt`, `vet`, `test`, and `build` in sequence

### Build Targets

| Target | Description |
|--------|-------------|
| `build` | Build for current platform |
| `build-all` | Build for all 6 platforms (linux, darwin, windows × amd64, arm64) |
| `install` | Install binary to `$GOPATH/bin` or `/usr/local/bin` |
| `clean` | Remove build artifacts and binaries |

### Test & Quality Targets

| Target | Description |
|--------|-------------|
| `test` | Run tests with race detector (shows only failures) |
| `fmt` | Format Go code using `gofmt -s -w .` |
| `vet` | Run `go vet` static analysis |

### Utility Targets

| Target | Description |
|--------|-------------|
| `checksums` | Generate SHA256 checksums for all build artifacts |
| `version` | Show version info, binary name, and Go version |
| `help` | Display help message with all targets |

## Build Output

- **Local build**: Binary named `vrackconverter` in project root
- **Multi-platform build**: Artifacts in `./build/` directory

### Platform Archives

| Platform | Archive Name |
|----------|--------------|
| linux-amd64 | `vrackconverter-linux-amd64.tar.gz` |
| linux-arm64 | `vrackconverter-linux-arm64.tar.gz` |
| darwin-amd64 | `vrackconverter-darwin-amd64.tar.gz` |
| darwin-arm64 | `vrackconverter-darwin-arm64.tar.gz` |
| windows-amd64 | `vrackconverter-windows-amd64.zip` |
| windows-arm64 | `vrackconverter-windows-arm64.zip` |

## Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VERSION` | `dev` | Version string injected into binary via ldflags |

## Examples

Build for current platform:
```bash
make build
```

Build with specific version:
```bash
VERSION=1.0.0 make build
```

Build for all platforms:
```bash
make build-all
```

Run tests:
```bash
make test
```

Generate checksums for release:
```bash
make checksums
```

Install locally:
```bash
make install
```

## Pre-commit Integration

The project includes a git pre-commit hook that runs `make test` before every commit. Commits are blocked if tests fail.
