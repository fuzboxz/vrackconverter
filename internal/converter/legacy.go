package converter

import (
	"fmt"
)

// PluginMapper defines how plugins are mapped between formats.
// Different formats have different plugin semantics - this interface
// allows each format to provide its own mapping strategy.
type PluginMapper interface {
	// NormalizePlugin converts source plugin to internal (v2) format.
	// Returns (newPlugin, wasModified).
	NormalizePlugin(plugin, model string) (string, bool)

	// DenormalizePlugin converts internal (v2) plugin to target format.
	// Returns (newPlugin, wasModified).
	DenormalizePlugin(plugin, model string) (string, bool)
}

// NoOpPluginMapper is used when no plugin conversion is needed.
// MiRack uses this - all modules use "Core" plugin (no Fundamental).
type NoOpPluginMapper struct{}

// NormalizePlugin returns the plugin unchanged.
func (NoOpPluginMapper) NormalizePlugin(plugin, model string) (string, bool) {
	return plugin, false
}

// DenormalizePlugin returns the plugin unchanged.
func (NoOpPluginMapper) DenormalizePlugin(plugin, model string) (string, bool) {
	return plugin, false
}

// V06PluginMapper handles Fundamental ↔ Core conversion for VCV Rack v0.6.
// VCV Rack v2 merged "Fundamental" into "Core", but v0.6 has them separate.
type V06PluginMapper struct{}

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

// NormalizePlugin converts Fundamental → Core for v0.6 → v2 conversion.
func (V06PluginMapper) NormalizePlugin(plugin, model string) (string, bool) {
	if plugin == "Fundamental" {
		return "Core", true
	}
	return plugin, false
}

// DenormalizePlugin converts Core → Fundamental for v2 → v0.6 conversion.
// Only modules that were originally in Fundamental plugin are converted back.
func (V06PluginMapper) DenormalizePlugin(plugin, model string) (string, bool) {
	if plugin == "Core" && fundamentalModules[model] {
		return "Fundamental", true
	}
	return plugin, false
}

// LegacyConverter provides shared conversion logic for v0.6-style formats.
// Each format provides its own PluginMapper for format-specific behavior.
type LegacyConverter struct {
	pluginMapper PluginMapper
}

// NewLegacyConverter creates a LegacyConverter with the given PluginMapper.
func NewLegacyConverter(mapper PluginMapper) *LegacyConverter {
	return &LegacyConverter{pluginMapper: mapper}
}

// NormalizeLegacy converts v0.6-style formats (array indices, wires) to v2 format.
// This is the shared normalization for both v0.6 and MiRack formats.
// The key difference is plugin mapping, handled by the PluginMapper.
func (lc *LegacyConverter) NormalizeLegacy(patch map[string]any, issues *[]string, formatName string) error {
	modules, ok := patch["modules"].([]any)
	if !ok {
		*issues = append(*issues, fmt.Sprintf("%s normalization: no modules array found", formatName))
		return nil
	}

	// Build index-to-ID mapping for cable reference conversion.
	// In v0.6/MiRack, wires use array indices, not module IDs.
	indexToID := make(map[int]int64)

	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("%s normalization: module[%d]: invalid module object", formatName, i))
			continue
		}

		// Apply plugin mapping using the format-specific mapper
		if plugin, ok := mod["plugin"].(string); ok {
			if model, hasModel := mod["model"].(string); hasModel {
				if newPlugin, modified := lc.pluginMapper.NormalizePlugin(plugin, model); modified {
					oldPlugin := plugin
					mod["plugin"] = newPlugin
					*issues = append(*issues, fmt.Sprintf("%s normalization: module[%d]: %s/%s → %s/%s", formatName, i, oldPlugin, model, newPlugin, model))
				}
			}
		}

		// Store index-to-ID mapping.
		// Check if "id" key exists (not just if value >= 0, since 0 is a valid ID).
		if _, hasID := mod["id"]; hasID {
			if id := getInt64FromMap(mod, "id"); id >= 0 {
				indexToID[i] = id
			} else {
				// ID exists but is negative, use array index as fallback
				indexToID[i] = int64(i)
			}
		} else {
			// Module has no ID field, use array index
			indexToID[i] = int64(i)
		}

		// Convert paramId to id in parameters
		transformParams(mod, i, issues)

		// Convert disabled to bypass (v2 format)
		if disabled, ok := mod["disabled"]; ok {
			if disabledBool, ok := disabled.(bool); ok {
				mod["bypass"] = disabledBool
			}
			delete(mod, "disabled")
		}

		// Remove format-specific fields not used in v2
		delete(mod, "sumPolyInputs")
	}

	// Store the index-to-ID mapping for later use during denormalization.
	// This ensures that when we convert back to v0.6/MiRack format, we can
	// correctly reverse the module ID → array index conversion.
	// Using underscore prefix to indicate this is an internal field.
	patch["_originalIndexToID"] = indexToID

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
					*issues = append(*issues, fmt.Sprintf("%s normalization: cable[%d]: invalid cable object", formatName, i))
					continue
				}

				// Get cable references (these are array indices in v0.6/MiRack)
				outputModuleIdx := getInt64FromMap(cable, "outputModuleId")
				inputModuleIdx := getInt64FromMap(cable, "inputModuleId")

				// Convert array indices to module IDs
				outputModuleID, outputExists := indexToID[int(outputModuleIdx)]
				inputModuleID, inputExists := indexToID[int(inputModuleIdx)]

				if !outputExists {
					*issues = append(*issues, fmt.Sprintf("%s normalization: cable[%d]: outputModuleId index %d out of range", formatName, i, outputModuleIdx))
					continue
				}
				if !inputExists {
					*issues = append(*issues, fmt.Sprintf("%s normalization: cable[%d]: inputModuleId index %d out of range", formatName, i, inputModuleIdx))
					continue
				}

				// Update cable with resolved module IDs
				cable["outputModuleId"] = outputModuleID
				cable["inputModuleId"] = inputModuleID

				// Convert color format if present (r,g,b,a object to hex string)
				if color, ok := cable["color"]; ok {
					switch v := color.(type) {
					case map[string]any:
						hexColor := convertColorToHex(v)
						if hexColor != "" {
							cable["color"] = hexColor
						} else {
							delete(cable, "color")
						}
					case float64:
						// colorIndex value - remove (format-specific)
						delete(cable, "color")
					}
				}

				validCables = append(validCables, cable)
			}
			patch["cables"] = validCables
		}
	}

	// Ensure version is set
	if version, ok := patch["version"].(string); !ok || version == "" {
		patch["version"] = "2.6.6"
		*issues = append(*issues, fmt.Sprintf("%s normalization: set default version to 2.6.6", formatName))
	}

	return nil
}

