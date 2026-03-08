package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
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

// Status indicators - use ASCII for compatibility
const (
	StatusReady     = "[OK]"
	StatusWarn      = "[!]"
	StatusError     = "[X]"
	StatusSkipped   = "[--]"
	StatusConverted = "[OK]"
)

// createMainLayout creates the main two-panel layout
func (g *ConverterGUI) createMainLayout() *fyne.Container {
	// Create menu bar
	g.createMenuBar()

	// Left column: Input Queue (top) + Log (middle) + Output controls (bottom)
	leftColumn := g.createLeftColumn()

	// Right column: Global Settings (top) + Patch Inspector (middle) + Convert button + version (bottom)
	rightColumn := g.createRightColumn()
	rightColumn.Resize(fyne.NewSize(minSidebarWidth, rightColumn.MinSize().Height))

	// Use HSplit for resizable panels with proper ratio
	split := container.NewHSplit(leftColumn, rightColumn)
	split.SetOffset(0.7) // Left panel gets 70%, right gets 30%

	return container.NewBorder(nil, nil, nil, nil, split)
}

// createLeftColumn creates the left column with Input Queue, Log, and Output controls
func (g *ConverterGUI) createLeftColumn() *fyne.Container {
	g.addBtn = widget.NewButton("Add +", func() {
		g.showAddFilesDialog()
	})
	g.addBtn.Importance = widget.MediumImportance

	g.removeBtn = widget.NewButton("Remove -", func() {
		g.RemoveSelected()
	})
	g.removeBtn.Disable()

	g.clearBtn = widget.NewButton("Clear x", func() {
		g.ClearFiles()
	})
	g.clearBtn.Disable()

	buttonRow := container.NewGridWithColumns(3, g.addBtn, g.removeBtn, g.clearBtn)

	// File list with status icon on left (fixed width), file name on right
	g.fileList = widget.NewList(
		func() int {
			return len(g.inputFiles)
		},
		func() fyne.CanvasObject {
			// Template: status icon on left (fixed width), file name on right
			statusIcon := widget.NewLabel(StatusReady)
			statusIcon.Alignment = fyne.TextAlignLeading
			statusIcon.Resize(fyne.NewSize(50, 32))

			fileNameLabel := widget.NewLabel("FileName.vcv")
			fileNameLabel.TextStyle = fyne.TextStyle{Bold: true}

			// Use HBox with fixed-width status column
			row := container.NewHBox(
				container.NewVBox(statusIcon),
				fileNameLabel,
			)
			row.Resize(fyne.NewSize(200, 32))
			return row
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= 0 && id < len(g.inputFiles) {
				path := g.inputFiles[id]
				name := filepath.Base(path)
				hbox := obj.(*fyne.Container)

				// Status icon (left, fixed width)
				statusContainer := hbox.Objects[0].(*fyne.Container)
				statusLabel := statusContainer.Objects[0].(*widget.Label)

				// File name (right)
				nameLabel := hbox.Objects[1].(*widget.Label)
				nameLabel.Text = name

				// Set status icon based on file status
				if status, ok := g.fileStatuses[path]; ok {
					switch status.Status {
					case "Ready":
						statusLabel.Text = StatusReady
					case "Warn":
						statusLabel.Text = StatusWarn
					case "Error":
						statusLabel.Text = StatusError
					case "Skipped":
						statusLabel.Text = StatusSkipped
					case "Converted":
						statusLabel.Text = StatusConverted
					default:
						statusLabel.Text = StatusReady
					}
				} else {
					statusLabel.Text = StatusReady
				}

				statusLabel.Refresh()
				nameLabel.Refresh()
			}
		},
	)

	// Handle file selection
	g.fileList.OnSelected = func(id widget.ListItemID) {
		g.SelectFile(id)
	}

	// Drag and drop overlay - shown when list is empty
	dndOverlay := container.NewVBox(
		container.NewCenter(
			container.NewVBox(
				widget.NewIcon(theme.DownloadIcon()),
				widget.NewLabel("Drag & drop files here"),
				widget.NewLabel("or click '+ Add'"),
			),
		),
	)
	dndOverlay.Hide()

	// Container for file list with overlay
	listContainer := container.NewStack(g.fileList, dndOverlay)

	// LOG section
	logHeaderRich := widget.NewRichTextFromMarkdown("### LOG")
	logHeader := container.NewPadded(logHeaderRich)
	logHeader.Resize(fyne.NewSize(200, 30))

	g.logWidget = widget.NewMultiLineEntry()
	g.logWidget.SetPlaceHolder("Activity log...")
	g.logWidget.Disable()
	g.logWidget.Wrapping = fyne.TextWrapWord
	g.logWidget.Resize(fyne.NewSize(200, minLogHeight))

	logContainer := container.NewVBox(
		logHeader,
		widget.NewSeparator(),
	)
	logContainer.Add(g.logWidget)
	logContainer.Resize(fyne.NewSize(200, minLogHeight+50))
	logContainer = container.NewPadded(logContainer)

	// OUTPUT section (at bottom of left column)
	outputLabel := widget.NewLabel("Output:")

	g.statusBarOutput = widget.NewLabel("Same as input")
	g.statusBarOutput.TextStyle = fyne.TextStyle{Italic: true}

	g.browseBtn = widget.NewButton("Browse...", func() {
		g.SelectOutputDirectory()
	})

	outputRow := container.NewBorder(
		nil, nil, outputLabel, g.browseBtn, g.statusBarOutput,
	)
	outputRow = container.NewPadded(outputRow)

	outputSection := container.NewVBox(
		widget.NewSeparator(),
		outputRow,
	)
	outputSection = container.NewPadded(outputSection)

	// Top section: buttons only
	topSection := container.NewVBox(
		buttonRow,
		widget.NewSeparator(),
	)
	topSection = container.NewPadded(topSection)

	// Use VSplit for vertical split between list+log and output
	// The list and log share a VSplit, output is at the bottom
	middleSplit := container.NewVSplit(
		container.NewBorder(topSection, nil, nil, nil, listContainer),
		logContainer,
	)
	middleSplit.SetOffset(0.7) // List gets 70%, log gets 30%

	// Left column: list/log in middle, output at bottom
	leftColumn := container.NewBorder(
		nil,           // Top
		outputSection, // Bottom
		nil,           // Left
		nil,           // Right
		middleSplit,   // Center (expands)
	)

	// Store reference to overlay for show/hide
	g.dndOverlay = dndOverlay
	g.updateDndOverlay()

	return leftColumn
}

