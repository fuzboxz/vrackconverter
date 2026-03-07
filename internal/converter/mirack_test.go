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
		data, err := handler.Read("../../test/mirackoutput.mrk")
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
		data, err := handler.Read("../../test/mirackoutput.mrk/patch.vcv")
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
				{"id": 2, "plugin": "Core", "model": "VCO", "params": []}
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
				{"id": 2, "plugin": "Core", "model": "VCO", "params": []}
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

	t.Run("handles modules without IDs", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"plugin": "Core", "model": "AudioInterface", "params": []},
				{"plugin": "Core", "model": "VCO", "params": []}
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
				"plugin": "Core",
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

// TestMiRackRoundtripPreservesAllCables tests that the roundtrip preserves all cables
// even when there are module ID changes or references that might not be in the fallback mapping.
// This is the key test for the bug fix where cables were being skipped.
func TestMiRackRoundtripPreservesAllCables(t *testing.T) {
	t.Run("preserves cables through roundtrip", func(t *testing.T) {
		// Start with a MiRack patch with non-sequential module IDs
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 100, "plugin": "Core", "model": "VCO", "params": []},
				{"id": 200, "plugin": "Core", "model": "VCA", "params": []},
				{"id": 300, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": [
				{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0},
				{"outputModuleId": 2, "outputId": 0, "inputModuleId": 1, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Count original wires
		originalWires := patch["wires"].([]any)
		originalWireCount := len(originalWires)

		// MiRack → V2 (normalize)
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// V2 → MiRack (denormalize)
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// Verify all cables are preserved
		finalWires := patch["wires"].([]any)
		finalWireCount := len(finalWires)

		if finalWireCount != originalWireCount {
			t.Errorf("Wire count mismatch: expected %d, got %d", originalWireCount, finalWireCount)
		}

		// Verify _originalIndexToID was cleaned up
		if _, hasMapping := patch["_originalIndexToID"]; hasMapping {
			t.Error("_originalIndexToID should be removed after denormalization")
		}
	})

	t.Run("uses stored mapping for correct reversal", func(t *testing.T) {
		// This test verifies that the stored _originalIndexToID mapping is used
		// during denormalization to correctly reverse the cable references
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 42, "plugin": "Core", "model": "VCO", "params": []},
				{"id": 99, "plugin": "Core", "model": "VCA", "params": []}
			],
			"wires": [
				{"outputModuleId": 1, "outputId": 0, "inputModuleId": 0, "inputId": 0}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// Verify _originalIndexToID was stored
		mapping, ok := patch["_originalIndexToID"].(map[int]int64)
		if !ok {
			t.Fatal("_originalIndexToID mapping should be stored during normalization")
		}

		// Verify the mapping: index 0 → ID 42, index 1 → ID 99
		if mapping[0] != 42 {
			t.Errorf("Index 0 should map to ID 42, got %d", mapping[0])
		}
		if mapping[1] != 99 {
			t.Errorf("Index 1 should map to ID 99, got %d", mapping[1])
		}

		// Denormalize
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// Verify wire references were correctly converted back to array indices
		wires := patch["wires"].([]any)
		if len(wires) != 1 {
			t.Fatalf("Expected 1 wire, got %d", len(wires))
		}

		wire := wires[0].(map[string]any)
		// Original: outputModuleId: 1 (array index for VCA, ID 99)
		// After roundtrip, should still be array index 1
		if getInt64FromMap(wire, "outputModuleId") != 1 {
			t.Errorf("outputModuleId should be 1 (array index), got %v", wire["outputModuleId"])
		}
		// Original: inputModuleId: 0 (array index for VCO, ID 42)
		// After roundtrip, should still be array index 0
		if getInt64FromMap(wire, "inputModuleId") != 0 {
			t.Errorf("inputModuleId should be 0 (array index), got %v", wire["inputModuleId"])
		}
	})
}

