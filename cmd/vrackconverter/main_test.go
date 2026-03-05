package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vrackconverter/internal/converter"
)

// binPath returns the absolute path to the vrackconverter binary
func binPath(t *testing.T) string {
	// Get the directory containing this test file
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Navigate up: cmd/vrackconverter -> vrackconverter (project root)
	// Use absolute path since tests run from temp directories
	relPath := filepath.Join(testDir, "..", "..", "vrackconverter")
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatal(err)
	}
	return absPath
}

func TestConvertFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	inputPath := filepath.Join(tmpDir, "input.vcv")
	outputPath := filepath.Join(tmpDir, "output.vcv")

	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test input: %v", err)
	}

	opts := converter.Options{Overwrite: true, Quiet: true}
	result := converter.ConvertFile(inputPath, outputPath, opts)

	if !result.Success {
		t.Errorf("conversion should succeed, got error: %v", result.Error)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("output file should exist: %v", err)
	}
}

func TestConvertFile_InvalidInput(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "nonexistent.vcv")
	outputPath := filepath.Join(tmpDir, "output.vcv")

	opts := converter.Options{Overwrite: true, Quiet: true}
	result := converter.ConvertFile(inputPath, outputPath, opts)

	if result.Success {
		t.Error("conversion should fail for nonexistent file")
	}
	if result.Error == nil {
		t.Error("expected error message")
	}
}

func TestConvertFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "invalid.vcv")
	outputPath := filepath.Join(tmpDir, "output.vcv")

	if err := os.WriteFile(inputPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to create test input: %v", err)
	}

	opts := converter.Options{Overwrite: true, Quiet: true}
	result := converter.ConvertFile(inputPath, outputPath, opts)

	if result.Success {
		t.Error("conversion should fail for invalid JSON")
	}
	if result.Error == nil {
		t.Error("expected error message")
	}
}

func TestIsMrkFile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"mrk extension lowercase", "test.mrk", true},
		{"mrk extension uppercase", "test.MRK", true},
		{"mrk extension mixed case", "test.Mrk", true},
		{"vcv extension", "test.vcv", false},
		{"no extension", "test", false},
		{"mrk in middle", "test.mrk.bak", false},
		{"path with mrk", "/path/to/file.mrk", true},
		{"vcv in path", "/path/to/file.vcv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMrkFile(tt.input); got != tt.want {
				t.Errorf("isMrkFile(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestCLI_FlagAfterInput tests that flags work when specified after the input path
func TestCLI_FlagAfterInput(t *testing.T) {
	tmpDir := t.TempDir()

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	inputPath := filepath.Join(tmpDir, "test.vcv")
	outputPath := filepath.Join(tmpDir, "output.vcv")

	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test input: %v", err)
	}

	// Test: input before flags
	cmd := exec.Command(binPath(t), inputPath, "-o", outputPath)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("conversion should succeed with flag after input: %v\noutput: %s", err, output)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("output file should exist: %v", err)
	}
}

// TestCLI_FlagBeforeInput tests that flags work when specified before the input path
func TestCLI_FlagBeforeInput(t *testing.T) {
	tmpDir := t.TempDir()

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	inputPath := filepath.Join(tmpDir, "test.vcv")
	outputPath := filepath.Join(tmpDir, "output.vcv")

	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test input: %v", err)
	}

	// Test: flags before input
	cmd := exec.Command(binPath(t), "-o", outputPath, inputPath)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("conversion should succeed with flag before input: %v\noutput: %s", err, output)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("output file should exist: %v", err)
	}
}

// TestCLI_NoOutputNoOverwrite_Errors tests that .vcv files require -o or --overwrite
func TestCLI_NoOutputNoOverwrite_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	inputPath := filepath.Join(tmpDir, "test.vcv")

	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test input: %v", err)
	}

	// Test: no flags should error
	cmd := exec.Command(binPath(t), inputPath)
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()

	if !strings.Contains(string(output), "must specify") {
		t.Errorf("should error when no -o or --overwrite specified for .vcv file\ngot: %s", output)
	}
}

// TestCLI_InPlaceOverwrite tests that --overwrite overwrites the input file
func TestCLI_InPlaceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	inputPath := filepath.Join(tmpDir, "test.vcv")

	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test input: %v", err)
	}

	// Get original file size and content
	infoBefore, err := os.Stat(inputPath)
	if err != nil {
		t.Fatalf("Failed to stat input: %v", err)
	}

	// Convert in-place
	cmd := exec.Command(binPath(t), inputPath, "--overwrite")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("conversion should succeed with --overwrite: %v\noutput: %s", err, output)
	}

	// File should still exist
	if _, err := os.Stat(inputPath); err != nil {
		t.Errorf("input file should still exist after in-place conversion: %v", err)
	}

	// File should be modified (different size since v2 format is tar+zstd)
	infoAfter, err := os.Stat(inputPath)
	if err != nil {
		t.Fatalf("Failed to stat input after: %v", err)
	}

	// v2 files are larger (tar+zstd wrapper) than plain JSON v0.6
	if infoAfter.Size() <= infoBefore.Size() {
		t.Errorf("file should be larger after conversion to v2 format (was %d, now %d)",
			infoBefore.Size(), infoAfter.Size())
	}

	// v2 files start with zstd magic bytes (0xFD 0x2B 0x52 0x58)
	content, _ := os.ReadFile(inputPath)
	if len(content) < 4 {
		t.Fatalf("converted file too small: %d bytes", len(content))
	}
	// Check for zstd magic number (0xFD2FB528 stored little-endian as 28 B5 2F FD)
	if content[0] != 0x28 || content[1] != 0xB5 || content[2] != 0x2F || content[3] != 0xFD {
		t.Errorf("file should be zstd compressed, got magic: %02x %02x %02x %02x",
			content[0], content[1], content[2], content[3])
	}
}

