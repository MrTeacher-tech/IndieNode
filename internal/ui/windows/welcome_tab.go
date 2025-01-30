package windows

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func NewWelcomeTab() *container.TabItem {
	// Load the logo image
	logo := canvas.NewImageFromFile("IndieNode_assets/indieNode_logo.png")
	logo.SetMinSize(fyne.NewSize(200, 200))
	logo.FillMode = canvas.ImageFillContain
	logo.Resize(fyne.NewSize(200, 200))

	// Create welcome text
	welcomeTitle := widget.NewLabelWithStyle("Welcome to IndieNode!", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	welcomeTitle.TextStyle.Bold = true
	welcomeTitle.Resize(fyne.NewSize(400, 40))

	descriptionText := `IndieNode is your decentralized marketplace platform that empowers creators to build and manage their own digital shops.

Key Features:
- 🚀 Create Your Shop – No coding needed! Just add your products and launch your storefront in minutes.
- 🌍 Host Instantly – Hit publish, and your shop goes live. Keep the app open, get a shareable URL, and start selling!
- 💰 Keep 100% of Your Sales – With seamless MetaMask integration, payments go directly to you—no platform cuts, no hidden fees.
- 🎉 Ready to go? Click ‘Create Shop’ and start selling today!`

	description := widget.NewTextGridFromString(descriptionText)

	// Create layout with padding
	content := container.NewPadded(
		container.NewVBox(
			container.NewCenter(logo),
			container.NewPadded(
				container.NewVBox(
					container.NewCenter(welcomeTitle),
					widget.NewSeparator(),
					container.NewPadded(description),
				),
			),
		),
	)

	return container.NewTabItem("Welcome", content)
}
