package converter

import (
	"fmt"
)

// fundamentalModules contains modules from VCV Rack v0.6 Fundamental plugin.
// This map is ONLY for v0.6 files - MiRack does NOT have "Fundamental" plugin.
var fundamentalModules = map[string]bool{
	// Oscillators
	"VCO-1": true, "VCO-2": true,
	// Filter
	"VCF": true,
	// Amplifiers
	"VCA-1": true, "VCA-2": true,
	// LFOs
	"LFO": true, "LFO-2": true,
	// Envelopes
	"ADSR": true, "Decay": true,
	// Mixers
	"VCMixer": true, "Unity": true,
	// Utilities
	"8vert":     true,
	"Merge":     true,
	"Split":     true,
	"Sum":       true,
	"Momentary": true, "Button": true, "Latch": true,
	"Gate":       true,
	"Clock":      true,
	"Noise":      true,
	"SampleHold": true,
	"Scope":      true,
	"Notes":      true,
	"Text":       true,
}

// V06StyleConfig contains format-specific overrides for V0.6-style formats.
// This allows V0.6 and MiRack to share 90% of their conversion logic while
// handling their differences (plugin mapping, color conversion) via callbacks.
type V06StyleConfig struct {
	// FormatName is the name of the format (for logging)
	FormatName string

	// HasFundamental indicates whether the format has a separate Fundamental plugin.
	// true = v0.6 (Fundamental → Core conversion needed)
	// false = MiRack (all modules already use Core)
	HasFundamental bool

	// ConvertColor is an optional callback for format-specific color conversion.
	// For MiRack: converts colorIndex to hex (normalize) or hex to colorIndex (denormalize).
	// For v0.6: nil (no color conversion needed, already hex).
	// The callback receives the cable/wire map and issues slice for logging.
	ConvertColor func(cable map[string]any, issues *[]string)

	// NormalizePlugin converts a plugin/model pair during normalization.
	// Returns (newPlugin, wasModified).
	NormalizePlugin func(plugin, model string) (string, bool)

	// DenormalizePlugin converts a plugin/model pair during denormalization.
	// Returns (newPlugin, wasModified).
	DenormalizePlugin func(plugin, model string) (string, bool)
}