// TestMiRackNoPluginConversion tests that MiRack does NOT convert plugins.
// MiRack does NOT have a "Fundamental" plugin - all modules use "Core".
// This is the key difference from VCV Rack v0.6.
func TestMiRackNoPluginConversion(t *testing.T) {
	t.Run("normalize: MiRack modules stay Core", func(t *testing.T) {
		// MiRack format - all modules use Core plugin
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "VCA-1", "params": []},
				{"id": 2, "plugin": "Core", "model": "VCO-1", "params": []},
				{"id": 3, "plugin": "Core", "model": "AudioInterface", "params": []}
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

		// All modules should stay Core (no plugin conversion)
		for i, m := range modules {
			mod := m.(map[string]any)
			plugin, _ := mod["plugin"].(string)
			if plugin != "Core" {
				t.Errorf("Module[%d]: expected plugin 'Core', got '%s'", i, plugin)
			}
		}

		// No plugin conversion issues should be logged
		for _, issue := range issues {
			if strings.Contains(issue, "→ Core/") {
				t.Errorf("MiRack should not convert plugins, but got issue: %s", issue)
			}
		}
	})

	t.Run("denormalize: V2 Core modules stay Core for MiRack", func(t *testing.T) {
		// V2 format with Core plugin
		// CRITICAL: MiRack does NOT have a "Fundamental" plugin - all modules use "Core"
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "VCA-1", "params": []},
				{"id": 2, "plugin": "Core", "model": "VCO-1", "params": []},
				{"id": 3, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize first to build ID-to-index mapping
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// Verify ALL modules stay Core (MiRack uses Core for everything)
		for i, m := range modules {
			mod := m.(map[string]any)
			plugin, _ := mod["plugin"].(string)
			if plugin != "Core" {
				t.Errorf("Module[%d]: expected plugin 'Core' for MiRack, got '%s'", i, plugin)
			}
		}
	})

	t.Run("roundtrip: MiRack → V2 → MiRack preserves Core plugin", func(t *testing.T) {
		// Start with MiRack format (ALL modules use Core plugin)
		// Note: MiRack does NOT have a "Fundamental" plugin
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "VCA-1", "params": []},
				{"id": 2, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// MiRack → V2 (normalize)
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// V2 → MiRack (denormalize)
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// Verify all modules use Core plugin (MiRack format)
		for i, m := range modules {
			mod := m.(map[string]any)
			plugin, _ := mod["plugin"].(string)
			if plugin != "Core" {
				t.Errorf("Module[%d]: roundtrip failed, expected plugin 'Core', got '%s'", i, plugin)
			}
		}
	})
}

