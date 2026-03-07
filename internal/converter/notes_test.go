package converter

import (
	"strings"
	"testing"
)

// TestNotesModuleTransformation_MiRackToV2 tests text field movement.
func TestNotesModuleTransformation_MiRackToV2(t *testing.T) {
	// MiRack format: text at module level
	mirackJSON := `{
        "version": "0.6.2",
        "modules": [{
            "id": 1,
            "plugin": "Core",
            "model": "Notes",
            "text": "Test patch notes",
            "params": []
        }],
        "wires": []
    }`

	patch, err := FromJSON([]byte(mirackJSON))
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("NormalizeMiRack failed: %v", err)
	}

	modules := patch["modules"].([]any)
	notesMod := modules[0].(map[string]any)

	// text should be in data.text, not at module level
	if _, hasModuleText := notesMod["text"]; hasModuleText {
		t.Error("After MiRack→V2, text should not be at module level")
	}

	data, ok := notesMod["data"].(map[string]any)
	if !ok {
		t.Fatal("Notes module should have data map after normalization")
	}

	text, ok := data["text"].(string)
	if !ok || text != "Test patch notes" {
		t.Errorf("Expected 'Test patch notes' in data.text, got: %v", data["text"])
	}
}

// TestNotesModuleTransformation_V2ToMiRack tests text field movement back.
func TestNotesModuleTransformation_V2ToMiRack(t *testing.T) {
	// V2 format: text in data.text
	v2JSON := `{
        "version": "2.6.6",
        "modules": [{
            "id": 1,
            "plugin": "Core",
            "model": "Notes",
            "params": [],
            "data": {"text": "Test patch notes"}
        }],
        "cables": []
    }`

	patch, err := FromJSON([]byte(v2JSON))
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := DenormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("DenormalizeMiRack failed: %v", err)
	}

	modules := patch["modules"].([]any)
	notesMod := modules[0].(map[string]any)

	// text should be at module level
	text, hasModuleText := notesMod["text"].(string)
	if !hasModuleText || text != "Test patch notes" {
		t.Errorf("Expected 'Test patch notes' at module level, got: %v", text)
	}

	// data.text should be removed
	if data, ok := notesMod["data"].(map[string]any); ok {
		if _, hasDataText := data["text"]; hasDataText {
			t.Error("After V2→MiRack, data.text should be removed")
		}
	}
}

// TestNotesModuleTransformation_Roundtrip tests full roundtrip.
func TestNotesModuleTransformation_Roundtrip(t *testing.T) {
	originalText := "Multi-line notes here"

	mirackJSON := `{
        "version": "0.6.2",
        "modules": [{
            "id": 1,
            "plugin": "Core",
            "model": "Notes",
            "text": "` + originalText + `",
            "params": []
        }],
        "wires": []
    }`

	patch, err := FromJSON([]byte(mirackJSON))
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Normalize: MiRack → V2
	var issues []string
	if err := NormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("NormalizeMiRack failed: %v", err)
	}

	// Denormalize: V2 → MiRack
	if err := DenormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("DenormalizeMiRack failed: %v", err)
	}

	modules := patch["modules"].([]any)
	notesMod := modules[0].(map[string]any)

	// Text should be preserved through roundtrip
	text, ok := notesMod["text"].(string)
	if !ok || text != originalText {
		t.Errorf("Text not preserved through roundtrip: got %v, want %s", text, originalText)
	}
}

// TestNotesModuleTransformation_EmptyText handles empty/null text.
func TestNotesModuleTransformation_EmptyText(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		expectDataText bool
	}{
		{
			name:           "empty string text",
			inputJSON:      `{"version":"0.6.2","modules":[{"id":1,"plugin":"Core","model":"Notes","text":"","params":[]}],"wires":[]}`,
			expectDataText: true,
		},
		{
			name:           "no text field",
			inputJSON:      `{"version":"0.6.2","modules":[{"id":1,"plugin":"Core","model":"Notes","params":[]}],"wires":[]}`,
			expectDataText: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := FromJSON([]byte(tt.inputJSON))
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var issues []string
			if err := NormalizeMiRack(patch, &issues); err != nil {
				t.Fatalf("NormalizeMiRack failed: %v", err)
			}

			modules := patch["modules"].([]any)
			notesMod := modules[0].(map[string]any)

			data, hasData := notesMod["data"].(map[string]any)
			hasDataText := false
			if hasData {
				_, hasDataText = data["text"]
			}

			if hasDataText != tt.expectDataText {
				t.Errorf("Expected data.text=%v, got %v", tt.expectDataText, hasDataText)
			}
		})
	}
}

// TestNotesModuleTransformation_MultiLineText tests handling of multi-line text content.
func TestNotesModuleTransformation_MultiLineText(t *testing.T) {
	multiLineText := "Line 1\nLine 2\nLine 3"

	mirackJSON := `{
        "version": "0.6.2",
        "modules": [{
            "id": 1,
            "plugin": "Core",
            "model": "Notes",
            "text": "` + strings.ReplaceAll(multiLineText, "\n", "\\n") + `",
            "params": []
        }],
        "wires": []
    }`

	patch, err := FromJSON([]byte(mirackJSON))
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("NormalizeMiRack failed: %v", err)
	}

	modules := patch["modules"].([]any)
	notesMod := modules[0].(map[string]any)

	data, ok := notesMod["data"].(map[string]any)
	if !ok {
		t.Fatal("Notes module should have data map")
	}

	text, ok := data["text"].(string)
	if !ok || text != multiLineText {
		t.Errorf("Multi-line text not preserved: got %q, want %q", text, multiLineText)
	}
}
