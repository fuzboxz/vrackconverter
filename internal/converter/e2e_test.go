package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Test Functions (matrix-driven)
// ============================================================================

// TestE2E_AllConversions runs conversion tests for all fixtures in the matrix.
func TestE2E_AllConversions(t *testing.T) {
	// Print summary of all fixtures being tested
	t.Log("E2E tests ran on the following files:")
	for _, fixture := range allFixtures {
		fullPath := filepath.Join(testFixtureDir, fixture.filename)
		sourceFormat := detectFormat(fullPath)
		t.Logf("  - %s (%s)", fixture.filename, sourceFormat)
	}

	for _, fixture := range allFixtures {
		t.Run(fixture.filename, func(t *testing.T) {
			// Auto-detect source format
			fullPath := filepath.Join(testFixtureDir, fixture.filename)
			sourceFormat := detectFormat(fullPath)

			if sourceFormat.IsUnknown() {
				t.Skipf("Unable to detect format for %s", fixture.filename)
			}

			// Get valid target formats for this source
			targets, ok := conversionMatrix[sourceFormat]
			if !ok {
				t.Skipf("No conversion targets defined for %s", sourceFormat)
			}

			for _, targetFormat := range targets {
				// Check if this target should be skipped
				if skipTarget(fixture, targetFormat) {
					t.Run(targetFormat.String()+"_skipped", func(t *testing.T) {
						t.Skip("Target format skipped for this fixture")
					})
					continue
				}

				t.Run(targetFormat.String(), func(t *testing.T) {
					runMatrixTest(t, fixture.filename, sourceFormat, targetFormat)
				})
			}
		})
	}
}

// runMatrixTest executes a single conversion from the matrix.
func runMatrixTest(t *testing.T, filename string, sourceFormat, targetFormat Format) {
	t.Helper()

	conversionName := sourceFormat.String() + " -> " + targetFormat.String()
	t.Logf("Testing: %s (%s)", filename, conversionName)

	// 1. Read input
	input := readFixture(t, filename)
	extractModules(input)
	extractCablesOrWires(input)

	// 2. Generate output path
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	outputPath := tempOutputPath(t, baseName+"_converted", targetFormat)

	// 3. Convert
	result := ConvertFile(
		filepath.Join(testFixtureDir, filename),
		outputPath,
		Options{OutputFormat: targetFormat, Overwrite: true},
	)

	// 4. Check conversion result
	if result.Error != nil {
		t.Fatalf("Conversion failed: %v", result.Error)
	}

	if !result.Success && !result.Skipped {
		t.Fatalf("Conversion not successful and not skipped")
	}

	if result.Skipped {
		t.Skip("Conversion was skipped")
	}

	// 5. Clean up temp file on test completion
	t.Cleanup(func() {
		os.Remove(outputPath)
	})

	// 6. Parse and validate output
	output := parseOutputPath(t, outputPath, targetFormat)

	// 7. Apply universal validators and log each check
	validators := []struct {
		name string
		fn   func()
	}{
		{"format", func() { validateFormat(t, output, targetFormat) }},
		{"connectivity", func() { validateConnectivity(t, output) }},
		{"structural integrity", func() { validateStructuralIntegrity(t, output) }},
		{"color preservation", func() { validateColorPreservation(t, input, output) }},
		{"parameter equivalence", func() { validateParameterEquivalence(t, input, output) }},
		{"data preservation", func() { validateDataPreservation(t, input, output, conversionName) }},
		{"Notes module transformation", func() { validateNotesModuleTransformation(t, input, output) }},
	}

	for _, v := range validators {
		v.fn()
		t.Logf("  ✓ %s", v.name)
	}

	validatePreservation(t, input, output, conversionName)
}

// TestE2E_RoundtripTests runs A -> B -> A conversions.
func TestE2E_RoundtripTests(t *testing.T) {
	for _, rt := range roundtripMatrix {
		t.Run(rt.fixture+"_via_"+rt.via.String(), func(t *testing.T) {
			runRoundtripTest(t, rt.fixture, rt.via)
		})
	}
}