// TestModuleIDZeroEdgeCase tests that module ID 0 is handled correctly.
// This is a critical test for the fix where we use hasInputModule flag instead of
// checking inputModuleID != 0, since 0 is a valid module ID.
func TestModuleIDZeroEdgeCase(t *testing.T) {
	t.Run("AudioInterfaceIn with id: 0 is preserved through roundtrip", func(t *testing.T) {
		// Create a MiRack patch with AudioInterfaceIn having id: 0
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{
					"id": 0,
					"plugin": "Core",
					"model": "AudioInterface",
					"params": [],
					"pos": [0, 0]
				},
				{
					"id": 0,
					"plugin": "Core",
					"model": "AudioInterfaceIn",
					"params": [],
					"pos": [3, 0]
				},
				{
					"id": 1,
					"plugin": "Core",
					"model": "VCO",
					"params": [],
					"pos": [6, 0]
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

		// Count original modules (should be 3)
		originalModules := patch["modules"].([]any)
		if len(originalModules) != 3 {
			t.Fatalf("Expected 3 modules initially, got %d", len(originalModules))
		}

		// MiRack → V2 (normalize)
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// After merge, should have 2 modules: AudioInterface2 + VCO
		normalizedModules := patch["modules"].([]any)
		if len(normalizedModules) != 2 {
			t.Errorf("After normalization: expected 2 modules (merged AudioInterface2 + VCO), got %d", len(normalizedModules))
		}

		// Check that AudioInterface2 exists
		var audioInterface2 map[string]any
		for _, m := range normalizedModules {
			mod := m.(map[string]any)
			if model, _ := mod["model"].(string); model == "AudioInterface2" {
				audioInterface2 = mod
				break
			}
		}
		if audioInterface2 == nil {
			t.Fatal("AudioInterface2 not found after normalization")
		}

		// Check that metadata has both output and input module IDs
		mergedData, ok := audioInterface2["_mergedAudioModule"].(map[string]any)
		if !ok {
			t.Fatal("Missing _mergedAudioModule metadata")
		}

		// Check that inputModuleID key exists (critical for the fix)
		// Key existence indicates that the input module existed, even if its ID is 0
		_, hasInputKey := mergedData["inputModuleID"]
		if !hasInputKey {
			t.Error("Expected inputModuleID key to exist in metadata (input module was present)")
		}
		// Also verify the actual ID value is 0
		inputID := getInt64FromMap(mergedData, "inputModuleID")
		if inputID != 0 {
			t.Errorf("Expected inputModuleID=0, got %d", inputID)
		}

		// V2 → MiRack (denormalize)
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// After roundtrip, should have 3 modules again (split back)
		finalModules := patch["modules"].([]any)
		if len(finalModules) != 3 {
			t.Errorf("After roundtrip: expected 3 modules, got %d", len(finalModules))
		}

		// Verify AudioInterfaceIn with id: 0 was restored
		var audioInterfaceIn *map[string]any
		for _, m := range finalModules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model == "AudioInterfaceIn" {
				mCopy := mod
				audioInterfaceIn = &mCopy
				break
			}
		}

		if audioInterfaceIn == nil {
			t.Fatal("AudioInterfaceIn not found after roundtrip - module ID 0 was lost!")
		}

		// Verify the restored AudioInterfaceIn has id: 0
		if id := getInt64FromMap(*audioInterfaceIn, "id"); id != 0 {
			t.Errorf("AudioInterfaceIn should have id=0, got %d", id)
		}
	})

	t.Run("AudioInterface output module with id: 0 works correctly", func(t *testing.T) {
		// Create a MiRack patch with AudioInterface (output) having id: 0
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{
					"id": 0,
					"plugin": "Core",
					"model": "AudioInterface",
					"params": [],
					"pos": [0, 0]
				},
				{
					"id": 5,
					"plugin": "Core",
					"model": "AudioInterfaceIn",
					"params": [],
					"pos": [3, 0]
				},
				{
					"id": 1,
					"plugin": "Core",
					"model": "VCO",
					"params": [],
					"pos": [6, 0]
				}
			],
			"wires": [
				{
					"outputModuleId": 0,
					"outputId": 0,
					"inputModuleId": 2,
					"inputId": 0
				}
			]
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// MiRack → V2 → MiRack (roundtrip)
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// Verify AudioInterface with id: 0 was preserved
		modules := patch["modules"].([]any)
		var audioInterface *map[string]any
		for _, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model == "AudioInterface" {
				mCopy := mod
				audioInterface = &mCopy
				break
			}
		}

		if audioInterface == nil {
			t.Fatal("AudioInterface not found after roundtrip")
		}

		if id := getInt64FromMap(*audioInterface, "id"); id != 0 {
			t.Errorf("AudioInterface should have id=0, got %d", id)
		}
	})

	t.Run("AudioInterface with NO input module - key should not exist", func(t *testing.T) {
		// Create a MiRack patch with AudioInterface but NO AudioInterfaceIn
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{
					"id": 7,
					"plugin": "Core",
					"model": "AudioInterface",
					"params": [],
					"pos": [0, 0]
				},
				{
					"id": 1,
					"plugin": "Core",
					"model": "VCO",
					"params": [],
					"pos": [6, 0]
				}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// MiRack → V2 (normalize)
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// Find the merged AudioInterface2 module
		normalizedModules := patch["modules"].([]any)
		var audioInterface2 map[string]any
		for _, m := range normalizedModules {
			mod := m.(map[string]any)
			if model, _ := mod["model"].(string); model == "AudioInterface2" {
				audioInterface2 = mod
				break
			}
		}

		if audioInterface2 == nil {
			t.Fatal("AudioInterface2 not found after normalization")
		}

		// Check that inputModuleID key does NOT exist (no input module was present)
		mergedData, ok := audioInterface2["_mergedAudioModule"].(map[string]any)
		if !ok {
			t.Fatal("Missing _mergedAudioModule metadata")
		}

		_, hasInputKey := mergedData["inputModuleID"]
		if hasInputKey {
			t.Error("Expected inputModuleID key to NOT exist in metadata (no input module was present)")
		}

		// V2 → MiRack (denormalize)
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		// After roundtrip, should have 2 modules (no AudioInterfaceIn created)
		finalModules := patch["modules"].([]any)
		if len(finalModules) != 2 {
			t.Errorf("After roundtrip: expected 2 modules, got %d", len(finalModules))
		}

		// Verify NO AudioInterfaceIn was created
		for _, m := range finalModules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model == "AudioInterfaceIn" {
				t.Error("AudioInterfaceIn should not have been created (no input module in original)")
			}
		}
	})
}