// DenormalizeLegacy converts v2 format to v0.6-style formats.
// This is the shared denormalization for both v0.6 and MiRack formats.
func (lc *LegacyConverter) DenormalizeLegacy(patch map[string]any, issues *[]string, formatName string) error {
	modules, ok := patch["modules"].([]any)
	if !ok {
		*issues = append(*issues, fmt.Sprintf("%s denormalization: no modules array found", formatName))
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
		// The stored map has string keys from JSON serialization
		switch v := indexToIDRaw.(type) {
		case map[int]int64:
			idToIndex = make(map[int64]int, len(v))
			for idx, id := range v {
				idToIndex[id] = idx
			}
		case map[string]int64:
			idToIndex = make(map[int64]int, len(v))
			for idxStr, id := range v {
				// Parse string key back to int
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

		// Apply plugin mapping using the format-specific mapper
		if plugin, ok := mod["plugin"].(string); ok {
			if model, hasModel := mod["model"].(string); hasModel {
				if newPlugin, modified := lc.pluginMapper.DenormalizePlugin(plugin, model); modified {
					oldPlugin := plugin
					mod["plugin"] = newPlugin
					*issues = append(*issues, fmt.Sprintf("%s denormalization: module[%d]: %s/%s → %s/%s", formatName, i, oldPlugin, model, newPlugin, model))
				}
			}
		}

		// Convert bypass to disabled
		if bypass, ok := mod["bypass"]; ok {
			if bypassBool, ok := bypass.(bool); ok {
				mod["disabled"] = bypassBool
				if bypassBool {
					*issues = append(*issues, fmt.Sprintf("%s denormalization: module[%d]: converted bypass=true to disabled", formatName, i))
				}
			}
			delete(mod, "bypass")
		}

		// Convert id to paramId in parameters
		if params, ok := mod["params"].([]any); ok {
			for j, p := range params {
				param, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if paramID, hasID := param["id"]; hasID {
					param["paramId"] = paramID
					delete(param, "id")
				} else if _, hasParamID := param["paramId"]; !hasParamID {
					param["paramId"] = j
				}
			}
		}

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
					*issues = append(*issues, fmt.Sprintf("%s denormalization: wire[%d]: outputModuleId %d not found in mapping (preserving with original ID)", formatName, i, outputModuleID))
					// Keep original module ID - target application may handle missing modules
				} else {
					wire["outputModuleId"] = outputIndex
				}

				if !inputExists {
					*issues = append(*issues, fmt.Sprintf("%s denormalization: wire[%d]: inputModuleId %d not found in mapping (preserving with original ID)", formatName, i, inputModuleID))
					// Keep original module ID - target application may handle missing modules
				} else {
					wire["inputModuleId"] = inputIndex
				}

				// Remove cable ID (v0.6/MiRack doesn't use cable IDs)
				delete(wire, "id")

				// Remove cable color (v0.6/MiRack uses colorIndex, not hex colors)
				// We can't reliably convert hex back to colorIndex, so just remove
				delete(wire, "color")

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
