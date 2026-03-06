# RackConverter - Development Notes

**Location**: `~/vrackconverter/`

A Go-based tool for converting patches between VCV Rack v0.6, MiRack, and VCV Rack v2.x formats.

---

## Development Workflow

### Base Directives

1. **Always run tests after code changes** - No code change is complete without passing tests
2. **Add tests for new functionality** - Features without tests are incomplete
3. **Tests run automatically before commits** - Git pre-commit hook enforces this

### Hooks

**Claude Code hooks** (`.claude/settings.json`):
- Runs `make fmt && make vet` before responding when code changes are discussed

### Testing Conventions

- **Use project temp folder**: When testing patch conversions, use `test/temp/` instead of `/tmp/`
  ```bash
  mkdir -p test/temp
  ./vrackconverter input.vcv -o test/temp/output.vcv
  ```

**Git hooks** (`.git/hooks/pre-commit`):
- Runs `make test` before every commit
- Commit fails if tests don't pass

### Make Targets

For full Makefile documentation, see [Makefile.md](../Makefile.md).

Quick reference:
```bash
make              # Format, vet, test, and build (default)
make build        # Build for current platform
make build-all    # Build for all platforms
make test         # Run tests
make fmt          # Format code
make vet          # Run go vet
make clean        # Remove build artifacts
make install      # Install to $GOPATH/bin or /usr/local/bin
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
// ConvertFile converts a patch using the format-aware pipeline.
// Detects input/output formats, applies transformations, writes output.
func ConvertFile(inputPath, outputPath string, opts Options) Result { ... }

// Build index-to-ID mapping for cable reference conversion.
// MiRack/v0.6 use array indices in cables, VCV Rack 2 uses module IDs.
for i, m := range modules {
    indexToID[i] = getModuleID(m)
}
```

### Testing

- Test files: `*_test.go` in the same package
- Test function names: `Test<FunctionName>_<Scenario>`
- Use `t.Run()` for subtests with different cases

```go
func TestNormalizeV06_WithIDs(t *testing.T) {
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
│   └── converter/
│       ├── converter.go     # Single pipeline orchestration
│       ├── format.go        # Format type definitions (Format enum, IsV2, etc.)
│       ├── archive.go       # Zstd tar I/O (ExtractJSONFromV2, CreateV2Patch)
│       ├── common.go        # Shared utilities (FromJSON, ToJSON, getInt64FromMap, etc.)
│       ├── legacy.go        # V06StyleConfig + shared V0.6/MiRack baseline
│       ├── metamodule.go    # MetaModule (HubMedium) module creation
│       ├── v2.go            # V2 format handler (NormalizeV2, DenormalizeV2)
│       ├── v06.go           # V0.6 format handler (uses legacy baseline + Fundamental plugin)
│       ├── mirack.go        # MiRack format handler (uses legacy baseline + color conversion)
│       ├── v2_test.go       # V2 format tests
│       ├── vcvv06_test.go   # V0.6 format tests
│       └── mirack_test.go  # MiRack format tests
└── Makefile
```

### Data Flow

```
input file → detectInputFormat() → Format + JSON bytes
    ↓
FromJSON() → map[string]any
    ↓
Normalize[Format]() → converts to internal (v2-like) representation
    ├─ V2: no-op (build mappings, validate)
    ├─ V0.6: Fundamental→Core, wires→cables, indices→IDs
    └─ MiRack: no plugin change, wires→cables, indices→IDs, colorIndex→hex
    ↓
[Optional: Add MetaModule]
    ↓
Denormalize[Format]() → converts from internal to target format
    ├─ V2: ensure cables, bypass, module IDs
    ├─ V0.6: Core→Fundamental, cables→wires, IDs→indices
    └─ MiRack: no plugin change, cables→wires, IDs→indices, hex→colorIndex
    ↓
ToJSON() → JSON bytes
    ↓
FormatHandler.Write() → output file
```

### Format Handlers

| Format | Handler | File Container | Plugin Semantics |
|--------|---------|----------------|------------------|
| **VCV Rack v0.6** | `V06Handler` | Zstd tar archive | Has Fundamental + Core plugins |
| **MiRack** | `MiRackHandler` | Directory bundle (.mrk) | NO Fundamental plugin (all Core) |
| **VCV Rack v2** | `V2Handler` (default) | Zstd tar archive | Core only (Fundamental merged in) |

### Key Format Differences

| Feature | v0.6 | MiRack | v2 |
|---------|------|--------|-----|
| Fundamental plugin | Yes | No | No (merged into Core) |
| File container | zstd tar | Directory (.mrk) | zstd tar |
| Cable/wire name | "wires" | "wires" | "cables" |
| Cable references | Array indices | Array indices | Module IDs |
| Parameter ID field | "paramId" | "paramId" | "id" |
| Bypass field | "disabled" | "disabled" | "bypass" |
| Cable color | Hex | colorIndex | Hex |

