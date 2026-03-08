package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"vrackconverter/internal/converter"
)

// FileStatus represents the status of a file in the queue
type FileStatus struct {
	Status  string // "Ready", "Warn", "Error", "Skipped", "Converted"
	Icon    string // Visual indicator
	Message string // Status message
}

// PatchInfo holds information about a patch file
type PatchInfo struct {
	FileName    string
	ModuleCount int
	CableCount  int
	Modules     []ModuleInfo // name, plugin, count
	Status      string
	StatusIcon  string
	StatusNote  string
}

// ModuleInfo represents a module type with its count
type ModuleInfo struct {
	Name   string
	Plugin string
	Count  int
}

// ConverterGUI holds the application state and UI widgets
type ConverterGUI struct {
	app    fyne.App
	window fyne.Window

	// State
	inputFiles        []string
	fileStatuses      map[string]*FileStatus
	selectedFileIndex int
	outputDir         string
	outputFormat      converter.Format
	options           converter.Options
	results           []converter.Result
	isConverting      bool
	mu                sync.RWMutex

	// UI widgets - Left Column
	fileList  *widget.List
	addBtn    *widget.Button
	removeBtn *widget.Button
	clearBtn  *widget.Button
	logWidget *widget.Entry

	// UI widgets - Right Column (Global Settings)
	formatSelect    *widget.Select
	metaModuleCheck *widget.Check
	overwriteCheck  *widget.Check
	outputDirEntry  *widget.Entry
	browseBtn       *widget.Button

	// UI widgets - Right Column (Patch Inspector)
	inspectorFileName *widget.Label
	inspectorOverview *widget.Label
	inspectorContents *widget.RichText
	inspectorStatus   *widget.Label

	// UI widgets - Right Column (Convert button)
	convertBtn  *widget.Button
	progressBar *widget.ProgressBar
}

// NewConverterGUI creates a new GUI application instance
func NewConverterGUI(app fyne.App) *ConverterGUI {
	gui := &ConverterGUI{
		app:               app,
		inputFiles:        []string{},
		fileStatuses:      make(map[string]*FileStatus),
		selectedFileIndex: -1,
		outputDir:         "",
		outputFormat:      converter.FormatVCV2, // Default to v2
		options: converter.Options{
			Overwrite:  false,
			Quiet:      true, // GUI always uses quiet mode
			MetaModule: false,
		},
		results:      []converter.Result{},
		isConverting: false,
	}

	// Initialize format strings
	gui.outputFormat = converter.FormatVCV2

	return gui
}

// SetWindow sets the window reference for the GUI
func (g *ConverterGUI) SetWindow(window fyne.Window) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.window = window
}

// EnableDragAndDrop enables file drag and drop on the window
func (g *ConverterGUI) EnableDragAndDrop() {
	g.mu.RLock()
	window := g.window
	g.mu.RUnlock()

	if window == nil {
		return
	}

	window.SetOnDropped(func(position fyne.Position, uris []fyne.URI) {
		var paths []string
		for _, uri := range uris {
			path := uri.Path()
			// Check if it's a file we can process
			if g.isPatchFile(path) {
				paths = append(paths, path)
			} else if g.isDirectory(path) {
				// If it's a directory, add all patch files inside
				dirFiles := g.getPatchFilesInDir(path)
				paths = append(paths, dirFiles...)
			}
		}
		if len(paths) > 0 {
			g.AddFiles(paths)
		}
	})
}

// isPatchFile checks if a file is a supported patch file
func (g *ConverterGUI) isPatchFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".vcv" || ext == ".mrk"
}

// isDirectory checks if a path is a directory
func (g *ConverterGUI) isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// getPatchFilesInDir returns all patch files in a directory
func (g *ConverterGUI) getPatchFilesInDir(dir string) []string {
	var paths []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return paths
	}

	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)

		if entry.IsDir() {
			// Check for .mrk bundle directories
			if strings.HasSuffix(strings.ToLower(name), ".mrk") {
				paths = append(paths, path)
			}
		} else if g.isPatchFile(path) {
			paths = append(paths, path)
		}
	}

	return paths
}

