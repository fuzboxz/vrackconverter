# VRackConverter

Convert VCV Rack v0.6 compatible patches (including MiRack) to VCV Rack v2.0 compatible format.

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
| `--overwrite` | Overwrite input file in place |
| `-m, --metamodule` | Add 4ms MetaModule (HubMedium) to converted patch |
| `-q, --quiet` | Suppress non-error output |
| `-V, --version` | Show version |
| `-h, --help` | Show help |

### Behavior

- **v2 files**: If a file is already in VCV Rack v2 format, it will be detected and skipped with an informational message
- **Mixed directories**: When converting directories, v2 files are shown as skipped and don't cause the operation to fail
- **Exit codes**: `0` = success (including skipped files), `1` = error

### Examples

```bash
# Convert to a new file
vrackconverter old-patch.vcv -o new-patch.vcv

# Overwrite the input file in place
vrackconverter old-patch.vcv --overwrite

# Convert .mrk (MiRack) bundle - auto-creates .vcv
vrackconverter my-patch.mrk

# Specify output for .mrk file
vrackconverter my-patch.mrk -o converted.vcv

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
- [MiRack](https://github.com/miRackModular/Rack) - MiRack modular synthesizer
- [Cardinal](https://github.com/DISTRHO/Cardinal) - Cardinal synthesizer plugin

## License

BSD-3-Clause - Compatible with VCV Rack, MiRack, and Cardinal licenses.
