package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================================
// MiRack Color Palette
// ============================================================================

// miRackColorPalette defines the 6 colors available in MiRack by colorIndex.
// Order: yellow (0), red (1), green (2), teal (3), orange (4), purple (5)
// Values are RGB bytes (0-255).
var miRackColorPalette = []struct {
	name    string
	r, g, b uint8
}{
	{"yellow", 255, 181, 0},  // colorIndex 0: #ffb500
	{"red", 242, 56, 74},     // colorIndex 1: #f2384a
	{"green", 0, 181, 110},   // colorIndex 2: #00b56e
	{"teal", 54, 149, 239},   // colorIndex 3: #3695ef
	{"orange", 255, 181, 56}, // colorIndex 4: #ffb538
	{"purple", 140, 74, 181}, // colorIndex 5: #8c4ab5
}

// miRackToV2ModuleMap maps MiRack model names to VCV Rack V2 model names.
// Only modules that have different names need to be listed.
// Note: Audio modules are handled by merge logic, not by this map.
// V2 audio modules: AudioInterface (8-ch), AudioInterface2 (2-ch), AudioInterface16 (16-ch).
var miRackToV2ModuleMap = map[string]string{
	// Audio modules - merged separately, these entries are for reference only
	"AudioInterface":     "AudioInterface",
	"AudioInterfaceIn":   "AudioInterface",
	"AudioInterface8":    "AudioInterface",
	"AudioInterfaceIn8":  "AudioInterface",
	"AudioInterface16":   "AudioInterface16",
	"AudioInterfaceIn16": "AudioInterface16",
	// MIDI modules
	"MIDIBasicInterfaceOut":   "CV-MIDI",
	"MIDICCInterface":         "MIDICCToCVInterface",
	"MIDICCInterfaceOut":      "CV-CC",
	"MIDITriggerInterface":    "MIDITriggerToCVInterface",
	"MIDITriggerInterfaceOut": "CV-Gate",
	// Polyphony modules
	"PolyMerger":   "Merge",
	"PolySplitter": "Split",
	"PolySummer":   "Sum",
}

// v2ToMiRackModuleMap is the reverse mapping for V2 → MiRack conversion.
// Initialized at package init time.
var v2ToMiRackModuleMap map[string]string

func init() {
	v2ToMiRackModuleMap = make(map[string]string, len(miRackToV2ModuleMap))
	for mirack, v2 := range miRackToV2ModuleMap {
		// Skip audio modules in the reverse map - they're handled specially by merge/split
		// The forward map has both "AudioInterface" and "AudioInterfaceIn" mapping to "AudioInterface2",
		// which would cause one to overwrite the other in the reverse map.
		if isMiRackAudioOutputModule(mirack) || isMiRackAudioInputModule(mirack) {
			continue
		}
		v2ToMiRackModuleMap[v2] = mirack
	}
}

// miRackColorIndexToHex converts a MiRack colorIndex to hex string "#rrggbb".
func miRackColorIndexToHex(index int) string {
	if index < 0 || index >= len(miRackColorPalette) {
		return "#ffffff" // Default to white for invalid index
	}
	c := miRackColorPalette[index]
	return rgbToHex(c.r, c.g, c.b)
}

// rgbToMiRackColorIndex finds the nearest MiRack colorIndex for given RGB bytes.
// Uses Euclidean distance in RGB space to find the closest match.
func rgbToMiRackColorIndex(r, g, b uint8) int {
	bestIndex := 0
	bestDistance := float64(255 * 255 * 3) // Max possible distance

	for i, c := range miRackColorPalette {
		// Calculate Euclidean distance in RGB space
		dr := int(r) - int(c.r)
		dg := int(g) - int(c.g)
		db := int(b) - int(c.b)
		distance := float64(dr*dr + dg*dg + db*db)

		if distance < bestDistance {
			bestDistance = distance
			bestIndex = i
		}
	}

	return bestIndex
}

// isPolyphonyModule checks if a V2 model is a polyphony module in Fundamental plugin.
func isPolyphonyModule(model string) bool {
	switch model {
	case "Merge", "Split", "Sum":
		return true
	}
	return false
}

// ============================================================================
// Audio Module Merge/Split for MiRack ↔ V2
// ============================================================================

// audioModuleInfo holds information about an audio module pair.
type audioModuleInfo struct {
	outputModule   map[string]any // The AudioInterface (output) module
	inputModule    map[string]any // The AudioInterfaceIn (input) module
	outputIndex    int            // Array index of output module
	inputIndex     int            // Array index of input module
	outputModuleID int64          // ID of output module
	inputModuleID  int64          // ID of input module
	channelCount   string         // "2", "8", or "16" (empty for default)
	v2ModelName    string         // "AudioInterface2", "AudioInterface8", etc.
}

// isMiRackAudioOutputModule checks if a model is a MiRack audio output module.
// Also recognizes V2 model names after module name mapping.
func isMiRackAudioOutputModule(model string) bool {
	switch model {
	case "AudioInterface", "AudioInterface2", "AudioInterface8", "AudioInterface16":
		return true
	}
	return false
}

// isMiRackAudioInputModule checks if a model is a MiRack audio input module.
// Also recognizes V2 model names after module name mapping.
func isMiRackAudioInputModule(model string) bool {
	switch model {
	case "AudioInterfaceIn", "AudioInterfaceIn8", "AudioInterfaceIn16":
		return true
	}
	return false
}

