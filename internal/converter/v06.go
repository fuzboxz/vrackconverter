package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// V06Handler implements FormatHandler for VCV Rack v0.6 format.
// V0.6 files can be either plain JSON or zstd tar archives with version "0.x.x".
//
// Key differences from v2:
// - Uses "wires" instead of "cables"
// - Cable references use array indices, not module IDs
// - Has separate "Fundamental" and "Core" plugins
// - Uses "paramId" instead of "id" in parameters
// - Uses "disabled" instead of "bypass"
//
// Key differences from MiRack:
// - File container can be plain JSON or zstd tar archive, not directory bundle
// - HAS "Fundamental" plugin (MiRack does NOT have Fundamental!)
// - Otherwise similar format (wires, array indices, etc.)
type V06Handler struct{}

// Read reads a v0.6 patch.
// V0.6 files can be either plain JSON or zstd tar archives.
// Try plain JSON first, fall back to zst archive extraction.
func (h *V06Handler) Read(path string) ([]byte, error) {
	// First, try reading as plain JSON
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if it's valid JSON
	var js map[string]any
	if err := json.Unmarshal(data, &js); err == nil {
		// Valid JSON, use it directly
		return data, nil
	}

	// Not valid JSON, try as zst archive
	return ExtractJSONFromV2(path)
}

// Write writes a v0.6 patch as plain JSON.
// Plain JSON is more compatible with older v0.6 versions than zst archives.
func (h *V06Handler) Write(data []byte, path string) error {
	return os.WriteFile(path, data, 0644)
}

// Extension returns ".vcv"
func (h *V06Handler) Extension() string {
	return ".vcv"
}

// NormalizeV06 converts v0.6 format to internal (v2) format.
//
// v0.6-specific behavior:
// - Fundamental → Core (preserve roundtrip info via fundamentalModules map)
// - Array indices → Module IDs for cables
// - wires → cables
// - paramId → id in parameters
// - disabled → bypass
func NormalizeV06(patch map[string]any, issues *[]string) error {
	config := V06StyleConfig{
		FormatName:        "v0.6",
		HasFundamental:    true, // v0.6 has Fundamental plugin
		ConvertColor:      nil,  // v0.6 uses hex, no conversion needed
		NormalizePlugin:   normalizeV06Plugin,
		DenormalizePlugin: denormalizeV06Plugin,
	}
	if err := NormalizeV06Style(patch, config, issues); err != nil {
		return err
	}

	// AudioInterface model name is compatible with V2 (no change needed)
	normalizeV06AudioModules(patch, issues)
	return nil
}

// normalizeV06Plugin converts Fundamental → Core for v0.6 → v2 conversion.
func normalizeV06Plugin(plugin, model string) (string, bool) {
	if plugin == "Fundamental" {
		return "Core", true
	}
	return plugin, false
}

// denormalizeV06Plugin converts Core → Fundamental for v2 → v0.6 conversion.
// Only modules that were originally in Fundamental plugin are converted back.
func denormalizeV06Plugin(plugin, model string) (string, bool) {
	if plugin == "Core" && fundamentalModules[model] {
		return "Fundamental", true
	}
	return plugin, false
}

// DenormalizeV06 converts internal (v2) format to v0.6 format.
//
// v0.6-specific behavior:
// - Core → Fundamental (restore original plugin for fundamentalModules)
// - Module IDs → Array indices for cables
// - cables → wires
// - bypass → disabled
// - id → paramId in parameters
func DenormalizeV06(patch map[string]any, issues *[]string) error {
	config := V06StyleConfig{
		FormatName:        "v0.6",
		HasFundamental:    true,
		ConvertColor:      nil,
		NormalizePlugin:   normalizeV06Plugin,
		DenormalizePlugin: denormalizeV06Plugin,
	}
	return DenormalizeV06Style(patch, config, issues)
}

// normalizeV06AudioModules keeps AudioInterface as-is.
// V0.6 AudioInterface is compatible with V2's plain AudioInterface model.
func normalizeV06AudioModules(patch map[string]any, issues *[]string) {
	// No change needed - AudioInterface model name is the same in both formats
	// The plugin conversion (Fundamental/Core → Core) is handled elsewhere
}

// DetectV06Format checks if the given path represents a VCV Rack v0.6 patch.
// A v0.6 patch is identified by:
// 1. The path has .vcv extension, AND
// 2. The file is NOT a MiRack bundle, AND
// 3. The file contains plain JSON with version "0.x.x" or is a zstd archive with version "0.x.x"
func DetectV06Format(path string, data []byte) bool {
	// Must be .vcv file
	if strings.ToLower(filepath.Ext(path)) != ".vcv" {
		return false
	}

	// Not inside .mrk bundle (MiRack takes precedence)
	parentDir := filepath.Dir(path)
	if strings.ToLower(filepath.Ext(parentDir)) == ".mrk" {
		return false
	}

	// Check version field - v0.6 has version "0.x.x"
	version, err := extractVersion(data)
	if err != nil {
		return false
	}
	return strings.HasPrefix(version, "0.")
}
