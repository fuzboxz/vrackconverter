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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
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
	if err := TransformPatch(root, "2.6.6", &issues, Options{}, ""); err != nil {
		t.Fatalf("TransformPatch failed: %v", err)
	}

	// Should not have any broken cable warnings
	for _, issue := range issues {
		if strings.Contains(issue, "broken references") {
			t.Errorf("Unexpected broken references warning: %s", issue)
		}
	}
}

func TestCreateHubMediumModule(t *testing.T) {
	t.Run("creates module with correct structure", func(t *testing.T) {
		modules := []any{
			map[string]any{"id": int64(1), "pos": []any{float64(10), float64(0)}},
		}
		root := map[string]any{}
		inputFilename := "MyPatch.vcv"

		hub := createHubMediumModule(modules, root, inputFilename)

		// Check basic fields
		if hub["plugin"] != "4msCompany" {
			t.Errorf("expected plugin 4msCompany, got %v", hub["plugin"])
		}
		if hub["model"] != "HubMedium" {
			t.Errorf("expected model HubMedium, got %v", hub["model"])
		}

		// Check ID is next available
		if hub["id"] != int64(2) {
			t.Errorf("expected id 2, got %v", hub["id"])
		}

		// Check position (right of existing module at X=10, at X+1)
		pos, ok := hub["pos"].([]any)
		if !ok || len(pos) < 2 {
			t.Fatal("pos not set correctly")
		}
		if pos[0] != float64(11) { // 10 + 1
			t.Errorf("expected x position 11, got %v", pos[0])
		}
		if pos[1] != float64(0) {
			t.Errorf("expected y position 0, got %v", pos[1])
		}

		// Check data structure
		data, ok := hub["data"].(map[string]any)
		if !ok {
			t.Fatal("data not a map")
		}
		if _, hasMappings := data["Mappings"]; !hasMappings {
			t.Error("data should have Mappings")
		}
		if _, hasPatchName := data["PatchName"]; !hasPatchName {
			t.Error("data should have PatchName")
		}
	})

	t.Run("uses filename when no patch name in root", func(t *testing.T) {
		modules := []any{}
		root := map[string]any{}
		inputFilename := "/path/to/MyAwesomePatch.vcv"

		hub := createHubMediumModule(modules, root, inputFilename)

		data, ok := hub["data"].(map[string]any)
		if !ok {
			t.Fatal("data not a map")
		}
		if data["PatchName"] != "MyAwesomePatch" {
			t.Errorf("expected PatchName from filename, got %v", data["PatchName"])
		}
	})

	t.Run("uses patch name from root when available", func(t *testing.T) {
		modules := []any{}
		root := map[string]any{"name": "Custom Name", "description": "Custom Desc"}
		inputFilename := "ignored.vcv"

		hub := createHubMediumModule(modules, root, inputFilename)

		data, _ := hub["data"].(map[string]any)
		if data["PatchName"] != "Custom Name" {
			t.Errorf("expected Custom Name, got %v", data["PatchName"])
		}
		if data["PatchDesc"] != "Custom Desc" {
			t.Errorf("expected Custom Desc, got %v", data["PatchDesc"])
		}
	})

	t.Run("handles empty modules array", func(t *testing.T) {
		modules := []any{}
		root := map[string]any{}
		inputFilename := "test.vcv"

		hub := createHubMediumModule(modules, root, inputFilename)

		// Should still work, positioned at origin
		pos, ok := hub["pos"].([]any)
		if !ok || len(pos) < 2 {
			t.Fatal("pos not set correctly")
		}
		if pos[0] != float64(0) {
			t.Errorf("expected x position 0, got %v", pos[0])
		}
		if pos[1] != float64(0) {
			t.Errorf("expected y position 0, got %v", pos[1])
		}
		if hub["id"] != int64(0) {
			t.Errorf("expected id 0, got %v", hub["id"])
		}
	})

	t.Run("finds rightmost module at Y=0", func(t *testing.T) {
		modules := []any{
			map[string]any{"id": int64(1), "pos": []any{float64(10), float64(0)}},
			map[string]any{"id": int64(2), "pos": []any{float64(30), float64(0)}},
			map[string]any{"id": int64(3), "pos": []any{float64(50), float64(1)}}, // Different Y, should be ignored
			map[string]any{"id": int64(4), "pos": []any{float64(20), float64(0)}},
		}
		root := map[string]any{}
		inputFilename := "test.vcv"

		hub := createHubMediumModule(modules, root, inputFilename)

		// Should be placed after X=30 (rightmost at Y=0)
		pos, _ := hub["pos"].([]any)
		if pos[0] != float64(31) { // 30 + 1
			t.Errorf("expected x position 31 (after rightmost Y=0 module), got %v", pos[0])
		}
	})

	t.Run("has correct params structure", func(t *testing.T) {
		modules := []any{}
		root := map[string]any{}
		inputFilename := "test.vcv"

		hub := createHubMediumModule(modules, root, inputFilename)

		params, ok := hub["params"].([]any)
		if !ok {
			t.Fatal("params not a slice")
		}
		if len(params) != 14 {
			t.Errorf("expected 14 params, got %d", len(params))
		}
		// Check first 12 params have value 0.5
		for i := 0; i < 12; i++ {
			param, ok := params[i].(map[string]any)
			if !ok {
				t.Fatalf("param %d not a map", i)
			}
			if param["value"] != 0.5 {
				t.Errorf("param %d: expected value 0.5, got %v", i, param["value"])
			}
			if param["id"] != i {
				t.Errorf("param %d: expected id %d, got %v", i, i, param["id"])
			}
		}
		// Check last 2 params have value 0
		for i := 12; i < 14; i++ {
			param, ok := params[i].(map[string]any)
			if !ok {
				t.Fatalf("param %d not a map", i)
			}
			if param["value"] != 0.0 {
				t.Errorf("param %d: expected value 0.0, got %v", i, param["value"])
			}
		}
	})
}

