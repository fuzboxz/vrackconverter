package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vrackconverter/internal/converter"
)

// testDataDir returns the path to the test data directory
func testDataDir() string {
	return "."
}

// TestE2E_RealWorldPatches tests conversion of actual patches from the test directory
func TestE2E_RealWorldPatches(t *testing.T) {
	tests := []struct {
		name        string
		inputFile   string
		isMrkBundle bool
	}{
		{
			name:        "legacy-patch.vcv",
			inputFile:   "legacy-patch.vcv",
			isMrkBundle: false,
		},
		{
			name:        "realistic-v06-patch.vcv",
			inputFile:   "realistic-v06-patch.vcv",
			isMrkBundle: false,
		},
		{
			name:        "mirackoutput.mrk",
			inputFile:   "mirackoutput.mrk",
			isMrkBundle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputPath string
			if tt.isMrkBundle {
				inputPath = filepath.Join(testDataDir(), tt.inputFile, "patch.vcv")
			} else {
				inputPath = filepath.Join(testDataDir(), tt.inputFile)
			}

			// Read the input file
			inputData, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("failed to read input: %v", err)
			}

			// Parse the input
			patch, err := converter.FromJSON(inputData)
			if err != nil {
				t.Fatalf("failed to parse input: %v", err)
			}

			// Verify it's a v0.6 format patch
			version, ok := patch["version"].(string)
			if !ok {
				t.Fatal("patch missing version")
			}
			if !strings.HasPrefix(version, "0.") {
				t.Fatalf("expected v0.6 format, got version: %s", version)
			}

			// Transform the patch
			var issues []string
			if err := converter.TransformPatch(patch, "2.6.6", &issues, converter.Options{}, inputPath); err != nil {
				t.Fatalf("failed to transform patch: %v", err)
			}

			// Verify transformation succeeded
			if version, ok := patch["version"].(string); !ok || version != "2.6.6" {
				t.Errorf("version not updated: got %v, want 2.6.6", version)
			}

			// Verify cables key exists (was "wires" in v0.6)
			if _, ok := patch["cables"]; !ok {
				t.Error("output missing 'cables' key")
			}

			// Verify old "wires" key doesn't exist
			if _, ok := patch["wires"]; ok {
				t.Error("output still has old 'wires' key")
			}

			// Serialize back to JSON
			outputJSON, err := converter.ToJSON(patch)
			if err != nil {
				t.Fatalf("failed to serialize output: %v", err)
			}

			// Verify the output is valid JSON
			if !strings.HasPrefix(string(outputJSON), "{") {
				t.Errorf("output doesn't look like JSON, starts with: %s", string(outputJSON[:min(50, len(outputJSON))]))
			}

			// Create v2 patch archive
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "output.vcv")
			if err := converter.CreateV2Patch(outputJSON, outputPath); err != nil {
				t.Fatalf("failed to create v2 patch: %v", err)
			}

			// Verify the output file exists
			if _, err := os.Stat(outputPath); err != nil {
				t.Errorf("output file doesn't exist: %v", err)
			}

			// Verify it's a zstd file (check magic bytes)
			content, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("failed to read output: %v", err)
			}
			if len(content) < 4 {
				t.Fatalf("output too small: %d bytes", len(content))
			}
			// Zstd magic number (little-endian): 0xFD2FB528
			if content[0] != 0x28 || content[1] != 0xB5 || content[2] != 0x2F || content[3] != 0xFD {
				t.Errorf("output doesn't have zstd magic bytes: %02x %02x %02x %02x",
					content[0], content[1], content[2], content[3])
			}
		})
	}
}

// TestE2E_ConversionIdempotency tests that converting the same patch twice produces the same JSON
func TestE2E_ConversionIdempotency(t *testing.T) {
	inputPath := filepath.Join(testDataDir(), "legacy-patch.vcv")

	// First conversion
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read input: %v", err)
	}

	patch1, err := converter.FromJSON(inputData)
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	var issues1 []string
	if err := converter.TransformPatch(patch1, "2.6.6", &issues1, converter.Options{}, inputPath); err != nil {
		t.Fatalf("failed to transform patch: %v", err)
	}

	json1, err := converter.ToJSON(patch1)
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Second conversion
	patch2, err := converter.FromJSON(inputData)
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	var issues2 []string
	if err := converter.TransformPatch(patch2, "2.6.6", &issues2, converter.Options{}, inputPath); err != nil {
		t.Fatalf("failed to transform patch: %v", err)
	}

	json2, err := converter.ToJSON(patch2)
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Compare JSON outputs
	if string(json1) != string(json2) {
		t.Error("converting the same input twice produced different JSON output")
		t.Logf("first:  %s", string(json1))
		t.Logf("second: %s", string(json2))
	}
}

// TestE2E_MrkBundleStructure verifies the .mrk bundle structure
func TestE2E_MrkBundleStructure(t *testing.T) {
	mrkPath := filepath.Join(testDataDir(), "mirackoutput.mrk")

	// Verify it's a directory
	info, err := os.Stat(mrkPath)
	if err != nil {
		t.Fatalf("failed to stat .mrk bundle: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("mirackoutput.mrk should be a directory")
	}

	// The only required file for conversion is patch.vcv
	// (Info.plist and preview.png are optional MiRack-specific files)
	patchPath := filepath.Join(mrkPath, "patch.vcv")
	if _, err := os.Stat(patchPath); err != nil {
		t.Fatalf("required file patch.vcv not found in .mrk bundle: %v", err)
	}

	// Verify patch.vcv is valid JSON
	patchData, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("failed to read patch.vcv: %v", err)
	}

	patch, err := converter.FromJSON(patchData)
	if err != nil {
		t.Fatalf("failed to parse patch.vcv: %v", err)
	}

	// Verify version
	if version, ok := patch["version"].(string); ok {
		if !strings.HasPrefix(version, "0.") {
			t.Errorf("expected v0.6 format, got version: %s", version)
		}
	} else {
		t.Error("patch missing version")
	}
}

