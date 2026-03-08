package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"vrackconverter/internal/converter"
)

const (
	// Minimum sizes for UI elements
	minSidebarWidth    = 280
	minLogHeight       = 120
	minInspectorHeight = 200
	minButtonHeight    = 36
)

// createMainLayout creates the main two-panel layout
func (g *ConverterGUI) createMainLayout() *fyne.Container {
	// Create menu bar
	g.createMenuBar()

	// Left column: Input Queue (top) + Log (bottom)
	leftColumn := g.createLeftColumn()

	// Right column: Global Settings (top) + Patch Inspector (middle) + Convert button (bottom)
	rightColumn := g.createRightColumn()
	rightColumn.Resize(fyne.NewSize(minSidebarWidth, rightColumn.MinSize().Height))

	// Use HSplit for resizable panels with proper ratio
	// The split will maintain the ratio and allow user adjustment
	split := container.NewHSplit(leftColumn, rightColumn)
	split.SetOffset(0.7) // Left panel gets 70%, right gets 30%

	return container.NewBorder(nil, nil, nil, nil, split)
}

// createLeftColumn creates the left column with Input Queue and Log
func (g *ConverterGUI) createLeftColumn() *fyne.Container {
	// INPUT QUEUE header and buttons
	inputHeader := widget.NewRichTextFromMarkdown("### INPUT QUEUE")
	inputHeader.Resize(fyne.NewSize(200, 30))

	g.addBtn = widget.NewButton(" + Add", func() {
		g.showAddFilesDialog()
	})
	g.addBtn.Importance = widget.MediumImportance
	g.addBtn.Resize(fyne.NewSize(100, minButtonHeight))

	g.removeBtn = widget.NewButton(" - Remove", func() {
		g.RemoveSelected()
	})
	g.removeBtn.Disable()
	g.removeBtn.Resize(fyne.NewSize(100, minButtonHeight))

	g.clearBtn = widget.NewButton(" x Clear", func() {
		g.ClearFiles()
	})
	g.clearBtn.Disable()
	g.clearBtn.Resize(fyne.NewSize(100, minButtonHeight))

	buttonRow := container.NewGridWithColumns(3, g.addBtn, g.removeBtn, g.clearBtn)

	// File list with custom widget showing file name and status
	g.fileList = widget.NewList(
		func() int {
			return len(g.inputFiles)
		},
		func() fyne.CanvasObject {
			// Template for list items with minimum height
			fileNameLabel := widget.NewLabel("FileName.vcv")
			fileNameLabel.TextStyle = fyne.TextStyle{Bold: true}
			statusLabel := widget.NewLabel("[Ready]")

			row := container.NewGridWithColumns(2,
				fileNameLabel,
				statusLabel,
			)
			row.Resize(fyne.NewSize(200, 32))
			return row
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= 0 && id < len(g.inputFiles) {
				path := g.inputFiles[id]
				name := filepath.Base(path)
				grid := obj.(*fyne.Container)

				// Get file name label
				nameLabel := grid.Objects[0].(*widget.Label)
				nameLabel.Text = name
				nameLabel.Refresh()

				// Get status label
				statusLabel := grid.Objects[1].(*widget.Label)
				if status, ok := g.fileStatuses[path]; ok {
					statusLabel.Text = fmt.Sprintf("[%s %s]", status.Icon, status.Status)
				} else {
					statusLabel.Text = "[✓ Ready]"
				}
				statusLabel.Refresh()
			}
		},
	)

	// Handle file selection
	g.fileList.OnSelected = func(id widget.ListItemID) {
		g.SelectFile(id)
	}

	// LOG section
	logHeader := widget.NewRichTextFromMarkdown("### LOG")
	logHeader.Resize(fyne.NewSize(200, 30))

	g.logWidget = widget.NewMultiLineEntry()
	g.logWidget.SetPlaceHolder("Activity log...")
	g.logWidget.Disable()
	g.logWidget.Wrapping = fyne.TextWrapWord
	g.logWidget.Resize(fyne.NewSize(200, minLogHeight))

	// Create log container with minimum height
	logContainer := container.NewVBox(
		logHeader,
		widget.NewSeparator(),
	)
	logContainer.Add(g.logWidget)
	logContainer.Resize(fyne.NewSize(200, minLogHeight+50))

	// Top section: header and buttons
	topSection := container.NewVBox(
		inputHeader,
		buttonRow,
		widget.NewSeparator(),
	)

	// Use VSplit for vertical split between list and log
	// This allows resizing and maintains proper ratio
	vsplit := container.NewVSplit(
		container.NewBorder(topSection, nil, nil, nil, g.fileList),
		logContainer,
	)
	vsplit.SetOffset(0.7) // List gets 70%, log gets 30%

	return container.NewBorder(nil, nil, nil, nil, vsplit)
}

