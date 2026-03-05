package converter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMiRackHandler_Read tests reading MiRack patches.
func TestMiRackHandler_Read(t *testing.T) {
	t.Run("reads from .mrk bundle directory", func(t *testing.T) {
		handler := &MiRackHandler{}

		// Use the test fixture .mrk bundle
		data, err := handler.Read("/Users/fuzboxz/vrackconverter/test/mirackoutput.mrk")
		if err != nil {
			t.Fatalf("Failed to read .mrk bundle: %v", err)
		}

		// Verify it's valid JSON
		var patch map[string]any
		if err := json.Unmarshal(data, &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Check it's a MiRack format patch
		if version, ok := patch["version"].(string); !ok || !strings.HasPrefix(version, "0.") {
			t.Errorf("Expected v0.6 version, got %v", patch["version"])
		}

		// Check for wires (MiRack uses "wires", not "cables")
		if _, hasWires := patch["wires"]; !hasWires {
			t.Error("MiRack patch should have 'wires' field")
		}
	})

	t.Run("reads from direct .vcv file path", func(t *testing.T) {
		handler := &MiRackHandler{}

		// Read the patch.vcv file directly
		data, err := handler.Read("/Users/fuzboxz/vrackconverter/test/mirackoutput.mrk/patch.vcv")
		if err != nil {
			t.Fatalf("Failed to read patch.vcv: %v", err)
		}

		// Verify it's valid JSON
		var patch map[string]any
		if err := json.Unmarshal(data, &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}
	})

	t.Run("returns error for non-existent path", func(t *testing.T) {
		handler := &MiRackHandler{}

		_, err := handler.Read("/nonexistent/path.mrk")
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
	})

	t.Run("returns error for v2 zstd archives", func(t *testing.T) {
		handler := &MiRackHandler{}

		// Create a v2 archive for testing
		tmpDir := t.TempDir()
		v2Path := filepath.Join(tmpDir, "test.vcv")
		testJSON := []byte(`{"version": "2.6.6", "modules": [], "cables": []}`)
		if err := CreateV2Patch(testJSON, v2Path); err != nil {
			t.Fatalf("Failed to create v2 archive: %v", err)
		}

		// MiRackHandler should reject v2 archives
		_, err := handler.Read(v2Path)
		if err == nil {
			t.Error("Expected error for v2 zstd archive")
		}
		if !strings.Contains(err.Error(), "zstd") {
			t.Errorf("Error should mention zstd archive, got: %v", err)
		}
	})
}

// TestMiRackHandler_Write tests writing MiRack patches.
func TestMiRackHandler_Write(t *testing.T) {
	t.Run("creates .mrk bundle with patch.vcv", func(t *testing.T) {
		handler := &MiRackHandler{}

		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "mirack-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		mrkPath := filepath.Join(tmpDir, "test.mrk")

		// Test data
		testData := []byte(`{"version": "0.6.13", "modules": [], "wires": []}`)

		// Write the bundle
		if err := handler.Write(testData, mrkPath); err != nil {
			t.Fatalf("Failed to write .mrk bundle: %v", err)
		}

		// Verify directory was created
		info, err := os.Stat(mrkPath)
		if err != nil {
			t.Fatalf("Failed to stat .mrk directory: %v", err)
		}
		if !info.IsDir() {
			t.Error(".mrk path should be a directory")
		}

		// Verify patch.vcv exists
		patchPath := filepath.Join(mrkPath, "patch.vcv")
		data, err := os.ReadFile(patchPath)
		if err != nil {
			t.Fatalf("Failed to read patch.vcv: %v", err)
		}

		// Verify content
		if string(data) != string(testData) {
			t.Errorf("Content mismatch: got %s, want %s", string(data), string(testData))
		}
	})

	t.Run("returns error for non-.mrk extension", func(t *testing.T) {
		handler := &MiRackHandler{}

		err := handler.Write([]byte("{}"), "/tmp/test.vcv")
		if err == nil {
			t.Error("Expected error for non-.mrk extension")
		}
		if !strings.Contains(err.Error(), ".mrk") {
			t.Errorf("Error should mention .mrk extension, got: %v", err)
		}
	})
}

// TestMiRackHandler_Extension tests the extension method.
func TestMiRackHandler_Extension(t *testing.T) {
	handler := &MiRackHandler{}
	if ext := handler.Extension(); ext != ".mrk" {
		t.Errorf("Expected extension .mrk, got %s", ext)
	}
}

// TestNormalizeMiRack tests MiRack to internal format conversion.
func TestNormalizeMiRack(t *testing.T) {
	t.Run("converts wires to cables", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []},
				{"id": 2, "plugin": "Fundamental", "model": "VCO", "params": []}
			],
			"wires": [
				{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// Wires should be converted to cables
		if _, hasWires := patch["wires"]; hasWires {
			t.Error("wires should be removed after normalization")
		}
		if _, hasCables := patch["cables"]; !hasCables {
			t.Error("cables array should exist after normalization")
		}
	})

	t.Run("converts array indices to module IDs", func(t *testing.T) {
		// MiRack uses array indices in wire references
		// In this test: outputModuleId: 1 = array index 1 = module with ID 2
		//                 inputModuleId: 0 = array index 0 = module with ID 1
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []},
				{"id": 2, "plugin": "Fundamental", "model": "VCO", "params": []}
			],
			"wires": [
				{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		cables := patch["cables"].([]any)
		if len(cables) != 1 {
			t.Fatalf("Expected 1 cable, got %d", len(cables))
		}

		cable := cables[0].(map[string]any)

		// Array index 1 should be converted to module ID 2
		if getInt64FromMap(cable, "outputModuleId") != 2 {
			t.Errorf("outputModuleId should be 2 (module ID), got %v", cable["outputModuleId"])
		}

		// Array index 0 should be converted to module ID 1
		if getInt64FromMap(cable, "inputModuleId") != 1 {
			t.Errorf("inputModuleId should be 1 (module ID), got %v", cable["inputModuleId"])
		}
	})

	t.Run("converts paramId to id", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{
					"id": 1,
					"plugin": "Core",
					"model": "AudioInterface",
					"params": [
						{"paramId": 0, "value": 0.5},
						{"paramId": 1, "value": 0.3}
					]
				}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)
		mod := modules[0].(map[string]any)
		params := mod["params"].([]any)

		for i, p := range params {
			param := p.(map[string]any)
			if _, hasID := param["id"]; !hasID {
				t.Errorf("Param %d: missing 'id' field", i)
			}
			if _, hasParamID := param["paramId"]; hasParamID {
				t.Errorf("Param %d: 'paramId' should be removed", i)
			}
		}
	})

	t.Run("converts disabled to bypass", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "disabled": true, "params": []}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)
		mod := modules[0].(map[string]any)

		if _, hasDisabled := mod["disabled"]; hasDisabled {
			t.Error("'disabled' field should be removed")
		}

		if bypass, ok := mod["bypass"].(bool); !ok || !bypass {
			t.Errorf("Expected bypass=true, got %v", mod["bypass"])
		}
	})

	t.Run("converts color format", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": [
				{
					"outputModuleId": 0,
					"outputId": 0,
					"inputModuleId": 0,
					"inputId": 0,
					"color": {"r": 1.0, "g": 0.0, "b": 0.0, "a": 1.0}
				}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		cables := patch["cables"].([]any)
		cable := cables[0].(map[string]any)

		// Color should be converted to hex string
		if color, ok := cable["color"].(string); !ok || color != "ff0000ff" {
			t.Errorf("Expected color 'ff0000ff', got %v", cable["color"])
		}
	})

	t.Run("handles modules without IDs", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"plugin": "Core", "model": "AudioInterface", "params": []},
				{"plugin": "Fundamental", "model": "VCO", "params": []}
			],
			"wires": [
				{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		cables := patch["cables"].([]any)
		cable := cables[0].(map[string]any)

		// Array indices should be used as IDs when modules don't have IDs
		if getInt64FromMap(cable, "outputModuleId") != 1 {
			t.Errorf("outputModuleId should be 1 (array index as ID), got %v", cable["outputModuleId"])
		}
		if getInt64FromMap(cable, "inputModuleId") != 0 {
			t.Errorf("inputModuleId should be 0 (array index as ID), got %v", cable["inputModuleId"])
		}
	})

	t.Run("removes MiRack-specific fields", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "sumPolyInputs": true, "params": []}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)
		mod := modules[0].(map[string]any)

		if _, hasField := mod["sumPolyInputs"]; hasField {
			t.Error("sumPolyInputs should be removed (MiRack-specific field)")
		}
	})
}

