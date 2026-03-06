package converter

import (
	"fmt"
	"os"
	"path/filepath"
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
// - NO plugin conversion (all modules already use Core plugin)
// - Array indices → Module IDs for cables
// - wires → cables
// - paramId → id in parameters
// - disabled → bypass
// - colorIndex → hex (for cables)
func NormalizeMiRack(patch map[string]any, issues *[]string) error {
	config := V06StyleConfig{
		FormatName:     "MiRack",
		HasFundamental: false, // MiRack does NOT have Fundamental plugin
		ConvertColor:   convertMiRackColorIndexToHex,
		NormalizePlugin: func(plugin, model string) (string, bool) {
			return plugin, false // No conversion needed
		},
		DenormalizePlugin: func(plugin, model string) (string, bool) {
			return plugin, false // No conversion needed
		},
	}
	return NormalizeV06Style(patch, config, issues)
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
// - NO plugin conversion (all modules stay Core, NOT Fundamental!)
// - Module IDs → Array indices for cables
// - cables → wires
// - bypass → disabled
// - id → paramId in parameters
// - hex → colorIndex (for cables)
func DenormalizeMiRack(patch map[string]any, issues *[]string) error {
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
	return DenormalizeV06Style(patch, config, issues)
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
