package converter

import (
	"encoding/json"
	"os"
	"testing"
)

// TestV06Roundtrip_VerifiesPreservedData tests that v0.6 → v2 → v0.6 roundtrip preserves all data.
func TestV06Roundtrip_VerifiesPreservedData(t *testing.T) {
	// Original v0.6 test data
	originalJSON := `{
		"version": "0.6.2c",
		"modules": [
			{
				"plugin": "Core",
				"version": "0.6.2c",
				"model": "MIDIToCVInterface",
				"params": [],
				"data": {
					"divisions": [24, 6],
					"midi": {"driver": 1, "channel": -1}
				},
				"pos": [30, 0]
			},
			{
				"plugin": "Core",
				"version": "0.6.2c",
				"model": "AudioInterface",
				"params": [],
				"data": {
					"audio": {
						"driver": 5,
						"deviceName": "",
						"offset": 0,
						"maxChannels": 8,
						"sampleRate": 44100,
						"blockSize": 256
					}
				},
				"pos": [38, 0]
			}
		],
		"wires": [
			{"color": "#0986ad", "outputModuleId": 0, "outputId": 0, "inputModuleId": 1, "inputId": 0},
			{"color": "#c9b70e", "outputModuleId": 0, "outputId": 1, "inputModuleId": 1, "inputId": 1},
			{"color": "#c91847", "outputModuleId": 0, "outputId": 2, "inputModuleId": 1, "inputId": 2},
			{"color": "#0c8e15", "outputModuleId": 0, "outputId": 3, "inputModuleId": 1, "inputId": 3}
		]
	}`

	// Parse original
	var original map[string]any
	if err := json.Unmarshal([]byte(originalJSON), &original); err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	// Roundtrip: v0.6 → v2 → v0.6
	var patch map[string]any
	if err := json.Unmarshal([]byte(originalJSON), &patch); err != nil {
		t.Fatalf("Failed to parse patch: %v", err)
	}

	var issues []string
	if err := NormalizeV06(patch, &issues); err != nil {
		t.Fatalf("NormalizeV06 failed: %v", err)
	}
	if err := DenormalizeV06(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV06 failed: %v", err)
	}

	roundtrip := patch

	// Verify module count
	originalModules := original["modules"].([]any)
	roundtripModules := roundtrip["modules"].([]any)
	if len(originalModules) != len(roundtripModules) {
		t.Errorf("Module count changed: %d → %d", len(originalModules), len(roundtripModules))
	}

	// Verify wire count
	originalWires := original["wires"].([]any)
	roundtripWires := roundtrip["wires"].([]any)
	if len(originalWires) != len(roundtripWires) {
		t.Errorf("Wire count changed: %d → %d", len(originalWires), len(roundtripWires))
	}

	// Verify each module
	for i := 0; i < len(originalModules) && i < len(roundtripModules); i++ {
		origMod := originalModules[i].(map[string]any)
		rtMod := roundtripModules[i].(map[string]any)

		model, _ := origMod["model"].(string)

		// Verify plugin
		if origPlugin, ok := origMod["plugin"].(string); ok {
			if rtPlugin, ok := rtMod["plugin"].(string); ok {
				if origPlugin != rtPlugin {
					t.Errorf("Module %d (%s): plugin changed: %s → %s", i, model, origPlugin, rtPlugin)
				}
			}
		}

		// Verify model
		if origModel, ok := origMod["model"].(string); ok {
			if rtModel, ok := rtMod["model"].(string); ok {
				if origModel != rtModel {
					t.Errorf("Module %d: model changed: %s → %s", i, origModel, rtModel)
				}
			}
		}

		// Verify params
		if origParams, ok := origMod["params"].([]any); ok {
			if rtParams, ok := rtMod["params"].([]any); ok {
				if len(origParams) != len(rtParams) {
					t.Errorf("Module %d: params count changed: %d → %d", i, len(origParams), len(rtParams))
				}
			}
		}

		// Verify pos
		if origPos, ok := origMod["pos"].([]any); ok {
			if rtPos, ok := rtMod["pos"].([]any); ok {
				if len(origPos) != len(rtPos) {
					t.Errorf("Module %d: pos count changed: %d → %d", i, len(origPos), len(rtPos))
				} else {
					for j := 0; j < len(origPos); j++ {
						origVal := getInt64(origPos[j])
						rtVal := getInt64(rtPos[j])
						if origVal != rtVal {
							t.Errorf("Module %d: pos[%d] changed: %d → %d", i, j, origVal, rtVal)
						}
					}
				}
			}
		}

		// Verify data
		if origData, ok := origMod["data"].(map[string]any); ok {
			if rtData, ok := rtMod["data"].(map[string]any); ok {
				// Check divisions for MIDIToCVInterface
				if model == "MIDIToCVInterface" {
					if origDivs, ok := origData["divisions"].([]any); ok {
						if rtDivs, ok := rtData["divisions"].([]any); ok {
							if len(origDivs) != len(rtDivs) {
								t.Errorf("Module %d: divisions count changed: %d → %d", i, len(origDivs), len(rtDivs))
							} else {
								for j := 0; j < len(origDivs); j++ {
									origVal := getInt64(origDivs[j])
									rtVal := getInt64(rtDivs[j])
									if origVal != rtVal {
										t.Errorf("Module %d: divisions[%d] changed: %d → %d", i, j, origVal, rtVal)
									}
								}
							}
						}
					}

					// Check midi data
					if origMidi, ok := origData["midi"].(map[string]any); ok {
						if rtMidi, ok := rtData["midi"].(map[string]any); ok {
							// Check driver
							origDriver := getInt64FromMap(origMidi, "driver")
							rtDriver := getInt64FromMap(rtMidi, "driver")
							if origDriver != rtDriver {
								t.Errorf("Module %d: midi.driver changed: %d → %d", i, origDriver, rtDriver)
							}

							// Check channel
							if origChannel, ok := origMidi["channel"].(float64); ok {
								if rtChannel, ok := rtMidi["channel"].(float64); ok {
									if origChannel != rtChannel {
										t.Errorf("Module %d: midi.channel changed: %f → %f", i, origChannel, rtChannel)
									}
								}
							}
						}
					}
				}

				// Check audio data for AudioInterface
				if model == "AudioInterface" {
					if origAudio, ok := origData["audio"].(map[string]any); ok {
						if rtAudio, ok := rtData["audio"].(map[string]any); ok {
							// Check all audio fields
							audioFields := []string{"driver", "maxChannels", "sampleRate", "blockSize"}
							for _, field := range audioFields {
								origVal := getInt64FromMap(origAudio, field)
								rtVal := getInt64FromMap(rtAudio, field)
								if origVal != rtVal {
									t.Errorf("Module %d: audio.%s changed: %d → %d", i, field, origVal, rtVal)
								}
							}

							// Check deviceName
							if origDeviceName, ok := origAudio["deviceName"].(string); ok {
								if rtDeviceName, ok := rtAudio["deviceName"].(string); ok {
									if origDeviceName != rtDeviceName {
										t.Errorf("Module %d: audio.deviceName changed: %s → %s", i, origDeviceName, rtDeviceName)
									}
								}
							}

							// Check offset
							origOffset := getInt64FromMap(origAudio, "offset")
							rtOffset := getInt64FromMap(rtAudio, "offset")
							if origOffset != rtOffset {
								t.Errorf("Module %d: audio.offset changed: %d → %d", i, origOffset, rtOffset)
							}
						}
					}
				}
			}
		}
	}

	// Verify wires
	for i := 0; i < len(originalWires) && i < len(roundtripWires); i++ {
		origWire := originalWires[i].(map[string]any)
		rtWire := roundtripWires[i].(map[string]any)

		// Verify color
		if origColor, ok := origWire["color"].(string); ok {
			if rtColor, ok := rtWire["color"].(string); ok {
				if origColor != rtColor {
					t.Errorf("Wire %d: color changed: %s → %s", i, origColor, rtColor)
				}
			}
		}

		// Verify connections
		origOutputModuleId := getInt64FromMap(origWire, "outputModuleId")
		rtOutputModuleId := getInt64FromMap(rtWire, "outputModuleId")
		if origOutputModuleId != rtOutputModuleId {
			t.Errorf("Wire %d: outputModuleId changed: %d → %d", i, origOutputModuleId, rtOutputModuleId)
		}

		origInputModuleId := getInt64FromMap(origWire, "inputModuleId")
		rtInputModuleId := getInt64FromMap(rtWire, "inputModuleId")
		if origInputModuleId != rtInputModuleId {
			t.Errorf("Wire %d: inputModuleId changed: %d → %d", i, origInputModuleId, rtInputModuleId)
		}

		origOutputId := getInt64FromMap(origWire, "outputId")
		rtOutputId := getInt64FromMap(rtWire, "outputId")
		if origOutputId != rtOutputId {
			t.Errorf("Wire %d: outputId changed: %d → %d", i, origOutputId, rtOutputId)
		}

		origInputId := getInt64FromMap(origWire, "inputId")
		rtInputId := getInt64FromMap(rtWire, "inputId")
		if origInputId != rtInputId {
			t.Errorf("Wire %d: inputId changed: %d → %d", i, origInputId, rtInputId)
		}
	}
}