// TestE2E_AllTestFilesAreValid tests that all test files can be parsed
func TestE2E_AllTestFilesAreValid(t *testing.T) {
	entries, err := os.ReadDir(testDataDir())
	if err != nil {
		t.Fatalf("failed to read test directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		if ext != ".vcv" && ext != ".mrk" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			var inputPath string
			if ext == ".mrk" {
				inputPath = filepath.Join(testDataDir(), name, "patch.vcv")
			} else {
				inputPath = filepath.Join(testDataDir(), name)
			}

			// Read the input file
			inputData, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("failed to read input: %v", err)
			}

			// Parse the input
			patch, err := converter.FromJSON(inputData)
			if err != nil {
				t.Fatalf("failed to parse input: %v", err)
			}

			// Verify it has a version
			if _, ok := patch["version"]; !ok {
				t.Error("patch missing version")
			}

			// Verify it has modules array
			if modules, ok := patch["modules"]; ok {
				if _, ok := modules.([]any); !ok {
					t.Error("modules is not an array")
				}
			} else {
				t.Error("patch missing modules")
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestE2E_MorningstarlingRegression tests conversion of a real-world complex patch.
//
// morningstarling.vcv - Source: https://patchstorage.com/starling/
// Author: agnetha (https://patchstorage.com/author/agnetha/)
// License: MIT
// A complex v0.6 patch with 58 modules for regression testing.
// Used to verify the converter handles real-world patches correctly.
func TestE2E_MorningstarlingRegression(t *testing.T) {
	inputPath := filepath.Join(testDataDir(), "morningstarling.vcv")

	// Read and convert the v0.6 source
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read input: %v", err)
	}

	patch, err := converter.FromJSON(inputData)
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	// Verify it's v0.6 format
	if version, ok := patch["version"].(string); !ok || !strings.HasPrefix(version, "0.") {
		t.Fatalf("expected v0.6 format, got version: %s", version)
	}

	// Transform the patch
	var issues []string
	if err := converter.TransformPatch(patch, "2.6.6", &issues, converter.Options{}, inputPath); err != nil {
		t.Fatalf("failed to transform patch: %v", err)
	}

	// Verify transformation succeeded
	if version, ok := patch["version"].(string); !ok || version != "2.6.6" {
		t.Errorf("version not updated: got %v, want 2.6.6", version)
	}

	// Verify "wires" was converted to "cables"
	if _, ok := patch["wires"]; ok {
		t.Error("output still has old 'wires' key")
	}
	if _, ok := patch["cables"]; !ok {
		t.Error("output missing 'cables' key")
	}

	// Create v2 archive and verify it's valid
	outputJSON, err := converter.ToJSON(patch)
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.vcv")
	if err := converter.CreateV2Patch(outputJSON, outputPath); err != nil {
		t.Fatalf("failed to create v2 patch: %v", err)
	}

	// Verify zstd magic bytes
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if len(content) < 4 {
		t.Fatalf("output too small: %d bytes", len(content))
	}
	if content[0] != 0x28 || content[1] != 0xB5 || content[2] != 0x2F || content[3] != 0xFD {
		t.Errorf("output doesn't have zstd magic bytes: %02x %02x %02x %02x",
			content[0], content[1], content[2], content[3])
	}

	// Verify we can extract and read the patch.json back
	// (This confirms the archive is valid VCV Rack 2 format)
	extractedData, err := converter.ExtractJSONFromV2(outputPath)
	if err != nil {
		t.Fatalf("failed to extract from v2 archive: %v", err)
	}
	if len(extractedData) == 0 {
		t.Error("extracted JSON is empty")
	}
}

// TestE2E_SkipV2Format tests that v2 files are detected and skipped.
func TestE2E_SkipV2Format(t *testing.T) {
	// First create a v2 file by converting a known v0.6 patch
	inputPath := filepath.Join(testDataDir(), "morningstarling.vcv")
	inputData, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read input: %v", err)
	}

	patch, err := converter.FromJSON(inputData)
	if err != nil {
		t.Fatalf("failed to parse input: %v", err)
	}

	var issues []string
	if err := converter.TransformPatch(patch, "2.6.6", &issues, converter.Options{}, inputPath); err != nil {
		t.Fatalf("failed to transform patch: %v", err)
	}

	outputJSON, err := converter.ToJSON(patch)
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	tmpDir := t.TempDir()
	v2Path := filepath.Join(tmpDir, "test.vcv")
	if err := converter.CreateV2Patch(outputJSON, v2Path); err != nil {
		t.Fatalf("failed to create v2 patch: %v", err)
	}

	// Now test that the v2 file is detected and skipped
	opts := converter.Options{Quiet: true}
	result := converter.ConvertFile(v2Path, tmpDir+"/output.vcv", opts)

	// Should be marked as skipped, not failed
	if !result.Skipped {
		t.Errorf("expected result.Skipped=true, got false")
	}
	if result.Success {
		t.Error("expected result.Success=false for v2 file")
	}
	// No error should be set for skipped files
	if result.Error != nil {
		t.Errorf("expected no error for skipped v2 file, got: %v", result.Error)
	}
}
