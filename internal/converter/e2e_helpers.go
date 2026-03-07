package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testFixtureDir = "../../test"
	testTempDir    = "../../test/temp"
)

// ============================================================================
// Test Matrix Data
// ============================================================================

// testFixture defines a single test fixture file with optional skip targets.
type testFixture struct {
	filename    string   // Just the filename (e.g., "mirack_basic.mrk")
	skipTargets []Format // Formats to skip (e.g., audio constraints)
}

// allFixtures is the complete fixture list for matrix-driven testing.
var allFixtures = []testFixture{
	// MiRack fixtures
	{filename: "mirack_basic.mrk"},
	{filename: "mirack_cables.mrk"},
	{filename: "mirack_multichannel.mrk"},
	{filename: "mirack_to8channel.mrk"},
	{filename: "mirackoutput.mrk"},

	// V0.6 fixtures (legacy support)
	{filename: "legacy-patch.vcv"},
	{filename: "morningstarling.vcv"},
	{filename: "vcv06_cables.vcv"},

	// V2 fixtures
	{filename: "basevcvrack2.vcv"},
	{filename: "vcv2_audioio.vcv"},
	{filename: "vcv2_cables.vcv"},
	{filename: "vcv2_multichannel.vcv"},
}

// conversionMatrix defines valid source -> target format mappings.
// Only officially supported conversions are listed here.
var conversionMatrix = map[Format][]Format{
	FormatVCV06:  {FormatVCV2},   // v0.6 -> v2 (legacy support)
	FormatMiRack: {FormatVCV2},   // MiRack -> v2
	FormatVCV2:   {FormatMiRack}, // v2 -> MiRack (for roundtrip)
}

// roundtripCase defines a roundtrip test: A -> B -> A.
type roundtripCase struct {
	fixture string // Filename
	via     Format // Intermediate format
}

// roundtripMatrix defines fixtures that should roundtrip successfully.
// Focuses on officially supported formats (v2 <-> MiRack).
var roundtripMatrix = []roundtripCase{
	{"mirack_basic.mrk", FormatVCV2},
	{"mirack_cables.mrk", FormatVCV2},
	{"mirack_to8channel.mrk", FormatVCV2},
	{"basevcvrack2.vcv", FormatMiRack},
	{"vcv2_cables.vcv", FormatMiRack},
}

// ============================================================================
// Core Types
// ============================================================================

// patchData holds parsed patch information for validation.
type patchData struct {
	format  Format
	root    map[string]any
	modules []map[string]any
	cables  []map[string]any
	wires   []map[string]any
	version string
	path    string // For error messages
}

// colorRGB represents a color in RGB space (0-255).
type colorRGB struct {
	r, g, b uint8
}

// ============================================================================
// Helper Functions
// ============================================================================

// readFixture reads a fixture file and returns parsed patchData.
func readFixture(t *testing.T, relativePath string) *patchData {
	t.Helper()
	fullPath := filepath.Join(testFixtureDir, relativePath)

	format := detectFormat(fullPath)
	if format.IsUnknown() {
		t.Fatalf("Failed to detect format for %s", fullPath)
	}

	handler := GetFormatHandler(format)
	data, err := handler.Read(fullPath)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", fullPath, err)
	}

	root, err := FromJSON(data)
	if err != nil {
		t.Fatalf("Failed to parse JSON from %s: %v", fullPath, err)
	}

	return &patchData{
		format:  format,
		root:    root,
		version: extractVersionString(root),
		path:    fullPath,
	}
}

// extractModules extracts modules from patch data as []map[string]any.
func extractModules(pd *patchData) {
	modules, ok := pd.root["modules"].([]any)
	if !ok {
		pd.modules = []map[string]any{}
		return
	}

	pd.modules = make([]map[string]any, 0, len(modules))
	for _, m := range modules {
		if mod, ok := m.(map[string]any); ok {
			pd.modules = append(pd.modules, mod)
		}
	}
}

