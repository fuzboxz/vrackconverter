package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
