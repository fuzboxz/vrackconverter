package converter

import (
	"encoding/json"
	"testing"
)

// TestMirackBasicFixture tests the actual mirack_basic.mrk test fixture.
// This fixture-based test verifies module names, Fundamental ↔ Core conversions, and roundtrip.
func TestMirackBasicFixture(t *testing.T) {
	t.Run("roundtrip with Fundamental plugin conversion", func(t *testing.T) {
		handler := &MiRackHandler{}

		// Read the test fixture
		data, err := handler.Read("../../test/mirack_basic.mrk")
		if err != nil {
			t.Fatalf("Failed to read mirack_basic.mrk: %v", err)
		}

		// Parse original MiRack patch
		var originalPatch map[string]any
		if err := json.Unmarshal(data, &originalPatch); err != nil {
			t.Fatalf("Failed to parse original JSON: %v", err)
		}

		// Store original modules for comparison
		originalModules := originalPatch["modules"].([]any)
		originalModuleData := make([]struct {
			plugin string
			model  string
		}, len(originalModules))
		for i, m := range originalModules {
			mod := m.(map[string]any)
			originalModuleData[i].plugin = mod["plugin"].(string)
			originalModuleData[i].model = mod["model"].(string)
		}

		// Create a copy for conversion
		dataCopy, _ := json.Marshal(originalPatch)
		var patch map[string]any
		if err := json.Unmarshal(dataCopy, &patch); err != nil {
			t.Fatalf("Failed to parse patch copy: %v", err)
		}

		// Normalize: MiRack → V2
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// Verify polyphony modules are in Fundamental plugin after normalization
		polyphonyV2Models := []string{"Split", "Merge", "Sum"}
		foundPolyphony := make(map[string]bool)
		for _, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			plugin, _ := mod["plugin"].(string)

			for _, pm := range polyphonyV2Models {
				if model == pm {
					foundPolyphony[model] = true
					if plugin != "Fundamental" {
						t.Errorf("V2 module %s should be in Fundamental plugin, got %s", model, plugin)
					}
				}
			}
		}

		for _, pm := range polyphonyV2Models {
			if !foundPolyphony[pm] {
				t.Errorf("Expected to find polyphony module %s in converted patch", pm)
			}
		}

		// Denormalize: V2 → MiRack
		issues = nil
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules = patch["modules"].([]any)

		// After roundtrip, polyphony modules should be back in Core with MiRack names
		foundMiRackPolyphony := make(map[string]bool)
		for _, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			plugin, _ := mod["plugin"].(string)

			// Check for MiRack polyphony module names
			mirackPolyphonyModules := []string{"PolySplitter", "PolyMerger", "PolySummer"}
			for _, mm := range mirackPolyphonyModules {
				if model == mm {
					foundMiRackPolyphony[model] = true
					if plugin != "Core" {
						t.Errorf("MiRack module %s should be in Core plugin, got %s", model, plugin)
					}
				}
			}
		}

		mirackPolyphonyModules := []string{"PolySplitter", "PolyMerger", "PolySummer"}
		for _, mm := range mirackPolyphonyModules {
			if !foundMiRackPolyphony[mm] {
				t.Errorf("Expected to find MiRack polyphony module %s after roundtrip", mm)
			}
		}

		// Verify cables/wires are preserved
		if _, hasWires := patch["wires"]; !hasWires {
			t.Error("After roundtrip, should have 'wires' field")
		}
		if _, hasCables := patch["cables"]; hasCables {
			t.Error("After roundtrip, should not have 'cables' field")
		}

		// No issues should be logged for expected conversions
		if len(issues) > 0 {
			t.Errorf("Expected no issues for roundtrip conversion, got: %v", issues)
		}
	})

	t.Run("MIDI modules stay in Core plugin", func(t *testing.T) {
		handler := &MiRackHandler{}

		data, err := handler.Read("../../test/mirack_basic.mrk")
		if err != nil {
			t.Fatalf("Failed to read mirack_basic.mrk: %v", err)
		}

		var patch map[string]any
		if err := json.Unmarshal(data, &patch); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Normalize: MiRack → V2
		var issues []string
		if err := NormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("NormalizeMiRack failed: %v", err)
		}

		modules := patch["modules"].([]any)

		// MIDI modules should be in Core plugin after normalization
		midiV2Models := []string{"CV-MIDI", "MIDICCToCVInterface", "MIDITriggerToCVInterface", "CV-CC", "CV-Gate"}
		for _, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			plugin, _ := mod["plugin"].(string)

			for _, mm := range midiV2Models {
				if model == mm && plugin != "Core" {
					t.Errorf("V2 MIDI module %s should be in Core plugin, got %s", model, plugin)
				}
			}
		}
	})
}