// updateDndOverlay shows/hides the drag-and-drop overlay based on file count
func (g *ConverterGUI) updateDndOverlay() {
	if g.dndOverlay == nil {
		return
	}
	if len(g.inputFiles) == 0 {
		g.dndOverlay.Show()
	} else {
		g.dndOverlay.Hide()
	}
}

// updateStatusBar updates the output label in the left column
func (g *ConverterGUI) updateStatusBar() {
	if g.statusBarOutput == nil {
		return
	}

	if g.outputDir == "" {
		g.statusBarOutput.SetText("Same as input")
	} else {
		g.statusBarOutput.SetText(g.outputDir)
	}
}

// createRightColumn creates the right column with Settings, Inspector, Convert button, and version
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

	// CONVERT button
	g.convertBtn = widget.NewButton("CONVERT", func() {
		go g.StartConversion()
	})
	g.convertBtn.Importance = widget.HighImportance
	g.convertBtn.Disable()

	// Wrap button in padding to avoid being too close to edge
	convertBtnWithPadding := container.NewPadded(g.convertBtn)

	// Middle section: inspector with progress bar
	middleSection := container.NewVBox(
		separatorAboveInspector,
		patchInspector,
		g.progressBar,
	)
	middleSection.Resize(fyne.NewSize(minSidebarWidth, minInspectorHeight))

	// Right column using border layout
	rightColumn := container.NewBorder(
		globalSettings,        // Top
		convertBtnWithPadding, // Bottom
		nil,                   // Left
		nil,                   // Right
		middleSection,         // Center (expands)
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

	// Combine all (output controls moved to status bar)
	vbox := container.NewVBox(
		header,
		widget.NewSeparator(),
		targetRow,
		widget.NewSeparator(),
		optionsBox,
	)

	return container.NewPadded(vbox)
}

// createPatchInspectorPanel creates the Patch Inspector panel
func (g *ConverterGUI) createPatchInspectorPanel() *fyne.Container {
	header := widget.NewRichTextFromMarkdown("### PATCH INSPECTOR")

	// File name
	g.inspectorFileName = widget.NewLabel("No file selected")
	g.inspectorFileName.TextStyle = fyne.TextStyle{Bold: true}

	// Overview
	g.inspectorOverview = widget.NewLabel("")

	// Contents (module list) - use regular Label for tighter spacing
	contentsLabel := widget.NewLabel("Contents:")

	g.inspectorContents = widget.NewLabel("No file selected")

	// Scrollable container for contents
	contentsScroll := container.NewScroll(g.inspectorContents)
	contentsScroll.SetMinSize(fyne.NewSize(200, 100))

	// Contents section (label + scroll) - stored separately for show/hide
	g.inspectorContentsContainer = container.NewVBox(
		contentsLabel,
		contentsScroll,
	)
	// Initially hide contents when no file is selected
	g.inspectorContentsContainer.Hide()

	// Status note
	g.inspectorStatus = widget.NewLabel("")
	g.inspectorStatus.Wrapping = fyne.TextWrapWord

	// Combine all
	vbox := container.NewVBox(
		header,
		widget.NewSeparator(),
		g.inspectorFileName,
		g.inspectorOverview,
		widget.NewSeparator(),
		g.inspectorContentsContainer, // This will be shown/hidden
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
	aboutText := fmt.Sprintf("vRackConverter v%s\n\n", Version)
	aboutText += "Convert between virtual modular synthesizer patches.\n\n"
	aboutText += "Supported formats:\n• VCV Rack v2\n• VCV Rack v0.6\n• MiRack\n\n"
	aboutText += "Source: https://github.com/vrackconverter/vrackconverter\n\n"
	aboutText += fmt.Sprintf("Build: %s", BuildInfo)

	dialog.ShowInformation(
		"About vRackConverter",
		aboutText,
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
