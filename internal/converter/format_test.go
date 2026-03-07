package converter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormat_String(t *testing.T) {
	tests := []struct {
		name     string
		format   Format
		expected string
	}{
		{"VCV06", FormatVCV06, "v0.6"},
		{"VCV2", FormatVCV2, "v2"},
		{"MiRack", FormatMiRack, "mirack"},
		{"Cardinal", FormatCardinal, "cardinal"},
		{"Unknown", FormatUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.String(); got != tt.expected {
				t.Errorf("Format.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormat_IsMethods(t *testing.T) {
	tests := []struct {
		name       string
		format     Format
		isV2       bool
		isVCV06    bool
		isMiRack   bool
		isCardinal bool
		isUnknown  bool
	}{
		{"VCV06", FormatVCV06, false, true, false, false, false},
		{"VCV2", FormatVCV2, true, false, false, false, false},
		{"MiRack", FormatMiRack, false, false, true, false, false},
		{"Cardinal", FormatCardinal, false, false, false, true, false},
		{"Unknown", FormatUnknown, false, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.IsV2(); got != tt.isV2 {
				t.Errorf("Format.IsV2() = %v, want %v", got, tt.isV2)
			}
			if got := tt.format.IsVCV06(); got != tt.isVCV06 {
				t.Errorf("Format.IsVCV06() = %v, want %v", got, tt.isVCV06)
			}
			if got := tt.format.IsMiRack(); got != tt.isMiRack {
				t.Errorf("Format.IsMiRack() = %v, want %v", got, tt.isMiRack)
			}
			if got := tt.format.IsCardinal(); got != tt.isCardinal {
				t.Errorf("Format.IsCardinal() = %v, want %v", got, tt.isCardinal)
			}
			if got := tt.format.IsUnknown(); got != tt.isUnknown {
				t.Errorf("Format.IsUnknown() = %v, want %v", got, tt.isUnknown)
			}
		})
	}
}

func TestConversion_String(t *testing.T) {
	tests := []struct {
		name     string
		conv     Conversion
		expected string
	}{
		{"MiRack to V2", Conversion{Source: FormatMiRack, Target: FormatVCV2}, "mirack -> v2"},
		{"VCV06 to V2", Conversion{Source: FormatVCV06, Target: FormatVCV2}, "v0.6 -> v2"},
		{"Unknown source", Conversion{Source: FormatUnknown, Target: FormatVCV2}, "? -> v2"},
		{"Unknown target", Conversion{Source: FormatMiRack, Target: FormatUnknown}, "mirack -> ?"},
		{"Both unknown", Conversion{Source: FormatUnknown, Target: FormatUnknown}, "? -> ?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conv.String(); got != tt.expected {
				t.Errorf("Conversion.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectFormat_ByExtension(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		data           []byte
		expectedFormat Format
	}{
		{"mrk extension", "patch.mrk", []byte(`{}`), FormatMiRack},
		{"vcv extension with v0.6 version", "patch.vcv", []byte(`{"version":"0.6.0"}`), FormatVCV06},
		{"vcv extension with v2 version", "patch.vcv", []byte(`{"version":"2.0.0"}`), FormatVCV2},
		{"vcv extension with v2.5 version", "patch.vcv", []byte(`{"version":"2.5.1"}`), FormatVCV2},
		{"unknown extension", "patch.txt", []byte(`{"version":"2.0.0"}`), FormatUnknown},
		{"vcv with invalid JSON", "patch.vcv", []byte(`not json`), FormatUnknown},
		{"vcv with no version field", "patch.vcv", []byte(`{"foo":"bar"}`), FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.filename, tt.data)
			if got != tt.expectedFormat {
				t.Errorf("DetectFormat() = %v, want %v", got, tt.expectedFormat)
			}
		})
	}
}

func TestDetectFormat_VersionPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected Format
	}{
		{"version 2.0.0", "2.0.0", FormatVCV2},
		{"version 2.1.3", "2.1.3", FormatVCV2},
		{"version 2.6.6", "2.6.6", FormatVCV2},
		{"version 2.99.0", "2.99.0", FormatVCV2},
		{"version 0.6.0", "0.6.0", FormatVCV06},
		{"version 0.6.2", "0.6.2", FormatVCV06},
		{"version 0.5.0", "0.5.0", FormatVCV06},
		{"version 0.0.1", "0.0.1", FormatVCV06},
		{"version 1.0.0", "1.0.0", FormatUnknown},
		{"version 3.0.0", "3.0.0", FormatUnknown},
		{"version empty", "", FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`{"version":"` + tt.version + `"}`)
			got := DetectFormat("patch.vcv", data)
			if got != tt.expected {
				t.Errorf("DetectFormat() with version %s = %v, want %v", tt.version, got, tt.expected)
			}
		})
	}
}

