package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FormatHandler defines I/O for a specific format.
type FormatHandler interface {
	Read(path string) ([]byte, error)
	Write(data []byte, path string) error
	Extension() string
}

// DefaultFormatHandler handles .vcv files (zstd tar archives).
type DefaultFormatHandler struct{}

func (h *DefaultFormatHandler) Read(path string) ([]byte, error) {
	// For v2 archives, extract the JSON first
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// If it's a zstd archive, extract JSON
	if IsV2Format(data) {
		return ExtractJSONFromV2(path)
	}
	return data, nil
}

func (h *DefaultFormatHandler) Write(data []byte, path string) error {
	return CreateV2Patch(data, path)
}

func (h *DefaultFormatHandler) Extension() string {
	return ".vcv"
}

// GetFormatHandler returns the appropriate FormatHandler for the given format.
func GetFormatHandler(fmt Format) FormatHandler {
	switch fmt {
	case FormatMiRack:
		return &MiRackHandler{}
	case FormatVCV06:
		return &V06Handler{}
	case FormatVCV2:
		return &V2Handler{}
	default:
		return &DefaultFormatHandler{}
	}
}

type Options struct {
	Overwrite    bool
	Quiet        bool
	MetaModule   bool   // Add 4ms HubMedium module to output
	OutputFormat Format // Explicit output format (overrides file extension)
}

type Result struct {
	InputPath  string
	OutputPath string
	Issues     []string
	Success    bool
	Skipped    bool // True when file is already in target format
	Error      error
}

// ConvertFile converts a patch using the format-aware pipeline.
// It detects input and output formats, applies appropriate transformations,
// and writes the output using the correct format handler.
func ConvertFile(inputPath, outputPath string, opts Options) Result {
	result := Result{
		InputPath:  inputPath,
		OutputPath: outputPath,
	}

	// Skip existence check for in-place conversion
	inPlace := inputPath == outputPath
	if !opts.Overwrite && !inPlace {
		if _, err := os.Stat(outputPath); err == nil {
			result.Error = fmt.Errorf("output file already exists: %s (use --overwrite to replace)", outputPath)
			return result
		}
	}

	// Detect input format
	inputData, inputFmt, err := detectInputFormat(inputPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to detect input format: %w", err)
		return result
	}

	// Detect output format
	outputFmt := DetectFormatFromExtension(outputPath)
	if outputFmt.IsUnknown() {
		outputFmt = FormatVCV2 // Default to v2
	}

	// Use explicit format if specified (overrides file extension)
	if opts.OutputFormat != "" {
		outputFmt = opts.OutputFormat
	}

	// Skip if same format and no conversion needed
	if inputFmt == outputFmt {
		result.Skipped = true
		return result
	}

	// Parse JSON
	root, err := FromJSON(inputData)
	if err != nil {
		result.Error = err
		return result
	}

	// Validate MiRack audio module constraints (for both MiRack input and output)
	if (inputFmt == FormatMiRack || outputFmt == FormatMiRack) && root != nil {
		if modules, ok := root["modules"].([]any); ok {
			if valid, reason := validateAudioModuleCount(modules); !valid {
				result.Skipped = true
				result.Issues = []string{reason}
				return result
			}
		}
	}

	var issues []string

	// Normalize input format
	switch inputFmt {
	case FormatVCV2:
		if err := NormalizeV2(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
		}
	case FormatMiRack:
		if err := NormalizeMiRack(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
		}
	case FormatVCV06:
		if err := NormalizeV06(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
		}
	}

	// Add MetaModule if requested
	if opts.MetaModule {
		modules, ok := root["modules"].([]any)
		if ok {
			hubModule := createHubMediumModule(modules, root, inputPath)
			root["modules"] = append(modules, hubModule)
		}
	}

	// Denormalize to output format
	switch outputFmt {
	case FormatVCV2:
		if err := DenormalizeV2(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
		}
	case FormatMiRack:
		if err := DenormalizeMiRack(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
		}
	case FormatVCV06:
		if err := DenormalizeV06(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
		}
	}

	result.Issues = issues

	// Serialize JSON
	patchJSON, err := ToJSON(root)
	if err != nil {
		result.Error = fmt.Errorf("failed to serialize JSON: %w", err)
		return result
	}

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		result.Error = fmt.Errorf("failed to create output directory: %w", err)
		return result
	}

	// Write output using format handler
	outputHandler := GetFormatHandler(outputFmt)
	if err := outputHandler.Write(patchJSON, outputPath); err != nil {
		result.Error = err
		return result
	}

	result.Success = true
	return result
}

