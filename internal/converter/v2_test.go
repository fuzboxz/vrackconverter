package converter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeV2_PreservesAllData(t *testing.T) {
	// V2 patches should have all data preserved - normalization is a no-op for data
	v2JSON := `{
		"version": "2.6.6",
		"masterModuleId": 1,
		"modules": [
			{
				"id": 1,
				"plugin": "Core",
				"model": "AudioInterface2",
				"version": "2.1.4",
				"params": [{"id": 0, "value": 0.0}],
				"data": {"audio": {"driver": 0, "device": 0}},
				"pos": [0, 0]
			},
			{
				"id": 2,
				"plugin": "Fundamental",
				"model": "VCO-2",
				"version": "2.0.2",
				"params": [{"id": 0, "value": 0.5}],
				"leftModuleId": 1,
				"rightModuleId": 1,
				"pos": [10, 0],
				"bypass": false
			}
		],
		"cables": [
			{
				"id": 0,
				"outputModuleId": 2,
				"outputId": 0,
				"inputModuleId": 1,
				"inputId": 0,
				"color": "ff0000ff"
			}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(patch, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Check that all v2-specific fields are preserved
	if getInt64FromMap(patch, "masterModuleId") != 1 {
		t.Errorf("masterModuleId should be preserved, got %v", patch["masterModuleId"])
	}

	modules := patch["modules"].([]any)
	if len(modules) != 2 {
		t.Fatalf("Expected 2 modules, got %d", len(modules))
	}

	// Check first module has version field
	mod0 := modules[0].(map[string]any)
	if mod0["version"] != "2.1.4" {
		t.Errorf("Module version should be preserved, got %v", mod0["version"])
	}

	// Check second module has expander fields
	mod1 := modules[1].(map[string]any)
	if getInt64FromMap(mod1, "leftModuleId") != 1 {
		t.Errorf("leftModuleId should be preserved, got %v", mod1["leftModuleId"])
	}
	if getInt64FromMap(mod1, "rightModuleId") != 1 {
		t.Errorf("rightModuleId should be preserved, got %v", mod1["rightModuleId"])
	}

	if bypass, ok := mod1["bypass"].(bool); !ok || bypass {
		t.Errorf("bypass should be preserved as false, got %v", mod1["bypass"])
	}

	// Check cable color is preserved
	cables := patch["cables"].([]any)
	cable := cables[0].(map[string]any)
	if cable["color"] != "ff0000ff" {
		t.Errorf("Cable color should be preserved, got %v", cable["color"])
	}
}

func TestNormalizeV2_BuildsIDToIndexMapping(t *testing.T) {
	v2JSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 5, "plugin": "Core", "model": "AudioInterface2"},
			{"id": 10, "plugin": "Fundamental", "model": "VCO-2"},
			{"id": 15, "plugin": "Fundamental", "model": "VCF"}
		],
		"cables": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(patch, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Check that ID-to-index mapping was created
	mapping := GetIDToIndexMapping(patch)
	if mapping == nil {
		t.Fatal("ID-to-index mapping should be created")
	}

	// Check mapping correctness
	if mapping[5] != 0 {
		t.Errorf("ID 5 should map to index 0, got %d", mapping[5])
	}
	if mapping[10] != 1 {
		t.Errorf("ID 10 should map to index 1, got %d", mapping[10])
	}
	if mapping[15] != 2 {
		t.Errorf("ID 15 should map to index 2, got %d", mapping[15])
	}
}

func TestNormalizeV2_DetectsDuplicateIDs(t *testing.T) {
	v2JSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2"},
			{"id": 1, "plugin": "Fundamental", "model": "VCO-2"}
		],
		"cables": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(patch, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Should have a duplicate ID warning
	foundWarning := false
	for _, issue := range issues {
		if strings.Contains(issue, "duplicate module ID") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected warning about duplicate module ID")
	}
}

func TestNormalizeV2_DetectsBrokenCableReferences(t *testing.T) {
	v2JSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2"}
		],
		"cables": [
			{"outputModuleId": 999, "outputId": 0, "inputModuleId": 1, "inputId": 0}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(patch, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Should have warning about missing module ID
	foundWarning := false
	for _, issue := range issues {
		if strings.Contains(issue, "outputModuleId 999 not found") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Expected warning about missing module ID in cable reference")
	}
}

func TestNormalizeV2_CreatesCablesArrayWhenMissing(t *testing.T) {
	v2JSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2"}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(patch, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Cables array should be created
	cables, ok := patch["cables"].([]any)
	if !ok {
		t.Error("Cables array should be created")
	}
	if len(cables) != 0 {
		t.Errorf("Expected empty cables array, got %d items", len(cables))
	}
}

func TestNormalizeV2_ConvertsWiresToCables(t *testing.T) {
	v2JSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2"}
		],
		"wires": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 1, "inputId": 0}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(patch, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Wires should be converted to cables
	if _, hasWires := patch["wires"]; hasWires {
		t.Error("wires should be removed")
	}
	if _, hasCables := patch["cables"]; !hasCables {
		t.Error("cables array should exist")
	}
}

func TestDenormalizeV2_ConvertsDisabledToBypass(t *testing.T) {
	patchJSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2", "disabled": true}
		],
		"cables": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	modules := patch["modules"].([]any)
	mod := modules[0].(map[string]any)

	// Check disabled was converted to bypass
	if _, hasDisabled := mod["disabled"]; hasDisabled {
		t.Error("disabled field should be removed")
	}

	if bypass, ok := mod["bypass"].(bool); !ok || !bypass {
		t.Errorf("Expected bypass=true, got %v", mod["bypass"])
	}

	// Should have issue message
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, "converted disabled") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected issue message about disabled→bypass conversion")
	}
}

func TestDenormalizeV2_AssignsMissingIDs(t *testing.T) {
	patchJSON := `{
		"version": "2.6.6",
		"modules": [
			{"plugin": "Core", "model": "AudioInterface2"},
			{"plugin": "Fundamental", "model": "VCO-2"}
		],
		"cables": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	modules := patch["modules"].([]any)

	// Check that IDs were assigned
	mod0 := modules[0].(map[string]any)
	if getInt64(mod0["id"]) != 0 {
		t.Errorf("Module 0: expected id 0, got %v", mod0["id"])
	}

	mod1 := modules[1].(map[string]any)
	if getInt64(mod1["id"]) != 1 {
		t.Errorf("Module 1: expected id 1, got %v", mod1["id"])
	}
}

func TestDenormalizeV2_ConvertsWiresToCables(t *testing.T) {
	patchJSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2"}
		],
		"wires": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 1, "inputId": 0}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	// Wires should be converted to cables
	if _, hasWires := patch["wires"]; hasWires {
		t.Error("wires should be removed")
	}
	if _, hasCables := patch["cables"]; !hasCables {
		t.Error("cables array should exist")
	}
}