func TestDetectFormatFromPath(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create test files
	v2File := filepath.Join(tmpDir, "v2.vcv")
	if err := os.WriteFile(v2File, []byte(`{"version":"2.0.0"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	v06File := filepath.Join(tmpDir, "v06.vcv")
	if err := os.WriteFile(v06File, []byte(`{"version":"0.6.0"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	mrkFile := filepath.Join(tmpDir, "test.mrk")
	if err := os.WriteFile(mrkFile, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		path           string
		expectedFormat Format
		expectError    bool
	}{
		{"V2 file", v2File, FormatVCV2, false},
		{"V0.6 file", v06File, FormatVCV06, false},
		{"MRK file", mrkFile, FormatMiRack, false},
		{"nonexistent file", filepath.Join(tmpDir, "nosuch.vcv"), FormatUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectFormatFromPath(tt.path)
			if tt.expectError {
				if err == nil {
					t.Errorf("DetectFormatFromPath() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("DetectFormatFromPath() unexpected error: %v", err)
				return
			}
			if got != tt.expectedFormat {
				t.Errorf("DetectFormatFromPath() = %v, want %v", got, tt.expectedFormat)
			}
		})
	}
}

func TestInferConversion(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create test input files
	v2File := filepath.Join(tmpDir, "v2.vcv")
	if err := os.WriteFile(v2File, []byte(`{"version":"2.0.0"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	v06File := filepath.Join(tmpDir, "v06.vcv")
	if err := os.WriteFile(v06File, []byte(`{"version":"0.6.0"}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	mrkFile := filepath.Join(tmpDir, "test.mrk")
	if err := os.WriteFile(mrkFile, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		inputPath      string
		outputPath     string
		expectedSource Format
		expectedTarget Format
	}{
		{"v0.6 to v2", v06File, "output.vcv", FormatVCV06, FormatVCV2},
		{"MiRack to v2", mrkFile, "output.vcv", FormatMiRack, FormatVCV2},
		{"v2 to v2 (no-op)", v2File, "output.vcv", FormatVCV2, FormatVCV2},
		{"to .mrk output", v06File, "output.mrk", FormatVCV06, FormatMiRack},
		{"unknown output extension", v06File, "output.txt", FormatVCV06, FormatVCV2}, // defaults to v2
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferConversion(tt.inputPath, tt.outputPath)
			if got.Source != tt.expectedSource {
				t.Errorf("InferConversion().Source = %v, want %v", got.Source, tt.expectedSource)
			}
			if got.Target != tt.expectedTarget {
				t.Errorf("InferConversion().Target = %v, want %v", got.Target, tt.expectedTarget)
			}
		})
	}
}

func TestSupportedSourceFormats(t *testing.T) {
	formats := SupportedSourceFormats()
	if len(formats) == 0 {
		t.Error("SupportedSourceFormats() returned empty slice")
	}

	// Check that v0.6, MiRack, and v2 are all included
	hasVCV06 := false
	hasMiRack := false
	hasV2 := false
	for _, f := range formats {
		if f == FormatVCV06 {
			hasVCV06 = true
		}
		if f == FormatMiRack {
			hasMiRack = true
		}
		if f == FormatVCV2 {
			hasV2 = true
		}
	}

	if !hasVCV06 {
		t.Error("SupportedSourceFormats() missing FormatVCV06")
	}
	if !hasMiRack {
		t.Error("SupportedSourceFormats() missing FormatMiRack")
	}
	if !hasV2 {
		t.Error("SupportedSourceFormats() missing FormatVCV2")
	}
}

func TestSupportedTargetFormats(t *testing.T) {
	formats := SupportedTargetFormats()
	if len(formats) == 0 {
		t.Error("SupportedTargetFormats() returned empty slice")
	}

	// Check that v2 is included
	hasV2 := false
	for _, f := range formats {
		if f == FormatVCV2 {
			hasV2 = true
		}
	}

	if !hasV2 {
		t.Error("SupportedTargetFormats() missing FormatVCV2")
	}
}
