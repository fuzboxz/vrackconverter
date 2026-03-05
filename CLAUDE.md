# RackConverter - Development Notes

**Location**: `~/vrackconverter/`

A Go-based tool for converting patches from VCV Rack v0.6/MiRack format to VCV Rack v2.x format.

---

## Development Workflow

### Base Directives

1. **Always run tests after code changes** - No code change is complete without passing tests
2. **Add tests for new functionality** - Features without tests are incomplete
3. **Tests run automatically before commits** - Git pre-commit hook enforces this

### Hooks

**Claude Code hooks** (`.claude/settings.json`):
- Runs `make fmt && make vet` before responding when code changes are discussed

**Git hooks** (`.git/hooks/pre-commit`):
- Runs `make test` before every commit
- Commit fails if tests don't pass

### Make Targets

```bash
# Build and test (default)
make

# Individual targets
make build          # Build for current platform
make build-all      # Build for all platforms
make test           # Run tests with race detector
make fmt            # Format code
make vet            # Run go vet
make clean          # Remove build artifacts
make install        # Install to $GOPATH/bin or /usr/local/bin
```

### Build with Version

```bash
VERSION=1.0.0 make build
```

### Commit Message Convention

**Format**: `type: description`

**Types**:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test changes
- `refactor:` - Code refactoring
- `chore:` - Maintenance tasks

**Structure**: List new features first, then bug fixes

```bash
feat: add batch directory conversion and v2 format detection

Features:
- Add directory/batch conversion support
  - Convert all patches in a directory at once
  - Auto-detect .mrk directories and create .vcv alongside
  - Support mixed directories with .vcv and .mrk files
- Add v2 format detection with graceful skip
  - Detect by version field, not compression format
  - Show info message and exit 0 for already-v2 files
  - Distinguish "skipped" from "failed" in directory output

Fixes:
- Fix v2 detection to check version instead of zstd magic bytes
  - Both v0.6 and v2 use zstd tar, must check version field
```

---

## Coding Standards

### Go Conventions

