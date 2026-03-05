package converter

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// IsV2Format checks if a file is already in VCV Rack v2 format.
// We detect this by extracting/reading the patch.json and checking the version field.
// V2 files have version "2.x.x", v0.6 files have version "0.x.x".
func IsV2Format(data []byte) bool {
	version, err := extractVersion(data)
	if err != nil {
		return false
	}
	return strings.HasPrefix(version, "2.")
}

// extractVersion attempts to extract the version field from patch data.
// Handles both plain JSON and zstd-compressed tar archives.
func extractVersion(data []byte) (string, error) {
	// First, try to parse as plain JSON
	var root map[string]any
	if err := json.Unmarshal(data, &root); err == nil {
		if version, ok := root["version"].(string); ok {
			return version, nil
		}
		return "", fmt.Errorf("no version field in JSON")
	}

	// Not plain JSON, try as zstd-compressed tar archive
	reader := bytes.NewReader(data)
	decoder, err := zstd.NewReader(reader)
	if err != nil {
		return "", fmt.Errorf("not zstd: %w", err)
	}
	defer decoder.Close()

	tarReader := tar.NewReader(decoder)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read error: %w", err)
		}

		if header.Name == "patch.json" {
			patchData, err := io.ReadAll(tarReader)
			if err != nil {
				return "", fmt.Errorf("failed to read patch.json: %w", err)
			}

			if err := json.Unmarshal(patchData, &root); err != nil {
				return "", fmt.Errorf("failed to parse patch.json: %w", err)
			}

			if version, ok := root["version"].(string); ok {
				return version, nil
			}
			return "", fmt.Errorf("no version field in patch.json")
		}
	}

	return "", fmt.Errorf("patch.json not found in archive")
}

// ExtractJSONFromV2 extracts patch.json from a VCV Rack v2 format file.
// Returns the raw JSON bytes for testing/comparison.
func ExtractJSONFromV2(vcvPath string) ([]byte, error) {
	file, err := os.Open(vcvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create zstd reader
	decoder, err := zstd.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer decoder.Close()

	// Create tar reader
	tarReader := tar.NewReader(decoder)

	// Find patch.json in the archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		if header.Name == "patch.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read patch.json: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("patch.json not found in archive")
}

func CreateV2Patch(patchJSON []byte, outputPath string) error {
	tmpDir, err := os.MkdirTemp("", "vrackconverter-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	patchPath := filepath.Join(tmpDir, "patch.json")
	if err := os.WriteFile(patchPath, patchJSON, 0644); err != nil {
		return fmt.Errorf("failed to write patch.json: %w", err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	encoder, err := zstd.NewWriter(outputFile, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return fmt.Errorf("failed to create zstd encoder: %w", err)
	}
	defer encoder.Close()

	tarWriter := tar.NewWriter(encoder)
	defer tarWriter.Close()

	info, err := os.Stat(patchPath)
	if err != nil {
		return fmt.Errorf("failed to stat patch.json: %w", err)
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}
	header.Name = "patch.json"

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	file, err := os.Open(patchPath)
	if err != nil {
		return fmt.Errorf("failed to open patch.json: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("failed to write patch.json to archive: %w", err)
	}

	return nil
}