// validateAudioModuleCount checks if the patch has valid audio module configuration for MiRack.
// MiRack only supports ONE audio output and ONE audio input module total (across all channel counts).
// Returns (isValid, skipReason) where skipReason is empty if valid.
func validateAudioModuleCount(modules []any) (valid bool, skipReason string) {
	outputCount := 0
	inputCount := 0

	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		model, _ := mod["model"].(string)
		if isMiRackAudioOutputModule(model) {
			outputCount++
		} else if isMiRackAudioInputModule(model) {
			inputCount++
		}
	}

	if outputCount > 1 {
		return false, fmt.Sprintf("patch has %d audio output modules (MiRack only supports 1)", outputCount)
	}
	if inputCount > 1 {
		return false, fmt.Sprintf("patch has %d audio input modules (MiRack only supports 1)", inputCount)
	}
	return true, ""
}

// getChannelCountFromAudioModel extracts the channel count from an audio module model name.
// Returns "2" for default (AudioInterface), or the number from the model name.
func getChannelCountFromAudioModel(model string) string {
	switch model {
	case "AudioInterface8", "AudioInterfaceIn8":
		return "8"
	case "AudioInterface16", "AudioInterfaceIn16":
		return "16"
	default:
		return "2" // Default to 2-channel for AudioInterface/AudioInterfaceIn
	}
}

// getV2AudioModelName returns the V2 model name for a given channel count.
// V2 audio modules: AudioInterface (8-ch), AudioInterface2 (2-ch), AudioInterface16 (16-ch).
func getV2AudioModelName(channelCount string) string {
	switch channelCount {
	case "2":
		return "AudioInterface2"
	case "8":
		return "AudioInterface"
	case "16":
		return "AudioInterface16"
	default:
		return "AudioInterface" // Default to 8-channel
	}
}

// getMiRackAudioOutputModelName returns the MiRack output model name for a given channel count.
func getMiRackAudioOutputModelName(channelCount string) string {
	switch channelCount {
	case "8":
		return "AudioInterface8"
	case "16":
		return "AudioInterface16"
	default:
		return "AudioInterface"
	}
}

// getMiRackAudioInputModelName returns the MiRack input model name for a given channel count.
func getMiRackAudioInputModelName(channelCount string) string {
	switch channelCount {
	case "8":
		return "AudioInterfaceIn8"
	case "16":
		return "AudioInterfaceIn16"
	default:
		return "AudioInterfaceIn"
	}
}

// findAudioModulePairs finds matching audio input/output module pairs in the modules array.
// Returns a list of pairs that should be merged.
// Uses the maximum channel count when input/output have different channel counts.
func findAudioModulePairs(modules []any) []audioModuleInfo {
	var pairs []audioModuleInfo
	pairedInputs := make(map[int]bool) // Track input modules already paired

	// First pass: find output modules and their matching input modules
	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		model, _ := mod["model"].(string)
		if !isMiRackAudioOutputModule(model) {
			continue
		}

		outputID := getInt64FromMap(mod, "id")
		channelCount := getChannelCountFromAudioModel(model)

		// Look for the matching input module
		var inputMod map[string]any
		var inputIdx int
		var inputID int64

		for j, n := range modules {
			if pairedInputs[j] {
				continue // Already paired with another output
			}

			inMod, ok := n.(map[string]any)
			if !ok {
				continue
			}

			inModel, _ := inMod["model"].(string)
			if !isMiRackAudioInputModule(inModel) {
				continue
			}

			// Pair with any unpaired input module
			// Use the MAXIMUM channel count between output and input
			// This handles mismatched channel counts (e.g., 2-channel out + 8-channel in)
			inputMod = inMod
			inputIdx = j
			inputID = getInt64FromMap(inMod, "id")
			pairedInputs[j] = true
			break
		}

		// Determine the final channel count for the merged module
		// Use the maximum of output and input channel counts to accommodate both
		finalChannelCount := channelCount
		if inputMod != nil {
			inModel := inputMod["model"].(string)
			inChannelCount := getChannelCountFromAudioModel(inModel)
			// Use the larger channel count
			if inChannelCount == "16" || (inChannelCount == "8" && finalChannelCount != "16") {
				finalChannelCount = inChannelCount
			}
		}

		// Create a pair even if we didn't find a matching input.
		// We'll use key existence in the metadata to track input module existence.
		// This is critical because inputModuleID can be 0, which is a valid ID.
		pair := audioModuleInfo{
			outputModule:   mod,
			inputModule:    inputMod,
			outputIndex:    i,
			inputIndex:     inputIdx,
			outputModuleID: outputID,
			inputModuleID:  inputID,
			channelCount:   finalChannelCount,
			v2ModelName:    getV2AudioModelName(finalChannelCount),
		}
		pairs = append(pairs, pair)
	}

	return pairs
}