// TestMiRackModuleMapping tests MiRack → V2 module name mappings.
func TestMiRackModuleMapping(t *testing.T) {
	t.Run("normalize: maps MiRack module names to V2", func(t *testing.T) {
		// MiRack uses different module names than VCV Rack V2
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "MIDIBasicInterfaceOut", "params": []},
				{"id": 2, "plugin": "Core", "model": "MIDICCInterface", "params": []},
				{"id": 3, "plugin": "Core", "model": "MIDICCInterfaceOut", "params": []},
				{"id": 4, "plugin": "Core", "model": "MIDITriggerInterface", "params": []},
				{"id": 5, "plugin": "Core", "model": "MIDITriggerInterfaceOut", "params": []},
				{"id": 6, "plugin": "Core", "model": "PolyMerger", "params": []},
				{"id": 7, "plugin": "Core", "model": "PolySplitter", "params": []},
				{"id": 8, "plugin": "Core", "model": "PolySummer", "params": []},
				{"id": 9, "plugin": "Core", "model": "MIDIToCVInterface", "params": []}
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

		// Expected mappings for each module index
		expectedModels := []string{
			"CV-MIDI",                  // MIDIBasicInterfaceOut
			"MIDICCToCVInterface",      // MIDICCInterface
			"CV-CC",                    // MIDICCInterfaceOut
			"MIDITriggerToCVInterface", // MIDITriggerInterface
			"CV-Gate",                  // MIDITriggerInterfaceOut
			"Merge",                    // PolyMerger
			"Split",                    // PolySplitter
			"Sum",                      // PolySummer
			"MIDIToCVInterface",        // MIDIToCVInterface (no change)
		}

		for i, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model != expectedModels[i] {
				t.Errorf("Module[%d]: expected model '%s', got '%s'", i, expectedModels[i], model)
			}
		}

	})

	t.Run("denormalize: maps V2 module names back to MiRack", func(t *testing.T) {
		// V2 format with mapped module names
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "CV-MIDI", "params": []},
				{"id": 2, "plugin": "Core", "model": "MIDICCToCVInterface", "params": []},
				{"id": 3, "plugin": "Core", "model": "CV-CC", "params": []},
				{"id": 4, "plugin": "Core", "model": "MIDITriggerToCVInterface", "params": []},
				{"id": 5, "plugin": "Core", "model": "CV-Gate", "params": []},
				{"id": 6, "plugin": "Core", "model": "Merge", "params": []},
				{"id": 7, "plugin": "Core", "model": "Split", "params": []},
				{"id": 8, "plugin": "Core", "model": "Sum", "params": []},
				{"id": 9, "plugin": "Core", "model": "MIDIToCVInterface", "params": []}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(v2JSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize first to build ID-to-index mapping
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Denormalize to MiRack format
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// Expected reverse mappings for each module index
		expectedModels := []string{
			"MIDIBasicInterfaceOut",   // CV-MIDI
			"MIDICCInterface",         // MIDICCToCVInterface
			"MIDICCInterfaceOut",      // CV-CC
			"MIDITriggerInterface",    // MIDITriggerToCVInterface
			"MIDITriggerInterfaceOut", // CV-Gate
			"PolyMerger",              // Merge
			"PolySplitter",            // Split
			"PolySummer",              // Sum
			"MIDIToCVInterface",       // MIDIToCVInterface (no change)
		}

		for i, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model != expectedModels[i] {
				t.Errorf("Module[%d]: expected model '%s', got '%s'", i, expectedModels[i], model)
			}
		}
	})

	t.Run("roundtrip: MiRack → V2 → MiRack preserves module names", func(t *testing.T) {
		originalJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "MIDIBasicInterfaceOut", "params": []},
				{"id": 2, "plugin": "Core", "model": "PolyMerger", "params": []}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// MiRack → V2 (normalize)
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		// V2 → MiRack (denormalize)
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// After roundtrip, module names should be back to original MiRack names
		expectedModels := []string{"MIDIBasicInterfaceOut", "PolyMerger"}
		for i, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model != expectedModels[i] {
				t.Errorf("Module[%d]: roundtrip failed, expected '%s', got '%s'", i, expectedModels[i], model)
			}
		}
	})
}

