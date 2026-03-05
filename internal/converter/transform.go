package converter

import (
	"encoding/json"
	"fmt"
)

func TransformPatch(root map[string]any, targetVersion string, issues *[]string) error {
	root["version"] = targetVersion

	delete(root, "path")

	modules, ok := root["modules"].([]any)
	if !ok {
		*issues = append(*issues, "no modules array found")
		return nil
	}

	// Apply plugin/model mappings for miRack compatibility
	applyPluginMappings(modules, issues)

	// Pass 1: Collect all existing IDs and build index-to-ID mapping
	usedIDs := make(map[int64]bool)
	modulesWithIDs := make([]struct {
		hasID bool
		id    int64
	}, len(modules))

	// Map from array index to actual module ID
	// In v0.6, cables reference modules by array index, not by ID
	indexToID := make(map[int]int64)

	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		if idVal, hasID := mod["id"]; hasID {
			var id int64
			switch v := idVal.(type) {
			case float64:
				id = int64(v)
			case int64:
				id = v
			case int:
				id = int64(v)
			}

			modulesWithIDs[i].hasID = true
			modulesWithIDs[i].id = id
			indexToID[i] = id

			// Check for duplicate
			if usedIDs[id] {
				*issues = append(*issues, fmt.Sprintf("module[%d]: duplicate ID %d detected", i, id))
			}
			usedIDs[id] = true
		} else {
			// Module has no ID, will get array index as ID
			indexToID[i] = int64(i)
		}
	}

	// Pass 2: Assign IDs - preserve existing IDs, assign sequential for missing
	nextID := int64(0)
	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("module[%d]: invalid module object", i))
			continue
		}

		if modulesWithIDs[i].hasID {
			// Module already has an ID - keep it (cables reference this ID)
			// Already marked as used in Pass 1
		} else {
			// No ID - assign next available sequential ID
			for usedIDs[nextID] {
				nextID++
			}
			mod["id"] = nextID
			usedIDs[nextID] = true
			nextID++
		}

		transformParams(mod, i, issues)

		// Convert "disabled" to "bypass" for v2 compatibility
		if disabled, ok := mod["disabled"]; ok {
			if disabledBool, ok := disabled.(bool); ok {
				mod["bypass"] = disabledBool
			}
			delete(mod, "disabled")
		}

		delete(mod, "sumPolyInputs")

		// Convert module-specific data
		convertModuleData(mod, issues)
	}

	wires, hasWires := root["wires"]
	if hasWires {
		root["cables"] = wires
		delete(root, "wires")
	}

	cables, ok := root["cables"].([]any)
	if !ok {
		return nil
	}

	// Process cables, converting array indices to module IDs
	// In v0.6, wires reference modules by array index, not by ID
	// So we need to convert: array index → module ID
	validCables := make([]any, 0)
	for i, c := range cables {
		cable, ok := c.(map[string]any)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("cable[%d]: invalid cable object", i))
			continue
		}

		// Get cable references (these are array indices in v0.6)
		outputModuleIdx := getInt64FromMap(cable, "outputModuleId")
		inputModuleIdx := getInt64FromMap(cable, "inputModuleId")

		// Convert array indices to module IDs
		outputModuleID, outputExists := indexToID[int(outputModuleIdx)]
		inputModuleID, inputExists := indexToID[int(inputModuleIdx)]

		if !outputExists {
			*issues = append(*issues, fmt.Sprintf("cable[%d]: outputModuleId index %d out of range", i, outputModuleIdx))
			continue
		}
		if !inputExists {
			*issues = append(*issues, fmt.Sprintf("cable[%d]: inputModuleId index %d out of range", i, inputModuleIdx))
			continue
		}

		// Update cable with resolved module IDs
		cable["outputModuleId"] = outputModuleID
		cable["inputModuleId"] = inputModuleID

		// Remap port IDs if needed
		outputID := getInt64FromMap(cable, "outputId")
		inputID := getInt64FromMap(cable, "inputId")

		// Get module info for port remapping
		outputModule := findModuleByID(modules, outputModuleID)
		inputModule := findModuleByID(modules, inputModuleID)

		if outputModule != nil {
			plugin, _ := outputModule["plugin"].(string)
			model, _ := outputModule["model"].(string)
			key := fmt.Sprintf("%s/%s", plugin, model)

			if portMap, exists := portMappings[key]; exists {
				if outputMap, exists := portMap["outputs"]; exists {
					if newID, shouldRemap := outputMap[outputID]; shouldRemap {
						cable["outputId"] = newID
						*issues = append(*issues, fmt.Sprintf("cable[%d]: remapped %s output port %d → %d", i, model, outputID, newID))
					}
				}
			}
		}

		if inputModule != nil {
			plugin, _ := inputModule["plugin"].(string)
			model, _ := inputModule["model"].(string)
			key := fmt.Sprintf("%s/%s", plugin, model)

			if portMap, exists := portMappings[key]; exists {
				if inputMap, exists := portMap["inputs"]; exists {
					if newID, shouldRemap := inputMap[inputID]; shouldRemap {
						cable["inputId"] = newID
						*issues = append(*issues, fmt.Sprintf("cable[%d]: remapped %s input port %d → %d", i, model, inputID, newID))
					}
				}
			}
		}

		if _, hasID := cable["id"]; !hasID {
			cable["id"] = len(validCables)
		}

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
				delete(cable, "color")
			}
		}

		validCables = append(validCables, cable)
	}

	// Replace cables array with filtered version
	root["cables"] = validCables

	// Validate conversion
	validateConversion(root, issues)

	return nil
}