// TestCLI_MrkAutoNaming tests that .mrk files auto-generate .vcv output name
func TestCLI_MrkAutoNaming(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock .mrk directory (macOS bundle format)
	mrkDir := filepath.Join(tmpDir, "test.mrk")
	if err := os.Mkdir(mrkDir, 0755); err != nil {
		t.Fatalf("Failed to create .mrk directory: %v", err)
	}

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	patchPath := filepath.Join(mrkDir, "patch.vcv")
	if err := os.WriteFile(patchPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create patch.vcv inside .mrk: %v", err)
	}

	// When no -o is specified with .mrk input, should create test.vcv in same dir
	cmd := exec.Command(binPath(t), mrkDir)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %s", output)
	}

	// Should create /tmpDir/test.vcv (same name as .mrk but with .vcv extension)
	expectedOutput := filepath.Join(tmpDir, "test.vcv")
	if _, err := os.Stat(expectedOutput); err != nil {
		t.Errorf("should create %s automatically for .mrk input: %v", expectedOutput, err)
	}

	// Verify the output is a valid v2 file (zstd compressed)
	content, _ := os.ReadFile(expectedOutput)
	if len(content) < 4 {
		t.Fatalf("converted file too small: %d bytes", len(content))
	}
	// Check for zstd magic number (0xFD2FB528 stored little-endian as 28 B5 2F FD)
	if content[0] != 0x28 || content[1] != 0xB5 || content[2] != 0x2F || content[3] != 0xFD {
		t.Errorf("output should be zstd compressed v2 file, got magic: %02x %02x %02x %02x",
			content[0], content[1], content[2], content[3])
	}
}

// TestCLI_MrkWithOverwrite_DoesNotModifyMrk tests that .mrk input is never modified
func TestCLI_MrkWithOverwrite_DoesNotModifyMrk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock .mrk directory
	mrkDir := filepath.Join(tmpDir, "test.mrk")
	if err := os.Mkdir(mrkDir, 0755); err != nil {
		t.Fatalf("Failed to create .mrk directory: %v", err)
	}

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	patchPath := filepath.Join(mrkDir, "patch.vcv")
	if err := os.WriteFile(patchPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create patch.vcv inside .mrk: %v", err)
	}

	// Get original file info
	infoBefore, err := os.Stat(patchPath)
	if err != nil {
		t.Fatalf("Failed to stat patch.vcv: %v", err)
	}

	// Try to convert with --overwrite flag
	cmd := exec.Command(binPath(t), mrkDir, "--overwrite")
	cmd.Dir = tmpDir
	output, _ := cmd.CombinedOutput()
	t.Logf("output: %s", output)

	// The original patch.vcv inside .mrk should not be modified
	infoAfter, err := os.Stat(patchPath)
	if err != nil {
		t.Fatalf("patch.vcv should still exist: %v", err)
	}

	// Check content hasn't changed
	content, _ := os.ReadFile(patchPath)
	if string(content) != string(testData) {
		t.Error("patch.vcv inside .mrk should not be modified")
	}

	// Mod time should be the same (file wasn't written to)
	if infoBefore.ModTime() != infoAfter.ModTime() {
		t.Error("patch.vcv inside .mrk should not be modified (mod time changed)")
	}

	// A new .vcv file should be created instead
	expectedOutput := filepath.Join(tmpDir, "test.vcv")
	if _, err := os.Stat(expectedOutput); err != nil {
		t.Errorf("should create %s instead of modifying .mrk: %v", expectedOutput, err)
	}
}

// TestCLI_MrkWithOutput_RespectsOutput tests that -o works with .mrk files
func TestCLI_MrkWithOutput_RespectsOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock .mrk directory
	mrkDir := filepath.Join(tmpDir, "test.mrk")
	if err := os.Mkdir(mrkDir, 0755); err != nil {
		t.Fatalf("Failed to create .mrk directory: %v", err)
	}

	testData := []byte(`{"version":"0.6.0","modules":[],"wires":[]}`)
	patchPath := filepath.Join(mrkDir, "patch.vcv")
	if err := os.WriteFile(patchPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create patch.vcv inside .mrk: %v", err)
	}

	// Specify explicit output
	customOutput := filepath.Join(tmpDir, "custom.vcv")
	cmd := exec.Command(binPath(t), mrkDir, "-o", customOutput)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("output: %s", output)
	}

	// Should create the custom output
	if _, err := os.Stat(customOutput); err != nil {
		t.Errorf("should create %s when -o is specified: %v", customOutput, err)
	}

	// Should NOT create auto-named output
	autoOutput := filepath.Join(tmpDir, "test.vcv")
	if _, err := os.Stat(autoOutput); err == nil {
		t.Error("should not create auto-named output when -o is specified")
	}
}
