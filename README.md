# vRackConverter

Convert VCV Rack v0.6 compatible patches (including MiRack) to VCV Rack v2.0 compatible format.

## Features

- **Cross-platform GUI** - Drag-and-drop interface with patch inspector
- **Command-line tool** - For batch processing and automation
- **Format support**: VCV Rack v2, VCV Rack v0.6, MiRack
- **Batch conversion** - Convert entire directories at once
- **MetaModule support** - Add 4ms MetaModule to converted patches

## Installation

### Download

Pre-built binaries are available on the [Releases](https://github.com/fuzboxz/vrackconverter/releases) page for:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### Build from source

```bash
make build       # Build GUI (default)
make build-cli   # Build CLI only
make all         # Build both
```

## Usage

### GUI Application

The default `vrackconverter` binary is a graphical application:

```bash
./vrackconverter
```

**Features:**
- **Drag & drop** - Drop `.vcv` files or `.mrk` directories onto the window
- **Batch processing** - Add multiple files and convert all at once
- **Patch inspector** - View module count, cable count, and module list before converting
- **Output selection** - Choose custom output directory or use default (same as input)
- **Format selection** - Target VCV v2, v0.6, or MiRack format

### Command-line Interface

For CLI usage, use `vrackconverter-cli`:

```bash
vrackconverter-cli <input> -o <output>     # Convert to new file
vrackconverter-cli <input> --overwrite     # Overwrite input file in place
vrackconverter-cli <input.mrk>             # Auto-create .vcv (never modifies .mrk)
```

#### Options

| Flag | Description |
|------|-------------|
| `-o, --output <path>` | Output file or directory |
| `--overwrite` | Overwrite input file in place |
| `-m, --metamodule` | Add 4ms MetaModule (HubMedium) to converted patch |
| `-q, --quiet` | Suppress non-error output |
| `-V, --version` | Show version |
| `-h, --help` | Show help |

#### Examples

```bash
# Convert to a new file
vrackconverter-cli old-patch.vcv -o new-patch.vcv

# Overwrite the input file in place
vrackconverter-cli old-patch.vcv --overwrite

# Convert .mrk (MiRack) bundle - auto-creates .vcv
vrackconverter-cli my-patch.mrk

# Specify output for .mrk file
vrackconverter-cli my-patch.mrk -o converted.vcv

# Convert with MetaModule support (adds 4ms MetaModule)
vrackconverter-cli old-patch.vcv -o new-patch.vcv --metamodule

# Convert a directory of patches
vrackconverter-cli ./patches/ -o ./converted/

# v2 files are detected and skipped gracefully
vrackconverter-cli already-v2.vcv -o output.vcv
# info: file is already in VCV Rack v2 format (no conversion needed)
```

### Behavior

- **v2 files**: If a file is already in VCV Rack v2 format, it will be detected and skipped with an informational message
- **Mixed directories**: When converting directories, v2 files are shown as skipped and don't cause the operation to fail
- **Exit codes**: `0` = success (including skipped files), `1` = error

## Supported Formats

| Format | Extension | Container |
|--------|-----------|-----------|
| VCV Rack v2 | `.vcv` | Zstd tar archive |
| VCV Rack v0.6 | `.vcv` | Zstd tar archive |
| MiRack | `.mrk` | Directory bundle |

## Make Targets

```bash
make build       # Build GUI for current platform
make build-cli   # Build CLI for current platform
make build-all   # Build GUI and CLI for all platforms
make run         # Build and run GUI
make test        # Run tests
make clean       # Remove build artifacts
```

## Credits & Thanks

This tool was made possible by the excellent work of:

- [VCV Rack](https://github.com/VCVRack/Rack) - Open-source virtual modular synthesizer
- [MiRack](https://github.com/miRackModular/Rack) - MiRack modular synthesizer
- [Cardinal](https://github.com/DISTRHO/Cardinal) - Cardinal synthesizer plugin
- [Fyne](https://fyne.io/) - Cross-platform GUI toolkit

## License

BSD-3-Clause - Compatible with VCV Rack, MiRack, and Cardinal licenses.
