# GitHub Actions CI/CD

## Overview

The CI/CD pipeline runs tests, builds CLI and GUI binaries for multiple platforms, and creates GitHub releases with checksums.

## Workflow File

- **Location**: `.github/workflows/build.yml`
- **Go Version**: 1.23
- **Runners**: `ubuntu-latest`, `macos-latest`, `macos-13`, `windows-latest`

## Triggers

| Event | Branches/Tags | Jobs Run |
|-------|--------------|----------|
| Pull Request | Any | Test only |
| Push to main | main | Test only |
| Push tag | `v*.*.*` | Test + Build + Release |

## Jobs

### Test Job

Runs on every PR and version tag. Checks pass before merge.

Steps:
1. Checkout code
2. Set up Go 1.23
3. Cache Go modules
4. Download and verify dependencies
5. Build CLI binary (required for E2E tests)
6. Run tests with race detector
7. On failure, re-run E2E tests with verbose output

### Build Job

Runs only on version tags (`v*.*.*`) for the canonical repository.

Builds CLI and GUI for all platforms using a matrix strategy:

| Runner | Builds | Description |
|--------|--------|-------------|
| `macos-latest` | GUI (macOS arm64) | Apple Silicon only (Intel users build locally) |
| `ubuntu-latest` | GUI (Linux) + CLI (all 6 platforms) | Linux GUI + cross-compile all CLI variants |
| `windows-latest` | GUI (Windows amd64) | Native Windows build |

**CLI variants** (built from Linux runner with `CGO_ENABLED=0`):
- linux-amd64, linux-arm64
- darwin-amd64, darwin-arm64
- windows-amd64, windows-arm64

### Release Job

Runs only on version tags for the canonical repository after build succeeds.

Creates a GitHub release with:
- All CLI archives (6 platforms)
- All GUI packages (4 platforms)
- SHA256 checksums file
- Auto-generated release notes with download instructions

## Build Artifacts

### CLI (`vrackconverter`)

| Platform | Archive Format | Binary Name |
|----------|---------------|-------------|
| linux-amd64 | tar.gz | vrackconverter |
| linux-arm64 | tar.gz | vrackconverter |
| darwin-amd64 | tar.gz | vrackconverter |
| darwin-arm64 | tar.gz | vrackconverter |
| windows-amd64 | zip | vrackconverter.exe |
| windows-arm64 | zip | vrackconverter.exe |

### GUI (`vrackconverter-gui`)

| Platform | Archive Format | Description |
|----------|---------------|-------------|
| darwin-arm64 | .app.tar.gz | macOS Apple Silicon app bundle |
| linux-amd64 | tar.gz | Linux executable |
| windows-amd64 | zip | Windows executable |

## Cross-Compilation

### CLI (Pure Go)
- All 6 platform variants built from single Linux runner
- `CGO_ENABLED=0` - No C dependencies
- `GOOS` and `GOARCH` set via environment variables
- Statically linked and portable

### GUI (Fyne)
- Requires CGO and platform-specific native toolchains
- Built on platform-specific GitHub runners
- macOS uses `scripts/macos-package.sh` to create .app bundles

## Release Process

To create a release:

1. Commit and push changes to main
2. Create and push a version tag:
   ```bash
   git tag -a v1.0.0 -m "Release 1.0.0"
   git push origin v1.0.0
   ```
3. GitHub Actions automatically builds and creates the release

For beta/pre-releases:
```bash
git tag -a v1.0.0-beta.1 -m "Beta 1.0.0"
git push origin v1.0.0-beta.1
```

Tags containing `-beta`, `-rc`, or `-alpha` are marked as pre-releases.

To re-run a failed release:
1. Delete the tag locally and remotely
2. Recreate and push the tag

## Security

- PRs from forks run tests but do not build or create releases
- Artifact retention: 90 days
- Permissions are scoped minimally (read for test, write for release)
