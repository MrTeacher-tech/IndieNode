package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type IndieNodeTheme struct{}

func NewIndieNodeTheme() fyne.Theme {
	return &IndieNodeTheme{}
}

func (t *IndieNodeTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return &color.NRGBA{R: 0xff, G: 0xfc, B: 0xe9, A: 0xff} // #fffce9 - warm off-white

	case theme.ColorNameForeground:
		return &color.NRGBA{R: 0x1d, G: 0x1d, B: 0x1d, A: 0xff} // #1d1d1d - dark gray

	case theme.ColorNamePrimary:
		return &color.NRGBA{R: 0x5a, G: 0xd9, B: 0xd5, A: 0xff} // #5ad9d5 - turquoise

	case theme.ColorNameButton:
		return &color.NRGBA{R: 0x5a, G: 0xd9, B: 0xd5, A: 0xff} // Same as primary

	case theme.ColorNameDisabled:
		return &color.NRGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff} // #cccccc - light gray

	case theme.ColorNamePlaceHolder:
		return &color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff} // #808080 - medium gray

	case theme.ColorNameShadow:
		return &color.NRGBA{R: 0, G: 0, B: 0, A: 0x40} // 25% opacity black

	case theme.ColorNameInputBackground:
		return &color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff} // #ffffff - white

	case theme.ColorNameHover:
		return &color.NRGBA{R: 0x6b, G: 0xea, B: 0xe6, A: 0xff} // #6beae6 - lighter turquoise

	case theme.ColorNameSelection:
		return &color.NRGBA{R: 0x5a, G: 0xd9, B: 0xd5, A: 0x80} // Primary with 50% opacity

	case theme.ColorNameOverlayBackground:
		return &color.NRGBA{R: 0xff, G: 0xfc, B: 0xe9, A: 0xf0} // Background with 94% opacity
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (t *IndieNodeTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *IndieNodeTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *IndieNodeTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