// mergeAudioModules merges MiRack's separate audio input/output modules into V2's single module.
// Called AFTER wire-to-cable conversion in NormalizeMiRack, so it works with module IDs, not indices.
func mergeAudioModules(patch map[string]any, issues *[]string) error {
	modules := getModules(patch)
	if modules == nil {
		return nil
	}

	pairs := findAudioModulePairs(modules)
	if len(pairs) == 0 {
		return nil // No audio modules to merge
	}

	// Build mapping for roundtrip (inputModuleID → outputModuleID)
	// Also track which modules to remove
	inputToOutput := make(map[int64]int64)
	modulesToRemove := make(map[int]bool)
	moduleIDToNewID := make(map[int64]int64) // For cable remapping

	for _, pair := range pairs {
		// Create the merged module
		mergedModule := make(map[string]any)

		// Copy all properties from output module first
		for k, v := range pair.outputModule {
			mergedModule[k] = v
		}

		// Set the V2 model name
		mergedModule["model"] = pair.v2ModelName

		// Store metadata for roundtrip
		// _mergedAudioModule stores the info needed to split back
		// Use key existence to indicate whether modules existed (handles ID 0 correctly)
		metadata := map[string]any{}
		if pair.outputModule != nil {
			metadata["outputModuleID"] = pair.outputModuleID
		}
		if pair.inputModule != nil {
			metadata["inputModuleID"] = pair.inputModuleID
		}
		// Store original positions for roundtrip
		if pos, ok := pair.outputModule["pos"].([]any); ok {
			metadata["outputModulePos"] = pos
		}
		if pair.inputModule != nil {
			if pos, ok := pair.inputModule["pos"].([]any); ok {
				metadata["inputModulePos"] = pos
			}
		}
		mergedModule["_mergedAudioModule"] = metadata

		// Update position to output module's position
		if pos, ok := pair.outputModule["pos"].([]any); ok {
			mergedModule["pos"] = pos
		}

		// Mark modules for removal
		modulesToRemove[pair.outputIndex] = true
		if pair.inputModule != nil {
			modulesToRemove[pair.inputIndex] = true
		}

		// Store mappings for cable remapping
		// Both input and output cables should reference the merged module (using output module's ID)
		mergedID := pair.outputModuleID
		moduleIDToNewID[pair.outputModuleID] = mergedID
		if pair.inputModule != nil {
			moduleIDToNewID[pair.inputModuleID] = mergedID
			inputToOutput[pair.inputModuleID] = pair.outputModuleID
		}

		// Replace the output module with the merged module in the array
		modules[pair.outputIndex] = mergedModule

		// We'll remove the input module and shift the array below
	}

	// Remove input modules (those marked but not replaced)
	// We need to rebuild the modules array
	var newModules []any
	for i, m := range modules {
		if modulesToRemove[i] {
			// Check if this was replaced (output module) or just removed (input)
			if mod, ok := m.(map[string]any); ok {
				if _, hasMerged := mod["_mergedAudioModule"]; hasMerged {
					// This is the merged module, keep it
					newModules = append(newModules, m)
				}
				// Otherwise skip (this is the input module being removed)
			}
		} else {
			newModules = append(newModules, m)
		}
	}
	patch["modules"] = newModules

	// Keep _originalIndexToID for roundtrip conversion
	// It contains the mapping from original indices (before merge) to module IDs,
	// which is needed for denormalization

	// Store the mapping for roundtrip splitting
	if len(inputToOutput) > 0 {
		patch["_audioInputToOutput"] = inputToOutput
	}

	// Update cable references (wires are already converted to cables at this point)
	// At this point, cables use module IDs (not array indices)
	if cables, ok := patch["cables"].([]any); ok {
		for _, c := range cables {
			cable, ok := c.(map[string]any)
			if !ok {
				continue
			}

			outputID := getInt64FromMap(cable, "outputModuleId")
			inputID := getInt64FromMap(cable, "inputModuleId")

			// Check if this cable's output comes from the input module
			// (AudioInterfaceIn's output going to another module)
			wasFromInputModuleOutput := false
			cableToOutputModule := false
			for _, pair := range pairs {
				if pair.inputModule == nil {
					continue
				}
				// Check based on original wire indices (stored before conversion)
				originalOutputIdx, _ := cable["_originalOutputIdx"].(int64)
				originalInputIdx, _ := cable["_originalInputIdx"].(int64)

				// Cable from AudioInterfaceIn's output
				if int(originalOutputIdx) == pair.inputIndex {
					wasFromInputModuleOutput = true
				}
				// Cable going TO AudioInterface's input (from another module)
				if int(originalInputIdx) == pair.outputIndex && originalOutputIdx != originalInputIdx {
					cableToOutputModule = true
				}
			}

			// Clean up original indices
			delete(cable, "_originalOutputIdx")
			delete(cable, "_originalInputIdx")

			// Update output module ID if it was merged
			if newID, exists := moduleIDToNewID[outputID]; exists {
				cable["outputModuleId"] = newID
				// Mark this cable as originating from input module's output
				if wasFromInputModuleOutput {
					cable["_cableFromInputModule"] = true
				}
			}

			// Update input module ID if it was merged
			if newID, exists := moduleIDToNewID[inputID]; exists {
				cable["inputModuleId"] = newID
				// Mark this cable as going TO the output module's input (AudioInterface input)
				if cableToOutputModule {
					cable["_cableToOutputModule"] = true
				}
			}
		}
	}

	return nil
}