// MakeUI creates and returns the main UI layout
func (g *ConverterGUI) MakeUI() *fyne.Container {
	return g.createMainLayout()
}

// AddFiles adds files to the input list
func (g *ConverterGUI) AddFiles(paths []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	added := 0
	for _, path := range paths {
		// Skip if already in list
		alreadyExists := false
		for _, existing := range g.inputFiles {
			if existing == path {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			g.inputFiles = append(g.inputFiles, path)
			// Initialize file status
			g.fileStatuses[path] = &FileStatus{
				Status:  "Ready",
				Icon:    "✓",
				Message: "Ready to convert",
			}
			added++
		}
	}

	if added > 0 {
		g.updateFileListUI()
		g.Log(fmt.Sprintf("Added %d file(s). Ready.", added))
	}
}

// ClearFiles removes all files from the list
func (g *ConverterGUI) ClearFiles() {
	g.mu.Lock()
	defer g.mu.Unlock()

	count := len(g.inputFiles)
	g.inputFiles = []string{}
	g.fileStatuses = make(map[string]*FileStatus)
	g.results = []converter.Result{}
	g.selectedFileIndex = -1
	g.updateFileListUI()
	g.updateInspector(nil)
	g.Log(fmt.Sprintf("Cleared %d file(s).", count))
}

// SelectOutputDirectory opens a directory chooser dialog
func (g *ConverterGUI) SelectOutputDirectory() {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		path := uri.Path()
		g.outputDir = path
		g.outputDirEntry.SetText(path)
	}, g.window)

	d.Show()
}

// SetOutputFormat sets the target output format
func (g *ConverterGUI) SetOutputFormat(formatStr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch strings.ToLower(strings.TrimSpace(formatStr)) {
	case "auto", "detect", "":
		g.outputFormat = "" // Auto-detect from extension
	case "v2", "vcv2", "2":
		g.outputFormat = converter.FormatVCV2
	case "v0.6", "v06", "vcv06", "0.6", "06":
		g.outputFormat = converter.FormatVCV06
	case "mirack", "mrk":
		g.outputFormat = converter.FormatMiRack
	}
	g.options.OutputFormat = g.outputFormat
}

// ToggleMetaModule toggles the MetaModule option
func (g *ConverterGUI) ToggleMetaModule(enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.options.MetaModule = enabled
}

// ToggleOverwrite toggles the overwrite option
func (g *ConverterGUI) ToggleOverwrite(enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.options.Overwrite = enabled
}

// GetInputFiles returns a copy of the input files list
func (g *ConverterGUI) GetInputFiles() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	files := make([]string, len(g.inputFiles))
	copy(files, g.inputFiles)
	return files
}

// GetFileCount returns the number of input files
func (g *ConverterGUI) GetFileCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.inputFiles)
}

// detectFormatFromPath detects the format of a file by its extension
func (g *ConverterGUI) detectFormatFromPath(path string) converter.Format {
	data, err := os.ReadFile(path)
	if err != nil {
		return converter.FormatUnknown
	}
	return converter.DetectFormat(path, data)
}

// getOutputPath generates the output path for an input file
func (g *ConverterGUI) getOutputPath(inputPath string) string {
	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	// Determine output extension
	var outputExt string
	if g.outputFormat != "" {
		switch g.outputFormat {
		case converter.FormatVCV2, converter.FormatVCV06:
			outputExt = ".vcv"
		case converter.FormatMiRack:
			outputExt = ".mrk"
		default:
			outputExt = ".vcv"
		}
	} else {
		// Auto-detect: default to v2 (.vcv)
		outputExt = ".vcv"
	}

	// If output directory is set, use it
	if g.outputDir != "" {
		return filepath.Join(g.outputDir, nameWithoutExt+outputExt)
	}

	// Otherwise, output next to input file
	inputDir := filepath.Dir(inputPath)
	return filepath.Join(inputDir, nameWithoutExt+outputExt)
}