// TestAudioModuleMergeSplit tests audio module merging and splitting.
func TestAudioModuleMergeSplit(t *testing.T) {
	t.Run("merges separate AudioInterface and AudioInterfaceIn into AudioInterface2", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []},
				{"id": 2, "plugin": "Core", "model": "AudioInterfaceIn", "params": []}
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

		// Should have 1 module (merged from 2)
		if len(modules) != 1 {
			t.Errorf("Expected 1 module after merge, got %d", len(modules))
		}

		// Check the merged module
		mod := modules[0].(map[string]any)
		model, _ := mod["model"].(string)
		if model != "AudioInterface2" {
			t.Errorf("Expected model 'AudioInterface2', got '%s'", model)
		}

		// Check that roundtrip metadata is stored
		if _, hasMerged := mod["_mergedAudioModule"]; !hasMerged {
			t.Error("Expected _mergedAudioModule metadata to be stored")
		}
	})

	t.Run("splits AudioInterface2 back into separate modules on roundtrip", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 10, "plugin": "Core", "model": "AudioInterface", "params": [], "pos": [5, 0]},
				{"id": 20, "plugin": "Core", "model": "AudioInterfaceIn", "params": [], "pos": [8, 0]}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(mirackJSON), &patch); err != nil {
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

		// Should have 2 modules after split
		if len(modules) != 2 {
			t.Errorf("Expected 2 modules after split, got %d", len(modules))
		}

		// Check the output module
		outputMod := modules[0].(map[string]any)
		model, _ := outputMod["model"].(string)
		if model != "AudioInterface" {
			t.Errorf("Expected output model 'AudioInterface', got '%s'", model)
		}
		if id := getInt64FromMap(outputMod, "id"); id != 10 {
			t.Errorf("Expected output module ID 10, got %d", id)
		}

		// Check the input module
		inputMod := modules[1].(map[string]any)
		model, _ = inputMod["model"].(string)
		if model != "AudioInterfaceIn" {
			t.Errorf("Expected input model 'AudioInterfaceIn', got '%s'", model)
		}
		if id := getInt64FromMap(inputMod, "id"); id != 20 {
			t.Errorf("Expected input module ID 20, got %d", id)
		}

		// Check that metadata was cleaned up
		if _, hasMerged := outputMod["_mergedAudioModule"]; hasMerged {
			t.Error("_mergedAudioModule metadata should be removed after roundtrip")
		}
	})

	t.Run("handles AudioInterface8 and AudioInterfaceIn8", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface8", "params": []},
				{"id": 2, "plugin": "Core", "model": "AudioInterfaceIn8", "params": []}
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
		model, _ := mod["model"].(string)

		if model != "AudioInterface8" {
			t.Errorf("Expected model 'AudioInterface8', got '%s'", model)
		}
	})

	t.Run("handles AudioInterface16 and AudioInterfaceIn16", func(t *testing.T) {
		mirackJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface16", "params": []},
				{"id": 2, "plugin": "Core", "model": "AudioInterfaceIn16", "params": []}
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
		model, _ := mod["model"].(string)

		if model != "AudioInterface16" {
			t.Errorf("Expected model 'AudioInterface16', got '%s'", model)
		}
	})
}