// splitAudioModules splits V2's single audio module into MiRack's separate input/output modules.
// Called before module name mapping in DenormalizeMiRack.
func splitAudioModules(patch map[string]any, issues *[]string) error {
	modules := getModules(patch)
	if modules == nil {
		return nil
	}

	// Check if we have roundtrip metadata
	hasRoundtripData := false
	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if _, hasMerged := mod["_mergedAudioModule"]; hasMerged {
			hasRoundtripData = true
			break
		}
	}

	if hasRoundtripData {
		// Roundtrip case: use stored metadata to split exactly back
		return splitAudioModulesRoundtrip(patch, issues)
	}

	// Native V2 case: analyze cables to determine what modules to create
	return splitAudioModulesNative(patch, issues)
}

// splitAudioModulesRoundtrip splits merged audio modules using stored metadata.
// Used when a MiRack patch was converted to V2 and is being converted back.
func splitAudioModulesRoundtrip(patch map[string]any, issues *[]string) error {
	modules := getModules(patch)
	if modules == nil {
		return nil
	}

	var newModules []any
	moduleIDToOutputID := make(map[int64]int64)
	moduleIDToInputID := make(map[int64]int64)

	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			newModules = append(newModules, m)
			continue
		}

		mergedData, hasMerged := mod["_mergedAudioModule"].(map[string]any)
		if !hasMerged {
			newModules = append(newModules, m)
			continue
		}

		// This is a merged module, split it back
		// Use key existence to determine which modules existed (handles ID 0 correctly)
		_, hasOutput := mergedData["outputModuleID"]
		_, hasInput := mergedData["inputModuleID"]

		outputID := getInt64FromMap(mergedData, "outputModuleID")
		inputID := getInt64FromMap(mergedData, "inputModuleID")
		mergedID := getInt64FromMap(mod, "id")

		// Determine channel count from model name
		model, _ := mod["model"].(string)
		channelCount := "2"
		if model == "AudioInterface8" {
			channelCount = "8"
		} else if model == "AudioInterface16" {
			channelCount = "16"
		}

		// Create output module (if it existed originally)
		var outputModule map[string]any
		if hasOutput {
			outputModule = make(map[string]any)
			for k, v := range mod {
				if k != "_mergedAudioModule" && k != "model" && k != "pos" {
					outputModule[k] = v
				}
			}
			outputModule["id"] = outputID
			outputModule["model"] = getMiRackAudioOutputModelName(channelCount)
			// Restore original position if stored
			if outputPos, ok := mergedData["outputModulePos"].([]any); ok {
				outputModule["pos"] = outputPos
			} else if pos, ok := mod["pos"].([]any); ok {
				outputModule["pos"] = pos
			}
		}

		// Create input module (if it existed originally)
		// Key existence handles ID 0 correctly - if inputModuleID key exists, module existed
		var inputModule map[string]any
		if hasInput {
			inputModule = make(map[string]any)
			for k, v := range mod {
				if k != "_mergedAudioModule" && k != "model" && k != "id" && k != "pos" {
					inputModule[k] = v
				}
			}
			inputModule["id"] = inputID
			inputModule["model"] = getMiRackAudioInputModelName(channelCount)
			// Restore original position if stored
			if inputPos, ok := mergedData["inputModulePos"].([]any); ok {
				inputModule["pos"] = inputPos
			}
		}

		// Store mappings for cable remapping
		if hasOutput {
			moduleIDToOutputID[mergedID] = outputID
		}
		if hasInput {
			moduleIDToInputID[mergedID] = inputID
		}

		// Add modules based on key existence (consistent pattern)
		if hasOutput {
			newModules = append(newModules, outputModule)
		}
		if hasInput {
			newModules = append(newModules, inputModule)
		}

		// Clean up metadata
		delete(patch, "_audioInputToOutput")
	}

	patch["modules"] = newModules

	// Remove _originalIndexToID since the module indices have changed after splitting
	// DenormalizeV06Style will build a fresh mapping from the new module array
	delete(patch, "_originalIndexToID")

	// Update cable references
	updateCablesForSplit(patch, moduleIDToOutputID, moduleIDToInputID)

	// Rebuild the _idToIndex mapping to reflect the new module array
	// This is needed for DenormalizeV06Style to correctly convert module IDs to array indices
	newIDToIndex := make(map[int64]int)
	for i, m := range newModules {
		if mod, ok := m.(map[string]any); ok {
			// Check if "id" key exists and is not nil (to distinguish "no ID" from "ID is 0")
			if idVal, hasID := mod["id"]; hasID && idVal != nil {
				if id := getInt64FromMap(mod, "id"); id >= 0 {
					newIDToIndex[id] = i
				}
			}
		}
	}
	patch["_idToIndex"] = newIDToIndex

	return nil
}

