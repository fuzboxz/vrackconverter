package converter

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// FromJSON parses JSON bytes into a map[string]any structure.
// This is a common utility used by all format handlers for JSON parsing.
func FromJSON(data []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return root, nil
}

// ToJSON serializes a map[string]any structure to indented JSON bytes.
// This is a common utility used by all format handlers for JSON serialization.
func ToJSON(root map[string]any) ([]byte, error) {
	return json.MarshalIndent(root, "", "  ")
}

// getInt64FromMap extracts an int64 value from a map using the given key.
// Handles float64 (JSON number default), int64, and int types.
// This is a common utility used by all format handlers for JSON number handling.
func getInt64FromMap(m map[string]any, key string) int64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		}
	}
	return 0
}

// convertColorToHex converts a color map with r,g,b,a float64 values (0-1 range)
// to a hexadecimal string format (rrggbbaa).
// This is a common utility used by all format handlers for color conversion.
func convertColorToHex(color map[string]any) string {
	r, rok := color["r"].(float64)
	g, gok := color["g"].(float64)
	b, bok := color["b"].(float64)
	a, aok := color["a"].(float64)

	if !rok || !gok || !bok {
		return ""
	}

	if !aok {
		a = 1.0
	}

	return fmt.Sprintf("%02x%02x%02x%02x", uint8(r*255), uint8(g*255), uint8(b*255), uint8(a*255))
}

// hexToRGB converts "#rrggbb" to (r, g, b) bytes (0-255).
// Accepts hex with or without "#" prefix.
// Returns ok=false for invalid input.
func hexToRGB(hexColor string) (r, g, b uint8, ok bool) {
	// Strip # prefix if present
	s := hexColor
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}

	// Validate length (expecting 6 chars for RGB)
	if len(s) != 6 {
		return 0, 0, 0, false
	}

	// Parse hex values
	values, err := hex.DecodeString(s)
	if err != nil || len(values) != 3 {
		return 0, 0, 0, false
	}

	return values[0], values[1], values[2], true
}

// rgbToHex converts RGB bytes to "#rrggbb" hex string.
func rgbToHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// findModuleByID searches for a module with the given ID in a modules array.
// Returns the module map if found, nil otherwise.
// This is a common utility used by all format handlers for module lookups.
func findModuleByID(modules []any, id int64) map[string]any {
	for _, m := range modules {
		if mod, ok := m.(map[string]any); ok {
			if modID := getInt64FromMap(mod, "id"); modID == id {
				return mod
			}
		}
	}
	return nil
}

// convertParamIDToID converts paramId to id in parameters.
// This is used when normalizing v0.6/MiRack formats to v2.
func convertParamIDToID(mod map[string]any, moduleIndex int, issues *[]string) {
	params, ok := mod["params"].([]any)
	if !ok {
		return
	}

	for i, p := range params {
		param, ok := p.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("module[%d]: param[%d]: invalid param object", moduleIndex, i))
			continue
		}

		if paramID, hasParamID := param["paramId"]; hasParamID {
			param["id"] = paramID
			delete(param, "paramId")
		} else if _, hasID := param["id"]; !hasID {
			param["id"] = i
		}
	}
}

// convertDisabledToBypass converts disabled to bypass.
// This is used when normalizing v0.6/MiRack formats to v2.
func convertDisabledToBypass(mod map[string]any, issues *[]string) {
	if disabled, ok := mod["disabled"]; ok {
		if disabledBool, ok := disabled.(bool); ok {
			mod["bypass"] = disabledBool
		}
		delete(mod, "disabled")
	}
}

// convertBypassToDisabled converts bypass to disabled.
// This is used when denormalizing v2 to v0.6/MiRack formats.
func convertBypassToDisabled(mod map[string]any, issues *[]string) {
	if bypass, ok := mod["bypass"]; ok {
		if bypassBool, ok := bypass.(bool); ok {
			mod["disabled"] = bypassBool
		}
		delete(mod, "bypass")
	}
}

// convertIDToParamID converts id to paramId in parameters.
// This is used when denormalizing v2 to v0.6/MiRack formats.
func convertIDToParamID(mod map[string]any) {
	if params, ok := mod["params"].([]any); ok {
		for _, p := range params {
			param, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if paramID, hasID := param["id"]; hasID {
				param["paramId"] = paramID
				delete(param, "id")
			} else if _, hasParamID := param["paramId"]; !hasParamID {
				// This shouldn't happen in valid v2 patches, but handle it
				// paramId will be assigned its array index during iteration
			}
		}
	}
}

// getModules safely extracts the modules array from a patch.
// Returns nil if modules array doesn't exist.
func getModules(patch map[string]any) []any {
	modules, ok := patch["modules"].([]any)
	if !ok {
		return nil
	}
	return modules
}

// parseStringKeyToInt64 parses a string key to int64.
// This is needed when handling JSON-deserialized maps with string keys.
func parseStringKeyToInt64(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
