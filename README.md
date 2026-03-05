# VRackConverter

Convert VCV Rack v0.6 compatible patches (including MiRack) to VCV Rack v2.0 compatible format.

## Build

```bash
go build -o vrackconverter ./cmd/vrackconverter/
```

## Usage

```bash
vrackconverter <input> -o <output>
```

### Options

- `-o, --output <path>` Output file/directory (required)
- `-q, --quiet` Suppress non-error output
- `--overwrite` Overwrite existing files
- `-V, --version` Show version
- `-h, --help` Show help

### Examples

```bash
vrackconverter patch.vcv -o converted-patch.vcv
vrackconverter ./patches/ -o ./converted/
```

## Credits & Thanks

This tool was made possible by the excellent work of:

- [VCV Rack](https://github.com/VCVRack/Rack) - Open-source virtual modular synthesizer
- [MiRack](https://github.com/miRackModular/Rack) - MiRack modular synthesizer
- [Cardinal](https://github.com/DISTRHO/Cardinal) - Cardinal synthesizer plugin

## License

BSD-3-Clause - Compatible with VCV Rack, MiRack, and Cardinal licenses.