// detectRequiredChannelCount analyzes cables to determine required audio channel count.
// Checks input and output ports SEPARATELY, returns max rounded up to available sizes.
// Returns "2", "8", "16", or error if exceeds 16 (MiRack limit).
func detectRequiredChannelCount(moduleID int64, patch map[string]any) (string, error) {
	cables, ok := patch["cables"].([]any)
	if !ok {
		return "2", nil // Default to 2-channel if no cables
	}

	maxInputPort := int64(-1)
	maxOutputPort := int64(-1)

	for _, c := range cables {
		cable, ok := c.(map[string]any)
		if !ok {
			continue
		}

		outputModuleID := getInt64FromMap(cable, "outputModuleId")
		inputModuleID := getInt64FromMap(cable, "inputModuleId")

		// Check output port (cable from this module)
		if outputModuleID == moduleID {
			outputPort := getInt64FromMap(cable, "outputId")
			if outputPort > maxOutputPort {
				maxOutputPort = outputPort
			}
		}

		// Check input port (cable to this module)
		if inputModuleID == moduleID {
			inputPort := getInt64FromMap(cable, "inputId")
			if inputPort > maxInputPort {
				maxInputPort = inputPort
			}
		}
	}

	// Determine required channels from max port numbers
	// Port numbering is 0-based: port 0 = channel 1, port 7 = channel 8
	requiredChannels := int64(0)
	if maxOutputPort >= 0 && maxOutputPort > requiredChannels {
		requiredChannels = maxOutputPort + 1
	}
	if maxInputPort >= 0 && maxInputPort > requiredChannels {
		requiredChannels = maxInputPort + 1
	}

	// Default to 2-channel if no cables connected
	if requiredChannels == 0 {
		return "2", nil
	}

	// Round up to available module sizes
	if requiredChannels <= 2 {
		return "2", nil
	} else if requiredChannels <= 8 {
		return "8", nil
	} else if requiredChannels <= 16 {
		return "16", nil
	}

	// Exceeds MiRack's 16-channel limit
	return "", fmt.Errorf("audio requires %d channels, exceeds MiRack's 16-channel limit", requiredChannels)
}

// splitAudioModulesNative splits V2 audio modules based on cable usage analysis.
// Used when converting native V2 patches (not from MiRack originally).
func splitAudioModulesNative(patch map[string]any, issues *[]string) error {
	modules := getModules(patch)
	if modules == nil {
		return nil
	}

	// Analyze each V2 audio module to determine if it needs splitting
	var newModules []any
	moduleIDToOutputID := make(map[int64]int64)
	moduleIDToInputID := make(map[int64]int64)

	for _, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			newModules = append(newModules, m)
			continue
		}

		model, _ := mod["model"].(string)
		// Handle plain "AudioInterface" (for native V2 patches) and explicit channel variants
		if model != "AudioInterface" && model != "AudioInterface2" && model != "AudioInterface8" && model != "AudioInterface16" {
			newModules = append(newModules, m)
			continue
		}

		moduleID := getInt64FromMap(mod, "id")

		// Detect required channel count from cable usage
		channelCount, err := detectRequiredChannelCount(moduleID, patch)
		if err != nil {
			// Store error for later reporting, skip this module
			*issues = append(*issues, fmt.Sprintf("V2 → MiRack: %v", err))
			newModules = append(newModules, m)
			continue
		}

		// This is an audio module, analyze cable usage
		// moduleID already declared above

		hasOutput := false
		hasInput := false
		hasSelfConnection := false

		if cables, ok := patch["cables"].([]any); ok {
			for _, c := range cables {
				cable, ok := c.(map[string]any)
				if !ok {
					continue
				}

				outputID := getInt64FromMap(cable, "outputModuleId")
				inputID := getInt64FromMap(cable, "inputModuleId")

				if outputID == moduleID && inputID == moduleID {
					hasSelfConnection = true
				} else if outputID == moduleID {
					hasOutput = true
				} else if inputID == moduleID {
					hasInput = true
				}
			}
		}

		// Determine channel count
		// channelCount already set by detectRequiredChannelCount above

		// Decide which modules to create based on usage
		createOutput := hasOutput || hasSelfConnection || (!hasInput && !hasOutput)
		createInput := hasInput || hasSelfConnection

		moduleIDx := float64(0)
		moduleIDy := float64(0)
		if pos, ok := mod["pos"].([]any); ok && len(pos) >= 2 {
			if x, ok := pos[0].(float64); ok {
				moduleIDx = x
			}
			if y, ok := pos[1].(float64); ok {
				moduleIDy = y
			}
		}

		// Generate new IDs (use negative IDs to avoid conflicts)
		outputID := moduleID
		inputID := moduleID - 1

		if createOutput && createInput {
			// Split into two modules
			outputModule := make(map[string]any)
			for k, v := range mod {
				outputModule[k] = v
			}
			outputModule["id"] = outputID
			outputModule["model"] = getMiRackAudioOutputModelName(channelCount)
			outputModule["pos"] = []any{moduleIDx, moduleIDy}

			inputModule := make(map[string]any)
			for k, v := range mod {
				inputModule[k] = v
			}
			inputModule["id"] = inputID
			inputModule["model"] = getMiRackAudioInputModelName(channelCount)
			inputModule["pos"] = []any{moduleIDx - 3, moduleIDy}

			moduleIDToOutputID[moduleID] = outputID
			moduleIDToInputID[moduleID] = inputID

			newModules = append(newModules, outputModule)
			newModules = append(newModules, inputModule)

		} else if createOutput {
			// Only output needed
			outputModule := make(map[string]any)
			for k, v := range mod {
				outputModule[k] = v
			}
			outputModule["id"] = outputID
			outputModule["model"] = getMiRackAudioOutputModelName(channelCount)

			moduleIDToOutputID[moduleID] = outputID
			newModules = append(newModules, outputModule)

		} else if createInput {
			// Only input needed
			inputModule := make(map[string]any)
			for k, v := range mod {
				inputModule[k] = v
			}
			inputModule["id"] = inputID
			inputModule["model"] = getMiRackAudioInputModelName(channelCount)

			moduleIDToInputID[moduleID] = inputID
			newModules = append(newModules, inputModule)

		} else {
			// Default: create both
			outputModule := make(map[string]any)
			for k, v := range mod {
				outputModule[k] = v
			}
			outputModule["id"] = outputID
			outputModule["model"] = getMiRackAudioOutputModelName(channelCount)
			outputModule["pos"] = []any{moduleIDx, moduleIDy}

			inputModule := make(map[string]any)
			for k, v := range mod {
				inputModule[k] = v
			}
			inputModule["id"] = inputID
			inputModule["model"] = getMiRackAudioInputModelName(channelCount)
			inputModule["pos"] = []any{moduleIDx - 3, moduleIDy}

			moduleIDToOutputID[moduleID] = outputID
			moduleIDToInputID[moduleID] = inputID

			newModules = append(newModules, outputModule)
			newModules = append(newModules, inputModule)
		}
	}

	if len(newModules) > 0 {
		patch["modules"] = newModules
	}

	// Remove _originalIndexToID since the module indices have changed after splitting
	// DenormalizeV06Style will build a fresh mapping from the new module array
	delete(patch, "_originalIndexToID")

	// Update cable references
	updateCablesForSplit(patch, moduleIDToOutputID, moduleIDToInputID)

	// Rebuild the _idToIndex mapping to reflect the new module array
	// This is needed for DenormalizeV06Style to correctly convert module IDs to array indices
	newIDToIndex := make(map[int64]int)
	for i, m := range newModules {
		if mod, ok := m.(map[string]any); ok {
			// Check if "id" key exists and is not nil (to distinguish "no ID" from "ID is 0")
			if idVal, hasID := mod["id"]; hasID && idVal != nil {
				if id := getInt64FromMap(mod, "id"); id >= 0 {
					newIDToIndex[id] = i
				}
			}
		}
	}
	patch["_idToIndex"] = newIDToIndex

	return nil
}