// NormalizeV06Style converts a V0.6-style format to v2.
// This is the SHARED baseline for both V0.6 and MiRack formats.
//
// The function handles all common transformations:
// - Build index-to-ID mapping for cable conversion
// - Assign module IDs (preserving existing, assigning sequential for missing)
// - Convert paramId→id, disabled→bypass
// - Convert wires→cables with array indices→module IDs
// - Store mappings for roundtrip conversion
//
// Format-specific behavior is provided via config:
// - HasFundamental: controls Fundamental→Core plugin conversion
// - ConvertColor: optional callback for MiRack colorIndex conversion
func NormalizeV06Style(patch map[string]any, config V06StyleConfig, issues *[]string) error {
	modules := getModules(patch)
	if modules == nil {
		*issues = append(*issues, fmt.Sprintf("%s normalization: no modules array found", config.FormatName))
		return nil
	}

	// Build index-to-ID mapping for cable reference conversion.
	// In v0.6/MiRack, wires use array indices, not module IDs.
	indexToID := make(map[int]int64)
	nextID := int64(0)

	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("%s normalization: module[%d]: invalid module object", config.FormatName, i))
			continue
		}

		// Apply plugin mapping using the format-specific callback
		if plugin, ok := mod["plugin"].(string); ok {
			if model, hasModel := mod["model"].(string); hasModel {
				if config.NormalizePlugin != nil {
					if newPlugin, modified := config.NormalizePlugin(plugin, model); modified {
						oldPlugin := plugin
						mod["plugin"] = newPlugin
						*issues = append(*issues, fmt.Sprintf("%s normalization: module[%d]: %s/%s → %s/%s", config.FormatName, i, oldPlugin, model, newPlugin, model))
					}
				}
			}
		}

		// Assign valid module ID for VCV Rack 2.
		// V2 requires positive IDs - reassign negative IDs to sequential positives.
		var moduleID int64
		if _, hasID := mod["id"]; hasID {
			moduleID = getInt64FromMap(mod, "id")
			if moduleID < 0 {
				// Reassign negative ID to positive sequential ID
				oldID := moduleID
				moduleID = nextID
				nextID++
				mod["id"] = moduleID
				*issues = append(*issues, fmt.Sprintf("%s normalization: module[%d]: reassigned negative ID %d to %d", config.FormatName, i, oldID, moduleID))
			} else {
				// Keep positive ID and track next available
				if moduleID >= nextID {
					nextID = moduleID + 1
				}
			}
		} else {
			// Module has no ID field, assign sequential ID
			moduleID = nextID
			nextID++
			mod["id"] = moduleID
		}
		indexToID[i] = moduleID

		// Convert paramId to id in parameters
		convertParamIDToID(mod, i, issues)

		// Convert disabled to bypass (v2 format)
		convertDisabledToBypass(mod, issues)

		// Remove format-specific fields not used in v2
		delete(mod, "sumPolyInputs")
	}

	// Store the index-to-ID mapping for later use during denormalization.
	// This ensures that when we convert back to v0.6/MiRack format, we can
	// correctly reverse the module ID → array index conversion.
	patch["_originalIndexToID"] = indexToID

	// Store expander links (leftModuleId/rightModuleId) for V2 roundtrip.
	// v0.6/MiRack don't support expander links, so we save them to restore later.
	expanderLinks := make(map[int64]map[string]int64)
	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if id := getInt64FromMap(mod, "id"); id >= 0 {
			links := make(map[string]int64)
			if leftID, ok := mod["leftModuleId"]; ok && leftID != nil {
				switch v := leftID.(type) {
				case float64:
					links["leftModuleId"] = int64(v)
				case int64:
					links["leftModuleId"] = v
				}
			}
			if rightID, ok := mod["rightModuleId"]; ok && rightID != nil {
				switch v := rightID.(type) {
				case float64:
					links["rightModuleId"] = int64(v)
				case int64:
					links["rightModuleId"] = v
				}
			}
			if len(links) > 0 {
				expanderLinks[id] = links
			}
		}
	}
	if len(expanderLinks) > 0 {
		patch["_expanderLinks"] = expanderLinks
	}

	// Convert wires to cables
	if wires, hasWires := patch["wires"]; hasWires {
		patch["cables"] = wires
		delete(patch, "wires")

		// Convert cable references from array indices to module IDs
		if cables, ok := patch["cables"].([]any); ok {
			validCables := make([]any, 0, len(cables))
			for i, c := range cables {
				cable, ok := c.(map[string]any)
				if !ok {
					*issues = append(*issues, fmt.Sprintf("%s normalization: cable[%d]: invalid cable object", config.FormatName, i))
					continue
				}

				// Get cable references (these are array indices in v0.6/MiRack)
				outputModuleIdx := getInt64FromMap(cable, "outputModuleId")
				inputModuleIdx := getInt64FromMap(cable, "inputModuleId")

				// Convert array indices to module IDs
				outputModuleID, outputExists := indexToID[int(outputModuleIdx)]
				inputModuleID, inputExists := indexToID[int(inputModuleIdx)]

				if !outputExists {
					*issues = append(*issues, fmt.Sprintf("%s normalization: cable[%d]: outputModuleId index %d out of range", config.FormatName, i, outputModuleIdx))
					continue
				}
				if !inputExists {
					*issues = append(*issues, fmt.Sprintf("%s normalization: cable[%d]: inputModuleId index %d out of range", config.FormatName, i, inputModuleIdx))
					continue
				}

				// Update cable with resolved module IDs
				cable["outputModuleId"] = outputModuleID
				cable["inputModuleId"] = inputModuleID

				// Apply format-specific color conversion if provided
				if config.ConvertColor != nil {
					config.ConvertColor(cable, issues)
				}

				validCables = append(validCables, cable)
			}
			patch["cables"] = validCables
		}
	}

	// Ensure version is set
	patch["version"] = "2.6.6"

	return nil
}