func TestDenormalizeV2_AssignsCableIDs(t *testing.T) {
	patchJSON := `{
		"version": "2.6.6",
		"modules": [
			{"id": 1, "plugin": "Core", "model": "AudioInterface2"}
		],
		"cables": [
			{"outputModuleId": 1, "outputId": 0, "inputModuleId": 1, "inputId": 0},
			{"outputModuleId": 1, "outputId": 1, "inputModuleId": 1, "inputId": 1}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	cables := patch["cables"].([]any)

	// Check that cable IDs were assigned
	for i, c := range cables {
		cable := c.(map[string]any)
		if getInt64(cable["id"]) != int64(i) {
			t.Errorf("Cable %d: expected id %d, got %v", i, i, cable["id"])
		}
	}
}

func TestDenormalizeV2_SetsVersion(t *testing.T) {
	patchJSON := `{
		"version": "0.6.0",
		"modules": [{"id": 1, "plugin": "Core", "model": "AudioInterface2"}],
		"cables": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	// Version should be set to v2
	if patch["version"] != "2.6.6" {
		t.Errorf("Expected version 2.6.6, got %v", patch["version"])
	}
}

func TestDenormalizeV2_RemovesInternalFields(t *testing.T) {
	patchJSON := `{
		"version": "2.6.6",
		"modules": [{"id": 1, "plugin": "Core", "model": "AudioInterface2"}],
		"cables": []
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Add internal field
	patch["_idToIndex"] = map[int64]int{1: 0}

	var issues []string
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	// Internal field should be removed
	if _, hasMapping := patch["_idToIndex"]; hasMapping {
		t.Error("_idToIndex should be removed from output")
	}
}

func TestNormalizeV2_Roundtrip(t *testing.T) {
	// Test that normalizing a v2 patch doesn't lose data
	originalJSON := `{
		"version": "2.5.0",
		"masterModuleId": 5,
		"name": "Test Patch",
		"modules": [
			{
				"id": 1,
				"plugin": "Core",
				"model": "AudioInterface2",
				"version": "2.1.0",
				"params": [{"id": 0, "value": 0.5}],
				"leftModuleId": 2,
				"rightModuleId": 3,
				"bypass": true,
				"data": {"custom": "data"}
			}
		],
		"cables": [
			{
				"id": 0,
				"outputModuleId": 1,
				"outputId": 0,
				"inputModuleId": 1,
				"inputId": 0,
				"color": "ff00ff00"
			}
		]
	}`

	var original map[string]any
	if err := json.Unmarshal([]byte(originalJSON), &original); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var issues []string
	if err := NormalizeV2(original, &issues); err != nil {
		t.Fatalf("NormalizeV2 failed: %v", err)
	}

	// Check all critical fields are preserved
	if getInt64FromMap(original, "masterModuleId") != 5 {
		t.Error("masterModuleId should be preserved")
	}
	if original["name"] != "Test Patch" {
		t.Error("name should be preserved")
	}

	modules := original["modules"].([]any)
	mod := modules[0].(map[string]any)

	if mod["version"] != "2.1.0" {
		t.Error("module version should be preserved")
	}
	if getInt64FromMap(mod, "leftModuleId") != 2 {
		t.Error("leftModuleId should be preserved")
	}
	if getInt64FromMap(mod, "rightModuleId") != 3 {
		t.Error("rightModuleId should be preserved")
	}
	if mod["bypass"] != true {
		t.Error("bypass should be preserved")
	}
	if mod["data"].(map[string]any)["custom"] != "data" {
		t.Error("module data should be preserved")
	}

	cables := original["cables"].([]any)
	cable := cables[0].(map[string]any)
	if cable["color"] != "ff00ff00" {
		t.Error("cable color should be preserved")
	}
}

func TestGetIDToIndexMapping(t *testing.T) {
	t.Run("returns nil for non-normalized patch", func(t *testing.T) {
		patch := map[string]any{
			"version": "2.6.6",
			"modules": []any{},
		}

		mapping := GetIDToIndexMapping(patch)
		if mapping != nil {
			t.Error("Expected nil for non-normalized patch")
		}
	})

	t.Run("returns mapping for normalized patch", func(t *testing.T) {
		patch := map[string]any{
			"version": "2.6.6",
			"modules": []any{
				map[string]any{"id": int64(1)},
				map[string]any{"id": int64(5)},
			},
			"cables": []any{},
		}

		var issues []string
		NormalizeV2(patch, &issues)

		mapping := GetIDToIndexMapping(patch)
		if mapping == nil {
			t.Fatal("Expected mapping after normalization")
		}
		if mapping[1] != 0 {
			t.Errorf("ID 1 should map to index 0, got %d", mapping[1])
		}
		if mapping[5] != 1 {
			t.Errorf("ID 5 should map to index 1, got %d", mapping[5])
		}
	})
}