// updateFileListUI updates the file list display
func (g *ConverterGUI) updateFileListUI() {
	g.fileList.Refresh()
	count := len(g.inputFiles)
	if count == 0 {
		g.convertBtn.Disable()
		g.removeBtn.Disable()
		g.clearBtn.Disable()
	} else {
		g.convertBtn.Enable()
		g.removeBtn.Enable()
		g.clearBtn.Enable()
	}
}

// setConverting sets the conversion state and updates UI accordingly
func (g *ConverterGUI) setConverting(converting bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.isConverting = converting

	if converting {
		g.convertBtn.Disable()
		g.addBtn.Disable()
		g.removeBtn.Disable()
		g.clearBtn.Disable()
		g.formatSelect.Disable()
		g.progressBar.Show()
	} else {
		if len(g.inputFiles) > 0 {
			g.convertBtn.Enable()
			g.addBtn.Enable()
			g.removeBtn.Enable()
			g.clearBtn.Enable()
		}
		g.formatSelect.Enable()
		g.progressBar.Hide()
	}
}

// IsConverting returns true if a conversion is in progress
func (g *ConverterGUI) IsConverting() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isConverting
}

// Log adds a message to the log widget
func (g *ConverterGUI) Log(message string) {
	if g.logWidget != nil {
		currentText := g.logWidget.Text
		if currentText == "" {
			g.logWidget.SetText("> " + message)
		} else {
			g.logWidget.SetText(currentText + "\n> " + message)
		}
		// Scroll to bottom
		g.logWidget.CursorRow = len(strings.Split(g.logWidget.Text, "\n")) - 1
	}
}

// SelectFile sets the selected file and updates the inspector
func (g *ConverterGUI) SelectFile(index int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if index < 0 || index >= len(g.inputFiles) {
		g.selectedFileIndex = -1
		g.updateInspector(nil)
		return
	}

	g.selectedFileIndex = index
	path := g.inputFiles[index]
	info := g.GetPatchInfo(path)
	g.updateInspector(&info)
}

// updateInspector updates the patch inspector display
func (g *ConverterGUI) updateInspector(info *PatchInfo) {
	if info == nil {
		g.inspectorFileName.SetText("No file selected")
		g.inspectorOverview.SetText("")
		g.inspectorContents.Segments = []widget.RichTextSegment{}
		g.inspectorStatus.SetText("")
		g.inspectorContents.Refresh()
		return
	}

	g.inspectorFileName.SetText(info.FileName)
	g.inspectorOverview.SetText(fmt.Sprintf("Overview: %d Modules | %d Cables", info.ModuleCount, info.CableCount))

	// Build module list
	var segments []widget.RichTextSegment
	if len(info.Modules) > 0 {
		for _, mod := range info.Modules {
			text := fmt.Sprintf("• %s", mod.Name)
			if mod.Count > 1 {
				text = fmt.Sprintf("• %s (x%d)", mod.Name, mod.Count)
			}
			segments = append(segments, &widget.TextSegment{Text: text})
			segments = append(segments, &widget.TextSegment{Text: "\n"})
		}
	}
	g.inspectorContents.Segments = segments
	g.inspectorContents.Refresh()

	// Update status
	statusText := info.StatusIcon + " " + info.StatusNote
	g.inspectorStatus.SetText(statusText)
}