func validateConversion(root map[string]any, issues *[]string) {
	modules, ok := root["modules"].([]any)
	if !ok {
		return
	}

	cables, ok := root["cables"].([]any)
	if !ok {
		return
	}

	// Build module ID set
	moduleIDs := make(map[int64]bool)
	for _, m := range modules {
		if mod, ok := m.(map[string]any); ok {
			if id := getInt64FromMap(mod, "id"); id >= 0 {
				moduleIDs[id] = true
			}
		}
	}

	// Count issues
	missingPlugins := 0
	missingModels := 0

	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		plugin, _ := mod["plugin"].(string)
		model, _ := mod["model"].(string)

		// Check if plugin/model mapping suggests incompatibility
		if plugin == "" {
			missingPlugins++
		}
		if model == "" {
			missingModels++
		}
	}

	// Add summary warnings
	if missingPlugins > 0 {
		*issues = append(*issues, fmt.Sprintf("warning: %d modules have missing plugin info", missingPlugins))
	}
	if missingModels > 0 {
		*issues = append(*issues, fmt.Sprintf("warning: %d modules have missing model info", missingModels))
	}

	// Check cable integrity
	brokenCables := 0
	for _, c := range cables {
		cable, ok := c.(map[string]any)
		if !ok {
			continue
		}

		outID := getInt64FromMap(cable, "outputModuleId")
		inID := getInt64FromMap(cable, "inputModuleId")

		if !moduleIDs[outID] || !moduleIDs[inID] {
			brokenCables++
		}
	}

	if brokenCables > 0 {
		*issues = append(*issues, fmt.Sprintf("warning: %d cables have broken references (should be 0)", brokenCables))
	}
}

// Plugin and model mappings for miRack compatibility
var pluginMappings = map[string]string{
	// Add known miRack→VCV plugin renames here
	// Example: "miRackPlugin": "VCVPlugin",
}

var modelMappings = map[string]map[string]string{
	// Format: "plugin": {"oldModel": "newModel"}
	// Example: "Core": {"AudioInterface": "AudioInterface2"},
}


// Port ID remappings for modules that changed between miRack and VCV Rack 2.
// Format: "plugin/model": {portType: {oldID: newID}}
// portType is "inputs" or "outputs"
var portMappings = map[string]map[string]map[int64]int64{}

func applyPluginMappings(modules []any, issues *[]string) {
	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		plugin, hasPlugin := mod["plugin"].(string)
		if !hasPlugin {
			continue
		}

		// Check for plugin rename
		if newPlugin, exists := pluginMappings[plugin]; exists {
			mod["plugin"] = newPlugin
			*issues = append(*issues, fmt.Sprintf("mapped plugin %s → %s", plugin, newPlugin))
			plugin = newPlugin
		}

		// Check for model rename
		model, hasModel := mod["model"].(string)
		if !hasModel {
			continue
		}

		if pluginModels, exists := modelMappings[plugin]; exists {
			if newModel, exists := pluginModels[model]; exists {
				mod["model"] = newModel
				*issues = append(*issues, fmt.Sprintf("mapped %s/%s → %s", plugin, model, newModel))
			}
		}
	}
}

func convertModuleData(mod map[string]any, issues *[]string) {
	data, ok := mod["data"].(map[string]any)
	if !ok {
		return
	}

	plugin, _ := mod["plugin"].(string)
	model, _ := mod["model"].(string)

	// Convert audio device configuration
	if plugin == "Core" && (model == "AudioInterface" || model == "AudioInterface2") {
		if audio, ok := data["audio"].(map[string]any); ok {
			convertAudioData(audio, issues)
		}
	}

	// Add other module-specific conversions here
}

func convertAudioData(audio map[string]any, issues *[]string) {
	// miRack audio data may have different field names or structure
	// VCV Rack 2 expects:
	// - driver: number
	// - device: number
	// - sampleRate: number
	// - blockSize: number

	// Convert deviceName to device number if present (miRack specific)
	if deviceName, ok := audio["deviceName"].(string); ok {
		if deviceName == "Default" {
			// Remove deviceName, keep device as 0 (default)
			delete(audio, "deviceName")
			if _, hasDevice := audio["device"]; !hasDevice {
				audio["device"] = 0
			}
		}
	}

	// Ensure sampleRate is present
	if _, hasSampleRate := audio["sampleRate"]; !hasSampleRate {
		audio["sampleRate"] = 44100
	}

	// Ensure blockSize is present
	if _, hasBlockSize := audio["blockSize"]; !hasBlockSize {
		audio["blockSize"] = 256
	}

	// Remove miRack-specific fields
	delete(audio, "offset")
	delete(audio, "maxChannels")
}

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

func transformParams(mod map[string]any, moduleIndex int, issues *[]string) {
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

func ToJSON(root map[string]any) ([]byte, error) {
	return json.MarshalIndent(root, "", "  ")
}

func FromJSON(data []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return root, nil
}
