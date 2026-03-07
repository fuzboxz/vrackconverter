package converter

import (
	"encoding/json"
	"testing"
)

// TestMirackCablesFixture tests the actual mirack_cables.mrk test fixture.
// This fixture-based test verifies correct module mappings and roundtrip conversion.
func TestMirackCablesFixture(t *testing.T) {
	t.Run("roundtrip conversion with module mapping", func(t *testing.T) {
		handler := &MiRackHandler{}

		// Read the test fixture
		data, err := handler.Read("../../test/mirack_cables.mrk")
		if err != nil {
			t.Fatalf("Failed to read mirack_cables.mrk: %v", err)
		}

		// Parse original MiRack patch
		var originalPatch map[string]any
		if err := json.Unmarshal(data, &originalPatch); err != nil {
			t.Fatalf("Failed to parse original JSON: %v", err)
		}

		// Store original module names for comparison
		originalModules := originalPatch["modules"].([]any)
		originalModelNames := make([]string, len(originalModules))
		for i, m := range originalModules {
			mod := m.(map[string]any)
			originalModelNames[i] = mod["model"].(string)
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

		// Expected V2 module names after normalization
		expectedV2Models := []string{
			"CV-MIDI",           // MIDIBasicInterfaceOut
			"MIDIToCVInterface", // MIDIToCVInterface (no change)
		}

		for i, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model != expectedV2Models[i] {
				t.Errorf("After normalize, module[%d]: expected '%s', got '%s'", i, expectedV2Models[i], model)
			}
		}

		// Denormalize: V2 → MiRack
		if err := DenormalizeMiRack(patch, &issues); err != nil {
			t.Fatalf("DenormalizeMiRack failed: %v", err)
		}

		modules = patch["modules"].([]any)

		// After roundtrip, should be back to original MiRack names
		for i, m := range modules {
			mod := m.(map[string]any)
			model, _ := mod["model"].(string)
			if model != originalModelNames[i] {
				t.Errorf("After roundtrip, module[%d]: expected '%s', got '%s'", i, originalModelNames[i], model)
			}
		}

		// Verify cables/wires are preserved
		if _, hasWires := patch["wires"]; !hasWires {
			t.Error("After roundtrip, should have 'wires' field")
		}
		if _, hasCables := patch["cables"]; hasCables {
			t.Error("After roundtrip, should not have 'cables' field")
		}
	})
}
