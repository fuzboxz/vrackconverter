package converter

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

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