// DenormalizeV06Style converts v2 format to V0.6-style format.
// This is the shared denormalization for both v0.6 and MiRack formats.
//
// The function handles all common transformations:
// - Build ID-to-index mapping for cable conversion
// - Convert bypass→disabled, id→paramId
// - Convert cables→wires with module IDs→array indices
// - Remove v2-specific fields
//
// Format-specific behavior is provided via config:
// - HasFundamental: controls Core→Fundamental plugin conversion
// - ConvertColor: optional callback for hex→MiRack colorIndex conversion
func DenormalizeV06Style(patch map[string]any, config V06StyleConfig, issues *[]string) error {
	modules := getModules(patch)
	if modules == nil {
		*issues = append(*issues, fmt.Sprintf("%s denormalization: no modules array found", config.FormatName))
		return nil
	}

	// Build ID-to-index mapping for cable conversion.
	// Priority order:
	// 1. Use _originalIndexToID if available (stored during v0.6/MiRack → v2 normalization)
	// 2. Use _idToIndex if available (stored during v2 normalization)
	// 3. Build mapping on-the-fly from current modules
	var idToIndex map[int64]int

	// First try to get the original index-to-ID mapping from v0.6/MiRack normalization
	if indexToIDRaw, ok := patch["_originalIndexToID"]; ok {
		// Reverse the mapping: module ID → array index
		switch v := indexToIDRaw.(type) {
		case map[int]int64:
			idToIndex = make(map[int64]int, len(v))
			for idx, id := range v {
				idToIndex[id] = idx
			}
		case map[string]int64:
			idToIndex = make(map[int64]int, len(v))
			for idxStr, id := range v {
				idx := 0
				if _, err := fmt.Sscanf(idxStr, "%d", &idx); err == nil {
					idToIndex[id] = idx
				}
			}
		}
	}

	// If no original mapping found, try the v2 normalization mapping
	if idToIndex == nil {
		idToIndex = GetIDToIndexMapping(patch)
	}

	// Fall back to building mapping on-the-fly
	if idToIndex == nil {
		idToIndex = make(map[int64]int)
		for i, m := range modules {
			if mod, ok := m.(map[string]any); ok {
				if id := getInt64FromMap(mod, "id"); id >= 0 {
					idToIndex[id] = i
				}
			}
		}
	}

	// Convert modules
	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		// Apply plugin mapping using the format-specific callback
		if plugin, ok := mod["plugin"].(string); ok {
			if model, hasModel := mod["model"].(string); hasModel {
				if config.DenormalizePlugin != nil {
					newPlugin := plugin
					modified := false

					// Special handling for MiRack: Fundamental → Core
					// MiRack doesn't have a Fundamental plugin, so all Fundamental modules must be Core
					if !config.HasFundamental && plugin == "Fundamental" {
						newPlugin = "Core"
						modified = true
					} else {
						newPlugin, modified = config.DenormalizePlugin(plugin, model)
					}

					if modified {
						oldPlugin := plugin
						mod["plugin"] = newPlugin
						*issues = append(*issues, fmt.Sprintf("%s denormalization: module[%d]: %s/%s → %s/%s", config.FormatName, i, oldPlugin, model, newPlugin, model))
					}
				}
			}
		}

		// Convert bypass to disabled
		convertBypassToDisabled(mod, issues)

		// Convert id to paramId in parameters
		convertIDToParamID(mod)

		// Remove v2-specific fields not supported by v0.6/MiRack
		delete(mod, "version")       // Module version not supported
		delete(mod, "leftModuleId")  // Expander links not supported
		delete(mod, "rightModuleId") // Expander links not supported
	}

	// Convert cables to wires
	if cables, hasCables := patch["cables"]; hasCables {
		patch["wires"] = cables
		delete(patch, "cables")

		// Convert cable references from module IDs to array indices
		if wires, ok := patch["wires"].([]any); ok {
			validWires := make([]any, 0, len(wires))
			for i, w := range wires {
				wire, ok := w.(map[string]any)
				if !ok {
					continue
				}

				// Get module IDs
				outputModuleID := getInt64FromMap(wire, "outputModuleId")
				inputModuleID := getInt64FromMap(wire, "inputModuleId")

				// Convert module IDs to array indices
				outputIndex, outputExists := idToIndex[outputModuleID]
				inputIndex, inputExists := idToIndex[inputModuleID]

				// Preserve the wire even if module references are not found.
				// This can happen when converting v2 patches with module IDs that
				// weren't present in the original v0.6/MiRack patch.
				if !outputExists {
					*issues = append(*issues, fmt.Sprintf("%s denormalization: wire[%d]: outputModuleId %d not found in mapping (preserving with original ID)", config.FormatName, i, outputModuleID))
					// Keep original module ID - target application may handle missing modules
				} else {
					wire["outputModuleId"] = outputIndex
				}

				if !inputExists {
					*issues = append(*issues, fmt.Sprintf("%s denormalization: wire[%d]: inputModuleId %d not found in mapping (preserving with original ID)", config.FormatName, i, inputModuleID))
					// Keep original module ID - target application may handle missing modules
				} else {
					wire["inputModuleId"] = inputIndex
				}

				// Remove cable ID (v0.6/MiRack doesn't use cable IDs)
				delete(wire, "id")

				// Apply format-specific color conversion if provided
				if config.ConvertColor != nil {
					config.ConvertColor(wire, issues)
				}

				validWires = append(validWires, wire)
			}
			patch["wires"] = validWires
		}
	}

	// Remove v2-specific fields not supported by v0.6/MiRack
	delete(patch, "masterModuleId")     // Master module not supported
	delete(patch, "_idToIndex")         // Internal field, don't serialize
	delete(patch, "_originalIndexToID") // Internal field, don't serialize

	// Set version to v0.6 format (0.6.13 - the last v0.6 release)
	patch["version"] = "0.6.13"

	return nil
}