// createRightColumn creates the right column with Settings, Inspector, and Convert button
func (g *ConverterGUI) createRightColumn() *fyne.Container {
	// GLOBAL SETTINGS panel
	globalSettings := g.createGlobalSettingsPanel()

	// Separator above Patch Inspector
	separatorAboveInspector := widget.NewSeparator()

	// PATCH INSPECTOR panel
	patchInspector := g.createPatchInspectorPanel()

	// Progress bar (hidden initially)
	g.progressBar = widget.NewProgressBar()
	g.progressBar.Hide()

	// CONVERT NOW button
	g.convertBtn = widget.NewButton("CONVERT NOW", func() {
		go g.StartConversion()
	})
	g.convertBtn.Importance = widget.HighImportance
	g.convertBtn.Disable()

	// Middle section: inspector with progress bar
	middleSection := container.NewVBox(
		separatorAboveInspector,
		patchInspector,
		g.progressBar,
	)
	middleSection.Resize(fyne.NewSize(minSidebarWidth, minInspectorHeight))

	// Right column using border layout
	rightColumn := container.NewBorder(
		globalSettings, // Top
		g.convertBtn,   // Bottom
		nil,            // Left
		nil,            // Right
		middleSection,  // Center (expands)
	)

	return container.NewPadded(rightColumn)
}

// createGlobalSettingsPanel creates the Global Settings panel
func (g *ConverterGUI) createGlobalSettingsPanel() *fyne.Container {
	header := widget.NewRichTextFromMarkdown("### GLOBAL SETTINGS")

	// Target dropdown
	targetLabel := widget.NewLabel("Target:")
	targetLabel.Resize(fyne.NewSize(60, 20))

	g.formatSelect = widget.NewSelect([]string{
		"Auto-detect",
		"VCV Rack v2",
		"VCV Rack v0.6",
		"MiRack",
	}, func(selected string) {
		g.SetOutputFormat(selected)
		g.Log(fmt.Sprintf("Target: %s", selected))
	})
	g.formatSelect.SetSelected("Auto-detect")
	g.formatSelect.Resize(fyne.NewSize(200, 30))

	targetRow := container.NewBorder(
		nil, nil, targetLabel, nil,
		g.formatSelect,
	)

	// Options
	optionsLabel := widget.NewLabel("Options:")
	optionsLabel.Resize(fyne.NewSize(60, 20))

	g.metaModuleCheck = widget.NewCheck("Add MetaModule", func(checked bool) {
		g.ToggleMetaModule(checked)
		if checked {
			g.Log("Option enabled: Add MetaModule")
		} else {
			g.Log("Option disabled: Add MetaModule")
		}
	})

	g.overwriteCheck = widget.NewCheck("Overwrite", func(checked bool) {
		g.ToggleOverwrite(checked)
		if checked {
			g.Log("Option enabled: Overwrite existing files")
		} else {
			g.Log("Option disabled: Overwrite existing files")
		}
	})

	optionsBox := container.NewVBox(
		optionsLabel,
		g.metaModuleCheck,
		g.overwriteCheck,
	)

	// Output directory - entry expands properly
	outputLabel := widget.NewLabel("Output:")
	outputLabel.Resize(fyne.NewSize(60, 20))

	g.outputDirEntry = widget.NewEntry()
	g.outputDirEntry.SetPlaceHolder("Same as input (default)")
	g.outputDirEntry.Disable()

	g.browseBtn = widget.NewButton("Browse...", func() {
		g.SelectOutputDirectory()
	})
	g.browseBtn.Resize(fyne.NewSize(90, minButtonHeight))

	// Entry expands, button fixed at right
	outputRow := container.NewBorder(
		nil, nil, // Top, Bottom
		outputLabel,      // Left
		g.browseBtn,      // Right
		g.outputDirEntry, // Center (expands)
	)

	// Combine all
	vbox := container.NewVBox(
		header,
		widget.NewSeparator(),
		targetRow,
		widget.NewSeparator(),
		optionsBox,
		widget.NewSeparator(),
		outputRow,
	)

	return container.NewPadded(vbox)
}