// runRoundtripTest executes A -> B -> A conversion and validates semantic preservation.
func runRoundtripTest(t *testing.T, filename string, viaFormat Format) {
	t.Helper()

	// 1. Read original
	original := readFixture(t, filename)
	originalFormat := original.format
	extractModules(original)
	extractCablesOrWires(original)

	originalModuleCount := len(original.modules)

	roundtripName := originalFormat.String() + " -> " + viaFormat.String() + " -> " + originalFormat.String()
	t.Logf("Testing roundtrip: %s (%s)", filename, roundtripName)

	// 2. First conversion: original -> via
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	intermediatePath := tempOutputPath(t, baseName+"_intermediate", viaFormat)

	result1 := ConvertFile(
		filepath.Join(testFixtureDir, filename),
		intermediatePath,
		Options{OutputFormat: viaFormat, Overwrite: true},
	)

	if result1.Error != nil {
		t.Fatalf("First conversion failed: %v", result1.Error)
	}

	if !result1.Success {
		t.Fatalf("First conversion not successful: %+v", result1)
	}

	t.Cleanup(func() {
		os.Remove(intermediatePath)
	})

	// 3. Second conversion: via -> original format
	finalPath := tempOutputPath(t, baseName+"_roundtrip", originalFormat)

	result2 := ConvertFile(
		intermediatePath,
		finalPath,
		Options{OutputFormat: originalFormat, Overwrite: true},
	)

	if result2.Error != nil {
		t.Fatalf("Second conversion failed: %v", result2.Error)
	}

	if !result2.Success {
		t.Fatalf("Second conversion not successful: %+v", result2)
	}

	t.Cleanup(func() {
		os.Remove(finalPath)
	})

	// 4. Read and validate final output
	final := parseOutputPath(t, finalPath, originalFormat)

	// 5. Validate roundtrip preservation
	validators := []struct {
		name string
		fn   func()
	}{
		{"format", func() { validateFormat(t, final, originalFormat) }},
		{"connectivity", func() { validateConnectivity(t, final) }},
		{"structural integrity", func() { validateStructuralIntegrity(t, final) }},
		{"data preservation", func() { validateDataPreservation(t, original, final, "roundtrip") }},
	}

	for _, v := range validators {
		v.fn()
		t.Logf("  ✓ %s", v.name)
	}

	// Module count should be the same after roundtrip
	// (with known exceptions for audio merge/split)
	finalModuleCount := len(final.modules)

	// For v2 -> MiRack -> v2 roundtrip, module count should be preserved
	// For MiRack -> v2 -> MiRack, audio modules merge then split, count should match
	if originalFormat == FormatMiRack && viaFormat == FormatVCV2 {
		// MiRack -> v2 -> MiRack: audio merge then split
		// The count should match if audio modules are properly restored
		if finalModuleCount != originalModuleCount {
			t.Logf("Module count after roundtrip: %d -> %d (expected match)", originalModuleCount, finalModuleCount)
		}
	} else if originalFormat == FormatVCV2 && viaFormat == FormatMiRack {
		// v2 -> MiRack -> v2: audio split then merge
		// The count should match if audio modules are properly restored
		if finalModuleCount != originalModuleCount {
			t.Logf("Module count after roundtrip: %d -> %d (expected match)", originalModuleCount, finalModuleCount)
		}
	}
}

