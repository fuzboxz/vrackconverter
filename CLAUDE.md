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
│       ├── mirack.go        # MiRack format handler (module name mappings, color conversion)
│       ├── *_test.go        # Unit tests (co-located with source files)
│       └── mirack_cables_test.go  # Fixture-based test for test/mirack_cables.mrk
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
    └─ MiRack: module name mappings, wires→cables, indices→IDs, colorIndex→hex
    ↓
[Optional: Add MetaModule]
    ↓
Denormalize[Format]() → converts from internal to target format
    ├─ V2: ensure cables, bypass, module IDs
    ├─ V0.6: Core→Fundamental, cables→wires, IDs→indices
    └─ MiRack: module name mappings (reverse), cables→wires, IDs→indices, hex→colorIndex
    ↓
ToJSON() → JSON bytes
    ↓
FormatHandler.Write() → output file
```

### Format Handlers

| Format | Handler | File Container | Plugin Semantics |
|--------|---------|----------------|------------------|
| **VCV Rack v0.6** | `V06Handler` | Zstd tar archive | Has Fundamental + Core plugins |
| **MiRack** | `MiRackHandler` | Directory bundle (.mrk) | Uses Core and Fundamental plugins |
| **VCV Rack v2** | `V2Handler` (default) | Zstd tar archive | Core + Fundamental plugins |

### Key Format Differences

| Feature | v0.6 | MiRack | v2 |
|---------|------|--------|-----|
| Fundamental plugin | Yes | Converted to Core/Fundamental as needed | Yes |
| File container | zstd tar | Directory (.mrk) | zstd tar |
| Cable/wire name | "wires" | "wires" | "cables" |
| Cable references | Array indices | Array indices | Module IDs |
| Parameter ID field | "paramId" | "paramId" | "id" |
| Bypass field | "disabled" | "disabled" | "bypass" |
| Cable color | Hex | colorIndex | Hex |

### V06StyleConfig Pattern

V0.6 and MiRack share 90% of their conversion logic. The `V06StyleConfig` struct enables code reuse via function callbacks that handle format-specific differences:

**Configuration fields:**
- `FormatName` - For logging/error messages
- `HasFundamental` - Whether format has Fundamental plugin (v0.6: true, MiRack: false)
- `ConvertColor` - Color conversion callback (MiRack uses colorIndex, v0.6 uses hex)
- `NormalizePlugin` - Plugin name conversion during normalization
- `DenormalizePlugin` - Plugin name conversion during denormalization

**Key differences handled:**
- v0.6: Fundamental → Core plugin conversion
- MiRack: No plugin conversion, but colorIndex → hex conversion

### Module Mapping Summary

| Direction | Format | Conversion |
|-----------|--------|------------|
| v0.6 → V2 | NormalizeV06 | Fundamental → Core |
| V2 → v0.6 | DenormalizeV06 | Core → Fundamental (for known modules) |
| MiRack → V2 | NormalizeMiRack | Module name mappings (e.g., MIDIBasicInterfaceOut → CV-MIDI) |
| V2 → MiRack | DenormalizeMiRack | Reverse module name mappings |

### Audio Module Handling

Audio modules have different architectures across VCV Rack v0.6, V2, and MiRack formats. The converter handles these differences automatically.

#### Architecture Differences

| Format | Module Structure | Input/Output | Model Names |
|--------|------------------|--------------|-------------|
| **V0.6** | Single module | 8 separate inputs, 8 separate outputs | `AudioInterface` |
| **V2** | Single module | X paired inputs + X paired outputs | `AudioInterface` (8-ch), `AudioInterface2` (2-ch), `AudioInterface16` (16-ch) |
| **MiRack** | Two separate modules | Input module + Output module | `AudioInterfaceInX` + `AudioInterfaceX` |

#### V0.6 → V2 Conversion

V0.6's `AudioInterface` (8 channels) is converted to `AudioInterface` in V2. Both have 8 separate inputs and outputs. Implemented in `normalizeV06AudioModules()` in `v06.go`.

#### MiRack → V2 Conversion

MiRack's separate `AudioInterface` (output) and `AudioInterfaceInX` (input) modules are merged into a single V2 audio module. The channel count uses the **maximum** of input and output channel counts to handle mismatched pairs (e.g., 2-ch output + 8-ch input → 8-ch merged module).

**Channel count mapping:**
- 2-channel → `AudioInterface2`
- 8-channel → `AudioInterface`
- 16-channel → `AudioInterface16`

Implemented in `findAudioModulePairs()` and `mergeAudioModules()` in `mirack.go`.

#### V2 → MiRack Conversion

V2's single audio module is split into two separate MiRack modules: one for output (`AudioInterfaceX`) and one for input (`AudioInterfaceInX`).

Both modules receive the **same** channel count, determined by analyzing cable usage to find the maximum port number used, then rounding up to available sizes (2, 8, or 16 channels).

**Roundtrip case**: If the patch was originally converted from MiRack, stored metadata is used to recreate the original module structure exactly.

**Native V2 case**: For patches created in V2, cable analysis determines the required channels.

Implemented in `detectRequiredChannelCount()`, `splitAudioModulesNative()`, and `splitAudioModulesRoundtrip()` in `mirack.go`.

### Key Files

| File | Purpose |
|------|---------|
| `converter.go` | `ConvertFile()`, `ConvertDirectory()`, `detectFormat()`, `detectInputFormat()` |
| `format.go` | `Format` enum, `FormatHandler` interface |
| `archive.go` | `ExtractJSONFromV2()`, `CreateV2Patch()`, `extractVersion()` |
| `common.go` | `FromJSON()`, `ToJSON()`, `getInt64FromMap()`, color helpers |
| `legacy.go` | `NormalizeV06Style()`, `DenormalizeV06Style()`, `V06StyleConfig` |
| `v2.go` | `NormalizeV2()`, `DenormalizeV2()`, `V2Handler`, `DetectV2Format()` |
| `v06.go` | `NormalizeV06()`, `DenormalizeV06()`, `V06Handler`, `DetectV06Format()` |
| `mirack.go` | `NormalizeMiRack()`, `DenormalizeMiRack()`, `MiRackHandler`, `DetectMiRackFormat()`, module name maps, color palette |
| `metamodule.go` | `createHubMediumModule()` for --metamodule flag |
| `converter_test.go` | Format detection tests |

---

## Format Detection Architecture

Format detection is separated from I/O operations, with format-specific detection functions in each format handler file.

### Detection Flow

```
input path
    ↓
