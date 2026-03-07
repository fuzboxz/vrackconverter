package converter

import (
	"testing"
)

// TestDetectFormat_MiRackBundle tests that .mrk directory bundles are detected as MiRack format.
func TestDetectFormat_MiRackBundle(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectedFmt Format
	}{
		{
			name:        ".mrk directory bundle",
			path:        "../../test/mirack_basic.mrk",
			expectedFmt: FormatMiRack,
		},
		{
			name:        "patch.vcv inside .mrk bundle",
			path:        "../../test/mirack_basic.mrk/patch.vcv",
			expectedFmt: FormatMiRack,
		},
		{
			name:        "mirack_cables.mrk bundle",
			path:        "../../test/mirack_cables.mrk",
			expectedFmt: FormatMiRack,
		},
		{
			name:        "v2 .vcv file",
			path:        "../../test/vcv2_cables.vcv",
			expectedFmt: FormatVCV2,
		},
		{
			name:        "v2 audio .vcv file",
			path:        "../../test/vcv2_audioio.vcv",
			expectedFmt: FormatVCV2,
		},
		{
			name:        "v0.6 .vcv file",
			path:        "../../test/vcv06_cables.vcv",
			expectedFmt: FormatVCV06,
		},
		{
			name:        "base vcvrack2 file",
			path:        "../../test/basevcvrack2.vcv",
			expectedFmt: FormatVCV2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualFmt := detectFormat(tt.path)
			if actualFmt != tt.expectedFmt {
				t.Errorf("Expected %v, got %v", tt.expectedFmt, actualFmt)
			}
		})
	}
}

// TestDetectMiRackFormat tests path-based MiRack detection.
func TestDetectMiRackFormat(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     ".mrk directory bundle",
			path:     "my-patch.mrk",
			expected: true,
		},
		{
			name:     "patch.vcv inside .mrk bundle (Unix)",
			path:     "my-patch.mrk/patch.vcv",
			expected: true,
		},
		{
			name:     "patch.vcv inside .mrk bundle (Windows)",
			path:     "my-patch.mrk\\patch.vcv",
			expected: true,
		},
		{
			name:     "uppercase .MRK extension",
			path:     "my-patch.MRK",
			expected: true,
		},
		{
			name:     "regular .vcv file (not MiRack)",
			path:     "my-patch.vcv",
			expected: false,
		},
		{
			name:     "mixed case .Mrk",
			path:     "my-patch.Mrk",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := DetectMiRackFormat(tt.path)
			if actual != tt.expected {
				t.Errorf("DetectMiRackFormat(%q) = %v, want %v", tt.path, actual, tt.expected)
			}
		})
	}
}

// TestDetectV2Format tests V2 format detection.
func TestDetectV2Format(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     []byte
		expected bool
	}{
		{
			name:     ".vcv file with v2 version",
			path:     "patch.vcv",
			data:     []byte(`{"version":"2.6.6","modules":[]}`),
			expected: true,
		},
		{
			name:     ".vcv file with v2.0 version",
			path:     "patch.vcv",
			data:     []byte(`{"version":"2.0.0","modules":[]}`),
			expected: true,
		},
		{
			name:     ".vcv file with v0.6 version (not v2)",
			path:     "patch.vcv",
			data:     []byte(`{"version":"0.6.2","modules":[]}`),
			expected: false,
		},
		{
			name:     "non-.vcv file",
			path:     "patch.json",
			data:     []byte(`{"version":"2.6.6","modules":[]}`),
			expected: false,
		},
		{
			name:     "patch.vcv inside .mrk bundle (MiRack takes precedence)",
			path:     "bundle.mrk/patch.vcv",
			data:     []byte(`{"version":"2.6.6","modules":[]}`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := DetectV2Format(tt.path, tt.data)
			if actual != tt.expected {
				t.Errorf("DetectV2Format(%q, data) = %v, want %v", tt.path, actual, tt.expected)
			}
		})
	}
}

// TestDetectV06Format tests V0.6 format detection.
func TestDetectV06Format(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		data     []byte
		expected bool
	}{
		{
			name:     ".vcv file with v0.6 version",
			path:     "patch.vcv",
			data:     []byte(`{"version":"0.6.2","modules":[]}`),
			expected: true,
		},
		{
			name:     ".vcv file with v0.6.0 version",
			path:     "patch.vcv",
			data:     []byte(`{"version":"0.6.0","modules":[]}`),
			expected: true,
		},
		{
			name:     ".vcv file with v2 version (not v0.6)",
			path:     "patch.vcv",
			data:     []byte(`{"version":"2.6.6","modules":[]}`),
			expected: false,
		},
		{
			name:     "non-.vcv file",
			path:     "patch.json",
			data:     []byte(`{"version":"0.6.2","modules":[]}`),
			expected: false,
		},
		{
			name:     "patch.vcv inside .mrk bundle (MiRack takes precedence)",
			path:     "bundle.mrk/patch.vcv",
			data:     []byte(`{"version":"0.6.2","modules":[]}`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := DetectV06Format(tt.path, tt.data)
			if actual != tt.expected {
				t.Errorf("DetectV06Format(%q, data) = %v, want %v", tt.path, actual, tt.expected)
			}
		})
	}
}

// TestDetectInputFormat tests the full detectInputFormat function with real files.
func TestDetectInputFormat(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectedFmt Format
	}{
		{
			name:        "MiRack bundle",
			path:        "../../test/mirack_basic.mrk",
			expectedFmt: FormatMiRack,
		},
		{
			name:        "patch.vcv inside MiRack bundle",
			path:        "../../test/mirack_basic.mrk/patch.vcv",
			expectedFmt: FormatMiRack,
		},
		{
			name:        "v2 cables file",
			path:        "../../test/vcv2_cables.vcv",
			expectedFmt: FormatVCV2,
		},
		{
			name:        "v0.6 cables file",
			path:        "../../test/vcv06_cables.vcv",
			expectedFmt: FormatVCV06,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, actualFmt, err := detectInputFormat(tt.path)
			if err != nil {
				t.Fatalf("Failed to detect format: %v", err)
			}
			if actualFmt != tt.expectedFmt {
				t.Errorf("Expected %v, got %v", tt.expectedFmt, actualFmt)
			}
		})
	}
}