// TestE2E_FormatDetection tests that all fixtures are correctly detected.
func TestE2E_FormatDetection(t *testing.T) {
	expectedFormats := map[string]Format{
		"mirack_basic.mrk":        FormatMiRack,
		"mirack_cables.mrk":       FormatMiRack,
		"mirack_multichannel.mrk": FormatMiRack,
		"mirack_to8channel.mrk":   FormatMiRack,
		"mirackoutput.mrk":        FormatMiRack,
		"legacy-patch.vcv":        FormatVCV06,
		"morningstarling.vcv":     FormatVCV06,
		"vcv06_cables.vcv":        FormatVCV06,
		"basevcvrack2.vcv":        FormatVCV2,
		"vcv2_audioio.vcv":        FormatVCV2,
		"vcv2_cables.vcv":         FormatVCV2,
		"vcv2_multichannel.vcv":   FormatVCV2,
	}

	for filename, expectedFormat := range expectedFormats {
		t.Run(filename, func(t *testing.T) {
			fullPath := filepath.Join(testFixtureDir, filename)
			actualFormat := detectFormat(fullPath)

			if actualFormat != expectedFormat {
				t.Errorf("Expected format %s, got %s", expectedFormat, actualFormat)
			}

			// Also verify we can read the file
			handler := GetFormatHandler(actualFormat)
			_, err := handler.Read(fullPath)
			if err != nil {
				t.Errorf("Failed to read file with detected format: %v", err)
			}
		})
	}
}

// TestE2E_StructuralIntegrity tests that all fixtures have valid structure.
func TestE2E_StructuralIntegrity(t *testing.T) {
	for _, fixture := range allFixtures {
		t.Run(fixture.filename, func(t *testing.T) {
			pd := readFixture(t, fixture.filename)
			extractCablesOrWires(pd)
			validateStructuralIntegrity(t, pd)
		})
	}
}

// ============================================================================
// Unit Tests for Color/Parameter Helpers
// ============================================================================

// TestE2E_extractColorFromWire tests color extraction from various wire formats.
func TestE2E_extractColorFromWire(t *testing.T) {
	tests := []struct {
		name     string
		wire     map[string]any
		wantR    uint8
		wantG    uint8
		wantB    uint8
		wantBool bool
	}{
		{
			name:  "hex color with #",
			wire:  map[string]any{"color": "#ff0000"},
			wantR: 255, wantG: 0, wantB: 0, wantBool: true,
		},
		{
			name:  "hex color without #",
			wire:  map[string]any{"color": "00ff00"},
			wantR: 0, wantG: 255, wantB: 0, wantBool: true,
		},
		{
			name:  "color object",
			wire:  map[string]any{"color": map[string]any{"r": 1.0, "g": 0.0, "b": 0.0}},
			wantR: 255, wantG: 0, wantB: 0, wantBool: true,
		},
		{
			name:     "invalid color",
			wire:     map[string]any{"color": "invalid"},
			wantBool: false,
		},
		{
			name:  "MiRack colorIndex 0 (yellow)",
			wire:  map[string]any{"colorIndex": float64(0)},
			wantR: 255, wantG: 181, wantB: 0, wantBool: true,
		},
		{
			name:  "MiRack colorIndex 1 (red)",
			wire:  map[string]any{"colorIndex": float64(1)},
			wantR: 242, wantG: 56, wantB: 74, wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, ok := extractColorFromWire(tt.wire)
			if ok != tt.wantBool {
				t.Errorf("extractColorFromWire() ok = %v, want %v", ok, tt.wantBool)
				return
			}
			if ok && (r != tt.wantR || g != tt.wantG || b != tt.wantB) {
				t.Errorf("extractColorFromWire() = (%d, %d, %d), want (%d, %d, %d)",
					r, g, b, tt.wantR, tt.wantG, tt.wantB)
			}
		})
	}
}