// updateCablesForSplit updates cable references after splitting audio modules.
// Cables referencing the merged module are redirected to the appropriate split module.
func updateCablesForSplit(patch map[string]any, moduleIDToOutputID, moduleIDToInputID map[int64]int64) {
	cables, ok := patch["cables"].([]any)
	if !ok {
		return
	}

	for _, c := range cables {
		cable, ok := c.(map[string]any)
		if !ok {
			continue
		}

		oldOutputID := getInt64FromMap(cable, "outputModuleId")
		oldInputID := getInt64FromMap(cable, "inputModuleId")

		// Check cable markers
		wasFromInputModule := false
		if val, ok := cable["_cableFromInputModule"]; ok {
			if b, ok := val.(bool); ok {
				wasFromInputModule = b
			}
			delete(cable, "_cableFromInputModule")
		}

		toOutputModule := false
		if val, ok := cable["_cableToOutputModule"]; ok {
			if b, ok := val.(bool); ok {
				toOutputModule = b
			}
			delete(cable, "_cableToOutputModule")
		}

		// Handle self-connecting cables (both ends on the same merged module)
		if oldOutputID == oldInputID {
			if _, outputExists := moduleIDToOutputID[oldOutputID]; outputExists {
				if _, inputExists := moduleIDToInputID[oldInputID]; inputExists {
					// Self-connecting cable: route from INPUT module to OUTPUT module
					cable["outputModuleId"] = moduleIDToInputID[oldInputID]
					cable["inputModuleId"] = moduleIDToOutputID[oldOutputID]
					continue
				}
			}
		}

		// Handle cables from the input module (AudioInterfaceIn output → other modules)
		// These should route from the INPUT module (AudioInterfaceIn), not the OUTPUT module (AudioInterface)
		if wasFromInputModule {
			// Check if this output ID maps to an input module (i.e., was from a merged audio module)
			if inputID, hasMapping := moduleIDToInputID[oldOutputID]; hasMapping {
				cable["outputModuleId"] = inputID
			}
		} else {
			// Regular cables or from output module: update normally
			if newOutputID, exists := moduleIDToOutputID[oldOutputID]; exists {
				cable["outputModuleId"] = newOutputID
			}
		}

		// Handle cables going TO the output module (other modules → AudioInterface input)
		// These should route to the OUTPUT module (AudioInterface), not the INPUT module (AudioInterfaceIn)
		if toOutputModule {
			// Use the output module ID, not the input module ID
			if outputID, hasMapping := moduleIDToOutputID[oldInputID]; hasMapping {
				cable["inputModuleId"] = outputID
			}
		} else {
			// Regular input side update
			if newInputID, exists := moduleIDToInputID[oldInputID]; exists {
				cable["inputModuleId"] = newInputID
			}
		}
	}
}

// ============================================================================
// MiRack Format Handler
// ============================================================================