detectFormat()
    ├─ DetectMiRackFormat(path) → path-based (.mrk extension)
    ├─ DetectV2Format(path, data) → version "2.x.x" + .vcv extension
    └─ DetectV06Format(path, data) → version "0.x.x" + .vcv extension
    ↓
Format enum
    ↓
detectInputFormat()
    ├─ GetFormatHandler(format)
    └─ handler.Read(path) → JSON bytes + Format
```

### Detection Priority

Detection checks formats in priority order:

1. **MiRack** - Path-based detection (most specific)
   - `.mrk` directory bundle
   - `.mrk/patch.vcv` (patch file inside bundle)

2. **V2** - Content-based detection
   - `.vcv` extension
   - NOT inside `.mrk` directory
   - Version field starts with "2."

3. **V0.6** - Content-based detection (fallback)
   - `.vcv` extension
   - NOT inside `.mrk` directory
   - Version field starts with "0."

### Why This Order?

MiRack patches contain plain JSON with version "0.x.x" (same as v0.6), so they must be detected by path pattern first. Otherwise, a `.mrk/patch.vcv` file would be misidentified as v0.6 format.

### Separation of Concerns

- **Detection** (`Detect*Format()`) - Pure functions, return bool, no I/O
- **Reading** (`FormatHandler.Read()`) - I/O operations, format-specific
- **Orchestration** (`detectFormat()`, `detectInputFormat()`) - Coordinates detection and reading

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

Located in `internal/converter/legacy.go`, the conversion happens in two passes:

1. **Pass 1**: Build index-to-ID mapping by iterating through modules array
2. **Pass 2**: Convert cable references from array indices to module IDs using the mapping

### Roundtrip Mapping Preservation

To enable V2 → v0.6/MiRack conversion, the original index-to-ID mapping is stored in the patch metadata (`_originalIndexToID`). During denormalization, this mapping is retrieved and reversed (module ID → array index) to convert cable references back to array indices.

---

## Common Pitfalls

### Pitfall 1: Assuming ID 0 is Invalid

**WRONG**: Assuming module ID 0 is invalid and removing cables referencing it.

**CORRECT**: Check if array index is out of range (>= len(modules)), not if ID equals 0.

### Pitfall 2: Mixed References

**All** cable references in MiRack/v0.6 are array indices, regardless of whether modules have explicit IDs.

### Pitfall 3: Preserving Module IDs but Not Converting Cables

When preserving module IDs during conversion, cable references must also be converted from array indices to module IDs. Failing to do so breaks because module ID 0 may not exist, but array index 0 does.

---

## MiRack-Specific Conversions

**Color**: MiRack uses a 6-color palette (colorIndex 0-5) instead of hex. Converter maps between colorIndex and nearest hex color.

**Module Names**: MiRack uses different module names than VCV Rack V2.

## MiRack Module Name Mappings

MiRack uses different module names than VCV Rack V2. The converter maps between them during conversion.

| MiRack Model | V2 Model |
|--------------|----------|
| `MIDIBasicInterfaceOut` | `CV-MIDI` |
| `MIDICCInterface` | `MIDICCToCVInterface` |
| `MIDICCInterfaceOut` | `CV-CC` |
| `MIDITriggerInterface` | `MIDITriggerToCVInterface` |
| `MIDITriggerInterfaceOut` | `CV-Gate` |
| `PolyMerger` | `Merge` |
| `PolySplitter` | `Split` |
| `PolySummer` | `Sum` |

See `internal/converter/mirack.go`: `miRackToV2ModuleMap` and `v2ToMiRackModuleMap`.

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

The `extractVersion()` function in `archive.go` extracts the version field from both plain JSON and zstd tar archives. Version string format determines the format (v2: "2.x.x", v0.6: "0.x.x").

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

Located in `internal/converter/metamodule.go`, `createHubMediumModule()` generates a 4ms MetaModule (HubMedium) with:
- Plugin: "4msCompany", Model: "HubMedium"
- 14 parameters (12 knobs @ 0.5, 2 mode params @ 0)
- Positioned at maxX + 1 (immediately after rightmost module)

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
