package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
)

var (
	// Version is injected by build
	Version = "dev"
	// BuildInfo is injected by build
	BuildInfo = "dev build"
)

func main() {
	fyneApp := app.NewWithID("com.vrackconverter.gui")
	fyneApp.Settings().SetTheme(&RackConverterTheme{})

	gui := NewConverterGUI(fyneApp)

	window := fyneApp.NewWindow("vRackConverter - " + Version)
	gui.SetWindow(window)
	window.SetContent(gui.MakeUI())

	// Enable drag and drop for files
	gui.EnableDragAndDrop()

	window.Resize(fyne.NewSize(900, 600))
	window.CenterOnScreen()
	window.ShowAndRun()
}

// RackConverterTheme provides a clean, modern theme for the application
type RackConverterTheme struct{}

func (t *RackConverterTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return &color.NRGBA{R: 0x1a, G: 0x1a, B: 0x2e, A: 0xff} // Dark blue
	case theme.ColorNameBackground:
		if variant == theme.VariantDark {
			return &color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1e, A: 0xff}
		}
		return &color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf7, A: 0xff} // Light gray
	case theme.ColorNameForeground:
		if variant == theme.VariantDark {
			return &color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf7, A: 0xff}
		}
		return &color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff} // Dark gray
	case theme.ColorNameButton:
		return &color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff} // Blue
	case theme.ColorNameDisabled:
		return &color.NRGBA{R: 0xbd, G: 0xbd, B: 0xbd, A: 0xff} // Gray
	case theme.ColorNameHover:
		return &color.NRGBA{R: 0x60, G: 0xa5, B: 0xfa, A: 0xff} // Light blue
	case theme.ColorNameError:
		return &color.NRGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff} // Red
	case theme.ColorNameSuccess:
		return &color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff} // Green
	case theme.ColorNameWarning:
		return &color.NRGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff} // Orange
	case theme.ColorNameSeparator:
		if variant == theme.VariantDark {
			return &color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2e, A: 0xff}
		}
		return &color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (t *RackConverterTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *RackConverterTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *RackConverterTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
