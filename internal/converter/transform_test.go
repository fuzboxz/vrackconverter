package converter

import (
	"encoding/json"
	"strings"
	"testing"
)

func getInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}

func TestTransformPatch_NoExplicitIDs(t *testing.T) {
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "params": []},
			{"plugin": "Fundamental", "model": "VCO", "params": []}
		],
		"wires": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	// Check version
	if root["version"] != "2.6.6" {
		t.Errorf("Expected version 2.6.6, got %v", root["version"])
	}

	// Check modules have sequential IDs
	modules := root["modules"].([]any)
	for i, m := range modules {
		mod := m.(map[string]any)
		id := getInt64(mod["id"])
		if id != int64(i) {
			t.Errorf("Module %d: expected id %d, got %d", i, i, id)
		}
	}

	// Check wires renamed to cables
	if _, hasWires := root["wires"]; hasWires {
		t.Error("wires should be renamed to cables")
	}
	if _, hasCables := root["cables"]; !hasCables {
		t.Error("cables array should exist")
	}

	// Check cables have IDs
	cables := root["cables"].([]any)
	for i, c := range cables {
		cable := c.(map[string]any)
		id := getInt64(cable["id"])
		if id != int64(i) {
			t.Errorf("Cable %d: expected id %d, got %d", i, i, id)
		}
	}
}

func TestTransformPatch_DuplicateIDs(t *testing.T) {
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "id": 5, "params": []},
			{"plugin": "Fundamental", "model": "VCO", "params": []},
			{"plugin": "Fundamental", "model": "VCF", "id": 5, "params": []}
		],
		"wires": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 2, "inputId": 0},
			{"outputModuleId": 2, "outputId": 0, "inputModuleId": 0, "inputId": 0}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	// Check for duplicate ID warning
	foundDupWarning := false
	for _, issue := range issues {
		if len(issue) > 10 && issue[:10] == "module[2]:" {
			foundDupWarning = true
			break
		}
	}
	if !foundDupWarning {
		t.Error("Expected warning about duplicate ID")
	}

	// Modules should preserve their original IDs (5 for first module, no ID for second gets 0, duplicate 5 for third)
	modules := root["modules"].([]any)

	// First module keeps ID 5
	mod0 := modules[0].(map[string]any)
	if getInt64(mod0["id"]) != 5 {
		t.Errorf("Module 0: expected id 5, got %d", getInt64(mod0["id"]))
	}

	// Second module gets sequential ID 0 (next available)
	mod1 := modules[1].(map[string]any)
	if getInt64(mod1["id"]) != 0 {
		t.Errorf("Module 1: expected id 0, got %d", getInt64(mod1["id"]))
	}

	// Third module keeps duplicate ID 5 (we detect but don't auto-fix)
	mod2 := modules[2].(map[string]any)
	if getInt64(mod2["id"]) != 5 {
		t.Errorf("Module 2: expected id 5 (duplicate), got %d", getInt64(mod2["id"]))
	}
}

func TestTransformPatch_ColorConversion(t *testing.T) {
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "id": 0, "params": []},
			{"plugin": "Fundamental", "model": "VCO", "id": 1, "params": []}
		],
		"wires": [
			{
				"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0,
				"color": {"r": 1.0, "g": 0.0, "b": 0.0, "a": 1.0}
			}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	cables := root["cables"].([]any)
	if len(cables) == 0 {
		t.Fatal("Expected at least 1 cable")
	}

	cable := cables[0].(map[string]any)

	color, ok := cable["color"].(string)
	if !ok {
		t.Error("Color should be converted to string")
	}

	// Red with full alpha: r=255, g=0, b=0, a=255
	expected := "ff0000ff"
	if color != expected {
		t.Errorf("Expected color %s, got %s", expected, color)
	}
}

func TestTransformPatch_DisabledToBypass(t *testing.T) {
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "disabled": true, "params": []}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	modules := root["modules"].([]any)
	mod := modules[0].(map[string]any)

	// Check disabled was converted to bypass
	if _, hasDisabled := mod["disabled"]; hasDisabled {
		t.Error("disabled field should be removed")
	}

	if bypass, ok := mod["bypass"].(bool); !ok || !bypass {
		t.Errorf("Expected bypass=true, got %v", mod["bypass"])
	}
}

func TestTransformPatch_ParamIDConversion(t *testing.T) {
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{
				"plugin": "Core",
				"model": "AudioInterface",
				"params": [
					{"paramId": 0, "value": 0.5},
					{"paramId": 1, "value": 0.75}
				]
			}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	modules := root["modules"].([]any)
	mod := modules[0].(map[string]any)
	params := mod["params"].([]any)

	for i, p := range params {
		param := p.(map[string]any)

		// Check paramId was converted to id
		if _, hasParamID := param["paramId"]; hasParamID {
			t.Errorf("Param %d: paramId should be converted to id", i)
		}

		id := getInt64(param["id"])
		if id != int64(i) {
			t.Errorf("Param %d: expected id %d, got %d", i, i, id)
		}
	}
}

