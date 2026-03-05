package converter

import (
	"path/filepath"
	"strings"
)

// createHubMediumModule creates a 4ms HubMedium (MetaModule) module.
// It positions the module immediately to the right of the rightmost module at Y=0.
func createHubMediumModule(existingModules []any, root map[string]any, inputFilename string) map[string]any {
	// Find the top-right position (module with max X at Y=0)
	// Initialize to -1 so that with no modules, we start at position 0
	maxX := -1
	topY := 0
	for _, m := range existingModules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if pos, ok := mod["pos"].([]any); ok && len(pos) >= 2 {
			x, xOk := pos[0].(float64)
			y, yOk := pos[1].(float64)
			if xOk && yOk && int(y) == topY && int(x) > maxX {
				maxX = int(x)
			}
		}
	}

	// Position immediately to the right of the rightmost module at Y=0
	// We don't have module width info, so we position it at maxX + 1
	// This may overlap if the last module is wider than 1 HP, but user can adjust
	posX := maxX + 1
	posY := topY

	// If no modules found (maxX == -1), start at position 0
	if maxX < 0 {
		posX = 0
	}

	// Extract patch name and description from source
	patchName := "Enter Patch Name"
	patchDesc := "Patch Description"

	if name, ok := root["name"].(string); ok && name != "" {
		patchName = name
	} else if inputFilename != "" {
		// Use filename without extension as fallback
		patchName = strings.TrimSuffix(filepath.Base(inputFilename), filepath.Ext(inputFilename))
	}

	if desc, ok := root["description"].(string); ok && desc != "" {
		patchDesc = desc
	}

	return map[string]any{
		"id":      getNextModuleID(existingModules),
		"plugin":  "4msCompany",
		"model":   "HubMedium",
		"version": "2.1.4",
		"params":  getDefaultHubMediumParams(),
		"data":    getDefaultHubMediumData(patchName, patchDesc),
		"pos":     []any{float64(posX), float64(posY)},
	}
}

// getDefaultHubMediumParams returns the default parameters for HubMedium.
// 14 params: values 0.5 for knobs 0-11, 0 for 12-13.
func getDefaultHubMediumParams() []any {
	params := make([]any, 14)
	for i := 0; i < 12; i++ {
		params[i] = map[string]any{"value": 0.5, "id": i}
	}
	params[12] = map[string]any{"value": 0.0, "id": 12}
	params[13] = map[string]any{"value": 0.0, "id": 13}
	return params
}

// getDefaultHubMediumData returns the default data structure for HubMedium.
func getDefaultHubMediumData(patchName, patchDesc string) map[string]any {
	return map[string]any{
		"Mappings":     make([]any, 8),
		"KnobSetNames": make([]any, 8),
		"Alias": map[string]any{
			"Input":  make([]any, 8),
			"Output": make([]any, 8),
		},
		"PatchName":           patchName,
		"PatchDesc":           patchDesc,
		"MappingMode":         0,
		"SuggestedSampleRate": 0,
		"SuggestedBlockSize":  0,
		"DefaultKnobSet":      0,
	}
}

// getNextModuleID returns the next available module ID.
func getNextModuleID(modules []any) int64 {
	maxID := int64(-1)
	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if id := getInt64FromMap(mod, "id"); id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}
