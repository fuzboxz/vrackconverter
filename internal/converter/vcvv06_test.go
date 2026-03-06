package converter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestNormalizeV06 tests VCV Rack v0.6 to internal format conversion.
func TestNormalizeV06(t *testing.T) {
	t.Run("converts Fundamental to Core", func(t *testing.T) {
		// VCV Rack v0.6 has separate Fundamental and Core plugins
		v06JSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Fundamental", "model": "VCA-1", "params": []},
				{"id": 2, "plugin": "Fundamental", "model": "VCO-1", "params": []},
				{"id": 3, "plugin": "Core", "model": "AudioInterface", "params": []}
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

		// All Fundamental modules should be mapped to Core
		for i, m := range modules {
			mod := m.(map[string]any)
			plugin, _ := mod["plugin"].(string)
			if plugin != "Core" {
				t.Errorf("Module[%d]: expected plugin 'Core', got '%s'", i, plugin)
			}
		}

		// Check that issues were logged for the conversion
		hasFundamentalIssue := false
		for _, issue := range issues {
			if strings.Contains(issue, "v0.6 normalization") && strings.Contains(issue, "Fundamental") && strings.Contains(issue, "Core") {
				hasFundamentalIssue = true
				break
			}
		}
		if !hasFundamentalIssue {
			t.Errorf("Expected issues to contain v0.6 Fundamental → Core conversion messages, got: %v", issues)
		}
	})

	t.Run("converts wires to cables", func(t *testing.T) {
		v06JSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": [
				{"outputModuleId": 0, "outputId": 0, "inputModuleId": 0, "inputId": 0}
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

		// Wires should be converted to cables
		if _, hasWires := patch["wires"]; hasWires {
			t.Error("wires should be removed after normalization")
		}
		if _, hasCables := patch["cables"]; !hasCables {
			t.Error("cables array should exist after normalization")
		}
	})

	t.Run("converts array indices to module IDs", func(t *testing.T) {
		v06JSON := `{
			"version": "0.6.2",
			"modules": [
				{"id": 10, "plugin": "Fundamental", "model": "VCO-1", "params": []},
				{"id": 20, "plugin": "Core", "model": "AudioInterface", "params": []}
			],
			"wires": [
				{"outputModuleId": 0, "outputId": 0, "inputModuleId": 1, "inputId": 0}
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

		cables := patch["cables"].([]any)
		if len(cables) != 1 {
			t.Fatalf("Expected 1 cable, got %d", len(cables))
		}

		cable := cables[0].(map[string]any)

		// Array index 0 should be converted to module ID 10
		if getInt64FromMap(cable, "outputModuleId") != 10 {
			t.Errorf("outputModuleId should be 10 (module ID), got %v", cable["outputModuleId"])
		}

		// Array index 1 should be converted to module ID 20
		if getInt64FromMap(cable, "inputModuleId") != 20 {
			t.Errorf("inputModuleId should be 20 (module ID), got %v", cable["inputModuleId"])
		}
	})
}

// TestDenormalizeV06 tests internal format to VCV Rack v0.6 conversion.
func TestDenormalizeV06(t *testing.T) {
	t.Run("converts Core back to Fundamental for v0.6", func(t *testing.T) {
		// V2 format with Core plugin (after normalization from v0.6)
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 1, "plugin": "Core", "model": "VCA-1", "params": []},
				{"id": 2, "plugin": "Core", "model": "VCO-1", "params": []},
				{"id": 3, "plugin": "Core", "model": "VCF", "params": []},
				{"id": 4, "plugin": "Core", "model": "LFO", "params": []},
				{"id": 5, "plugin": "Core", "model": "AudioInterface", "params": []}
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

		// Denormalize to v0.6 format
		issues = nil
		if err := DenormalizeV06(patch, &issues); err != nil {
			t.Fatalf("DenormalizeV06 failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// Fundamental modules should be restored to Fundamental plugin
		expectedMapping := map[string]string{
			"VCA-1":          "Fundamental",
			"VCO-1":          "Fundamental",
			"VCF":            "Fundamental",
			"LFO":            "Fundamental",
			"AudioInterface": "Core", // Not a Fundamental module
		}

		for i, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			plugin, _ := mod["plugin"].(string)

			expectedPlugin, ok := expectedMapping[model]
			if !ok {
				t.Errorf("Module[%d]: unexpected model '%s'", i, model)
				continue
			}

			if plugin != expectedPlugin {
				t.Errorf("Module[%d] (%s): expected plugin '%s', got '%s'", i, model, expectedPlugin, plugin)
			}
		}
	})

	t.Run("converts module IDs to array indices", func(t *testing.T) {
		v2JSON := `{
			"version": "2.6.6",
			"modules": [
				{"id": 10, "plugin": "Core", "model": "AudioInterface", "params": []},
				{"id": 20, "plugin": "Core", "model": "VCO-1", "params": []}
			],
			"cables": [
				{"outputModuleId": 20, "outputId": 0, "inputModuleId": 10, "inputId": 0}
			]
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

		// Denormalize to v0.6 format
		issues = nil
		if err := DenormalizeV06(patch, &issues); err != nil {
			t.Fatalf("DenormalizeV06 failed: %v", err)
		}

		wires := patch["wires"].([]any)
		if len(wires) != 1 {
			t.Fatalf("Expected 1 wire, got %d", len(wires))
		}

		wire := wires[0].(map[string]any)

		// Module ID 20 should be converted to array index 1
		if getInt64FromMap(wire, "outputModuleId") != 1 {
			t.Errorf("outputModuleId should be 1 (array index), got %v", wire["outputModuleId"])
		}

		// Module ID 10 should be converted to array index 0
		if getInt64FromMap(wire, "inputModuleId") != 0 {
			t.Errorf("inputModuleId should be 0 (array index), got %v", wire["inputModuleId"])
		}
	})

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

		// Normalize first
		var issues []string
		if err := NormalizeV2(patch, &issues); err != nil {
			t.Fatalf("NormalizeV2 failed: %v", err)
		}

		// Denormalize to v0.6 format
		issues = nil
		if err := DenormalizeV06(patch, &issues); err != nil {
			t.Fatalf("DenormalizeV06 failed: %v", err)
		}

		// Cables should be converted to wires
		if _, hasCables := patch["cables"]; hasCables {
			t.Error("cables should be removed after denormalization")
		}
		if _, hasWires := patch["wires"]; !hasWires {
			t.Error("wires array should exist after denormalization")
		}
	})
}