// MiRackHandler implements FormatHandler for MiRack .mrk bundles.
// MiRack bundles are directories containing patch.vcv (plain JSON, not compressed).
//
// IMPORTANT: MiRack does NOT have a "Fundamental" plugin. All basic modules in MiRack
// use "plugin": "Core" (AudioInterface, VCO-1, VCA-1, etc.). This is a key difference
// from VCV Rack v0.6, which has separate "Fundamental" and "Core" plugins.
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
//
// MiRack-specific behavior:
// - Module name mappings (MiRack → V2 model names)
// - Audio module merging (separate AudioInterface + AudioInterfaceIn → single AudioInterfaceX)
// - NO plugin conversion (all modules already use Core plugin)
// - Array indices → Module IDs for cables
// - wires → cables
// - paramId → id in parameters
// - disabled → bypass
// - colorIndex → hex (for cables)
func NormalizeMiRack(patch map[string]any, issues *[]string) error {
	// Pass 1: Build index-to-ID mapping BEFORE any module modifications
	// This is critical because mergeAudioModules will remove modules from the array,
	// which would cause wire indices to point to wrong modules.
	modules := getModules(patch)
	if modules == nil {
		return fmt.Errorf("no modules found")
	}

	// Build index-to-ID mapping using the ORIGINAL module array
	indexToID := make(map[int]int64)
	nextID := int64(0)
	for i, m := range modules {
		mod, ok := m.(map[string]any)
		if !ok {
			continue
		}

		// Get or assign module ID
		var moduleID int64
		if _, hasID := mod["id"]; hasID {
			moduleID = getInt64FromMap(mod, "id")
			if moduleID < 0 {
				moduleID = nextID
				nextID++
				mod["id"] = moduleID
			} else {
				if moduleID >= nextID {
					nextID = moduleID + 1
				}
			}
		} else {
			moduleID = nextID
			nextID++
			mod["id"] = moduleID
		}
		indexToID[i] = moduleID
	}
	// Store for later use
	patch["_originalIndexToID"] = indexToID

	// Pass 2: Convert wires to cables using the ORIGINAL module array
	// This must happen BEFORE module merging, because merging removes modules
	// and would cause wire indices to become invalid.
	if wires, hasWires := patch["wires"]; hasWires {
		patch["cables"] = wires
		delete(patch, "wires")

		if cables, ok := patch["cables"].([]any); ok {
			validCables := make([]any, 0, len(cables))
			for _, c := range cables {
				cable, ok := c.(map[string]any)
				if !ok {
					continue
				}

				// Get wire indices (these are array indices in the ORIGINAL array)
				outputModuleIdx := getInt64FromMap(cable, "outputModuleId")
				inputModuleIdx := getInt64FromMap(cable, "inputModuleId")

				// Convert array indices to module IDs using ORIGINAL mapping
				outputModuleID, outputExists := indexToID[int(outputModuleIdx)]
				inputModuleID, inputExists := indexToID[int(inputModuleIdx)]

				if !outputExists || !inputExists {
					continue
				}

				cable["outputModuleId"] = outputModuleID
				cable["inputModuleId"] = inputModuleID

				// Store original wire indices for later use in merge
				cable["_originalOutputIdx"] = outputModuleIdx
				cable["_originalInputIdx"] = inputModuleIdx

				// Apply color conversion
				convertMiRackColorIndexToHex(cable, issues)

				validCables = append(validCables, cable)
			}
			patch["cables"] = validCables
		}
	}

	// Pass 3: Merge separate audio input/output modules into single V2 audio module
	// Now works with cables (using module IDs) instead of wires (using indices)
	if err := mergeAudioModules(patch, issues); err != nil {
		return err
	}

	// Pass 4: Apply MiRack → V2 module name mappings (for non-audio modules)
	// Polyphony modules (Merge, Split, Sum) also need plugin → Fundamental
	// Skip audio modules - they're already handled by the merge logic
	if modules, ok := patch["modules"].([]any); ok {
		for _, m := range modules {
			mod, ok := m.(map[string]any)
			if !ok {
				continue
			}
			model, _ := mod["model"].(string)

			// Skip audio modules - they're already merged with correct names
			if isMiRackAudioOutputModule(model) || isMiRackAudioInputModule(model) {
				continue
			}

			if v2Model, exists := miRackToV2ModuleMap[model]; exists {
				mod["model"] = v2Model
				// Polyphony modules are in Fundamental plugin in V2, not Core
				if isPolyphonyModule(v2Model) {
					mod["plugin"] = "Fundamental"
				}
			}
		}
	}

	// Pass 5: Complete remaining V06-style normalization
	// We've already handled wires→cables and module IDs, so we just need:
	// - paramId→id conversion
	// - disabled→bypass conversion
	// - Remove format-specific fields
	if modules, ok := patch["modules"].([]any); ok {
		for i, m := range modules {
			mod, ok := m.(map[string]any)
			if !ok {
				continue
			}
			// Convert paramId to id in parameters
			convertParamIDToID(mod, i, issues)
			// Convert disabled to bypass (v2 format)
			convertDisabledToBypass(mod, issues)
			// Remove format-specific fields not used in v2
			delete(mod, "sumPolyInputs")
		}
	}

	// Store expander links (leftModuleId/rightModuleId) for V2 roundtrip.
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

	// Ensure version is set
	patch["version"] = "2.6.6"

	// Pass 6: Handle Notes module text field conversion
	// MiRack stores notes text in module-level "text" field
	// V2 stores notes text in data.text field
	if modules, ok := patch["modules"].([]any); ok {
		for _, m := range modules {
			mod, ok := m.(map[string]any)
			if !ok {
				continue
			}
			model, _ := mod["model"].(string)
			if model == "Notes" {
				// Move module-level "text" to "data.text" for V2 format
				if text, ok := mod["text"].(string); ok {
					if mod["data"] == nil {
						mod["data"] = make(map[string]any)
					}
					if data, ok := mod["data"].(map[string]any); ok {
						data["text"] = text
					}
					delete(mod, "text")
				}
			}
		}
	}

	return nil
}