// GetPatchInfo reads and parses a patch file
func (g *ConverterGUI) GetPatchInfo(path string) PatchInfo {
	info := PatchInfo{
		FileName:   filepath.Base(path),
		Status:     "Ready",
		StatusIcon: "✓",
		StatusNote: "All modules supported.",
	}

	// Detect format first
	data, err := os.ReadFile(path)
	if err != nil {
		info.Status = "Error"
		info.StatusIcon = "❌"
		info.StatusNote = fmt.Sprintf("Cannot read file: %v", err)
		return info
	}

	format := converter.DetectFormat(path, data)

	// For MiRack format (.mrk directories), read the patch.json inside
	var patchData map[string]any
	if format == converter.FormatMiRack {
		patchJSONPath := filepath.Join(path, "patch.json")
		data, err = os.ReadFile(patchJSONPath)
		if err != nil {
			info.Status = "Error"
			info.StatusIcon = "❌"
			info.StatusNote = fmt.Sprintf("Cannot read patch.json: %v", err)
			return info
		}
	} else if format == converter.FormatVCV2 || format == converter.FormatVCV06 {
		// Extract JSON from v2/v0.6 archives
		jsonData, err := converter.ExtractJSONFromV2(path)
		if err != nil {
			info.Status = "Error"
			info.StatusIcon = "❌"
			info.StatusNote = fmt.Sprintf("Cannot extract patch data: %v", err)
			return info
		}
		data = jsonData
	}

	// Parse JSON
	if err := json.Unmarshal(data, &patchData); err != nil {
		info.Status = "Error"
		info.StatusIcon = "❌"
		info.StatusNote = fmt.Sprintf("Cannot parse patch: %v", err)
		return info
	}

	// Extract modules
	modulesVal, hasModules := patchData["modules"]
	if !hasModules {
		info.Status = "Warn"
		info.StatusIcon = "⚠️"
		info.StatusNote = "No modules found in patch."
		return info
	}

	modulesArray, ok := modulesVal.([]any)
	if !ok {
		info.Status = "Warn"
		info.StatusIcon = "⚠️"
		info.StatusNote = "Invalid modules data."
		return info
	}

	info.ModuleCount = len(modulesArray)

	// Count modules by name
	moduleCounts := make(map[string]int)
	pluginCounts := make(map[string]string)

	for _, m := range modulesArray {
		module, ok := m.(map[string]any)
		if !ok {
			continue
		}

		plugin, _ := module["plugin"].(string)
		model, _ := module["model"].(string)

		if model == "" {
			continue
		}

		moduleCounts[model]++
		if plugin != "" {
			pluginCounts[model] = plugin
		}
	}

	// Build module list
	info.Modules = make([]ModuleInfo, 0, len(moduleCounts))
	for name, count := range moduleCounts {
		plugin := pluginCounts[name]
		info.Modules = append(info.Modules, ModuleInfo{
			Name:   name,
			Plugin: plugin,
			Count:  count,
		})
	}

	// Extract cables count
	if cablesVal, hasCables := patchData["cables"]; hasCables {
		if cablesArray, ok := cablesVal.([]any); ok {
			info.CableCount = len(cablesArray)
		}
	} else if wiresVal, hasWires := patchData["wires"]; hasWires {
		// v0.6/MiRack use "wires" instead of "cables"
		if wiresArray, ok := wiresVal.([]any); ok {
			info.CableCount = len(wiresArray)
		}
	}

	// Check for any warnings
	if info.ModuleCount == 0 {
		info.Status = "Warn"
		info.StatusIcon = "⚠️"
		info.StatusNote = "No modules found in patch."
	}

	return info
}

// RemoveSelected removes the currently selected file from the list
func (g *ConverterGUI) RemoveSelected() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.inputFiles) == 0 {
		return
	}

	if g.selectedFileIndex < 0 || g.selectedFileIndex >= len(g.inputFiles) {
		return
	}

	// Remove selected file
	path := g.inputFiles[g.selectedFileIndex]
	g.inputFiles = append(g.inputFiles[:g.selectedFileIndex], g.inputFiles[g.selectedFileIndex+1:]...)
	delete(g.fileStatuses, path)

	// Adjust selection
	if g.selectedFileIndex >= len(g.inputFiles) {
		g.selectedFileIndex = len(g.inputFiles) - 1
	}

	g.updateFileListUI()

	// Update inspector
	if g.selectedFileIndex >= 0 {
		path := g.inputFiles[g.selectedFileIndex]
		info := g.GetPatchInfo(path)
		g.updateInspector(&info)
	} else {
		g.updateInspector(nil)
	}

	g.Log(fmt.Sprintf("Removed: %s", filepath.Base(path)))
}
