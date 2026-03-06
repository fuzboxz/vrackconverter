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
	Overwrite  bool
	Quiet      bool
	MetaModule bool // Add 4ms HubMedium module to output
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

	// Skip if same format and no conversion needed
	if inputFmt == outputFmt && inputFmt.IsV2() {
		result.Skipped = true
		return result
	}

	// Parse JSON
	root, err := FromJSON(inputData)
	if err != nil {
		result.Error = err
		return result
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

// detectInputFormat detects the format of the input file and returns the data and format.
func detectInputFormat(inputPath string) ([]byte, Format, error) {
	// Try MiRack handler first (for .mrk bundles)
	mrkHandler := &MiRackHandler{}
	data, err := mrkHandler.Read(inputPath)
	if err == nil {
		return data, FormatMiRack, nil
	}

	// Check if .vcv file is v2 or v0.6 format
	rawData, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("failed to read input: %w", err)
	}

	if IsV2Format(rawData) {
		// Extract JSON from v2 archive
		jsonData, err := ExtractJSONFromV2(inputPath)
		if err != nil {
			return nil, FormatUnknown, fmt.Errorf("failed to extract JSON from v2 archive: %w", err)
		}
		return jsonData, FormatVCV2, nil
	}

	// Assume v0.6 format (plain JSON)
	return rawData, FormatVCV06, nil
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

		if ext == ".mrk" {
			// .mrk is a directory bundle containing patch.vcv
			actualInputPath = filepath.Join(inputDir, name, "patch.vcv")
			// Output becomes .vcv with same base name
			baseName := name[:len(name)-len(filepath.Ext(name))]
			actualOutputPath = filepath.Join(outputDir, baseName+".vcv")
		} else {
			// Regular .vcv file
			actualInputPath = filepath.Join(inputDir, name)
			actualOutputPath = filepath.Join(outputDir, name)
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