// convertMiRackColorIndexToHex converts MiRack colorIndex to hex during normalization.
func convertMiRackColorIndexToHex(cable map[string]any, issues *[]string) {
	// Handle colorIndex field (MiRack-specific)
	if colorIndex, ok := cable["colorIndex"]; ok {
		var idx int
		switch v := colorIndex.(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		}
		// Convert colorIndex to hex
		hexColor := miRackColorIndexToHex(idx)
		cable["color"] = hexColor
		delete(cable, "colorIndex")
		// Don't log - this is too verbose
	}
	// Also handle "color" field if it contains an integer
	if color, ok := cable["color"]; ok {
		switch v := color.(type) {
		case float64:
			cable["color"] = miRackColorIndexToHex(int(v))
		case int:
			cable["color"] = miRackColorIndexToHex(v)
		}
	}
}

// DenormalizeMiRack converts the internal format to MiRack format.
//
// MiRack-specific behavior:
// - Module name mappings (V2 → MiRack model names)
// - Audio module splitting (single AudioInterfaceX → separate AudioInterface + AudioInterfaceInX)
// - NO plugin conversion (all modules stay Core, NOT Fundamental!)
// - Module IDs → Array indices for cables
// - cables → wires
// - bypass → disabled
// - id → paramId in parameters
// - hex → colorIndex (for cables)
func DenormalizeMiRack(patch map[string]any, issues *[]string) error {
	// Pass 1: Split V2's single audio module into MiRack's separate input/output modules
	// This must happen BEFORE V06-style denormalization so cable references are correct
	if err := splitAudioModules(patch, issues); err != nil {
		return err
	}

	// Pass 2: Standard V06-style denormalization
	config := V06StyleConfig{
		FormatName:     "MiRack",
		HasFundamental: false,
		ConvertColor:   convertHexToMiRackColorIndex,
		NormalizePlugin: func(plugin, model string) (string, bool) {
			return plugin, false
		},
		DenormalizePlugin: func(plugin, model string) (string, bool) {
			return plugin, false
		},
	}
	if err := DenormalizeV06Style(patch, config, issues); err != nil {
		return err
	}

	// Pass 3: Apply V2 → MiRack module name mappings
	// Polyphony modules also need plugin → Core (MiRack doesn't have Fundamental)
	if modules, ok := patch["modules"].([]any); ok {
		for _, m := range modules {
			mod, ok := m.(map[string]any)
			if !ok {
				continue
			}
			model, _ := mod["model"].(string)

			if mirackModel, exists := v2ToMiRackModuleMap[model]; exists {
				mod["model"] = mirackModel
				// Polyphony modules are in Core plugin in MiRack, not Fundamental
				if isPolyphonyModule(model) {
					mod["plugin"] = "Core"
				}
			}
		}
	}

	// Pass 4: Handle Notes module text field conversion
	// V2 stores notes text in data.text field
	// MiRack stores notes text in module-level "text" field
	if modules, ok := patch["modules"].([]any); ok {
		for _, m := range modules {
			mod, ok := m.(map[string]any)
			if !ok {
				continue
			}
			model, _ := mod["model"].(string)
			if model == "Notes" {
				// Move "data.text" to module-level "text" for MiRack format
				if data, ok := mod["data"].(map[string]any); ok {
					if text, ok := data["text"].(string); ok {
						mod["text"] = text
						delete(data, "text")
						// Remove data object if it's now empty
						if len(data) == 0 {
							delete(mod, "data")
						}
					}
				}
			}
		}
	}

	// Clean up internal markers from wires/cables
	if wires, ok := patch["wires"].([]any); ok {
		for _, w := range wires {
			wire, ok := w.(map[string]any)
			if !ok {
				continue
			}
			delete(wire, "_fromInputModuleOutput")
			delete(wire, "_fromInputModuleInput")
		}
	}

	return nil
}

// convertHexToMiRackColorIndex converts hex to MiRack colorIndex during denormalization.
func convertHexToMiRackColorIndex(wire map[string]any, issues *[]string) {
	if color, ok := wire["color"].(string); ok {
		// Parse hex to RGB, then find nearest MiRack colorIndex
		r, g, b, ok := hexToRGB(color)
		if ok {
			wire["colorIndex"] = rgbToMiRackColorIndex(r, g, b)
		}
		delete(wire, "color")
	}
}

// DetectMiRackFormat checks if the given path represents a MiRack patch.
// A MiRack patch is identified by:
// 1. The path has .mrk extension (directory bundle), OR
// 2. The path ends with .mrk/patch.vcv (the patch.vcv file inside a .mrk bundle)
func DetectMiRackFormat(path string) bool {
	lowerPath := strings.ToLower(path)

	// Direct .mrk path (directory bundle)
	if strings.HasSuffix(lowerPath, ".mrk") {
		return true
	}

	// patch.vcv inside .mrk bundle (both Unix and Windows path separators)
	if strings.HasSuffix(lowerPath, ".mrk/patch.vcv") || strings.HasSuffix(lowerPath, ".mrk\\patch.vcv") {
		return true
	}

	return false
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
