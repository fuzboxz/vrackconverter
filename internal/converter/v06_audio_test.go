package converter

import (
	"encoding/json"
	"testing"
)

// TestNormalizeV06_AudioInterfacePreserved tests that V0.6 AudioInterface
// modules keep the same model name in V2 format (they're compatible).
func TestNormalizeV06_AudioInterfacePreserved(t *testing.T) {
	v06JSON := `{
		"version": "0.6.2",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}
		],
		"wires": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v06JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV06(patch, &issues); err != nil {
		t.Fatalf("NormalizeV06 failed: %v", err)
	}

	modules := patch["modules"].([]any)
	if len(modules) != 1 {
		t.Fatalf("Expected 1 module, got %d", len(modules))
	}

	mod := modules[0].(map[string]any)
	model, _ := mod["model"].(string)

	if model != "AudioInterface" {
		t.Errorf("Expected AudioInterface, got %s", model)
	}

	// Plugin should be Core
	plugin, _ := mod["plugin"].(string)
	if plugin != "Core" {
		t.Errorf("Expected plugin Core, got %s", plugin)
	}
}

// TestNormalizeV06_AudioInterfaceWithCables tests that AudioInterface with cables
// is preserved correctly and cables are converted.
func TestNormalizeV06_AudioInterfaceWithCables(t *testing.T) {
	v06JSON := `{
		"version": "0.6.2",
		"modules": [
			{"id": 0, "plugin": "Core", "model": "MIDIToCVInterface", "params": []},
			{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}
		],
		"wires": [
			{"outputModuleId": 0, "outputId": 0, "inputModuleId": 1, "inputId": 0},
			{"outputModuleId": 0, "outputId": 3, "inputModuleId": 1, "inputId": 3}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v06JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV06(patch, &issues); err != nil {
		t.Fatalf("NormalizeV06 failed: %v", err)
	}

	modules := patch["modules"].([]any)
	if len(modules) != 2 {
		t.Fatalf("Expected 2 modules, got %d", len(modules))
	}

	audioMod := modules[1].(map[string]any)
	model, _ := audioMod["model"].(string)

	if model != "AudioInterface" {
		t.Errorf("Expected AudioInterface, got %s", model)
	}

	// Check that cables were converted and preserved
	cables, ok := patch["cables"].([]any)
	if !ok {
		t.Fatal("Expected cables array after normalization")
	}

	if len(cables) != 2 {
		t.Errorf("Expected 2 cables, got %d", len(cables))
	}
}