### V06StyleConfig Pattern

V0.6 and MiRack share 90% of their conversion logic. The `V06StyleConfig` struct enables code reuse via function callbacks:

```go
// V06StyleConfig contains format-specific overrides for V0.6-style formats.
type V06StyleConfig struct {
    FormatName     string                                // For logging
    HasFundamental bool                                  // true=v0.6, false=MiRack
    ConvertColor   func(cable map[string]any, issues *[]string)  // Color conversion
    NormalizePlugin   func(plugin, model string) (string, bool)   // Source → Internal
    DenormalizePlugin func(plugin, model string) (string, bool)   // Internal → Target
}

// V0.6 uses this config:
config := V06StyleConfig{
    FormatName:     "v0.6",
    HasFundamental: true,  // Has Fundamental plugin
    ConvertColor:   nil,   // Uses hex already
    NormalizePlugin:   func(p, m string) (string, bool) {
        if p == "Fundamental" { return "Core", true }
        return p, false
    },
    DenormalizePlugin: func(p, m string) (string, bool) {
        if p == "Core" && fundamentalModules[m] { return "Fundamental", true }
        return p, false
    },
}

// MiRack uses this config:
config := V06StyleConfig{
    FormatName:     "MiRack",
    HasFundamental: false, // NO Fundamental plugin
    ConvertColor:   convertMiRackColorIndexToHex,  // Special color handling
    NormalizePlugin:   func(p, m string) (string, bool) { return p, false },  // No-op
    DenormalizePlugin: func(p, m string) (string, bool) { return p, false },  // No-op
}
```

### Module Mapping Summary

| Direction | Format | Plugin Conversion |
|-----------|--------|-------------------|
| v0.6 → V2 | NormalizeV06 | Fundamental → Core |
| V2 → v0.6 | DenormalizeV06 | Core → Fundamental (for known modules) |
| MiRack → V2 | NormalizeMiRack | None (already all Core) |
| V2 → MiRack | DenormalizeMiRack | None (stay all Core) |

### Key Files

| File | Purpose |
|------|---------|
| `converter.go` | `ConvertFile()`, `ConvertDirectory()`, format detection |
| `format.go` | `Format` enum, `FormatHandler` interface |
| `archive.go` | `ExtractJSONFromV2()`, `CreateV2Patch()`, `IsV2Format()` |
| `common.go` | `FromJSON()`, `ToJSON()`, `getInt64FromMap()`, color helpers |
| `legacy.go` | `NormalizeV06Style()`, `DenormalizeV06Style()`, `V06StyleConfig` |
| `v2.go` | `NormalizeV2()`, `DenormalizeV2()`, `V2Handler` |
| `v06.go` | `NormalizeV06()`, `DenormalizeV06()`, `V06Handler`, `fundamentalModules` |
| `mirack.go` | `NormalizeMiRack()`, `DenormalizeMiRack()`, `MiRackHandler`, color palette |
| `metamodule.go` | `createHubMediumModule()` for --metamodule flag |

---

## Critical: MiRack/v0.6 Cable Reference Semantics

### The Key Discovery

**MiRack/v0.6 use ARRAY INDICES for cable references, NOT module IDs.**

```go
// WRONG assumption:
// Cable references use module IDs
// inputModuleId=0 means "module with ID 0"

// CORRECT understanding:
// Cable references use array indices
// inputModuleId=0 means "module at array index 0"
```

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

The converter transforms this to:
```json
{
  "cables": [
    {"outputModuleId": 8, "inputModuleId": 1}  // Converted to actual IDs
  ]
}
```

### Conversion Implementation

Located in `internal/converter/legacy.go`:

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

### Roundtrip Mapping Preservation

To enable V2 → v0.6/MiRack conversion, the original index-to-ID mapping is stored:

```go
// During normalization - store the mapping
patch["_originalIndexToID"] = indexToID

// During denormalization - retrieve and reverse
if indexToIDRaw, ok := patch["_originalIndexToID"]; ok {
    // Reverse the mapping: module ID → array index
    for idx, id := range indexToIDRaw {
        idToIndex[id] = idx
    }
}
```

---

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

**All** cable references in MiRack/v0.6 are array indices, regardless of whether modules have explicit IDs.

### Pitfall 3: Preserving Module IDs but Not Converting Cables

```go
// WRONG:
module["id"] = module["id"]  // Keep ID 1, 2, 3, etc.
cable["outputModuleId"] = wire["outputModuleId"]  // Still 0, 1, 2, etc.
// This breaks because module ID 0 doesn't exist, but array index 0 does
```

---

## MiRack Color Handling

MiRack uses a fixed 6-color palette with integer indices (0-5) instead of hex colors.

### Color Palette

