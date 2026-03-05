package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Overwrite bool
	Quiet     bool
}

type Result struct {
	InputPath  string
	OutputPath string
	Issues     []string
	Success    bool
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

	root, err := FromJSON(data)
	if err != nil {
		result.Error = err
		return result
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
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

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".vcv") {
			continue
		}

		inputPath := filepath.Join(inputDir, name)
		outputPath := filepath.Join(outputDir, name)

		result := ConvertFile(inputPath, outputPath, opts)
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
