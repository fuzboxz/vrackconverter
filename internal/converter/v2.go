package converter

import (
	"fmt"
)

// NormalizeV2 converts a VCV Rack v2 patch to the internal format.
// This is a no-op for v2 patches since v2 is the "superset" format -
// we preserve all data including v2-specific fields.
// The main purpose is to build the ID-to-index mapping for potential
// conversion to other formats and validate the patch structure.
func NormalizeV2(patch map[string]any, issues *[]string) error {
	modules, ok := patch["modules"].([]any)
	if !ok {
		*issues = append(*issues, "v2 normalization: no modules array found")
		return nil
	}

	// Build ID-to-index mapping for potential use in other conversions
	// In v2, cables use module IDs, so we need to map IDs to array indices
	idToIndex := make(map[int64]int)
	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("v2 normalization: module[%d]: invalid module object", i))
			continue
		}

		if id := getInt64FromMap(mod, "id"); id >= 0 {
			if existingIdx, exists := idToIndex[id]; exists {
				*issues = append(*issues, fmt.Sprintf("v2 normalization: duplicate module ID %d at indices %d and %d", id, existingIdx, i))
			}
			idToIndex[id] = i
		}
	}

	// Store the mapping in the patch for later use (e.g., for v0.6 conversion)
	// We use a special key that won't be serialized
	patch["_idToIndex"] = idToIndex

	// Validate that cables reference valid module IDs
	cables, hasCables := patch["cables"].([]any)
	if hasCables {
		for i, c := range cables {
			cable, ok := c.(map[string]any)
			if !ok {
				*issues = append(*issues, fmt.Sprintf("v2 normalization: cable[%d]: invalid cable object", i))
				continue
			}

			outputModID := getInt64FromMap(cable, "outputModuleId")
			inputModID := getInt64FromMap(cable, "inputModuleId")

			if _, exists := idToIndex[outputModID]; !exists {
				*issues = append(*issues, fmt.Sprintf("v2 normalization: cable[%d]: outputModuleId %d not found", i, outputModID))
			}
			if _, exists := idToIndex[inputModID]; !exists {
				*issues = append(*issues, fmt.Sprintf("v2 normalization: cable[%d]: inputModuleId %d not found", i, inputModID))
			}
		}
	}

	// Ensure cables array exists (v2 uses "cables", not "wires")
	if _, hasCables := patch["cables"]; !hasCables {
		if wires, hasWires := patch["wires"]; hasWires {
			// This is unusual for a v2 patch but handle it
			patch["cables"] = wires
			delete(patch, "wires")
			*issues = append(*issues, "v2 normalization: converted 'wires' to 'cables'")
		} else {
			// No cables at all, create empty array
			patch["cables"] = []any{}
		}
	}

	// Ensure version is set to v2 format
	if version, ok := patch["version"].(string); !ok || version == "" {
		patch["version"] = "2.6.6"
		*issues = append(*issues, "v2 normalization: set default version to 2.6.6")
	}

	return nil
}

// DenormalizeV2 converts the internal format to VCV Rack v2 format.
// This adds v2-specific fields and ensures the patch is valid v2 format.
// For v0.6/MiRack source patches, this converts disabled→bypass.
func DenormalizeV2(patch map[string]any, issues *[]string) error {
	modules, ok := patch["modules"].([]any)
	if !ok {
		*issues = append(*issues, "v2 denormalization: no modules array found")
		return nil
	}

	// Ensure all modules have required v2 fields
	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		// Convert "disabled" to "bypass" for v0.6 source patches
		if disabled, ok := mod["disabled"]; ok {
			if disabledBool, ok := disabled.(bool); ok {
				mod["bypass"] = disabledBool
				*issues = append(*issues, fmt.Sprintf("v2 denormalization: module[%d]: converted disabled=%v to bypass", i, disabledBool))
			}
			delete(mod, "disabled")
		}

		// Ensure module has an ID (required for v2)
		if _, hasID := mod["id"]; !hasID {
			mod["id"] = int64(i)
			*issues = append(*issues, fmt.Sprintf("v2 denormalization: module[%d]: assigned ID %d", i, i))
		}
	}

	// Ensure cables array exists (v2 uses "cables")
	if _, hasCables := patch["cables"]; !hasCables {
		if wires, hasWires := patch["wires"]; hasWires {
			// Convert wires to cables
			patch["cables"] = wires
			delete(patch, "wires")
			*issues = append(*issues, "v2 denormalization: converted 'wires' to 'cables'")
		} else {
			// No cables, create empty array
			patch["cables"] = []any{}
		}
	}

	// Ensure all cables have IDs (v2 format)
	if cables, ok := patch["cables"].([]any); ok {
		for i, c := range cables {
			cable, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if _, hasID := cable["id"]; !hasID {
				cable["id"] = i
			}
		}
	}

	// Set version to v2 format
	patch["version"] = "2.6.6"

	// Clean up internal fields that shouldn't be in v2 output
	delete(patch, "_idToIndex")

	return nil
}

// GetIDToIndexMapping retrieves the ID-to-index mapping from a normalized v2 patch.
// Returns nil if the patch hasn't been normalized.
func GetIDToIndexMapping(patch map[string]any) map[int64]int {
	if mapping, ok := patch["_idToIndex"].(map[int64]int); ok {
		return mapping
	}
	return nil
}
