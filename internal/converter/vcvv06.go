package converter

// V06Handler implements FormatHandler for VCV Rack v0.6 format.
// V0.6 files are zstd tar archives with version "0.x.x".
//
// Key differences from v2:
// - Uses "wires" instead of "cables"
// - Cable references use array indices, not module IDs
// - Has separate "Fundamental" and "Core" plugins
// - Uses "paramId" instead of "id" in parameters
// - Uses "disabled" instead of "bypass"
//
// Key differences from MiRack:
// - File container is zstd tar archive, not directory bundle
// - HAS "Fundamental" plugin (MiRack does NOT have Fundamental!)
// - Otherwise similar format (wires, array indices, etc.)
type V06Handler struct{}

// Read reads a v0.6 patch (zstd tar archive).
func (h *V06Handler) Read(path string) ([]byte, error) {
	// v0.6 files are zstd tar archives, extract JSON
	return ExtractJSONFromV2(path)
}

// Write writes a v0.6 patch (zstd tar archive).
func (h *V06Handler) Write(data []byte, path string) error {
	return CreateV2Patch(data, path)
}

// Extension returns ".vcv"
func (h *V06Handler) Extension() string {
	return ".vcv"
}

// v06Converter is the shared converter instance for v0.6 format.
// Uses V06PluginMapper to handle Fundamental ↔ Core conversion.
var v06Converter = NewLegacyConverter(V06PluginMapper{})

// NormalizeV06 converts v0.6 format to internal (v2) format.
//
// v0.6-specific behavior:
// - Fundamental → Core (preserve roundtrip info via fundamentalModules map)
// - Array indices → Module IDs for cables
// - wires → cables
// - paramId → id in parameters
// - disabled → bypass
func NormalizeV06(patch map[string]any, issues *[]string) error {
	return v06Converter.NormalizeLegacy(patch, issues, "v0.6")
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
	return v06Converter.DenormalizeLegacy(patch, issues, "v0.6")
}