// detectFormat determines the format of a patch file at the given path.
// Returns FormatUnknown if the format cannot be determined.
func detectFormat(path string) Format {
	// Try format-specific detection in priority order
	// MiRack first (has most specific detection - .mrk extension check)
	if DetectMiRackFormat(path) {
		return FormatMiRack
	}

	// Read file content for content-based detection
	// For .mrk directories, we'll read the patch.vcv inside
	var readPath string
	var data []byte
	var err error

	// Check if path is a directory (MiRack bundle)
	info, statErr := os.Stat(path)
	if statErr == nil && info.IsDir() {
		// It's a directory, try reading patch.vcv inside
		readPath = filepath.Join(path, "patch.vcv")
		data, err = os.ReadFile(readPath)
	} else {
		readPath = path
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return FormatUnknown
	}

	// V2 next (zstd archives with version "2.x.x")
	if DetectV2Format(readPath, data) {
		return FormatVCV2
	}

	// V0.6 last (plain JSON with version "0.x.x")
	if DetectV06Format(readPath, data) {
		return FormatVCV06
	}

	return FormatUnknown
}

// detectInputFormat detects the format and reads the input file.
// Returns the JSON data, detected format, and any error.
func detectInputFormat(inputPath string) ([]byte, Format, error) {
	// First, detect the format
	format := detectFormat(inputPath)
	if format.IsUnknown() {
		return nil, FormatUnknown, fmt.Errorf("unable to detect format for: %s", inputPath)
	}

	// Then read using the appropriate handler
	handler := GetFormatHandler(format)
	data, err := handler.Read(inputPath)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("failed to read %s file: %w", format, err)
	}

	return data, format, nil
}

// DetectFormatFromExtension detects format from file extension.
func DetectFormatFromExtension(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mrk":
		return FormatMiRack
	case ".vcv":
		// Could be v0.6 or v2 - default to v2 for output
		return FormatVCV2
	default:
		return FormatUnknown
	}
}

// ConvertDirectory converts all patch files in a directory.
func ConvertDirectory(inputDir, outputDir string, opts Options) []Result {
	var results []Result

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		results = append(results, Result{
			InputPath: inputDir,
			Error:     fmt.Errorf("failed to read directory: %w", err),
		})
		return results
	}

	// Auto-create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		results = append(results, Result{
			InputPath: inputDir,
			Error:     fmt.Errorf("failed to create output directory: %w", err),
		})
		return results
	}

	// Determine output format for extension mapping
	targetFormat := opts.OutputFormat
	if targetFormat == "" {
		targetFormat = FormatVCV2 // Default to V2
	}
	targetHandler := GetFormatHandler(targetFormat)
	targetExt := targetHandler.Extension()

	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		// Skip non-.mrk directories (they could be anything)
		if entry.IsDir() && ext != ".mrk" {
			continue
		}

		// Process only .vcv files and .mrk directory bundles
		if ext != ".vcv" && ext != ".mrk" {
			continue
		}

		var actualInputPath, actualOutputPath string

		// Get base name without extension
		baseName := name[:len(name)-len(filepath.Ext(name))]

		if ext == ".mrk" {
			// .mrk is a directory bundle containing patch.vcv
			// Use the .mrk directory path for input (for format detection)
			actualInputPath = filepath.Join(inputDir, name)
			// Output uses target format extension
			actualOutputPath = filepath.Join(outputDir, baseName+targetExt)
		} else {
			// Regular .vcv file
			actualInputPath = filepath.Join(inputDir, name)
			// Output uses target format extension
			actualOutputPath = filepath.Join(outputDir, baseName+targetExt)
		}

		result := ConvertFile(actualInputPath, actualOutputPath, opts)
		results = append(results, result)
	}

	return results
}

// IsDirectory returns true if the path is a directory.
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