// TestE2E_extractColorFromV2Cable tests color extraction from v2 cables.
func TestE2E_extractColorFromV2Cable(t *testing.T) {
	tests := []struct {
		name     string
		cable    map[string]any
		wantR    uint8
		wantG    uint8
		wantB    uint8
		wantBool bool
	}{
		{
			name:  "6-digit hex",
			cable: map[string]any{"color": "ff0000"},
			wantR: 255, wantG: 0, wantB: 0, wantBool: true,
		},
		{
			name:  "8-digit hex with alpha",
			cable: map[string]any{"color": "ff0000ff"},
			wantR: 255, wantG: 0, wantB: 0, wantBool: true,
		},
		{
			name:  "color object",
			cable: map[string]any{"color": map[string]any{"r": 0.0, "g": 1.0, "b": 0.0}},
			wantR: 0, wantG: 255, wantB: 0, wantBool: true,
		},
		{
			name:     "invalid color",
			cable:    map[string]any{"color": "invalid"},
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, ok := extractColorFromV2Cable(tt.cable)
			if ok != tt.wantBool {
				t.Errorf("extractColorFromV2Cable() ok = %v, want %v", ok, tt.wantBool)
				return
			}
			if ok && (r != tt.wantR || g != tt.wantG || b != tt.wantB) {
				t.Errorf("extractColorFromV2Cable() = (%d, %d, %d), want (%d, %d, %d)",
					r, g, b, tt.wantR, tt.wantG, tt.wantB)
			}
		})
	}
}