Follow standard Go conventions as described in [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Code Style

- Run `make fmt` before committing - this runs `gofmt -s -w .`
- Run `make vet` before committing - this catches common mistakes
- Use `golint` suggestions where reasonable (not enforced)

### Error Handling

```go
// DO: Wrap errors with context
return fmt.Errorf("failed to read input file: %w", err)

// DON'T: Discard errors silently
if err != nil {
    return
}
```

### Naming

| Context | Convention | Example |
|---------|------------|---------|
| Packages | lowercase, single word | `converter`, `patch` |
| Constants | UPPER_SNAKE_CASE | `MaxModules`, `DefaultVersion` |
| Variables | camelCase | `inputPath`, `indexToID` |
| Acronyms | Capitalize all letters | `JSON`, `ID`, `HTTP` (not `Json`, `Id`) |
| Interfaces | `-er` suffix | `Reader`, `Writer`, `Converter` |

### Comments

- Exported functions must have doc comments
- Add comments for non-obvious logic
- Explain WHY, not WHAT (code shows what)

```go
// ConvertFile reads a v0.6 patch and writes it in v2 format.
// Returns a Result with Success=true on success, or Error on failure.
func ConvertFile(inputPath, outputPath string, opts Options) Result { ... }

// Build index-to-ID mapping for cable reference conversion.
// MiRack uses array indices in cables, but VCV Rack 2 uses module IDs.
for i, m := range modules {
    indexToID[i] = getModuleID(m)
}
```

### Testing

- Test files: `*_test.go` in the same package
- Test function names: `Test<FunctionName>_<Scenario>`
- Use `t.Run()` for subtests with different cases

```go
func TestTransformPatch_WithIDs(t *testing.T) {
    t.Run("preserves existing module IDs", func(t *testing.T) { ... })
    t.Run("assigns sequential IDs when missing", func(t *testing.T) { ... })
}
```

### Project-Specific Rules

1. **Type flexibility**: Use `map[string]any` for JSON patch data - the structure is dynamic and user-supplied
2. **In-place transformation**: Modify the `map[string]any` directly rather than defining rigid structs
3. **Error collection**: Use `[]string` for non-fatal warnings, return `error` for fatal issues
4. **Module IDs**: Always `int64` - JSON numbers default to float64, but IDs are integers

### What NOT to Do

- Don't create rigid structs for every JSON field - the format is too variable
- Don't add unnecessary abstractions - this is a straightforward converter
- Don't ignore test coverage - core logic needs tests
- Don't add external dependencies unless absolutely necessary

---

## Architecture

### Project Structure

```
vrackconverter/
├── cmd/
│   └── vrackconverter/
│       ├── main.go          # CLI entry point
│       └── main_test.go     # CLI integration tests
├── internal/
│   ├── converter/
│   │   ├── converter.go     # File/directory conversion orchestration
│   │   ├── transform.go     # Core patch transformation logic
│   │   ├── metamodule.go    # MetaModule (HubMedium) module creation
│   │   ├── archive.go       # v0.6 JSON ↔ v2 tar archive handling, v2 format detection
│   │   └── transform_test.go # Transformation tests
│   └── patch/
│       └── patch.go         # Shared data structures (Patch, Module, Cable)
└── Makefile
```

### Data Flow

```
input.vcv (v0.6 JSON or v2 archive)
    ↓
ConvertFile() reads file bytes
    ↓
[IsV2Format() check] ← If zstd magic bytes present, set Skipped=true
    ↓
FromJSON() → map[string]any
    ↓
TransformPatch() → transforms in place
    ├── Update version
    ├── Assign module IDs
    ├── Convert wires → cables
    ├── Convert array indices → module IDs  ← CRITICAL
    ├── Convert color format
    ├── Convert disabled → bypass
    ├── Convert paramId → id
    └── Clean audio data
    ↓
ToJSON() → JSON bytes
    ↓
CreateV2Patch() → tar+compress → output.vcv
```

### Key Modules

| Package | Responsibility |
|---------|----------------|
| `converter` | Orchestration, file I/O, error handling |
| `patch` | Type definitions for patch structures |
| `main` | CLI parsing and user interaction |

### Testing Strategy

- **Unit tests** (`internal/converter/transform_test.go`) - Test transformation logic in isolation
- **Integration tests** (`cmd/vrackconverter/main_test.go`) - Test end-to-end conversion

---

## Critical: MiRack/v0.6 Cable Reference Semantics

### The Key Discovery

**MiRack/v0.6 uses ARRAY INDICES for cable references, NOT module IDs.**

This is the most important thing to understand when converting MiRack patches:

```go
// WRONG assumption:
// Cable references use module IDs
// inputModuleId=0 means "module with ID 0"

// CORRECT understanding:
// Cable references use array indices
// inputModuleId=0 means "module at array index 0"
```

### Evidence from MiRack Source Code

In `/tmp/miRack/src/app/RackWidget.cpp`:

```cpp
// Line 288: Building moduleWidgets map
json_array_foreach(modulesJ, moduleId, moduleJ) {
    ModuleWidget *moduleWidget = moduleFromJson(moduleJ);
    // ...
    moduleWidgets[moduleId] = moduleWidget;  // moduleId is ARRAY INDEX
}

// Line 331: Looking up modules for wires
ModuleWidget *outputModuleWidget = moduleWidgets[outputModuleId];
ModuleWidget *inputModuleWidget = moduleWidgets[inputModuleId];
```

The variable `moduleId` is the **array index** from `json_array_foreach`, not the module's explicit ID field.

### Example

Given this patch:
```json
{
  "modules": [
    {"id": 1, "model": "AudioInterface"},      // Array index 0
    {"id": 2, "model": "Plaits"},              // Array index 1
    {"id": 8, "model": "PingPong_Widget"}      // Array index 7
  ],
  "wires": [
    {"outputModuleId": 7, "inputModuleId": 0}  // References array indices!
  ]
}
```

The wire references:
- `outputModuleId: 7` = Array index 7 = PingPong_Widget (ID 8)
- `inputModuleId: 0` = Array index 0 = AudioInterface (ID 1)

The converter must transform this to:
```json
{
  "cables": [
    {"outputModuleId": 8, "inputModuleId": 1}  // Converted to actual IDs
  ]
}
```

## Conversion Algorithm

### Current Implementation

Located in `internal/converter/transform.go`:

```go
// Pass 1: Build index-to-ID mapping
indexToID := make(map[int]int64)
for i, m := range modules {
    if idVal, hasID := mod["id"]; hasID {
        indexToID[i] = id  // Map array index to actual ID
    } else {
        indexToID[i] = int64(i)  // No ID? Use array index as ID
    }
}

// Pass 2: Convert cable references
for _, c := range cables {
    outputModuleIdx := cable["outputModuleId"]  // This is array index
    inputModuleIdx := cable["inputModuleId"]    // This is array index

    // Convert array index to actual module ID
    outputModuleID := indexToID[outputModuleIdx]
    inputModuleID := indexToID[inputModuleIdx]

    cable["outputModuleId"] = outputModuleID
    cable["inputModuleId"] = inputModuleID
}
```

### Why This Matters

1. **VCV Rack 2 uses module IDs** in cable references
2. **MiRack/v0.6 uses array indices** in cable references
3. The converter MUST translate between these two systems

## Common Pitfalls

### Pitfall 1: Assuming ID 0 is Invalid

```go
// WRONG:
if moduleId == 0 {
    // Remove this cable, module ID 0 doesn't exist
    continue
}

// CORRECT:
if moduleId >= len(modules) {
    // Remove this cable, array index out of range
    continue
}
```

### Pitfall 2: Mixed References

We initially thought MiRack used a mix of IDs and indices. This is WRONG.

**All** cable references in MiRack/v0.6 are array indices, regardless of whether modules have explicit IDs.

### Pitfall 3: Preserving Module IDs but Not Converting Cables

```go
// WRONG:
// Preserve module IDs
module["id"] = module["id"]  // Keep ID 1, 2, 3, etc.

// But don't convert cable references
cable["outputModuleId"] = wire["outputModuleId"]  // Still 0, 1, 2, etc.

// This breaks because:
// - Module with ID 1 exists (AudioInterface)
// - Cable references module ID 0 (doesn't exist)
// - VCV Rack silently ignores the broken cable
```

## Port Number Mappings

Some modules have different port numbering between MiRack and VCV Rack 2.

## MetaModule Support

The `--mm` flag adds a 4ms MetaModule module to converted patches. This enables preset mapping and modular storage functionality.

**Note**: In VCV Rack, the 4ms MetaModule module is called "HubMedium" (plugin: "4msCompany", model: "HubMedium"). The code uses this internal name, but the feature is referred to as "MetaModule" in user-facing documentation.

### How It Works

1. **Module Addition**: When `--mm` is specified, a HubMedium module is added to the output patch
2. **Positioning**: HubMedium is placed immediately after the rightmost module at Y=0 (top row)
3. **Patch Name**: Uses the input filename (without extension) as the patch name in HubMedium

### Usage

```bash
vrackconverter input.vcv -o output.vcv --mm
vrackconverter input.mrk --mm  # Auto-generates .vcv with MetaModule
```

### Implementation

Located in `internal/converter/metamodule.go`:

```go
// createHubMediumModule generates a HubMedium (4ms MetaModule) with:
// - Plugin: "4msCompany", Model: "HubMedium"
// - 14 parameters (12 knobs @ 0.5, 2 mode params @ 0)
// - Default data structure with empty mappings
// - Positioned at maxX + 1 (immediately after rightmost module)
func createHubMediumModule(existingModules []any, root map[string]any, inputFilename string) map[string]any
```

### Notes

- HubMedium is the 4ms MetaModule module in VCV Rack
- The module is added silently (no warning message)
- Patch name comes from filename since MiRack patches lack `name`/`description` fields
- Position assumes 1 HP spacing; manual adjustment may be needed if modules overlap

### Example: Plaits

**VCV Rack 2:**
- Outputs: 0 (OUT), 1 (AUX)

**MiRack:**
- May have different port numbers
- Port mappings can be added to `portMappings` in transform.go

```go
var portMappings = map[string]map[string]map[int64]int64{
    "AudibleInstruments/Plaits": {
        "outputs": {
            12: 0,  // Map old port 12 to main OUT (0)
        },
    },
}
```

## Test Cases

### Critical Test: Array Index Conversion

```go
func TestArrayIndexConversion(t *testing.T) {
    patch := `{
        "modules": [
            {"id": 1, "model": "AudioInterface"},
            {"id": 2, "model": "Plaits"}
        ],
        "wires": [
            {"outputModuleId": 1, "inputModuleId": 0}  // Array indices!
        ]
    }`

    // After conversion:
    // outputModuleId should be 2 (Plaits, was array index 1)
    // inputModuleId should be 1 (AudioInterface, was array index 0)
}
```

## Debugging Tips

### If cables are missing in VCV Rack 2:

1. Check the converted patch JSON:
   ```bash
   tar --zstd -xOf patch.vcv patch.json | jq '.cables'
   ```

2. Verify module IDs exist:
   ```bash
   tar --zstd -xOf patch.vcv patch.json | jq '.modules[].id'
   ```

3. Verify cable references match module IDs:
   ```bash
   # All cable references should be in the module ID list
   ```

### If cables go to wrong modules:

1. Check if the original patch uses array indices (it should!)
2. Verify the index-to-ID mapping is correct
3. Check module order hasn't changed during conversion

## V2 Format Detection

### How It Works

The app detects v2 format by **checking the version field** in the patch JSON, not by compression format.

**Critical**: Both VCV Rack v0.6 and v2 use zstd-compressed tar archives. Detection must be by version number:
- v0.6 patches have version "0.x.x" (e.g., "0.6.2")
- v2 patches have version "2.x.x" (e.g., "2.6.6")

**File formats:**
- **MiRack (.mrk)**: Directory bundle containing plain JSON `patch.vcv` file
- **VCV Rack v0.6**: Zstd-compressed tar archive with version "0.x.x"
- **VCV Rack v2**: Zstd-compressed tar archive with version "2.x.x"

The test files in `test/` are MiRack exports (plain JSON), not VCV Rack v0.6 exports.

### Implementation

Located in `internal/converter/archive.go`:

```go
// IsV2Format extracts the version field and checks if it starts with "2."
func IsV2Format(data []byte) bool {
    version, err := extractVersion(data)
    if err != nil {
        return false
    }
    return strings.HasPrefix(version, "2.")
}

// extractVersion handles both plain JSON and zstd-compressed tar archives
func extractVersion(data []byte) (string, error) {
    // 1. Try parsing as plain JSON
    // 2. If that fails, try as zstd tar archive
    // 3. Extract version from patch.json
}
```

### Result States

The `Result` struct has three possible states:

| State | Success | Skipped | Error | Exit Code |
|-------|---------|---------|-------|-----------|
| Converted | true | false | nil | 0 |
| Already v2 | false | true | nil | 0 |
| Error | false | false | set | 1 |

### CLI Behavior

```bash
# Single v2 file:
$ ./vrackconverter already-v2.vcv -o output.vcv
info: file is already in VCV Rack v2 format (no conversion needed)
# Exit code: 0

# Mixed directory:
$ ./vrackconverter ./mixed/ -o ./output/
Converting directory: ./mixed/ -> ./output/
  ✓ v06-patch.vcv
  ⊘ already-v2.vcv (already v2)

Complete: 1 succeeded, 1 skipped, 0 failed
# Exit code: 0
```

## File Format Differences

### MiRack (.mrk bundles)
- Structure: Directory containing `patch.vcv` (plain JSON)
- Wires: Called "wires"
- Module IDs: Optional
- Cable refs: Array indices

### VCV Rack v0.6
- File: Zstd-compressed tar archive (same format as v2)
- Version: "0.x.x" in patch.json
- Wires: Called "wires"
- Module IDs: Optional
- Cable refs: Array indices

### VCV Rack v2.x
- File: Zstd-compressed tar archive
- Wires: Called "cables"
- Module IDs: Required
- Cable refs: Module IDs

## Lessons Learned

1. **Always check source code** - Don't assume, read the actual implementation
2. **Array indices vs IDs** - This is the #1 conversion issue
3. **Test with real patches** - Synthetic tests miss real-world issues
4. **Validate both sides** - Check the original AND converted patch
5. **User feedback is gold** - The user spotted the issue immediately

## References

- MiRack source: https://github.com/miRackModular/Rack
- Key file: `src/app/RackWidget.cpp` lines 280-370
- VCV Rack 2 source: https://github.com/VCVRack/Rack
- Key file: `src/app/RackWidget.cpp` lines 396-448

## Future Improvements

1. **Better validation** - Detect when cables reference out-of-range indices
2. **Port mapping database** - Collect known port mappings from community
3. **Batch testing** - Test converter against many MiRack patches
4. **Visual diff** - Tool to show cable differences before/after conversion

## Quick Reference

### Build
```bash
make build
```

### Test
```bash
make test
```

### Convert
```bash
./vrackconverter input.vcv -o output.vcv --overwrite
```

---

## CI/CD & Releases

### GitHub Actions Workflow

**Location**: `.github/workflows/build.yml`

**Jobs**:
1. **test** - Runs on every push/PR to `ubuntu-latest`
2. **build** - Cross-compiles all platforms from single `ubuntu-latest` runner (only on version tags)
3. **release** - Creates GitHub release with checksums (only on version tags)

### Build Platforms

| Platform | Archive | Binary |
|----------|---------|--------|
| linux-amd64 | `vrackconverter-linux-amd64.tar.gz` | `vrackconverter` |
| linux-arm64 | `vrackconverter-linux-arm64.tar.gz` | `vrackconverter` |
| darwin-amd64 | `vrackconverter-darwin-amd64.tar.gz` | `vrackconverter` |
| darwin-arm64 | `vrackconverter-darwin-arm64.tar.gz` | `vrackconverter` |
| windows-amd64 | `vrackconverter-windows-amd64.zip` | `vrackconverter.exe` |
| windows-arm64 | `vrackconverter-windows-arm64.zip` | `vrackconverter.exe` |

### Cross-Compilation

All builds are done from a single Linux runner using Go's cross-compilation:
- `CGO_ENABLED=0` - Pure Go builds (no C dependencies)
- `GOOS` and `GOARCH` set via environment variables
- Binaries are statically linked and portable

### Release Process

```bash
# 1. Commit and push changes to main
git checkout main && git pull

# 2. Create and push version tag
git tag -a v1.0.0 -m "Release 1.0.0"
git push origin v1.0.0

# 3. GitHub Actions builds and creates release automatically
#    - Builds all 6 platform binaries
#    - Creates tar.gz/zip archives (each contains a directory with the binary)
#    - Generates SHA256 checksums (checksums.txt)
#    - Creates GitHub release with release notes (excludes co-authored commits)
```

### Re-running a Release

If the release fails, fix the issue and recreate the tag:

```bash
# Delete tag locally and remotely
git tag -d v1.0.0
git push origin :refs/tags/v1.0.0

# Recreate tag
git tag -a v1.0.0 -m "Release 1.0.0"
git push origin v1.0.0
```

### Go Version

- **CI/CD**: Go 1.23
- **Local development**: Go 1.23+ recommended

### Dependencies

- `github.com/klauspost/compress/zstd` - Pure Go zstd compression (required for v2 patch format)
- No other external dependencies