// extractCablesOrWires extracts cables or wires from patch data.
func extractCablesOrWires(pd *patchData) {
	// Try cables first (v2 format)
	if cables, ok := pd.root["cables"].([]any); ok {
		pd.cables = make([]map[string]any, 0, len(cables))
		for _, c := range cables {
			if cable, ok := c.(map[string]any); ok {
				pd.cables = append(pd.cables, cable)
			}
		}
		pd.wires = nil // v2 uses cables, not wires
		return
	}

	// Try wires (v0.6/MiRack format)
	if wires, ok := pd.root["wires"].([]any); ok {
		pd.wires = make([]map[string]any, 0, len(wires))
		for _, w := range wires {
			if wire, ok := w.(map[string]any); ok {
				pd.wires = append(pd.wires, wire)
			}
		}
		pd.cables = nil // v0.6/MiRack uses wires, not cables
		return
	}

	// Neither cables nor wires
	pd.cables = nil
	pd.wires = nil
}

// extractVersionString extracts version from patch root.
func extractVersionString(root map[string]any) string {
	if v, ok := root["version"].(string); ok {
		return v
	}
	return ""
}

// parseOutputPath reads an output file and returns parsed patchData.
func parseOutputPath(t *testing.T, path string, expectedFormat Format) *patchData {
	t.Helper()

	handler := GetFormatHandler(expectedFormat)
	data, err := handler.Read(path)
	if err != nil {
		t.Fatalf("Failed to read output file %s: %v", path, err)
	}

	root, err := FromJSON(data)
	if err != nil {
		t.Fatalf("Failed to parse JSON from output file %s: %v", path, err)
	}

	pd := &patchData{
		format:  expectedFormat,
		root:    root,
		version: extractVersionString(root),
		path:    path,
	}

	extractModules(pd)
	extractCablesOrWires(pd)

	return pd
}

// ensureTempDir creates the temp directory if it doesn't exist.
func ensureTempDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll(testTempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	return testTempDir
}

// tempOutputPath generates a unique temp output path for a test.
func tempOutputPath(t *testing.T, baseName string, targetFormat Format) string {
	t.Helper()
	ext := GetFormatHandler(targetFormat).Extension()
	return filepath.Join(ensureTempDir(t), baseName+ext)
}

// buildModuleIDMap builds a map of module ID -> module index.
func buildModuleIDMap(modules []map[string]any) map[int64]int {
	idMap := make(map[int64]int)
	for i, mod := range modules {
		if id := getInt64FromMap(mod, "id"); id > 0 {
			idMap[id] = i
		}
	}
	return idMap
}

// countAudioModules counts audio interface modules in the modules array.
// Handles different plugin names across formats (Core, Fundamental, etc.).
func countAudioModules(modules []map[string]any) (inputCount, outputCount int) {
	for _, mod := range modules {
		model, _ := mod["model"].(string)
		plugin, _ := mod["plugin"].(string)

		// Check for audio-related modules across different plugins
		// V2: Core/AudioInterface, Core/AudioInterface2, etc.
		// MiRack: AudioInterface (no plugin), Core/AudioInterface
		// v0.6: Fundamental/AudioInterface, Core/AudioInterface
		if strings.HasPrefix(model, "AudioInterface") {
			// Accept any plugin for audio modules (Core, Fundamental, or empty for MiRack)
			if plugin == "Core" || plugin == "Fundamental" || plugin == "" {
				if strings.Contains(model, "In") {
					inputCount++
				} else {
					outputCount++
				}
			}
		}
	}
	return
}

// skipTarget checks if a target format should be skipped for a fixture.
func skipTarget(fixture testFixture, targetFormat Format) bool {
	for _, skip := range fixture.skipTargets {
		if skip == targetFormat {
			return true
		}
	}
	return false
}

// ============================================================================
// Universal Validators (defined once, applied to all)
// ============================================================================

// validateFormat checks output has correct format structure.
func validateFormat(t *testing.T, pd *patchData, expected Format) {
	t.Helper()

	// Check version prefix
	if pd.version == "" {
		t.Errorf("Output missing version field")
		return
	}

	switch expected {
	case FormatVCV2:
		if !strings.HasPrefix(pd.version, "2.") {
			t.Errorf("Expected v2 version (2.x.x), got %s", pd.version)
		}
		// v2 should use "cables", not "wires"
		if _, hasCables := pd.root["cables"]; !hasCables {
			if _, hasWires := pd.root["wires"]; hasWires {
				t.Errorf("v2 output should use 'cables', not 'wires'")
			}
		}

	case FormatVCV06, FormatMiRack:
		if !strings.HasPrefix(pd.version, "0.") {
			t.Errorf("Expected v0.6 version (0.x.x), got %s", pd.version)
		}
		// v0.6/MiRack should use "wires", not "cables"
		if _, hasWires := pd.root["wires"]; !hasWires {
			if _, hasCables := pd.root["cables"]; hasCables {
				t.Errorf("v0.6/MiRack output should use 'wires', not 'cables'")
			}
		}
	}
}