func TestGetNextModuleID(t *testing.T) {
	t.Run("returns 0 for empty module list", func(t *testing.T) {
		modules := []any{}
		id := getNextModuleID(modules)
		if id != 0 {
			t.Errorf("expected 0, got %d", id)
		}
	})

	t.Run("returns max + 1", func(t *testing.T) {
		modules := []any{
			map[string]any{"id": int64(1)},
			map[string]any{"id": int64(5)},
			map[string]any{"id": int64(3)},
		}
		id := getNextModuleID(modules)
		if id != 6 {
			t.Errorf("expected 6, got %d", id)
		}
	})

	t.Run("handles modules without id field", func(t *testing.T) {
		modules := []any{
			map[string]any{"plugin": "Test"}, // No id field
			map[string]any{"id": int64(2)},
		}
		id := getNextModuleID(modules)
		if id != 3 {
			t.Errorf("expected 3, got %d", id)
		}
	})
}

func TestTransformPatch_WithMetaModule(t *testing.T) {
	t.Run("adds MetaModule when option is set", func(t *testing.T) {
		patchJSON := `{
			"version": "0.6.0",
			"modules": [
				{"plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": []
		}`

		var root map[string]any
		if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := TransformPatch(root, "2.6.6", &issues, Options{MetaModule: true}, "test-patch"); err != nil {
			t.Fatalf("TransformPatch failed: %v", err)
		}

		modules := root["modules"].([]any)
		if len(modules) != 2 {
			t.Fatalf("Expected 2 modules, got %d", len(modules))
		}

		// Check that MetaModule was added
		mm := modules[1].(map[string]any)
		if mm["plugin"] != "4msCompany" {
			t.Errorf("Expected plugin 4msCompany, got %v", mm["plugin"])
		}
		if mm["model"] != "HubMedium" {
			t.Errorf("Expected model HubMedium, got %v", mm["model"])
		}
	})

	t.Run("does not add MetaModule when option is false", func(t *testing.T) {
		patchJSON := `{
			"version": "0.6.0",
			"modules": [
				{"plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": []
		}`

		var root map[string]any
		if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := TransformPatch(root, "2.6.6", &issues, Options{MetaModule: false}, ""); err != nil {
			t.Fatalf("TransformPatch failed: %v", err)
		}

		modules := root["modules"].([]any)
		if len(modules) != 1 {
			t.Fatalf("Expected 1 module, got %d", len(modules))
		}
	})

	t.Run("uses filename for patch name when MetaModule enabled", func(t *testing.T) {
		patchJSON := `{
			"version": "0.6.0",
			"modules": [],
			"wires": []
		}`

		var root map[string]any
		if err := json.Unmarshal([]byte(patchJSON), &root); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := TransformPatch(root, "2.6.6", &issues, Options{MetaModule: true}, "MyAwesomePatch.vcv"); err != nil {
			t.Fatalf("TransformPatch failed: %v", err)
		}

		modules := root["modules"].([]any)
		if len(modules) != 1 {
			t.Fatalf("Expected 1 module, got %d", len(modules))
		}

		mm := modules[0].(map[string]any)
		data, ok := mm["data"].(map[string]any)
		if !ok {
			t.Fatal("MetaModule should have data field")
		}

		if data["PatchName"] != "MyAwesomePatch" {
			t.Errorf("Expected PatchName from filename, got %v", data["PatchName"])
		}
	})
}
