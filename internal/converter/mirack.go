package converter

import (
	"fmt"
	"os"
	"path/filepath"
)

// MiRackHandler implements FormatHandler for MiRack .mrk bundles.
// MiRack bundles are directories containing patch.vcv (plain JSON, not compressed).
type MiRackHandler struct{}

// Read reads a MiRack patch from path.
// For .mrk bundles, path is the directory, and we read path/patch.vcv.
// For direct .vcv files (inside .mrk), path is the file itself.
//
// Returns error if the file is not a valid MiRack patch (e.g., it's a zstd archive).
func (h *MiRackHandler) Read(path string) ([]byte, error) {
	// Check if path is a directory (.mrk bundle)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	var actualPath string
	if info.IsDir() {
		// .mrk bundle: read patch.vcv inside
		actualPath = filepath.Join(path, "patch.vcv")
	} else {
		// Direct file path
		actualPath = path
	}

	data, err := os.ReadFile(actualPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MiRack patch: %w", err)
	}

	// Validate that this is actually a MiRack patch (plain JSON).
	// VCV Rack v2 files are zstd archives (start with 0xFD2FB528).
	if len(data) >= 4 && data[0] == 0x28 && data[1] == 0xB5 && data[2] == 0x2F && data[3] == 0xFD {
		return nil, fmt.Errorf("file is a zstd archive (VCV Rack v2), not a MiRack patch")
	}

	return data, nil
}

// Write writes a MiRack patch to path.
// Creates a .mrk directory bundle with patch.vcv (plain JSON) inside.
func (h *MiRackHandler) Write(data []byte, path string) error {
	return CreateMrkBundle(data, path)
}

// Extension returns the file extension for MiRack format.
func (h *MiRackHandler) Extension() string {
	return ".mrk"
}

// NormalizeMiRack converts a MiRack patch to the internal format.
// This preserves all MiRack-specific data for potential round-trip conversion.
// The internal format is based on VCV Rack v2 (the superset format).
func NormalizeMiRack(patch map[string]any, issues *[]string) error {
	modules, ok := patch["modules"].([]any)
	if !ok {
		*issues = append(*issues, "MiRack normalization: no modules array found")
		return nil
	}

	// Build index-to-ID mapping for cable reference conversion
	// In MiRack, wires use array indices, not module IDs
	indexToID := make(map[int]int64)

	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("MiRack normalization: module[%d]: invalid module object", i))
			continue
		}

		// Store index-to-ID mapping
		// Check if "id" key exists (not just if value >= 0, since 0 is a valid ID)
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

		// Remove MiRack-specific fields not used in v2
		delete(mod, "sumPolyInputs")
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
					*issues = append(*issues, fmt.Sprintf("MiRack normalization: cable[%d]: invalid cable object", i))
					continue
				}

				// Get cable references (these are array indices in MiRack)
				outputModuleIdx := getInt64FromMap(cable, "outputModuleId")
				inputModuleIdx := getInt64FromMap(cable, "inputModuleId")

				// Convert array indices to module IDs
				outputModuleID, outputExists := indexToID[int(outputModuleIdx)]
				inputModuleID, inputExists := indexToID[int(inputModuleIdx)]

				if !outputExists {
					*issues = append(*issues, fmt.Sprintf("MiRack normalization: cable[%d]: outputModuleId index %d out of range", i, outputModuleIdx))
					continue
				}
				if !inputExists {
					*issues = append(*issues, fmt.Sprintf("MiRack normalization: cable[%d]: inputModuleId index %d out of range", i, inputModuleIdx))
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
						// colorIndex value - remove (MiRack-specific)
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
		*issues = append(*issues, "MiRack normalization: set default version to 2.6.6")
	}

	return nil
}

// DenormalizeMiRack converts the internal format to MiRack format.
// This is the KEY function for v2 → MiRack conversion.
// It converts cables→wires, module IDs→array indices, bypass→disabled, id→paramId.
func DenormalizeMiRack(patch map[string]any, issues *[]string) error {
	modules, ok := patch["modules"].([]any)
	if !ok {
		*issues = append(*issues, "MiRack denormalization: no modules array found")
		return nil
	}

	// Build ID-to-index mapping from the normalized patch
	// This was stored by NormalizeV2 in the _idToIndex field
	idToIndex := GetIDToIndexMapping(patch)
	if idToIndex == nil {
		// Build mapping on-the-fly if not available
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

		// Convert bypass to disabled
		if bypass, ok := mod["bypass"]; ok {
			if bypassBool, ok := bypass.(bool); ok {
				mod["disabled"] = bypassBool
				if bypassBool {
					*issues = append(*issues, fmt.Sprintf("MiRack denormalization: module[%d]: converted bypass=true to disabled", i))
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

		// Remove v2-specific fields not supported by MiRack
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

				if !outputExists {
					*issues = append(*issues, fmt.Sprintf("MiRack denormalization: wire[%d]: outputModuleId %d not found in module list", i, outputModuleID))
					continue
				}
				if !inputExists {
					*issues = append(*issues, fmt.Sprintf("MiRack denormalization: wire[%d]: inputModuleId %d not found in module list", i, inputModuleID))
					continue
				}

				// Update wire with array indices
				wire["outputModuleId"] = outputIndex
				wire["inputModuleId"] = inputIndex

				// Remove cable ID (MiRack doesn't use cable IDs)
				delete(wire, "id")

				// Remove cable color (MiRack uses colorIndex, not hex colors)
				// We can't reliably convert hex back to colorIndex, so just remove
				delete(wire, "color")

				validWires = append(validWires, wire)
			}
			patch["wires"] = validWires
		}
	}

	// Remove v2-specific fields not supported by MiRack
	delete(patch, "masterModuleId") // Master module not supported
	delete(patch, "_idToIndex")     // Internal field, don't serialize

	// Set version to MiRack format (0.6.13 - the last v0.6 release)
	patch["version"] = "0.6.13"

	return nil
}

// CreateMrkBundle creates a .mrk directory bundle with patch.vcv inside.
// The data should be plain JSON bytes (not compressed).
func CreateMrkBundle(data []byte, path string) error {
	// Ensure path ends with .mrk
	if filepath.Ext(path) != ".mrk" {
		return fmt.Errorf("MiRack bundles must use .mrk extension")
	}

	// Create the .mrk directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create .mrk directory: %w", err)
	}

	// Write patch.vcv as plain JSON
	patchPath := filepath.Join(path, "patch.vcv")
	if err := os.WriteFile(patchPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write patch.vcv: %w", err)
	}

	return nil
}
