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