// TestValidateAudioModuleCount tests MiRack audio module validation.
func TestValidateAudioModuleCount(t *testing.T) {
	t.Run("valid: single audio interface", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface"},
			map[string]any{"model": "VCO"},
		}
		valid, _ := validateAudioModuleCount(modules)
		if !valid {
			t.Error("Expected valid for single audio module")
		}
	})

	t.Run("valid: paired input/output", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface"},
			map[string]any{"model": "AudioInterfaceIn"},
		}
		valid, _ := validateAudioModuleCount(modules)
		if !valid {
			t.Error("Expected valid for paired audio modules")
		}
	})

	t.Run("valid: no audio modules", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "VCO"},
			map[string]any{"model": "VCA"},
		}
		valid, _ := validateAudioModuleCount(modules)
		if !valid {
			t.Error("Expected valid for no audio modules")
		}
	})

	t.Run("valid: V2 audio modules are recognized", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface2"},
			map[string]any{"model": "AudioInterface8"},
		}
		valid, reason := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected invalid for multiple V2 output modules")
		}
		if reason == "" {
			t.Error("Expected skip reason for multiple output modules")
		}
	})

	t.Run("invalid: multiple output modules", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface"},
			map[string]any{"model": "AudioInterface8"},
		}
		valid, reason := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected invalid for multiple output modules")
		}
		if reason == "" {
			t.Error("Expected skip reason")
		}
		if !strings.Contains(reason, "2 audio output modules") {
			t.Errorf("Expected reason to mention 2 output modules, got: %s", reason)
		}
	})

	t.Run("invalid: multiple input modules", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterfaceIn"},
			map[string]any{"model": "AudioInterfaceIn8"},
		}
		valid, reason := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected invalid for multiple input modules")
		}
		if reason == "" {
			t.Error("Expected skip reason")
		}
		if !strings.Contains(reason, "2 audio input modules") {
			t.Errorf("Expected reason to mention 2 input modules, got: %s", reason)
		}
	})

	t.Run("invalid: three output modules", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface"},
			map[string]any{"model": "AudioInterface8"},
			map[string]any{"model": "AudioInterface16"},
		}
		valid, reason := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected invalid for 3 output modules")
		}
		if !strings.Contains(reason, "3 audio output modules") {
			t.Errorf("Expected reason to mention 3 output modules, got: %s", reason)
		}
	})

	t.Run("invalid: mixed types exceed limit", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface"},
			map[string]any{"model": "AudioInterface8"},
			map[string]any{"model": "AudioInterfaceIn"},
		}
		valid, _ := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected invalid for 2 outputs + 1 input")
		}
	})

	t.Run("handles non-map modules gracefully", func(t *testing.T) {
		modules := []any{
			map[string]any{"model": "AudioInterface"},
			"not a map",
			nil,
		}
		valid, _ := validateAudioModuleCount(modules)
		if !valid {
			t.Error("Expected valid when only one audio module exists (with non-map items)")
		}
	})
}

// TestMiRackMultipleAudioModules_Skipped tests that patches with multiple
// audio modules are skipped during conversion.
func TestMiRackMultipleAudioModules_Skipped(t *testing.T) {
	t.Run("MiRack to V2 with multiple outputs is skipped", func(t *testing.T) {
		// Create a MiRack patch with 2 AudioInterface modules (invalid for MiRack)
		patchJSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 0, "plugin": "Core", "model": "AudioInterface", "params": [], "pos": [0, 0]},
				{"id": 1, "plugin": "Core", "model": "AudioInterface8", "params": [], "pos": [3, 0]}
			],
			"wires": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Validate should fail
		modules := patch["modules"].([]any)
		valid, reason := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected validation to fail for multiple output modules")
		}
		if reason == "" {
			t.Error("Expected skip reason")
		}
	})

	t.Run("V2 to MiRack with multiple AudioInterface2 is skipped", func(t *testing.T) {
		// Create a V2 patch with 2 AudioInterface2 modules (invalid for MiRack)
		patchJSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 0, "plugin": "Core", "model": "AudioInterface2", "params": [], "pos": [0, 0]},
				{"id": 1, "plugin": "Core", "model": "AudioInterface2", "params": [], "pos": [3, 0]}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Validate should fail
		modules := patch["modules"].([]any)
		valid, reason := validateAudioModuleCount(modules)
		if valid {
			t.Error("Expected validation to fail for multiple output modules")
		}
		if !strings.Contains(reason, "2 audio output modules") {
			t.Errorf("Expected reason about 2 output modules, got: %s", reason)
		}
	})

	t.Run("valid single AudioInterface2 passes validation", func(t *testing.T) {
		patchJSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 0, "plugin": "Core", "model": "AudioInterface2", "params": [], "pos": [0, 0]}
			],
			"cables": []
		}`

		var patch map[string]any
		if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		modules := patch["modules"].([]any)
		valid, reason := validateAudioModuleCount(modules)
		if !valid {
			t.Errorf("Expected validation to pass, got reason: %s", reason)
		}
	})
}