// TestV06Roundtrip tests full v0.6 → internal → v0.6 roundtrip.
func TestV06Roundtrip(t *testing.T) {
	originalJSON := `{
		"version": "0.6.2",
		"modules": [
			{
				"id": 1,
				"plugin": "Fundamental",
				"model": "VCA-1",
				"params": [
					{"paramId": 0, "value": 0.5}
				],
				"pos": [0, 0]
			},
			{
				"id": 2,
				"plugin": "Core",
				"model": "AudioInterface",
				"params": [
					{"paramId": 0, "value": 0.0}
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

	// v0.6 → V2 (normalize)
	var issues []string
	if err := NormalizeV06(patch, &issues); err != nil {
		t.Fatalf("NormalizeV06 failed: %v", err)
	}

	// V2 → v0.6 (denormalize)
	if err := DenormalizeV06(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV06 failed: %v", err)
	}

	// Verify Fundamental plugin was restored
	modules := patch["modules"].([]any)
	vcaModule := modules[0].(map[string]any)
	if plugin, _ := vcaModule["plugin"].(string); plugin != "Fundamental" {
		t.Errorf("VCA-1 should have plugin 'Fundamental', got '%s'", plugin)
	}

	audioModule := modules[1].(map[string]any)
	if plugin, _ := audioModule["plugin"].(string); plugin != "Core" {
		t.Errorf("AudioInterface should have plugin 'Core', got '%s'", plugin)
	}

	// Check wires exist
	if _, hasWires := patch["wires"]; !hasWires {
		t.Error("Should have 'wires' field")
	}

	// Check cables don't exist
	if _, hasCables := patch["cables"]; hasCables {
		t.Error("Should not have 'cables' field")
	}
}

// TestV06PluginMapping tests the v0.6-specific Fundamental ↔ Core plugin mapping.
func TestV06PluginMapping(t *testing.T) {
	t.Run("all Fundamental modules are in the map", func(t *testing.T) {
		// Verify all Fundamental modules are tracked
		expectedModules := []string{
			"VCO-1", "VCO-2",
			"VCF",
			"VCA-1", "VCA-2",
			"LFO", "LFO-2",
			"ADSR", "Decay",
			"VCMixer", "Unity",
			"8vert",
			"Merge", "Split", "Sum",
			"Momentary", "Button", "Latch",
			"Gate",
			"Clock",
			"Noise",
			"SampleHold",
			"Scope",
			"Notes",
			"Text",
		}

		for _, model := range expectedModules {
			if !fundamentalModules[model] {
				t.Errorf("Model '%s' should be in fundamentalModules map", model)
			}
		}
	})

	t.Run("normalize: Fundamental → Core", func(t *testing.T) {
		// Test plugin mapping through NormalizeV06
		for model := range fundamentalModules {
			patchJSON := fmt.Sprintf(`{
				"version": "0.6.2",
				"modules": [
					{"id": 1, "plugin": "Fundamental", "model": "%s", "params": []}
				],
				"wires": []
			}`, model)

			var patch map[string]any
			if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var issues []string
			if err := NormalizeV06(patch, &issues); err != nil {
				t.Fatalf("NormalizeV06 failed: %v", err)
			}

			modules := patch["modules"].([]any)
			mod := modules[0].(map[string]any)
			plugin, _ := mod["plugin"].(string)

			if plugin != "Core" {
				t.Errorf("Expected 'Core' for Fundamental/%s, got '%s'", model, plugin)
			}
		}
	})

	t.Run("denormalize: Core → Fundamental for known modules", func(t *testing.T) {
		// Test plugin mapping through DenormalizeV06
		for model := range fundamentalModules {
			patchJSON := fmt.Sprintf(`{
				"version": "2.6.6",
				"modules": [
					{"id": 1, "plugin": "Core", "model": "%s", "params": []}
				],
				"cables": []
			}`, model)

			var patch map[string]any
			if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var issues []string
			if err := NormalizeV2(patch, &issues); err != nil {
				t.Fatalf("NormalizeV2 failed: %v", err)
			}

			issues = nil
			if err := DenormalizeV06(patch, &issues); err != nil {
				t.Fatalf("DenormalizeV06 failed: %v", err)
			}

			modules := patch["modules"].([]any)
			mod := modules[0].(map[string]any)
			plugin, _ := mod["plugin"].(string)

			if plugin != "Fundamental" {
				t.Errorf("Expected 'Fundamental' for Core/%s, got '%s'", model, plugin)
			}
		}
	})

	t.Run("denormalize: Core stays Core for non-Fundamental modules", func(t *testing.T) {
		nonFundamentalModels := []string{"AudioInterface", "Blank"}

		for _, model := range nonFundamentalModels {
			patchJSON := fmt.Sprintf(`{
				"version": "2.6.6",
				"modules": [
					{"id": 1, "plugin": "Core", "model": "%s", "params": []}
				],
				"cables": []
			}`, model)

			var patch map[string]any
			if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var issues []string
			if err := NormalizeV2(patch, &issues); err != nil {
				t.Fatalf("NormalizeV2 failed: %v", err)
			}

			issues = nil
			if err := DenormalizeV06(patch, &issues); err != nil {
				t.Fatalf("DenormalizeV06 failed: %v", err)
			}

			modules := patch["modules"].([]any)
			mod := modules[0].(map[string]any)
			plugin, _ := mod["plugin"].(string)

			if plugin != "Core" {
				t.Errorf("Expected 'Core' for non-Fundamental module %s, got '%s'", model, plugin)
			}
		}
	})
}

// TestV06ToV2Conversion_CablesFile tests full v0.6 → v2 conversion of the cables test file.
// This is an integration test that verifies the complete conversion pipeline.
func TestV06ToV2Conversion_CablesFile(t *testing.T) {
	// The test file contains:
	// - 2 modules (MIDIToCVInterface, AudioInterface) with null IDs
	// - 4 wires between the modules
	// - Cable colors: #0986ad, #c9b70e, #c91847, #0c8e15
	v06JSON := `{
		"version": "0.6.2c",
		"modules": [
			{
				"plugin": "Core",
				"version": "0.6.2c",
				"model": "MIDIToCVInterface",
				"params": [],
				"data": {
					"divisions": [24, 6],
					"midi": {"driver": 1, "channel": -1}
				},
				"pos": [30, 0]
			},
			{
				"plugin": "Core",
				"version": "0.6.2c",
				"model": "AudioInterface",
				"params": [],
				"data": {
					"audio": {
						"driver": 5,
						"deviceName": "",
						"offset": 0,
						"maxChannels": 8,
						"sampleRate": 44100,
						"blockSize": 256
					}
				},
				"pos": [38, 0]
			}
		],
		"wires": [
			{
				"color": "#0986ad",
				"outputModuleId": 0,
				"outputId": 0,
				"inputModuleId": 1,
				"inputId": 0
			},
			{
				"color": "#c9b70e",
				"outputModuleId": 0,
				"outputId": 1,
				"inputModuleId": 1,
				"inputId": 1
			},
			{
				"color": "#c91847",
				"outputModuleId": 0,
				"outputId": 2,
				"inputModuleId": 1,
				"inputId": 2
			},
			{
				"color": "#0c8e15",
				"outputModuleId": 0,
				"outputId": 3,
				"inputModuleId": 1,
				"inputId": 3
			}
		]
	}`

	var patch map[string]any
	if err := json.Unmarshal([]byte(v06JSON), &patch); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Step 1: Normalize from v0.6 to internal format
	var issues []string
	if err := NormalizeV06(patch, &issues); err != nil {
		t.Fatalf("NormalizeV06 failed: %v", err)
	}

	// Step 2: Denormalize to v2 format
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	// Verify V2 compatibility: version should start with "2."
	version, ok := patch["version"].(string)
	if !ok {
		t.Fatal("Version should be a string")
	}
	if version != "2.6.6" {
		t.Errorf("Expected version 2.6.6, got %s", version)
	}

	// Verify modules exist
	modules, ok := patch["modules"].([]any)
	if !ok {
		t.Fatal("modules should be an array")
	}
	if len(modules) != 2 {
		t.Fatalf("Expected 2 modules, got %d", len(modules))
	}

	// Verify module IDs are assigned (original had null IDs)
	mod0 := modules[0].(map[string]any)
	mod1 := modules[1].(map[string]any)

	if getInt64FromMap(mod0, "id") != 0 {
		t.Errorf("Module 0: expected id 0, got %v", mod0["id"])
	}
	if getInt64FromMap(mod1, "id") != 1 {
		t.Errorf("Module 1: expected id 1, got %v", mod1["id"])
	}

	// Verify module models
	if mod0["model"] != "MIDIToCVInterface" {
		t.Errorf("Module 0: expected model MIDIToCVInterface, got %v", mod0["model"])
	}
	if mod1["model"] != "AudioInterface" {
		t.Errorf("Module 1: expected model AudioInterface, got %v", mod1["model"])
	}

	// Verify cables exist (wires should be converted to cables)
	cables, ok := patch["cables"].([]any)
	if !ok {
		t.Fatal("cables should be an array")
	}
	if len(cables) != 4 {
		t.Fatalf("Expected 4 cables, got %d", len(cables))
	}

	// Verify wires field is removed
	if _, hasWires := patch["wires"]; hasWires {
		t.Error("wires field should be removed after conversion to v2")
	}

	// Verify cable properties (IDs, colors, connections)
	expectedCables := []struct {
		id             int64
		color          string
		outputModuleId int64
		outputId       int64
		inputModuleId  int64
		inputId        int64
	}{
		{0, "#0986ad", 0, 0, 1, 0},
		{1, "#c9b70e", 0, 1, 1, 1},
		{2, "#c91847", 0, 2, 1, 2},
		{3, "#0c8e15", 0, 3, 1, 3},
	}

	for i, expected := range expectedCables {
		if i >= len(cables) {
			t.Fatalf("Missing cable at index %d", i)
		}
		cable := cables[i].(map[string]any)

		if getInt64FromMap(cable, "id") != expected.id {
			t.Errorf("Cable %d: expected id %d, got %v", i, expected.id, cable["id"])
		}
		if cable["color"] != expected.color {
			t.Errorf("Cable %d: expected color %s, got %v", i, expected.color, cable["color"])
		}
		if getInt64FromMap(cable, "outputModuleId") != expected.outputModuleId {
			t.Errorf("Cable %d: expected outputModuleId %d, got %v", i, expected.outputModuleId, cable["outputModuleId"])
		}
		if getInt64FromMap(cable, "outputId") != expected.outputId {
			t.Errorf("Cable %d: expected outputId %d, got %v", i, expected.outputId, cable["outputId"])
		}
		if getInt64FromMap(cable, "inputModuleId") != expected.inputModuleId {
			t.Errorf("Cable %d: expected inputModuleId %d, got %v", i, expected.inputModuleId, cable["inputModuleId"])
		}
		if getInt64FromMap(cable, "inputId") != expected.inputId {
			t.Errorf("Cable %d: expected inputId %d, got %v", i, expected.inputId, cable["inputId"])
		}
	}
}

// TestV06ToV2Conversion_ModuleDataPreserved tests that module data is preserved during conversion.
func TestV06ToV2Conversion_ModuleDataPreserved(t *testing.T) {
	v06JSON := `{
		"version": "0.6.2c",
		"modules": [
			{
				"plugin": "Core",
				"model": "MIDIToCVInterface",
				"params": [],
				"data": {
					"divisions": [24, 6],
					"midi": {"driver": 1, "channel": -1}
				},
				"pos": [30, 0]
			}
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
	if err := DenormalizeV2(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV2 failed: %v", err)
	}

	modules := patch["modules"].([]any)
	mod := modules[0].(map[string]any)

	// Verify module data is preserved
	data, ok := mod["data"].(map[string]any)
	if !ok {
		t.Fatal("Module data should be preserved")
	}

	// Check divisions
	divisions, ok := data["divisions"].([]any)
	if !ok {
		t.Fatal("divisions should be preserved")
	}
	if len(divisions) != 2 {
		t.Errorf("Expected 2 divisions, got %d", len(divisions))
	}

	// Check midi data
	midi, ok := data["midi"].(map[string]any)
	if !ok {
		t.Fatal("midi data should be preserved")
	}
	if getInt64FromMap(midi, "driver") != 1 {
		t.Errorf("Expected midi.driver 1, got %v", midi["driver"])
	}

	// Check position
	pos, ok := mod["pos"].([]any)
	if !ok {
		t.Fatal("pos should be preserved")
	}
	if len(pos) != 2 {
		t.Errorf("Expected pos to have 2 values, got %d", len(pos))
	}
}