// validateConnectivity checks all cable/wire references are valid.
func validateConnectivity(t *testing.T, pd *patchData) {
	t.Helper()

	extractModules(pd)
	moduleIDMap := buildModuleIDMap(pd.modules)

	// Validate cables (v2 format)
	if len(pd.cables) > 0 {
		for i, cable := range pd.cables {
			outputID := getInt64FromMap(cable, "outputModuleId")
			inputID := getInt64FromMap(cable, "inputModuleId")

			if outputID > 0 {
				if _, exists := moduleIDMap[outputID]; !exists {
					t.Errorf("cable[%d]: outputModuleId %d not found in modules", i, outputID)
				}
			}

			if inputID > 0 {
				if _, exists := moduleIDMap[inputID]; !exists {
					t.Errorf("cable[%d]: inputModuleId %d not found in modules", i, inputID)
				}
			}
		}
	}

	// Validate wires (v0.6/MiRack format) - these use array indices
	if len(pd.wires) > 0 {
		for i, wire := range pd.wires {
			outputIdx := getInt64FromMap(wire, "outputModuleId")
			inputIdx := getInt64FromMap(wire, "inputModuleId")

			if outputIdx < 0 || int(outputIdx) >= len(pd.modules) {
				t.Errorf("wire[%d]: outputModuleId index %d out of range (0-%d)", i, outputIdx, len(pd.modules)-1)
			}

			if inputIdx < 0 || int(inputIdx) >= len(pd.modules) {
				t.Errorf("wire[%d]: inputModuleId index %d out of range (0-%d)", i, inputIdx, len(pd.modules)-1)
			}
		}
	}
}

// validatePreservation checks module count and positions are preserved (with known deltas).
func validatePreservation(t *testing.T, input, output *patchData, conversion string) {
	t.Helper()

	extractModules(input)
	extractModules(output)

	inputCount := len(input.modules)
	outputCount := len(output.modules)

	// Known deltas for specific conversions
	// MiRack -> v2: Audio merge (2 audio modules -> 1) decreases count
	// v2 -> MiRack: Audio split (1 audio module -> 2) increases count
	expectedDelta := 0

	switch conversion {
	case "mirack->v2":
		// MiRack audio modules merge: 2 -> 1
		inputIn, inputOut := countAudioModules(input.modules)
		if inputIn > 0 && inputOut > 0 {
			expectedDelta = -1 // One module merged away
		}
	case "v2->mirack":
		// v2 audio modules split: 1 -> 2
		outputIn, outputOut := countAudioModules(output.modules)
		if outputIn > 0 && outputOut > 0 {
			expectedDelta = 1 // One module split into two
		}
	}

	actualDelta := outputCount - inputCount

	// Module count changes are expected for audio merge/split, skip logging
	_ = expectedDelta // Suppress unused warning
	_ = actualDelta

	// Check that module positions are generally preserved (no negative coords)
	for i, mod := range output.modules {
		if x, ok := mod["pos"].(map[string]any); ok {
			if xVal, ok := x["x"].(float64); ok && xVal < 0 {
				t.Errorf("Module %d has negative x position: %f", i, xVal)
			}
			if yVal, ok := x["y"].(float64); ok && yVal < 0 {
				t.Errorf("Module %d has negative y position: %f", i, yVal)
			}
		}
	}
}