// TestDenormalizeMiRack tests internal format to MiRack conversion.
func TestDenormalizeMiRack(t *testing.T) {
	t.Run("converts cables to wires", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"cables": [
				{"outputModuleId": 1, "outputId": 0, "inputModuleId": 1, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// First normalize to build ID-to-index mapping
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// Cables should be converted to wires
		if _, hasCables := patch["cables"]; hasCables {
			t.Error("cables should be removed after denormalization")
		}
		if _, hasWires := patch["wires"]; !hasWires {
			t.Error("wires array should exist after denormalization")
		}
	})

	t.Run("converts module IDs to array indices", func(t *testing.T) {
		// This is the KEY test for v2 → MiRack conversion
		// We need to verify that module IDs in cables are converted to array indices
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 5, "plugin": "Core", "model": "AudioInterface", "params": []},
				{"id": 10, "plugin": "Fundamental", "model": "VCO", "params": []}
			],
			"cables": [
				{"outputModuleId": 10, "outputId": 0, "inputModuleId": 5, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// First normalize to build ID-to-index mapping
		// ID 5 → index 0, ID 10 → index 1
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Verify mapping was created
		mapping := GetIDToIndexMapping(patch)
		if mapping == nil {
			t.Fatal("ID-to-index mapping should be created")
		}
		if mapping[5] != 0 {
			t.Errorf("ID 5 should map to index 0, got %d", mapping[5])
		}
		if mapping[10] != 1 {
			t.Errorf("ID 10 should map to index 1, got %d", mapping[10])
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		wires := patch["wires"].([]any)
		if len(wires) != 1 {
			t.Fatalf("Expected 1 wire, got %d", len(wires))
		}

		wire := wires[0].(map[string]any)

		// Module ID 10 should be converted to array index 1
		if getInt64FromMap(wire, "outputModuleId") != 1 {
			t.Errorf("outputModuleId should be 1 (array index), got %v", wire["outputModuleId"])
		}

		// Module ID 5 should be converted to array index 0
		if getInt64FromMap(wire, "inputModuleId") != 0 {
			t.Errorf("inputModuleId should be 0 (array index), got %v", wire["inputModuleId"])
		}
	})

	t.Run("converts bypass to disabled", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "bypass": true, "params": []}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize first
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)
		mod := modules[0].(map[string]any)

		if _, hasBypass := mod["bypass"]; hasBypass {
			t.Error("'bypass' field should be removed")
		}

		if disabled, ok := mod["disabled"].(bool); !ok || !disabled {
			t.Errorf("Expected disabled=true, got %v", mod["disabled"])
		}
	})

	t.Run("converts id to paramId", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{
					"id": 1,
					"plugin": "Core",
					"model": "AudioInterface",
					"params": [
						{"id": 0, "value": 0.5},
						{"id": 1, "value": 0.3}
					]
				}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize first
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)
		mod := modules[0].(map[string]any)
		params := mod["params"].([]any)

		for i, p := range params {
			param := p.(map[string]any)
			if _, hasID := param["id"]; hasID {
				t.Errorf("Param %d: 'id' should be removed", i)
			}
			if _, hasParamID := param["paramId"]; !hasParamID {
				t.Errorf("Param %d: missing 'paramId' field", i)
			}
		}
	})

	t.Run("removes cable ID and color", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"cables": [
				{
					"id": 42,
					"outputModuleId": 1,
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

		// Normalize first
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		wires := patch["wires"].([]any)
		wire := wires[0].(map[string]any)

		if _, hasID := wire["id"]; hasID {
			t.Error("Cable 'id' should be removed (MiRack doesn't use cable IDs)")
		}

		if _, hasColor := wire["color"]; hasColor {
			t.Error("Cable 'color' should be removed (MiRack uses colorIndex, not hex)")
		}
	})

	t.Run("removes v2-specific fields", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"masterModuleId": 1,
			"modules": [
				{
					"id": 1,
					"plugin": "Core",
					"model": "AudioInterface",
					"version": "2.1.4",
					"leftModuleId": 2,
					"rightModuleId": 3,
					"params": []
				}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize first
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// Check root-level fields
		if _, hasField := patch["masterModuleId"]; hasField {
			t.Error("masterModuleId should be removed (not supported by MiRack)")
		}
		if _, hasField := patch["_idToIndex"]; hasField {
			t.Error("_idToIndex should be removed (internal field)")
		}

		// Check module-level fields
		modules := patch["modules"].([]any)
		mod := modules[0].(map[string]any)

		if _, hasField := mod["version"]; hasField {
			t.Error("module 'version' should be removed (not supported by MiRack)")
		}
		if _, hasField := mod["leftModuleId"]; hasField {
			t.Error("'leftModuleId' should be removed (expander links not supported)")
		}
		if _, hasField := mod["rightModuleId"]; hasField {
			t.Error("'rightModuleId' should be removed (expander links not supported)")
		}
	})

	t.Run("sets version to 0.6.13", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"modules": [{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize first
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Then denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		if patch["version"] != "0.6.13" {
			t.Errorf("Expected version 0.6.13, got %v", patch["version"])
		}
	})

	t.Run("builds ID-to-index mapping when not available", func(t *testing.T) {
		// Test the fallback case where _idToIndex is not available
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 10, "plugin": "Core", "model": "AudioInterface", "params": []},
				{"id": 20, "plugin": "Fundamental", "model": "VCO", "params": []}
			],
			"cables": [
				{"outputModuleId": 20, "outputId": 0, "inputModuleId": 10, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// DON'T normalize - directly denormalize
		// This tests the fallback ID-to-index mapping logic
		var issues []string
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		wires := patch["wires"].([]any)
		wire := wires[0].(map[string]any)

		// Should still convert correctly
		if getInt64FromMap(wire, "outputModuleId") != 1 {
			t.Errorf("outputModuleId should be 1 (array index of ID 20), got %v", wire["outputModuleId"])
		}
		if getInt64FromMap(wire, "inputModuleId") != 0 {
			t.Errorf("inputModuleId should be 0 (array index of ID 10), got %v", wire["inputModuleId"])
		}
	})
}

// TestCreateMrkBundle tests creating .mrk directory bundles.
func TestCreateMrkBundle(t *testing.T) {
	t.Run("creates valid .mrk bundle", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "mirack-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		mrkPath := filepath.Join(tmpDir, "test.mrk")
		testData := []byte(`{"version": "0.6.13", "modules": [], "wires": []}`)

		if err := CreateMrkBundle(testData, mrkPath); err != nil {
			t.Fatalf("CreateMrkBundle failed: %v", err)
		}

		// Verify directory structure
		info, err := os.Stat(mrkPath)
		if err != nil {
			t.Fatalf("Failed to stat .mrk directory: %v", err)
		}
		if !info.IsDir() {
			t.Error(".mrk path should be a directory")
		}

		// Verify patch.vcv exists and has correct content
		patchPath := filepath.Join(mrkPath, "patch.vcv")
		data, err := os.ReadFile(patchPath)
		if err != nil {
			t.Fatalf("Failed to read patch.vcv: %v", err)
		}

		if string(data) != string(testData) {
			t.Errorf("Content mismatch: got %s, want %s", string(data), string(testData))
		}
	})

	t.Run("returns error for non-.mrk extension", func(t *testing.T) {
		testData := []byte(`{}`)

		err := CreateMrkBundle(testData, "/tmp/test.vcv")
		if err == nil {
			t.Error("Expected error for non-.mrk extension")
		}
	})
}