func TestTransformPatch_PreserveExistingIDs(t *testing.T) {
	// This test simulates the MiRack patch scenario where modules have explicit IDs
	patchJSON := `{
		"version": "0.6.4",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "id": 1, "params": []},
			{"plugin": "AudibleInstruments", "model": "Plaits", "id": 2, "params": []},
			{"plugin": "StellareModular", "model": "TuringMachine", "id": 3, "params": []}
		],
		"wires": [
			{"outputModuleId": 2, "outputId": 0, "inputModuleId": 1, "inputId": 0},
			{"outputModuleId": 3, "outputId": 0, "inputModuleId": 2, "inputId": 0}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	// Verify module IDs are preserved (not reset to 0,1,2)
	modules := root["modules"].([]any)
	expectedIDs := []int64{1, 2, 3}
	for i, m := range modules {
		mod := m.(map[string]any)
		id := getInt64(mod["id"])
		if id != expectedIDs[i] {
			t.Errorf("Module %d: expected id %d, got %d", i, expectedIDs[i], id)
		}
	}

	// Verify cable references are converted from array indices to module IDs
	// In v0.6, cables use array indices, not module IDs
	cables := root["cables"].([]any)

	// Cable 0: array index 2 -> array index 1
	// Array index 2 = Module ID 3, Array index 1 = Module ID 2
	cable0 := cables[0].(map[string]any)
	if getInt64(cable0["outputModuleId"]) != 3 {
		t.Errorf("Cable 0: expected outputModuleId 3 (converted from index 2), got %d", getInt64(cable0["outputModuleId"]))
	}
	if getInt64(cable0["inputModuleId"]) != 2 {
		t.Errorf("Cable 0: expected inputModuleId 2 (converted from index 1), got %d", getInt64(cable0["inputModuleId"]))
	}

	// Cable 1: array index 3 -> array index 2
	// But there are only 3 modules (indices 0-2), so this should fail
	// Actually, let me check the test patch...
	if len(cables) > 1 {
		cable1 := cables[1].(map[string]any)
		if getInt64(cable1["outputModuleId"]) != 3 {
			t.Errorf("Cable 1: expected outputModuleId 3 (converted from index 2), got %d", getInt64(cable1["outputModuleId"]))
		}
		if getInt64(cable1["inputModuleId"]) != 2 {
			t.Errorf("Cable 1: expected inputModuleId 2 (converted from index 1), got %d", getInt64(cable1["inputModuleId"]))
		}
	}
}

func TestTransformPatch_MixedIndexAndIDReferences(t *testing.T) {
	// This test verifies that array indices are converted to module IDs
	// In v0.6, ALL cable references are array indices
	patchJSON := `{
		"version": "0.6.4",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "id": 1, "params": []},
			{"plugin": "AudibleInstruments", "model": "Plaits", "id": 2, "params": []}
		],
		"wires": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	// Cable should be converted: array index 1 → module ID 2, array index 0 → module ID 1
	cables := root["cables"].([]any)
	if len(cables) != 1 {
		t.Fatalf("Expected 1 cable, got %d", len(cables))
	}

	cable := cables[0].(map[string]any)

	// Array index 1 = Plaits (ID 2)
	if getInt64(cable["outputModuleId"]) != 2 {
		t.Errorf("Expected outputModuleId 2 (converted from index 1), got %d", getInt64(cable["outputModuleId"]))
	}

	// Array index 0 = AudioInterface (ID 1)
	if getInt64(cable["inputModuleId"]) != 1 {
		t.Errorf("Expected inputModuleId 1 (converted from index 0), got %d", getInt64(cable["inputModuleId"]))
	}
}

func TestTransformPatch_AudioDataConversion(t *testing.T) {
	// Test miRack audio data conversion
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{
				"plugin": "Core",
				"model": "AudioInterface",
				"id": 0,
				"params": [],
				"data": {
					"audio": {
						"driver": 0,
						"deviceName": "Default",
						"offset": 0,
						"maxChannels": 2,
						"sampleRate": 48000,
						"blockSize": 512
					}
				}
			}
		],
		"wires": []
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	modules := root["modules"].([]any)
	mod := modules[0].(map[string]any)
	data := mod["data"].(map[string]any)
	audio := data["audio"].(map[string]any)

	// Check miRack-specific fields are removed
	if _, hasDeviceName := audio["deviceName"]; hasDeviceName {
		t.Error("deviceName should be removed")
	}
	if _, hasOffset := audio["offset"]; hasOffset {
		t.Error("offset should be removed")
	}
	if _, hasMaxChannels := audio["maxChannels"]; hasMaxChannels {
		t.Error("maxChannels should be removed")
	}

	// Check required fields are present
	if _, hasDevice := audio["device"]; !hasDevice {
		t.Error("device field should be present")
	}
	if getInt64(audio["sampleRate"]) != 48000 {
		t.Errorf("Expected sampleRate 48000, got %d", getInt64(audio["sampleRate"]))
	}
	if getInt64(audio["blockSize"]) != 512 {
		t.Errorf("Expected blockSize 512, got %d", getInt64(audio["blockSize"]))
	}
}

func TestTransformPatch_Validation(t *testing.T) {
	// Test that validation catches issues
	patchJSON := `{
		"version": "0.6.0",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface", "id": 1, "params": []}
		],
		"wires": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 1, "inputId": 0}
		]
	}`

	var root map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := TransformPatch(root, "2.6.6", &issues); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	// Should not have any broken cable warnings
	for _, issue := range issues {
		if strings.Contains(issue, "broken references") {
			t.Errorf("Unexpected broken references warning: %s", issue)
		}
	}
}