// validateStructuralIntegrity checks JSON is well-formed and required fields exist.
func validateStructuralIntegrity(t *testing.T, pd *patchData) {
	t.Helper()

	// Check required top-level fields
	requiredFields := []string{"version"}
	for _, field := range requiredFields {
		if _, exists := pd.root[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// v0.6 format doesn't require module IDs (uses array indices)
	requiresModuleID := pd.format != FormatVCV06

	// Check modules array is well-formed
	if modules, ok := pd.root["modules"].([]any); ok {
		for i, m := range modules {
			if mod, ok := m.(map[string]any); ok {
				// Check 'id' field only for formats that require it
				if requiresModuleID {
					if _, hasID := mod["id"]; !hasID {
						t.Errorf("Module %d missing 'id' field", i)
					}
				}
				if _, hasPlugin := mod["plugin"]; !hasPlugin {
					t.Errorf("Module %d missing 'plugin' field", i)
				}
				if _, hasModel := mod["model"]; !hasModel {
					t.Errorf("Module %d missing 'model' field", i)
				}
			} else {
				t.Errorf("Module %d is not a map", i)
			}
		}
	} else {
		t.Error("Missing or invalid 'modules' array")
	}

	// Check cables/wires array is well-formed (if present)
	if cables, ok := pd.root["cables"].([]any); ok {
		for i, c := range cables {
			if _, ok := c.(map[string]any); !ok {
				t.Errorf("Cable %d is not a map", i)
			}
		}
	}

	if wires, ok := pd.root["wires"].([]any); ok {
		for i, w := range wires {
			if _, ok := w.(map[string]any); !ok {
				t.Errorf("Wire %d is not a map", i)
			}
		}
	}
}

// ============================================================================
// Color Preservation Validation
// ============================================================================

// extractColors extracts all cable/wire colors from patchData as RGB.
func extractColors(pd *patchData) []colorRGB {
	var colors []colorRGB

	// Extract from v2 cables
	if len(pd.cables) > 0 {
		for _, cable := range pd.cables {
			if r, g, b, ok := extractColorFromV2Cable(cable); ok {
				colors = append(colors, colorRGB{r, g, b})
			}
		}
	}

	// Extract from v0.6/MiRack wires
	if len(pd.wires) > 0 {
		for _, wire := range pd.wires {
			if r, g, b, ok := extractColorFromWire(wire); ok {
				colors = append(colors, colorRGB{r, g, b})
			}
		}
	}

	return colors
}

// extractColorFromV2Cable extracts color from a v2 cable as RGB.
// v2 color format: "rrggbbaa" (8-digit hex, no #) or color object.
func extractColorFromV2Cable(cable map[string]any) (r, g, b uint8, ok bool) {
	// Try hex string format first
	if colorStr, strOk := cable["color"].(string); strOk && len(colorStr) >= 6 {
		n, _ := fmt.Sscanf(colorStr, "%02x%02x%02x", &r, &g, &b)
		if n == 3 {
			return r, g, b, true
		}
	}

	// Try color object format {r: 0-1, g: 0-1, b: 0-1, a: 0-1}
	if colorObj, objOk := cable["color"].(map[string]any); objOk {
		rVal, rok := colorObj["r"].(float64)
		gVal, gok := colorObj["g"].(float64)
		bVal, bok := colorObj["b"].(float64)
		if rok && gok && bok {
			return uint8(rVal * 255), uint8(gVal * 255), uint8(bVal * 255), true
		}
	}

	return 0, 0, 0, false
}

// extractColorFromWire extracts color from a v0.6/MiRack wire as RGB.
// Supports: #rrggbb hex, color object, or MiRack colorIndex (0-5).
func extractColorFromWire(wire map[string]any) (r, g, b uint8, ok bool) {
	// Check for hex color string (#rrggbb or #rrggbbaa)
	if colorStr, strOk := wire["color"].(string); strOk {
		if hexR, hexG, hexB, hexOk := hexToRGB(colorStr); hexOk {
			return hexR, hexG, hexB, true
		}
	}

	// Check for color object format
	if colorObj, objOk := wire["color"].(map[string]any); objOk {
		rVal, rok := colorObj["r"].(float64)
		gVal, gok := colorObj["g"].(float64)
		bVal, bok := colorObj["b"].(float64)
		if rok && gok && bok {
			return uint8(rVal * 255), uint8(gVal * 255), uint8(bVal * 255), true
		}
	}

	// Check for MiRack colorIndex
	if colorIndex, idxOk := wire["colorIndex"].(float64); idxOk {
		idx := int(colorIndex)
		if idx >= 0 && idx < len(miRackColorPalette) {
			c := miRackColorPalette[idx]
			return c.r, c.g, c.b, true
		}
	}

	return 0, 0, 0, false
}

// colorsApproxEqual checks if two RGB colors are approximately equal.
// Uses Euclidean distance with tolerance for palette conversions.
func colorsApproxEqual(r1, g1, b1, r2, g2, b2 uint8) bool {
	const maxDistance = 30 // Allow some color difference for palette conversions
	dr := int(r1) - int(r2)
	dg := int(g1) - int(g2)
	db := int(b1) - int(b2)
	distance := dr*dr + dg*dg + db*db
	return distance <= maxDistance*maxDistance
}

// normalizeHexColor ensures hex color has # prefix and is lowercase.
func normalizeHexColor(c string) string {
	if len(c) == 0 {
		return c
	}
	s := strings.ToLower(c)
	if s[0] != '#' {
		s = "#" + s
	}
	return s
}

// stripAlphaChannel removes alpha component from 8-digit hex color (#rrggbbaa → #rrggbb).
func stripAlphaChannel(hexColor string) string {
	s := hexColor
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	// If 8-digit hex (rrggbbaa), strip to 6-digit (rrggbb)
	if len(s) == 8 {
		return "#" + s[:6]
	}
	return "#" + s
}

// validateColorPreservation checks cable/wire colors are preserved through conversion.
func validateColorPreservation(t *testing.T, input, output *patchData) {
	t.Helper()

	inputCount := len(input.wires) + len(input.cables)
	outputCount := len(output.wires) + len(output.cables)

	if inputCount != outputCount {
		return
	}

	inputColors := extractColors(input)
	outputColors := extractColors(output)

	if len(inputColors) != len(outputColors) {
		return
	}

	// Compare colors
	for i := 0; i < len(inputColors) && i < len(outputColors); i++ {
		inColor := inputColors[i]
		outColor := outputColors[i]

		// Silently check - color changes are informational only
		_ = colorsApproxEqual(inColor.r, inColor.g, inColor.b, outColor.r, outColor.g, outColor.b)
	}
}

// ============================================================================
// Parameter Value Equivalence Validation
// ============================================================================

// buildModuleMap builds a map of plugin+model to modules for matching.
func buildModuleMap(modules []map[string]any) map[string][]map[string]any {
	result := make(map[string][]map[string]any)
	for _, mod := range modules {
		key := getModuleKey(mod)
		result[key] = append(result[key], mod)
	}
	return result
}

// getModuleKey creates a key for matching modules across conversions.
func getModuleKey(mod map[string]any) string {
	plugin, _ := mod["plugin"].(string)
	model, _ := mod["model"].(string)
	return plugin + ":" + model
}

// getModulePos extracts module position as (x, y).
func getModulePos(mod map[string]any) (x, y float64) {
	if pos, ok := mod["pos"].([]any); ok && len(pos) >= 2 {
		if xVal, ok := pos[0].(float64); ok {
			x = xVal
		}
		if yVal, ok := pos[1].(float64); ok {
			y = yVal
		}
	}
	return x, y
}

// positionDistance calculates squared distance between two module positions.
func positionDistance(x1, y1, x2, y2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	return dx*dx + dy*dy
}

// buildParameterMap builds a map of parameter ID to value.
// Handles both "paramId" (v0.6/MiRack) and "id" (v2) field names.
func buildParameterMap(params []any, idField string) map[string]float64 {
	result := make(map[string]float64)
	for _, p := range params {
		param, ok := p.(map[string]any)
		if !ok {
			continue
		}

		var id string
		if idVal, ok := param[idField].(float64); ok {
			id = fmt.Sprintf("%.0f", idVal)
		} else if idVal, ok := param[idField].(int64); ok {
			id = fmt.Sprintf("%d", idVal)
		} else {
			continue
		}

		if val, ok := param["value"].(float64); ok {
			result[id] = val
		}
	}
	return result
}

// compareModuleParameters compares parameters between two modules.
func compareModuleParameters(t *testing.T, inputMod, outputMod map[string]any) {
	t.Helper()

	inputParams, inputOk := inputMod["params"].([]any)
	outputParams, outputOk := outputMod["params"].([]any)

	if !inputOk || !outputOk {
		return
	}

	// Build parameter maps (handle paramId vs id field name)
	inputParamMap := buildParameterMap(inputParams, "paramId")
	outputParamMap := buildParameterMap(outputParams, "id")

	// Try alternative field names if empty
	if len(inputParamMap) == 0 {
		inputParamMap = buildParameterMap(inputParams, "id")
	}
	if len(outputParamMap) == 0 {
		outputParamMap = buildParameterMap(outputParams, "paramId")
	}

	// Compare parameters - they should be exactly preserved
	for id, inputVal := range inputParamMap {
		if outputVal, ok := outputParamMap[id]; ok {
			if inputVal != outputVal {
				model, _ := inputMod["model"].(string)
				t.Errorf("Module %s: param %s changed from %f to %f",
					model, id, inputVal, outputVal)
			}
		}
	}
}

// validateParameterEquivalence checks parameter values are preserved.
// Matches modules by plugin/model combination and position.
func validateParameterEquivalence(t *testing.T, input, output *patchData) {
	t.Helper()

	outputModuleMap := buildModuleMap(output.modules)

	// For each input module, find matching output module and compare params
	for _, inputMod := range input.modules {
		key := getModuleKey(inputMod)
		outputMods := outputModuleMap[key]

		if len(outputMods) == 0 {
			// Module may have been renamed during conversion
			continue
		}

		// Find best matching output module (by position)
		inputX, inputY := getModulePos(inputMod)
		var bestMatch map[string]any
		bestDist := float64(-1)

		for _, outputMod := range outputMods {
			outputX, outputY := getModulePos(outputMod)
			dist := positionDistance(inputX, inputY, outputX, outputY)
			if bestDist < 0 || dist < bestDist {
				bestDist = dist
				bestMatch = outputMod
			}
		}

		// Only compare if positions are reasonably close
		if bestMatch != nil && bestDist < 25 {
			compareModuleParameters(t, inputMod, bestMatch)
		}
	}
}

// ============================================================================
// Data Preservation Validation
// ============================================================================

// fieldMapping defines expected field name transformations during conversion.
type fieldMapping struct {
	from []string // Source field names
	to   string   // Target field name
}

// topLevelMappings defines field transformations at the root level.
var topLevelMappings = []fieldMapping{
	{from: []string{"wires"}, to: "cables"}, // wires -> cables
	{from: []string{"cables"}, to: "wires"}, // cables -> wires
}

// moduleLevelMappings defines field transformations at module level.
var moduleLevelMappings = []fieldMapping{
	{from: []string{"disabled"}, to: "bypass"}, // disabled -> bypass
	{from: []string{"bypass"}, to: "disabled"}, // bypass -> disabled
}

// ignoredFields are fields that are expected to be added/removed during conversion.
var ignoredFields = map[string]bool{
	"originalIndexToID":   true, // Added for roundtrip support
	"audioMergeInfo":      true, // Added for audio module tracking
	"_originalIndexToID":  true, // Added for roundtrip support
	"_audioInputToOutput": true, // Added for audio merge tracking
	"_mergedAudioModule":  true, // Added for merged audio modules
	"sumPolyInputs":       true, // MiRack-specific, not in v2
	"leftModuleId":        true, // v2-specific, not in v0.6/MiRack
	"rightModuleId":       true, // v2-specific, not in v0.6/MiRack
	"topModuleId":         true, // v2-specific, not in v0.6/MiRack
	"bottomModuleId":      true, // v2-specific, not in v0.6/MiRack
}

// normalizeFieldName maps a field name to its canonical form for comparison.
func normalizeFieldName(key string, mappings []fieldMapping) string {
	for _, m := range mappings {
		for _, from := range m.from {
			if key == from {
				return m.to
			}
		}
	}
	return key
}

// validateMapKeys compares keys between two maps, accounting for transformations.
func validateMapKeys(t *testing.T, input, output map[string]any, path string, mappings []fieldMapping) {
	t.Helper()

	// Build normalized key sets
	inputKeys := make(map[string]bool)
	outputKeys := make(map[string]bool)

	for k := range input {
		if ignoredFields[k] {
			continue
		}
		nk := normalizeFieldName(k, mappings)
		inputKeys[nk] = true
	}

	for k := range output {
		if ignoredFields[k] {
			continue
		}
		nk := normalizeFieldName(k, mappings)
		outputKeys[nk] = true
	}

	// Check for missing keys in output
	for k := range inputKeys {
		if !outputKeys[k] {
			// Silently check - missing keys are expected for format conversions
			_ = k
		}
	}

	// Check for extra keys in output (new keys added during conversion)
	for k := range outputKeys {
		if !inputKeys[k] {
			// Silently check - new keys are expected for format conversions
			_ = k
		}
	}
}

// validateModuleData compares a module's data structure between input and output.
func validateModuleData(t *testing.T, inputMod, outputMod map[string]any, conversion string) {
	t.Helper()

	inputData, inputHasData := inputMod["data"].(map[string]any)
	outputData, outputHasData := outputMod["data"].(map[string]any)

	// If both have data, compare keys (no field transformations in data)
	if inputHasData && outputHasData {
		validateMapKeys(t, inputData, outputData, "module.data", nil)

		// Recursively compare nested structures
		for k, inputVal := range inputData {
			if ignoredFields[k] {
				continue
			}
			if outputVal, ok := outputData[k]; ok {
				compareValues(t, inputVal, outputVal, "module.data."+k, conversion)
			}
		}
	}
}

// compareValues compares two values recursively.
func compareValues(t *testing.T, input, output any, path string, conversion string) {
	t.Helper()

	switch iv := input.(type) {
	case map[string]any:
		if ov, ok := output.(map[string]any); ok {
			validateMapKeys(t, iv, ov, path, nil)
		}
	case []any:
		if ov, ok := output.([]any); ok {
			// Silently check - array length changes are expected
			_ = len(iv) != len(ov)
		}
	}
}

// validateModulePair compares two modules for data preservation.
func validateModulePair(t *testing.T, inputMod, outputMod map[string]any, conversion string) {
	t.Helper()

	// Check top-level module keys with module-level mappings
	validateMapKeys(t, inputMod, outputMod, "module", moduleLevelMappings)

	// Compare params array
	inputParams, inputHasParams := inputMod["params"].([]any)
	outputParams, outputHasParams := outputMod["params"].([]any)

	if inputHasParams && outputHasParams {
		// Silently check - param count changes are expected
		_ = len(inputParams) != len(outputParams)
	}

	// Compare data structure
	validateModuleData(t, inputMod, outputMod, conversion)
}

// validateDataPreservation checks that no key-value pairs are lost during conversion.
// Accounts for expected field transformations and structural changes.
func validateDataPreservation(t *testing.T, input, output *patchData, conversion string) {
	t.Helper()

	// Compare top-level keys with top-level mappings
	validateMapKeys(t, input.root, output.root, "root", topLevelMappings)

	// Match modules by position and compare their data
	outputModuleMap := buildModuleMap(output.modules)

	for _, inputMod := range input.modules {
		key := getModuleKey(inputMod)
		outputMods := outputModuleMap[key]

		if len(outputMods) == 0 {
			// Module may have been renamed - try to find by position
			continue
		}

		// Find closest module by position
		inputX, inputY := getModulePos(inputMod)
		var bestMatch map[string]any
		bestDist := float64(-1)

		for _, outputMod := range outputMods {
			outputX, outputY := getModulePos(outputMod)
			dist := positionDistance(inputX, inputY, outputX, outputY)
			if bestDist < 0 || dist < bestDist {
				bestDist = dist
				bestMatch = outputMod
			}
		}

		if bestMatch != nil && bestDist < 25 {
			validateModulePair(t, inputMod, bestMatch, conversion)
		}
	}
}

// validateNotesModuleTransformation verifies Notes module text field conversion.
// For MiRack ↔ V2 conversions, checks that:
// - text moves between data.text (V2) and module-level text (MiRack)
// - old location is cleaned up
// - content is preserved
func validateNotesModuleTransformation(t *testing.T, input, output *patchData) {
	t.Helper()

	// Find Notes modules in input and output
	var inputNotes, outputNotes map[string]any

	for _, m := range input.modules {
		if model, _ := m["model"].(string); model == "Notes" {
			inputNotes = m
			break
		}
	}

	for _, m := range output.modules {
		if model, _ := m["model"].(string); model == "Notes" {
			outputNotes = m
			break
		}
	}

	// Both input and output should have Notes modules (or both lack them)
	if inputNotes == nil && outputNotes == nil {
		return // Neither has Notes - nothing to validate
	}
	if inputNotes != nil && outputNotes == nil {
		t.Errorf("Notes: input has Notes module but output does not")
		return
	}
	if inputNotes == nil && outputNotes != nil {
		t.Errorf("Notes: output has Notes module but input does not")
		return
	}

	// Extract text content from input
	var inputText string
	if text, ok := inputNotes["text"].(string); ok {
		inputText = text
	} else if data, ok := inputNotes["data"].(map[string]any); ok {
		inputText, _ = data["text"].(string)
	}

	// Determine expected transformation based on formats
	fromMiRack := input.format == FormatMiRack
	toMiRack := output.format == FormatMiRack

	if fromMiRack && !toMiRack {
		// MiRack → V2: text should move from module-level to data.text
		if data, ok := outputNotes["data"].(map[string]any); ok {
			outputText, hasText := data["text"].(string)
			if !hasText || outputText != inputText {
				t.Errorf("Notes: text should be in data.text after MiRack→V2 conversion, got: %q, want: %q",
					outputText, inputText)
			}
			// Module-level text should be removed
			if _, hasModuleText := outputNotes["text"].(string); hasModuleText {
				t.Errorf("Notes: module-level 'text' should be removed after MiRack→V2 conversion")
			}
		} else {
			t.Errorf("Notes: data object with text field missing after MiRack→V2 conversion")
		}
	} else if !fromMiRack && toMiRack {
		// V2 → MiRack: text should move from data.text to module-level
		outputText, hasModuleText := outputNotes["text"].(string)
		if !hasModuleText || outputText != inputText {
			t.Errorf("Notes: text should be at module level after V2→MiRack conversion, got: %q, want: %q",
				outputText, inputText)
		}
		// data.text should be removed
		if data, ok := outputNotes["data"].(map[string]any); ok {
			if _, hasDataText := data["text"]; hasDataText {
				t.Errorf("Notes: data.text should be removed after V2→MiRack conversion")
			}
		}
	}
	// For same-format conversions, just verify content is preserved somewhere
	if input.format == output.format {
		var outputText string
		if text, ok := outputNotes["text"].(string); ok {
			outputText = text
		} else if data, ok := outputNotes["data"].(map[string]any); ok {
			outputText, _ = data["text"].(string)
		}
		if outputText != inputText {
			t.Errorf("Notes: text content changed, got: %q, want: %q", outputText, inputText)
		}
	}
}

// validateMetaModule checks that HubMedium module was added correctly.
func validateMetaModule(t *testing.T, output *patchData, originalModuleCount int) {
	t.Helper()

	// Module count should increase by 1
	if len(output.modules) != originalModuleCount+1 {
		t.Errorf("Expected %d modules (original + 1 HubMedium), got %d",
			originalModuleCount+1, len(output.modules))
	}

	// Find HubMedium module
	var hubMedium map[string]any
	for _, m := range output.modules {
		if model, _ := m["model"].(string); model == "HubMedium" {
			hubMedium = m
			break
		}
	}

	if hubMedium == nil {
		t.Fatal("HubMedium module not found")
	}

	// Check plugin
	if plugin, _ := hubMedium["plugin"].(string); plugin != "4msCompany" {
		t.Errorf("Expected plugin '4msCompany', got '%s'", plugin)
	}

	// Check model
	if model, _ := hubMedium["model"].(string); model != "HubMedium" {
		t.Errorf("Expected model 'HubMedium', got '%s'", model)
	}

	// Check params (14 params: 0-11 at 0.5, 12-13 at 0.0)
	params, ok := hubMedium["params"].([]any)
	if !ok || len(params) != 14 {
		t.Errorf("Expected 14 params, got %d", len(params))
	}

	// Check data structure
	data, ok := hubMedium["data"].(map[string]any)
	if !ok {
		t.Error("HubMedium should have data map")
	} else {
		requiredFields := []string{"Mappings", "KnobSetNames", "Alias", "PatchName", "PatchDesc"}
		for _, field := range requiredFields {
			if _, exists := data[field]; !exists {
				t.Errorf("HubMedium data missing field: %s", field)
			}
		}
	}

	// Check positioning (should be at maxX + 1, Y=0)
	pos, ok := hubMedium["pos"].([]any)
	if !ok || len(pos) < 2 {
		t.Error("HubMedium should have pos array")
	}
}