| Index | Color Name | RGB | Hex |
|-------|-----------|-----|-----|
| 0 | Yellow | 255, 181, 0 | #ffb500 |
| 1 | Red | 242, 56, 74 | #f2384a |
| 2 | Green | 0, 181, 110 | #00b56e |
| 3 | Teal | 54, 149, 239 | #3695ef |
| 4 | Orange | 255, 181, 56 | #ffb538 |
| 5 | Purple | 140, 74, 181 | #8c4ab5 |

### Conversion

Located in `internal/converter/mirack.go`:

```go
// Normalize: colorIndex → hex
func convertMiRackColorIndexToHex(cable map[string]any, issues *[]string) {
    if colorIndex, ok := cable["colorIndex"]; ok {
        idx := int(colorIndex.(float64))
        hexColor := miRackColorIndexToHex(idx)
        cable["color"] = hexColor
        delete(cable, "colorIndex")
    }
}

// Denormalize: hex → nearest colorIndex
func convertHexToMiRackColorIndex(wire map[string]any, issues *[]string) {
    if color, ok := wire["color"].(string); ok {
        r, g, b, _ := hexToRGB(color)
        wire["colorIndex"] = rgbToMiRackColorIndex(r, g, b)
        delete(wire, "color")
    }
}
```

---

## V2 Format Detection

The app detects v2 format by **checking the version field** in the patch JSON, not by compression format.

**Critical**: Both VCV Rack v0.6 and v2 use zstd-compressed tar archives. Detection must be by version number:
- v0.6 patches have version "0.x.x" (e.g., "0.6.2")
- v2 patches have version "2.x.x" (e.g., "2.6.6")

**File formats:**
- **MiRack (.mrk)**: Directory bundle containing plain JSON `patch.vcv` file
- **VCV Rack v0.6**: Zstd-compressed tar archive with version "0.x.x"
- **VCV Rack v2**: Zstd-compressed tar archive with version "2.x.x"

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
```

### Result States

| State | Success | Skipped | Error | Exit Code |
|-------|---------|---------|-------|-----------|
| Converted | true | false | nil | 0 |
| Already v2 | false | true | nil | 0 |
| Error | false | false | set | 1 |

---

## MetaModule Support

The `-m, --metamodule` flag adds a 4ms MetaModule (HubMedium) module to converted patches.

**Note**: In VCV Rack, the 4ms MetaModule module is called "HubMedium" (plugin: "4msCompany", model: "HubMedium").

### Usage

```bash
vrackconverter input.vcv -o output.vcv --metamodule
vrackconverter input.mrk -m  # Auto-generates .vcv with MetaModule
```

### Implementation

Located in `internal/converter/metamodule.go`:

```go
// createHubMediumModule generates a HubMedium with:
// - Plugin: "4msCompany", Model: "HubMedium"
// - 14 parameters (12 knobs @ 0.5, 2 mode params @ 0)
// - Positioned at maxX + 1 (immediately after rightmost module)
func createHubMediumModule(existingModules []any, root map[string]any, inputFilename string) map[string]any
```

---

## Adding a New Format

The Normalize/Denormalize pattern enables scalable conversions:

```
                    Normalize (to v2-like)        Denormalize (from v2-like)
                    ┌─────────────────────┐      ┌──────────────────────┐
V0.6 ─────────────▶│  V0.6 → common      │─────▶│  common → V0.6       │◀─────── V0.6
MiRack ───────────▶│  MiRack → common    │─────▶│  common → MiRack     │◀─────── MiRack
V2 ───────────────▶│  V2 → common (noop) │─────▶│  common → V2 (noop)  │◀─────── V2
Cardinal ──────────▶│  Cardinal → common  │─────▶│  common → Cardinal   │◀─────── Cardinal
                    └─────────────────────┘      └──────────────────────┘
```

**Adding a new format** requires:
1. Create handler functions: `Read()`, `Write()`, `Extension()`
2. Create `NormalizeX()` and `DenormalizeX()` functions
3. Add cases to the switch statements in `ConvertFile()`

**Any conversion** works as: `source.Normalize() → target.Denormalize()`

---

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

3. Verify cable references match module IDs

### If cables go to wrong modules:

1. Check if the original patch uses array indices
2. Verify the index-to-ID mapping is correct
3. Check module order hasn't changed during conversion

---

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
# MiRack to V2
./vrackconverter input.mrk

# V2 to MiRack
./vrackconverter input.vcv -o output.mrk

# In-place conversion
./vrackconverter input.vcv --overwrite
```

---

## CI/CD & Releases

For GitHub Actions workflow details, see [.github/workflows/README.md](.github/workflows/README.md).

For Makefile build targets and variables, see [Makefile.md](Makefile.md).

### Quick Links

- **Build Platforms**: linux, darwin, windows × amd64, arm64
- **Go Version**: 1.23
- **Dependencies**: `github.com/klauspost/compress/zstd` (pure Go)
