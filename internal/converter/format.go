package converter

import (
	"os"
	"path/filepath"
	"strings"
)

// Format represents a Rack patch file format.
type Format string

const (
	// FormatVCV06 represents VCV Rack v0.6 format (zstd tar archive, version "0.x.x").
	FormatVCV06 Format = "v0.6"
	// FormatVCV2 represents VCV Rack v2 format (zstd tar archive, version "2.x.x").
	FormatVCV2 Format = "v2"
	// FormatMiRack represents MiRack format (.mrk directory bundle with plain JSON).
	FormatMiRack Format = "mirack"
	// FormatCardinal represents Cardinal format.
	FormatCardinal Format = "cardinal"
	// FormatUnknown represents an unknown or undetectable format.
	FormatUnknown Format = ""
)

// String returns the string representation of the format.
func (f Format) String() string {
	return string(f)
}

// IsV2 returns true if the format is VCV Rack v2.
func (f Format) IsV2() bool {
	return f == FormatVCV2
}

// IsVCV06 returns true if the format is VCV Rack v0.6.
func (f Format) IsVCV06() bool {
	return f == FormatVCV06
}

// IsMiRack returns true if the format is MiRack.
func (f Format) IsMiRack() bool {
	return f == FormatMiRack
}

// IsCardinal returns true if the format is Cardinal.
func (f Format) IsCardinal() bool {
	return f == FormatCardinal
}

// IsUnknown returns true if the format is unknown.
func (f Format) IsUnknown() bool {
	return f == FormatUnknown
}

// Conversion represents a source-to-target format conversion.
type Conversion struct {
	Source Format
	Target Format
}

// String returns a string representation of the conversion (e.g., "mirack -> v2").
func (c Conversion) String() string {
	sourceStr := c.Source.String()
	if sourceStr == "" {
		sourceStr = "?"
	}
	targetStr := c.Target.String()
	if targetStr == "" {
		targetStr = "?"
	}
	return sourceStr + " -> " + targetStr
}

// DetectFormat determines the format of a patch file.
// It checks the file extension first, then inspects the file contents for .vcv files.
// For .vcv files, it reads the version field to distinguish between v0.6 and v2.
func DetectFormat(path string, data []byte) Format {
	// Check file extension first
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".mrk":
		return FormatMiRack
	case ".vcv":
		// Distinguish v0.6 from v2 by version field
		version, err := extractVersion(data)
		if err != nil {
			return FormatUnknown
		}
		if strings.HasPrefix(version, "2.") {
			return FormatVCV2
		}
		if strings.HasPrefix(version, "0.") {
			return FormatVCV06
		}
		return FormatUnknown
	default:
		return FormatUnknown
	}
}

// DetectFormatFromPath determines the format by reading the file at path.
// This is a convenience wrapper around DetectFormat that handles file I/O.
func DetectFormatFromPath(path string) (Format, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FormatUnknown, err
	}
	return DetectFormat(path, data), nil
}

// InferConversion determines the source and target formats from input and output paths.
// For .vcv output, it assumes VCV v2 format.
// For .mrk output, it assumes MiRack format.
func InferConversion(inputPath, outputPath string) Conversion {
	var inputFormat, outputFormat Format

	// Detect input format
	inputData, err := os.ReadFile(inputPath)
	if err == nil {
		inputFormat = DetectFormat(inputPath, inputData)
	}

	// Infer output format from extension
	outputExt := strings.ToLower(filepath.Ext(outputPath))
	switch outputExt {
	case ".mrk":
		outputFormat = FormatMiRack
	case ".vcv":
		outputFormat = FormatVCV2
	default:
		// Default to v2 for unknown extensions (VCV Rack is the primary target)
		outputFormat = FormatVCV2
	}

	return Conversion{
		Source: inputFormat,
		Target: outputFormat,
	}
}

// SupportedSourceFormats returns all formats that can be used as conversion sources.
func SupportedSourceFormats() []Format {
	return []Format{FormatVCV06, FormatMiRack, FormatVCV2}
}

// SupportedTargetFormats returns all formats that can be used as conversion targets.
func SupportedTargetFormats() []Format {
	return []Format{FormatVCV2, FormatVCV06}
}
