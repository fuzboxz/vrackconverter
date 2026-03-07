# VRackConverter

Convert patches between VCV Rack v0.6, MiRack, and VCV Rack v2 formats.

## Installation

### Download

Pre-built binaries are available on the [Releases](https://github.com/fuzboxz/vrackconverter/releases) page for:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Build from source

```bash
make build
```

## Usage

```
vrackconverter <input> -o <output>     # Convert to new file
vrackconverter <input> --overwrite     # Overwrite input file in place
vrackconverter <input.mrk>             # Auto-create .vcv (never modifies .mrk)
```

### Options

| Flag | Description |
|------|-------------|
| `-o, --output <path>` | Output file or directory |
| `-f, --format <format>` | Override auto-detected output format: `vcv2`, `vcv06`, or `mirack` |
| `--overwrite` | Overwrite input file in place |
| `-m, --metamodule` | Add 4ms MetaModule (HubMedium) when converting to VCV Rack v2 |
| `-q, --quiet` | Suppress non-error output |
| `-V, --version` | Show version |
| `-h, --help` | Show help |

### Behavior

- **Format detection**: Output format is auto-detected from the file extension (`.vcv` → VCV Rack v2, `.mrk` → MiRack)
- **Format override**: Use `-f/--format` to force a specific output format regardless of extension
- **Same-format skip**: If input and output formats are the same, conversion is skipped with an informational message
- **Mixed directories**: When converting directories, files already in the target format are skipped and don't cause failure
- **Exit codes**: `0` = success (including skipped files), `1` = error

### Supported Formats

| Format | Flag | Description |
|--------|------|-------------|
| VCV Rack v2 | `vcv2` | Current default format, uses `cables` and module IDs |
| MiRack | `mirack` | macOS/iPadOS Rack format, directory bundle (.mrk) |
| VCV Rack v0.6 | `vcv06` | Legacy format (experimental, unsupported) |

The converter handles the complex differences between formats:
- **Core modules**: Maps between Fundamental/Core plugin naming conventions
- **Audio interfaces**: Merges/splits audio modules appropriately (MiRack uses separate in/out modules)
- **Cables/Wires**: Converts between array index references and module IDs
- **Colors**: Maps MiRack's 6-color palette to/from hex colors

### Examples

```bash
# Convert to a new file (defaults to VCV Rack v2)
vrackconverter old-patch.vcv -o new-patch.vcv

# Overwrite the input file in place
vrackconverter old-patch.vcv --overwrite

# Convert .mrk (MiRack) bundle - auto-creates .vcv
vrackconverter my-patch.mrk

# Convert to MiRack format
vrackconverter my-patch.vcv -o my-patch.mrk -f mirack

# Convert with MetaModule support (adds 4ms MetaModule)
vrackconverter old-patch.vcv -o new-patch.vcv --metamodule

# Convert a directory of patches
vrackconverter ./patches/ -o ./converted/

# v2 files are detected and skipped gracefully
vrackconverter already-v2.vcv -o output.vcv
# info: file is already in VCV Rack v2 format (no conversion needed)
```

## Credits & Thanks

This tool was made possible by the excellent work of:

- [VCV Rack](https://github.com/VCVRack/Rack) - Open-source virtual modular synthesizer
- [MiRack](https://github.com/miRackModular/Rack) - Virtual modular synthesizer for macOS/iPadOS
- [Cardinal](https://github.com/DISTRHO/Cardinal) - Cardinal synthesizer plugin

## License

BSD-3-Clause - Compatible with VCV Rack, MiRack, and Cardinal licenses.
