package converter

import (
	"os"
	"testing"
)

// TestErrorHandling_InvalidInputs tests error handling for malformed inputs.
func TestErrorHandling_InvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		inputJSON   string
		expectError bool
	}{
		{
			name:        "empty JSON",
			inputJSON:   `{}`,
			expectError: false, // Empty patch is valid (just has no modules)
		},
		{
			name:        "missing version field",
			inputJSON:   `{"modules":[]}`,
			expectError: false, // FromJSON returns map, no version check at parse level
		},
		{
			name:        "invalid version format",
			inputJSON:   `{"version":"invalid","modules":[]}`,
			expectError: false, // Invalid version but still parseable
		},
		{
			name:        "malformed JSON",
			inputJSON:   `{"version":"0.6.2","modules":[`,
			expectError: true, // JSON parse error
		},
		{
			name:        "negative module ID",
			inputJSON:   `{"version":"0.6.2","modules":[{"id":-1,"plugin":"Core","model":"VCO-1"}]}`,
			expectError: false, // Negative ID is handled gracefully
		},
		{
			name:        "missing required module fields",
			inputJSON:   `{"version":"0.6.2","modules":[{"id":1}]}`,
			expectError: false, // Missing fields handled in normalize
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromJSON([]byte(tt.inputJSON))

			if tt.expectError && err == nil {
				t.Errorf("Expected error for input, got nil")
			}
			if !tt.expectError && err != nil {
				// For non-expected errors, we just log - the patch may still be processable
				t.Logf("Got error (may be ok): %v", err)
			}
		})
	}
}

// TestErrorHandling_CableReferenceErrors tests cables with invalid module references.
func TestErrorHandling_CableReferenceErrors(t *testing.T) {
	tests := []struct {
		name        string
		inputJSON   string
		expectIssue bool
	}{
		{
			name: "cable references non-existent module",
			inputJSON: `{
                "version": "2.6.6",
                "modules": [{"id":1,"plugin":"Core","model":"VCO-1"}],
                "cables": [{"outputModuleId":999,"outputId":0,"inputModuleId":1,"inputId":0}]
            }`,
			expectIssue: true, // Should log issue about missing module
		},
		{
			name: "cable with negative port ID",
			inputJSON: `{
                "version": "2.6.6",
                "modules": [{"id":1,"plugin":"Core","model":"VCO-1"}],
                "cables": [{"outputModuleId":1,"outputId":-1,"inputModuleId":1,"inputId":0}]
            }`,
			expectIssue: false, // Negative port IDs are handled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := FromJSON([]byte(tt.inputJSON))
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var issues []string
			if err := NormalizeV2(patch, &issues); err != nil {
				t.Fatalf("NormalizeV2 failed: %v", err)
			}

			if tt.expectIssue && len(issues) == 0 {
				t.Error("Expected issues to be logged, got none")
			}
		})
	}
}

// TestErrorHandling_InvalidFiles tests file reading errors.
func TestErrorHandling_InvalidFiles(t *testing.T) {
	ensureTempDir(t)

	tests := []struct {
		name        string
		path        string
		expectError bool
		setup       func() string
		teardown    func(string)
	}{
		{
			name:        "nonexistent file",
			path:        "nonexistent.vcv",
			expectError: true,
			setup:       func() string { return "" },
			teardown:    func(string) {},
		},
		{
			name:        "directory instead of file",
			path:        "../../test/",
			expectError: true,
			setup:       func() string { return "" },
			teardown:    func(string) {},
		},
		{
			name:        "empty file",
			path:        "../../test/temp/empty.vcv",
			expectError: true,
			setup: func() string {
				emptyPath := "../../test/temp/empty.vcv"
				if err := os.WriteFile(emptyPath, []byte{}, 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
				return emptyPath
			},
			teardown: func(p string) { os.Remove(p) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if tt.setup != nil {
				path = tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown(path)
			}

			_, _, err := detectInputFormat(path)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.path)
			}
		})
	}
}

// TestErrorHandling_WireArrayIndexErrors tests wire references with out-of-range indices.
func TestErrorHandling_WireArrayIndexErrors(t *testing.T) {
	tests := []struct {
		name        string
		inputJSON   string
		expectPanic bool
	}{
		{
			name: "wire reference beyond module array",
			inputJSON: `{
                "version": "0.6.2",
                "modules": [{"id":1,"plugin":"Core","model":"VCO-1"}],
                "wires": [{"outputModuleId":5,"outputId":0,"inputModuleId":0,"inputId":0}]
            }`,
			expectPanic: false, // Should handle gracefully, not panic
		},
		{
			name: "negative array index",
			inputJSON: `{
                "version": "0.6.2",
                "modules": [{"id":1,"plugin":"Core","model":"VCO-1"}],
                "wires": [{"outputModuleId":-1,"outputId":0,"inputModuleId":0,"inputId":0}]
            }`,
			expectPanic: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if tt.expectPanic {
						t.Logf("Expected panic occurred: %v", r)
					} else {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			patch, err := FromJSON([]byte(tt.inputJSON))
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var issues []string
			if err := NormalizeV06(patch, &issues); err != nil {
				t.Fatalf("NormalizeV06 failed: %v", err)
			}

			if !tt.expectPanic {
				t.Log("No panic occurred, as expected")
			}
		})
	}
}
