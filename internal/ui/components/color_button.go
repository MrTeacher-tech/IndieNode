package components

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/lusingander/colorpicker"
)

// ColorButton represents a button that opens a color picker when tapped
type ColorButton struct {
	widget.Button
	currentColor color.RGBA
	window       fyne.Window
	onSelected   func(color.Color)
	background   *canvas.Rectangle
}

// NewColorButton creates a new color button with the given initial color
func NewColorButton(label string, initialColor color.RGBA, window fyne.Window, onSelected func(color.Color)) *ColorButton {
	button := &ColorButton{
		currentColor: initialColor,
		window:       window,
		onSelected:   onSelected,
		background:   canvas.NewRectangle(initialColor),
	}
	button.ExtendBaseWidget(button)
	button.Text = label
	return button
}

// Color returns the currently selected color
func (b *ColorButton) Color() color.Color {
	return b.currentColor
}

// SetColor sets the current color
func (b *ColorButton) SetColor(c color.Color) {
	if rgba, ok := c.(color.RGBA); ok {
		b.currentColor = rgba
		b.background.FillColor = rgba
		b.background.Refresh()
		b.Refresh() // Refresh the entire button to update text contrast
	}
}

// OnChanged sets the callback for when the color changes
func (b *ColorButton) OnChanged(callback func(color.Color)) {
	b.onSelected = callback
}

// CreateRenderer implements fyne.Widget
func (b *ColorButton) CreateRenderer() fyne.WidgetRenderer {
	b.ExtendBaseWidget(b)
	if b.background == nil {
		b.background = canvas.NewRectangle(b.currentColor)
	}

	text := canvas.NewText(b.Text, color.Black)
	text.TextStyle = fyne.TextStyle{Bold: true}

	return &colorButtonRenderer{
		button:     b,
		background: b.background,
		text:       text,
		objects:    []fyne.CanvasObject{b.background, text},
	}
}

// Tapped handles the button tap event
func (b *ColorButton) Tapped(_ *fyne.PointEvent) {
	if b.window == nil {
		return
	}

	picker := colorpicker.New(200, colorpicker.StyleHue)
	// Convert our RGBA color to a normalized color.Color
	r, g, bb, a := float64(b.currentColor.R)/255, float64(b.currentColor.G)/255, float64(b.currentColor.B)/255, float64(b.currentColor.A)/255
	picker.SetColor(color.NRGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(bb * 255),
		A: uint8(a * 255),
	})
	content := container.NewVBox(picker)

	d := dialog.NewCustom("Choose Color", "Done", content, b.window)
	
	// Update button immediately when color changes
	picker.SetOnChanged(func(c color.Color) {
		var newColor color.RGBA
		if rgba, ok := c.(color.RGBA); ok {
			// If we get an RGBA color directly, use it
			newColor = rgba
		} else {
			// Otherwise convert from the 16-bit color values
			r, g, b, _ := c.RGBA()
			newColor = color.RGBA{
				R: uint8((r * 255) / 65535),
				G: uint8((g * 255) / 65535),
				B: uint8((b * 255) / 65535),
				A: 255,
			}
		}
		b.SetColor(newColor)
		if b.onSelected != nil {
			b.onSelected(newColor)
		}
	})

	d.Show()
}

// SetWindow sets the parent window for the color picker dialog
func (b *ColorButton) SetWindow(w fyne.Window) {
	b.window = w
}

// colorButtonRenderer handles the rendering of the ColorButton
type colorButtonRenderer struct {
	button     *ColorButton
	background *canvas.Rectangle
	text       *canvas.Text
	objects    []fyne.CanvasObject
}

func (r *colorButtonRenderer) MinSize() fyne.Size {
	return r.text.MinSize().Add(fyne.NewSize(40, 10))
}

func (r *colorButtonRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.background.Move(fyne.NewPos(0, 0))

	textSize := r.text.MinSize()
	textPos := fyne.NewPos(
		(size.Width-textSize.Width)/2,
		(size.Height-textSize.Height)/2,
	)
	r.text.Move(textPos)
	r.text.Resize(textSize)
}

func (r *colorButtonRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *colorButtonRenderer) Refresh() {
	r.text.Text = r.button.Text
	r.text.Color = getContrastingColor(r.button.currentColor)
	r.text.Refresh()
	r.background.FillColor = r.button.currentColor
	r.background.Refresh()
	r.Layout(r.button.Size())
}

func (r *colorButtonRenderer) Destroy() {
	// No cleanup needed
}

// getContrastingColor returns either black or white depending on the background color
func getContrastingColor(bg color.RGBA) color.Color {
	// Calculate relative luminance using the formula from WCAG 2.0
	luminance := (0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)) / 255.0
	if luminance > 0.5 {
		return color.Black
	}
	return color.White
}