// TestMiRackRoundtrip tests full MiRack → internal → MiRack roundtrip.
func TestMiRackRoundtrip(t *testing.T) {
	originalJSON := `{
		"version": "0.6.2",
		"modules": [
			{
				"id": 1,
				"plugin": "Core",
				"model": "AudioInterface",
				"params": [
					{"paramId": 0, "value": 0.5}
				],
				"pos": [0, 0]
			},
			{
				"id": 2,
				"plugin": "Fundamental",
				"model": "VCO",
				"params": [
					{"paramId": 0, "value": 0.0},
					{"paramId": 1, "value": 0.5}
				],
				"disabled": true,
				"pos": [10, 0]
			}
		],
		"wires": [
			{
				"outputModuleId": 1,
				"outputId": 0,
				"inputModuleId": 0,
				"inputId": 0
			}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Normalize: MiRack → internal
	var issues []string
	if err := NormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("NormalizeMiRack failed: %v", err)
	}

	// Denormalize: internal → MiRack
	if err := DenormalizeMiRack(patch, &issues); err != nil {
		t.Fatalf("DenormalizeMiRack failed: %v", err)
	}

	// Verify the result is valid MiRack format
	if version, ok := patch["version"].(string); !ok || version != "0.6.13" {
		t.Errorf("Expected version 0.6.13, got %v", patch["version"])
	}

	// Check wires exist
	if _, hasWires := patch["wires"]; !hasWires {
		t.Error("Should have 'wires' field")
	}

	// Check cables don't exist
	if _, hasCables := patch["cables"]; hasCables {
		t.Error("Should not have 'cables' field")
	}

	// Check modules have disabled, not bypass
	modules := patch["modules"].([]any)
	mod := modules[1].(map[string]any)
	if _, hasDisabled := mod["disabled"]; !hasDisabled {
		t.Error("Should have 'disabled' field")
	}
	if _, hasBypass := mod["bypass"]; hasBypass {
		t.Error("Should not have 'bypass' field")
	}

	// Check params have paramId, not id
	params := mod["params"].([]any)
	param := params[0].(map[string]any)
	if _, hasParamID := param["paramId"]; !hasParamID {
		t.Error("Should have 'paramId' field")
	}
	if _, hasID := param["id"]; hasID {
		t.Error("Should not have 'id' field in params")
	}

	// Check wire references use array indices, not module IDs
	wires := patch["wires"].([]any)
	wire := wires[0].(map[string]any)
	// Original: outputModuleId: 1 (array index 1) = module with ID 2
	// After roundtrip, should still be array index 1
	if getInt64FromMap(wire, "outputModuleId") != 1 {
		t.Errorf("outputModuleId should be 1 (array index), got %v", wire["outputModuleId"])
	}
	if getInt64FromMap(wire, "inputModuleId") != 0 {
		t.Errorf("inputModuleId should be 0 (array index), got %v", wire["inputModuleId"])
	}
}
