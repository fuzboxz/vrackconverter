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
	return os.ReadFile(path)
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
	Skipped    bool // True when file was already in v2 format
	Error      error
}

func ConvertFile(inputPath, outputPath string, opts Options) Result {
	result := Result{
		InputPath:  inputPath,
		OutputPath: outputPath,
	}

	// Skip existence check for in-place conversion (input == output with --overwrite)
	inPlace := inputPath == outputPath
	if !opts.Overwrite && !inPlace {
		if _, err := os.Stat(outputPath); err == nil {
			result.Error = fmt.Errorf("output file already exists: %s (use --overwrite to replace)", outputPath)
			return result
		}
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read input file: %w", err)
		return result
	}

	// Check if file is already v2 format
	if IsV2Format(data) {
		result.Skipped = true
		return result
	}

	root, err := FromJSON(data)
	if err != nil {
		result.Error = err
		return result
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues, opts, inputPath); err != nil {
		result.Error = err
		result.Issues = issues
		return result
	}
	result.Issues = issues

	patchJSON, err := ToJSON(root)
	if err != nil {
		result.Error = fmt.Errorf("failed to serialize JSON: %w", err)
		return result
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		result.Error = fmt.Errorf("failed to create output directory: %w", err)
		return result
	}

	if err := CreateV2Patch(patchJSON, outputPath); err != nil {
		result.Error = err
		return result
	}

	result.Success = true
	return result
}

// ConvertFileWithPipeline converts a patch using the format-aware pipeline.
// It detects input and output formats, applies appropriate transformations,
// and writes the output using the correct format handler.
func ConvertFileWithPipeline(inputPath, outputPath string, opts Options) Result {
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
		// V0.6 uses same transformation as MiRack for now
		if err := NormalizeMiRack(root, &issues); err != nil {
			result.Error = err
			result.Issues = issues
			return result
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

	// Try default handler
	defaultHandler := &DefaultFormatHandler{}
	data, err = defaultHandler.Read(inputPath)
	if err != nil {
		return nil, FormatUnknown, fmt.Errorf("failed to read input: %w", err)
	}

	// Check if it's v2 or v0.6
	if IsV2Format(data) {
		return data, FormatVCV2, nil
	}

	// Default to v0.6 for remaining .vcv files
	return data, FormatVCV06, nil
}

// DetectFormatFromExtension detects format from file extension.
func DetectFormatFromExtension(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mrk":
		return FormatMiRack
	case ".vcv":
		return FormatVCV2 // Could be v0.6 or v2, default to v2
	default:
		return FormatUnknown
	}
}

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

func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