// TestV06Roundtrip_WithActualFile tests roundtrip using the actual test file.
func TestV06Roundtrip_WithActualFile(t *testing.T) {
	testData, err := os.ReadFile("../../test/vcv06_cables.vcv")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Parse original
	var original map[string]any
	if err := json.Unmarshal(testData, &original); err != nil {
		t.Fatalf("Failed to parse original: %v", err)
	}

	// Roundtrip: v0.6 → v2 → v0.6
	var patch map[string]any
	if err := json.Unmarshal(testData, &patch); err != nil {
		t.Fatalf("Failed to parse patch: %v", err)
	}

	var issues []string
	if err := NormalizeV06(patch, &issues); err != nil {
		t.Fatalf("NormalizeV06 failed: %v", err)
	}
	if err := DenormalizeV06(patch, &issues); err != nil {
		t.Fatalf("DenormalizeV06 failed: %v", err)
	}

	roundtrip := patch

	// Verify key data is preserved
	originalModules := original["modules"].([]any)
	roundtripModules := roundtrip["modules"].([]any)

	// Module 0: MIDIToCVInterface
	origMod0 := originalModules[0].(map[string]any)
	rtMod0 := roundtripModules[0].(map[string]any)

	if origMod0["model"] != rtMod0["model"] {
		t.Errorf("Module 0 model changed: %v → %v", origMod0["model"], rtMod0["model"])
	}
	if origMod0["plugin"] != rtMod0["plugin"] {
		t.Errorf("Module 0 plugin changed: %v → %v", origMod0["plugin"], rtMod0["plugin"])
	}

	// Check data.divisions
	origData := origMod0["data"].(map[string]any)
	rtData := rtMod0["data"].(map[string]any)
	origDivs := origData["divisions"].([]any)
	rtDivs := rtData["divisions"].([]any)
	if len(origDivs) != len(rtDivs) {
		t.Errorf("Divisions count changed: %d → %d", len(origDivs), len(rtDivs))
	}
	for i := 0; i < len(origDivs); i++ {
		origVal := getInt64(origDivs[i])
		rtVal := getInt64(rtDivs[i])
		if origVal != rtVal {
			t.Errorf("Divisions[%d] changed: %d → %d", i, origVal, rtVal)
		}
	}

	// Check wires
	originalWires := original["wires"].([]any)
	roundtripWires := roundtrip["wires"].([]any)

	if len(originalWires) != len(roundtripWires) {
		t.Errorf("Wire count changed: %d → %d", len(originalWires), len(roundtripWires))
	}

	// Check wire colors are preserved
	expectedColors := []string{"#0986ad", "#c9b70e", "#c91847", "#0c8e15"}
	for i, color := range expectedColors {
		wire := roundtripWires[i].(map[string]any)
		if wire["color"] != color {
			t.Errorf("Wire %d color changed: expected %s, got %v", i, color, wire["color"])
		}
	}
}
