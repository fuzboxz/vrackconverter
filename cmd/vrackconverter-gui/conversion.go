package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"

	"fyne.io/fyne/v2/dialog"

	"vrackconverter/internal/converter"
)

// StartConversion initiates the conversion process for all input files
func (g *ConverterGUI) StartConversion() {
	files := g.GetInputFiles()
	if len(files) == 0 {
		dialog.ShowInformation("No Files", "Please add files to convert first.", g.window)
		return
	}

	if g.IsConverting() {
		return
	}

	g.setConverting(true)
	g.results = []converter.Result{}
	g.fileList.Refresh()

	// Reset file statuses to Ready
	g.mu.Lock()
	for path := range g.fileStatuses {
		g.fileStatuses[path] = &FileStatus{
			Status:  "Ready",
			Icon:    StatusReady,
			Message: "Converting...",
		}
	}
	g.mu.Unlock()

	// Start conversion in goroutine
	go g.runConversion(files)
}

// runConversion performs the actual conversion work
func (g *ConverterGUI) runConversion(files []string) {
	total := len(files)
	var completed int32
	var success int32
	var failed int32
	var skipped int32

	g.Log(fmt.Sprintf("Starting conversion of %d file(s)...", total))

	for i, inputPath := range files {
		// Update log
		g.Log(fmt.Sprintf("Converting (%d/%d): %s", i+1, total, filepath.Base(inputPath)))

		// Generate output path
		outputPath := g.getOutputPath(inputPath)

		// Run conversion
		result := converter.ConvertFile(inputPath, outputPath, g.options)

		// Add to results
		g.mu.Lock()
		g.results = append(g.results, result)

		// Update file status
		if result.Error != nil {
			g.fileStatuses[inputPath] = &FileStatus{
				Status:  "Error",
				Icon:    StatusError,
				Message: result.Error.Error(),
			}
		} else if result.Skipped {
			g.fileStatuses[inputPath] = &FileStatus{
				Status:  "Skipped",
				Icon:    StatusSkipped,
				Message: "Already in target format",
			}
		} else {
			g.fileStatuses[inputPath] = &FileStatus{
				Status:  "Converted",
				Icon:    StatusConverted,
				Message: "Successfully converted",
			}
		}
		g.mu.Unlock()

		// Update counters
		completed := atomic.AddInt32(&completed, 1)
		if result.Error != nil {
			atomic.AddInt32(&failed, 1)
			g.Log(fmt.Sprintf("Failed: %s - %s", filepath.Base(inputPath), result.Error.Error()))
		} else if result.Skipped {
			atomic.AddInt32(&skipped, 1)
			g.Log(fmt.Sprintf("Skipped: %s (already in target format)", filepath.Base(inputPath)))
		} else {
			atomic.AddInt32(&success, 1)
			g.Log(fmt.Sprintf("Success: %s → %s", filepath.Base(inputPath), filepath.Base(outputPath)))
		}

		// Update progress
		progress := float64(completed) / float64(total)
		g.progressBar.SetValue(progress)
		g.fileList.Refresh()
	}

	// Show completion summary
	g.showConversionSummary(total, int(success), int(failed), int(skipped))

	// Re-enable UI
	g.setConverting(false)
}

// showConversionSummary displays a dialog with conversion results
func (g *ConverterGUI) showConversionSummary(total, success, failed, skipped int) {
	var message string
	if failed > 0 {
		message = fmt.Sprintf("Conversion complete:\n%d succeeded, %d failed, %d skipped",
			success, failed, skipped)
		g.Log(fmt.Sprintf("Complete: %d succeeded, %d failed, %d skipped",
			success, failed, skipped))

		dialog.ShowError(fmt.Errorf("%d file(s) failed to convert", failed), g.window)
	} else if skipped > 0 && success == 0 {
		message = fmt.Sprintf("All %d file(s) were skipped (already in target format)", skipped)
		g.Log(message)

		dialog.ShowInformation("Conversion Skipped", message, g.window)
	} else {
		message = fmt.Sprintf("Successfully converted %d file(s)", success)
		if skipped > 0 {
			message += fmt.Sprintf(", %d skipped", skipped)
		}
		g.Log("Complete: " + message)

		dialog.ShowInformation("Conversion Complete", message, g.window)
	}
}

// ConvertSingleFile converts a single file (for drag-and-drop scenarios)
func (g *ConverterGUI) ConvertSingleFile(inputPath string) {
	g.AddFiles([]string{inputPath})

	files := g.GetInputFiles()
	if len(files) == 1 {
		go g.runConversion(files)
	}
}

// GetResults returns a copy of the conversion results
func (g *ConverterGUI) GetResults() []converter.Result {
	g.mu.RLock()
	defer g.mu.RUnlock()

	results := make([]converter.Result, len(g.results))
	copy(results, g.results)
	return results
}

// HasErrors returns true if any conversion resulted in an error
func (g *ConverterGUI) HasErrors() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, result := range g.results {
		if result.Error != nil {
			return true
		}
	}
	return false
}

// GetFailedResults returns only the failed conversions
func (g *ConverterGUI) GetFailedResults() []converter.Result {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var failed []converter.Result
	for _, result := range g.results {
		if result.Error != nil {
			failed = append(failed, result)
		}
	}
	return failed
}