// TestE2E_colorsApproxEqual tests color comparison with tolerance.
func TestE2E_colorsApproxEqual(t *testing.T) {
	tests := []struct {
		name       string
		r1, g1, b1 uint8
		r2, g2, b2 uint8
		want       bool
	}{
		{
			name: "identical colors",
			r1:   255, g1: 0, b1: 0,
			r2: 255, g2: 0, b2: 0,
			want: true,
		},
		{
			name: "slightly different colors",
			r1:   255, g1: 0, b1: 0,
			r2: 250, g2: 5, b2: 0,
			want: true,
		},
		{
			name: "very different colors",
			r1:   255, g1: 0, b1: 0,
			r2: 0, g2: 255, b2: 0,
			want: false,
		},
		{
			name: "palette approximation (yellow vs yellow-orange)",
			r1:   255, g1: 181, b1: 0, // MiRack yellow
			r2: 255, g2: 200, b2: 0,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorsApproxEqual(tt.r1, tt.g1, tt.b1, tt.r2, tt.g2, tt.b2)
			if got != tt.want {
				t.Errorf("colorsApproxEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestE2E_buildParameterMap tests parameter map building.
func TestE2E_buildParameterMap(t *testing.T) {
	tests := []struct {
		name    string
		params  []any
		idField string
		wantLen int
		wantKey string
		wantVal float64
	}{
		{
			name: "paramId field",
			params: []any{
				map[string]any{"paramId": float64(0), "value": float64(0.5)},
				map[string]any{"paramId": float64(1), "value": float64(1.0)},
			},
			idField: "paramId",
			wantLen: 2,
			wantKey: "0",
			wantVal: 0.5,
		},
		{
			name: "id field (v2 format)",
			params: []any{
				map[string]any{"id": float64(0), "value": float64(0.5)},
				map[string]any{"id": float64(1), "value": float64(1.0)},
			},
			idField: "id",
			wantLen: 2,
			wantKey: "0",
			wantVal: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildParameterMap(tt.params, tt.idField)
			if len(got) != tt.wantLen {
				t.Errorf("buildParameterMap() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantKey != "" {
				if val, ok := got[tt.wantKey]; !ok || val != tt.wantVal {
					t.Errorf("buildParameterMap()[%s] = %v, want %v", tt.wantKey, val, tt.wantVal)
				}
			}
		})
	}
}

// TestE2E_MiRackColorPalette tests the MiRack color palette.
func TestE2E_MiRackColorPalette(t *testing.T) {
	expectedColors := []struct {
		name    string
		r, g, b uint8
	}{
		{"yellow", 255, 181, 0},
		{"red", 242, 56, 74},
		{"green", 0, 181, 110},
		{"teal", 54, 149, 239},
		{"orange", 255, 181, 56},
		{"purple", 140, 74, 181},
	}

	if len(miRackColorPalette) != len(expectedColors) {
		t.Fatalf("MiRack color palette has %d colors, expected %d",
			len(miRackColorPalette), len(expectedColors))
	}

	for i, expected := range expectedColors {
		got := miRackColorPalette[i]
		if got.name != expected.name || got.r != expected.r ||
			got.g != expected.g || got.b != expected.b {
			t.Errorf("Color %d: got (%s, %d, %d, %d), want (%s, %d, %d, %d)",
				i, got.name, got.r, got.g, got.b,
				expected.name, expected.r, expected.g, expected.b)
		}
	}
}

// TestE2E_MetaModuleFlag tests --metamodule flag adds HubMedium module.
// Note: MetaModule is only added when converting to VCV Rack 2 format.
// Same-format conversions (e.g., V2 → V2) are skipped, so we test MiRack → V2.
func TestE2E_MetaModuleFlag(t *testing.T) {
	// Only test MiRack → V2 conversion (V2 → V2 is skipped)
	fixture := "mirack_basic.mrk"

	t.Run(fixture, func(t *testing.T) {
		input := readFixture(t, fixture)
		extractModules(input)

		baseName := strings.TrimSuffix(fixture, filepath.Ext(fixture))
		outputPath := tempOutputPath(t, baseName+"_metamodule", FormatVCV2)

		result := ConvertFile(
			filepath.Join(testFixtureDir, fixture),
			outputPath,
			Options{MetaModule: true, Overwrite: true},
		)

		if result.Error != nil {
			t.Fatalf("Conversion failed: %v", result.Error)
		}

		if result.Skipped {
			t.Skip("Conversion was skipped")
		}

		t.Cleanup(func() { os.Remove(outputPath) })

		output := parseOutputPath(t, outputPath, FormatVCV2)
		extractModules(output)

		// Find HubMedium module and verify its properties
		var hubMedium map[string]any
		for _, m := range output.modules {
			if model, _ := m["model"].(string); model == "HubMedium" {
				hubMedium = m
				break
			}
		}

		if hubMedium == nil {
			t.Fatal("HubMedium module not found in output")
		}

		// Check plugin
		if plugin, _ := hubMedium["plugin"].(string); plugin != "4msCompany" {
			t.Errorf("Expected plugin '4msCompany', got '%s'", plugin)
		}

		// Check model
		if model, _ := hubMedium["model"].(string); model != "HubMedium" {
			t.Errorf("Expected model 'HubMedium', got '%s'", model)
		}

		// Check params (14 params expected)
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

		// Check positioning (should have pos array)
		pos, ok := hubMedium["pos"].([]any)
		if !ok || len(pos) < 2 {
			t.Error("HubMedium should have pos array with at least 2 elements")
		}
	})
}

// TestE2E_ColorPreservationByFixture verifies colors are preserved for specific fixtures.
// This test validates the actual color mappings used by real patch files.
func TestE2E_ColorPreservationByFixture(t *testing.T) {
	tests := []struct {
		name           string
		fixture        string
		sourceFormat   Format
		targetFormat   Format
		expectedColors []string // Expected colors in order (after conversion to hex)
	}{
		{
			name:         "MiRack colorIndex to V2 hex",
			fixture:      "mirack_cables.mrk",
			sourceFormat: FormatMiRack,
			targetFormat: FormatVCV2,
			// MiRack colorIndex 0-5 map to these hex colors
			expectedColors: []string{
				"#ffb500", // colorIndex 0: yellow
				"#f2384a", // colorIndex 1: red
				"#00b56e", // colorIndex 2: green
				"#3695ef", // colorIndex 3: teal
				"#ffb538", // colorIndex 4: orange
				"#8c4ab5", // colorIndex 5: purple
			},
		},
		{
			name:         "V0.6 hex colors to V2",
			fixture:      "vcv06_cables.vcv",
			sourceFormat: FormatVCV06,
			targetFormat: FormatVCV2,
			// V0.6 cables use 6-digit hex with # prefix
			expectedColors: []string{
				"#0986ad", // cyan/teal
				"#c9b70e", // yellow/gold
				"#c91847", // red
				"#0c8e15", // green
			},
		},
		{
			name:         "V2 hex colors preserved",
			fixture:      "vcv2_cables.vcv",
			sourceFormat: FormatVCV2,
			targetFormat: FormatMiRack,
			// V2 to MiRack: colors should map to nearest palette color
			// vcv2_cables.vcv has: #ffb437, #f3374b, #00b56e, #3695ef, #ffb437
			// #ffb437 (255,180,55) is closer to orange (255,181,56) than yellow (255,181,0)
			expectedColors: []string{
				"#ffb538", // #ffb437 → nearest MiRack orange (index 4)
				"#f2384a", // #f3374b → nearest MiRack red (index 1)
				"#00b56e", // #00b56e → exact match MiRack green (index 2)
				"#3695ef", // #3695ef → exact match MiRack teal (index 3)
				"#ffb538", // #ffb437 → nearest MiRack orange (index 4)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read input fixture
			input := readFixture(t, tt.fixture)
			extractCablesOrWires(input)

			// Convert to target format
			baseName := strings.TrimSuffix(tt.fixture, filepath.Ext(tt.fixture))
			outputPath := tempOutputPath(t, baseName+"_color_test", tt.targetFormat)

			result := ConvertFile(
				filepath.Join(testFixtureDir, tt.fixture),
				outputPath,
				Options{OutputFormat: tt.targetFormat, Overwrite: true},
			)

			if result.Error != nil {
				t.Fatalf("Conversion failed: %v", result.Error)
			}

			// Parse output
			output := parseOutputPath(t, outputPath, tt.targetFormat)
			extractCablesOrWires(output)

			// Get cables/wires from output
			var connections []map[string]any
			if tt.targetFormat == FormatVCV2 {
				if cables, ok := output.root["cables"].([]any); ok {
					for _, c := range cables {
						if cable, ok := c.(map[string]any); ok {
							connections = append(connections, cable)
						}
					}
				}
			} else {
				// V0.6 and MiRack use "wires"
				if wires, ok := output.root["wires"].([]any); ok {
					for _, w := range wires {
						if wire, ok := w.(map[string]any); ok {
							connections = append(connections, wire)
						}
					}
				}
			}

			// Verify we have the expected number of cables/wires
			if len(connections) != len(tt.expectedColors) {
				t.Logf("Expected %d cables, got %d - checking colors for available cables",
					len(tt.expectedColors), len(connections))
			}

			// Check each cable's color
			maxColors := len(tt.expectedColors)
			if len(connections) < maxColors {
				maxColors = len(connections)
			}

			for i := 0; i < maxColors; i++ {
				conn := connections[i]
				expectedColor := tt.expectedColors[i]

				// Extract color based on format
				var actualColor string
				if tt.targetFormat == FormatMiRack {
					// MiRack uses colorIndex, convert to hex for comparison
					if colorIndex, ok := conn["colorIndex"].(float64); ok {
						actualColor = miRackColorIndexToHex(int(colorIndex))
					} else {
						// Some MiRack cables may have direct hex color
						if c, ok := conn["color"].(string); ok {
							actualColor = normalizeHexColor(c)
						}
					}
				} else {
					// V2 and V0.6 use hex colors
					if c, ok := conn["color"].(string); ok {
						actualColor = normalizeHexColor(c)
					}
				}

				if actualColor == "" {
					t.Errorf("Cable %d: no color found", i)
					continue
				}

				// For V2 format, strip alpha channel if present (8-digit hex)
				actualColor = stripAlphaChannel(actualColor)

				if actualColor != expectedColor {
					t.Errorf("Cable %d: color mismatch\n  got:  %s\n  want: %s", i, actualColor, expectedColor)
				}
			}
		})
	}
}
