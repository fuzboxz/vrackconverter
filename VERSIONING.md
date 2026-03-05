# Versioning Guide

## How Versioning Works

RackConverter uses Git tags for version management.

### For Developers

#### Creating a Release

1. **Tag the release:**
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically:**
   - Run all tests
   - Build binaries for all platforms
   - Create a GitHub Release
   - Upload all release assets with checksums

#### Version String Format

- **Tagged releases**: `v1.0.0`, `v1.2.3`, etc.
- **Main branch builds**: `main-abc1234` (branch name + commit SHA)
- **Local builds**: `dev` (default when building locally)

### For Users

#### Checking Version

```bash
rackconverter --version
# or
rackconverter -V
```

Output: `rackconverter version v1.0.0`

### Build Information

The version is injected at build time using Go ldflags:

```bash
go build -ldflags="-X main.Version=v1.0.0" ./cmd/rackconverter/
```

This is automatically handled by the GitHub Actions workflow.

## Release Process

### Automated Workflow

When you push a tag starting with `v`:

1. **Test Job**: Runs all tests
2. **Build Job**: Creates binaries for:
   - Linux (amd64, arm64)
   - Windows (amd64, arm64)
   - macOS (amd64, arm64)
3. **Checksum Job**: Generates SHA256 checksums
4. **Release Job**: Creates GitHub Release with all assets

### Manual Release

To create a manual release without a tag:

1. Push to main branch
2. GitHub Actions will build and upload artifacts
3. Download artifacts from the Actions run
4. Manually create a GitHub Release if desired

## Best Practices

### Semantic Versioning

Follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Incompatible API changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

Examples:
- `v1.0.0` → Initial release
- `v1.0.1` → Bug fix
- `v1.1.0` → New feature
- `v2.0.0` → Breaking change

### Pre-release Versions

For pre-release versions:
```bash
git tag -a v1.0.0-beta.1 -m "Beta release"
git push origin v1.0.0-beta.1
```

### Draft Releases

To create a draft release for testing:
- Modify `.github/workflows/build.yml`
- Change `draft: false` to `draft: true` in the release step
- This allows you to review before publishing