// createPatchInspectorPanel creates the Patch Inspector panel
func (g *ConverterGUI) createPatchInspectorPanel() *fyne.Container {
	header := widget.NewRichTextFromMarkdown("### PATCH INSPECTOR")

	// File name
	g.inspectorFileName = widget.NewLabel("No file selected")
	g.inspectorFileName.TextStyle = fyne.TextStyle{Bold: true}
	g.inspectorFileName.Resize(fyne.NewSize(200, 20))

	// Overview
	g.inspectorOverview = widget.NewLabel("")
	g.inspectorOverview.Resize(fyne.NewSize(200, 20))

	// Contents (module list)
	contentsLabel := widget.NewLabel("Contents:")
	contentsLabel.Resize(fyne.NewSize(60, 20))

	g.inspectorContents = widget.NewRichText(
		&widget.TextSegment{Text: "No file selected"},
	)
	g.inspectorContents.Wrapping = fyne.TextWrapWord

	// Scrollable container for contents
	contentsScroll := container.NewScroll(g.inspectorContents)
	contentsScroll.SetMinSize(fyne.NewSize(200, 100))

	// Status note
	g.inspectorStatus = widget.NewLabel("")
	g.inspectorStatus.Resize(fyne.NewSize(200, 40))
	g.inspectorStatus.Wrapping = fyne.TextWrapWord

	// Combine all
	vbox := container.NewVBox(
		header,
		widget.NewSeparator(),
		g.inspectorFileName,
		g.inspectorOverview,
		widget.NewSeparator(),
		contentsLabel,
		contentsScroll,
		widget.NewSeparator(),
		g.inspectorStatus,
	)

	return container.NewPadded(vbox)
}

// createMenuBar creates the application menu bar
func (g *ConverterGUI) createMenuBar() {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Add Files...", func() {
			g.showAddFilesDialog()
		}),
		fyne.NewMenuItem("Add Folder...", func() {
			g.showAddFolderDialog()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Clear File List", func() {
			g.ClearFiles()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			g.app.Quit()
		}),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			g.showAboutDialog()
		}),
	)

	mainMenu := fyne.NewMainMenu(
		fileMenu,
		helpMenu,
	)

	g.window.SetMainMenu(mainMenu)
}

// showAddFilesDialog shows the file picker dialog
func (g *ConverterGUI) showAddFilesDialog() {
	d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		reader.Close()

		// Get the path from URI
		uri := reader.URI()
		path := uri.Path()

		g.AddFiles([]string{path})
	}, g.window)

	d.SetFilter(storage.NewExtensionFileFilter([]string{".vcv", ".mrk"}))
	d.Show()
}

// showAddFolderDialog shows the folder picker dialog
func (g *ConverterGUI) showAddFolderDialog() {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}

		path := uri.Path()
		files, err := os.ReadDir(path)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}

		var patchFiles []string
		for _, file := range files {
			if file.IsDir() {
				// Check for .mrk bundle directories
				if filepath.Ext(file.Name()) == ".mrk" {
					patchFiles = append(patchFiles, filepath.Join(path, file.Name()))
				}
			} else {
				ext := filepath.Ext(file.Name())
				if ext == ".vcv" {
					patchFiles = append(patchFiles, filepath.Join(path, file.Name()))
				}
			}
		}

		if len(patchFiles) == 0 {
			dialog.ShowInformation(
				"No Files Found",
				"No .vcv or .mrk files found in the selected directory.",
				g.window,
			)
			return
		}

		g.AddFiles(patchFiles)
	}, g.window)

	d.Show()
}

// showAboutDialog shows the about dialog
func (g *ConverterGUI) showAboutDialog() {
	dialog.ShowInformation(
		"About RackConverter",
		fmt.Sprintf("RackConverter v%s\n\nConvert between VCV Rack and MiRack patch formats.\n\nSupported formats:\n• VCV Rack v2\n• VCV Rack v0.6\n• MiRack", Version),
		g.window,
	)
}

// getFormatBadge returns a format badge string for a file path
func (g *ConverterGUI) getFormatBadge(path string) string {
	format := g.detectFormatFromPath(path)

	switch format {
	case converter.FormatVCV2:
		return "v2"
	case converter.FormatVCV06:
		return "v0.6"
	case converter.FormatMiRack:
		return "MiRack"
	default:
		return "?"
	}
}
