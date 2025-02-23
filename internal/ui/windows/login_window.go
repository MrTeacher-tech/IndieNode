package windows

import (
	"IndieNode/internal/services/auth"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// LoginWindow represents the MetaMask login window
type LoginWindow struct {
	app       fyne.App
	window    fyne.Window
	authSvc   *auth.Service
	onSuccess func()

	// UI elements
	statusLabel    *widget.Label
	connectButton  *widget.Button
	loadingSpinner *widget.ProgressBarInfinite
}

// NewLoginWindow creates a new login window
func NewLoginWindow(app fyne.App, authSvc *auth.Service, onSuccess func()) *LoginWindow {
	w := &LoginWindow{
		app:       app,
		authSvc:   authSvc,
		onSuccess: onSuccess,
	}

	// Create the window
	w.window = app.NewWindow("Connect with MetaMask")
	w.window.Resize(fyne.NewSize(400, 500))
	w.window.SetFixedSize(true)
	w.window.CenterOnScreen()

	// Start auth server before creating UI
	if err := w.authSvc.StartServer(3000, func(address, message, signature string) {
		// Handle successful authentication
		if err := w.authSvc.AuthenticateWithEthereum(address, message, signature); err != nil {
			w.showError("Authentication failed: " + err.Error())
			return
		}

		// Update UI for success
		w.statusLabel.SetText("Successfully connected!")
		w.loadingSpinner.Hide()

		// Wait a moment before closing
		time.Sleep(2 * time.Second)

		// Call success callback and close window
		if w.onSuccess != nil {
			w.onSuccess()
		}
		w.Close()
	}); err != nil {
		// Show error dialog on server start failure
		dialog.ShowError(fmt.Errorf("Failed to start authentication service: %v", err), w.window)
	}

	// Create UI
	w.createUI()

	return w
}

// createUI creates the login window UI
func (w *LoginWindow) createUI() {
	// Load and display the logo
	logoURI := storage.NewFileURI("IndieNode_assets/indieNode_logo.png")
	logoRes, err := storage.LoadResourceFromURI(logoURI)
	if err != nil {
		fmt.Printf("Error loading logo: %v\n", err)
		return
	}
	
	logo := canvas.NewImageFromResource(logoRes)
	logo.SetMinSize(fyne.NewSize(300, 300))
	logo.FillMode = canvas.ImageFillContain
	logo.ScaleMode = canvas.ImageScaleSmooth

	title := canvas.NewText("Welcome to IndieNode", nil)
	title.TextSize = 24
	title.TextStyle.Bold = true

	subtitle := widget.NewLabel("Connect your wallet to continue")
	w.statusLabel = widget.NewLabel("")
	w.statusLabel.Hide()

	w.loadingSpinner = widget.NewProgressBarInfinite()
	w.loadingSpinner.Hide()

	w.connectButton = widget.NewButton("Connect with MetaMask", w.handleMetaMaskLogin)

	// Layout
	content := container.NewVBox(
		container.NewCenter(logo),
		container.NewCenter(title),
		widget.NewSeparator(),
		container.NewCenter(subtitle),
		container.NewCenter(w.connectButton),
		container.NewCenter(w.loadingSpinner),
		container.NewCenter(w.statusLabel),
	)

	// Add padding
	paddedContent := container.NewPadded(content)
	w.window.SetContent(paddedContent)
}

// handleMetaMaskLogin handles the MetaMask login process
func (w *LoginWindow) handleMetaMaskLogin() {
	// Disable button and show loading state
	w.connectButton.Disable()
	w.loadingSpinner.Show()
	w.statusLabel.SetText("Connecting to MetaMask...")
	w.statusLabel.Show()
}

// showError displays an error message and resets the UI
func (w *LoginWindow) showError(message string) {
	w.statusLabel.SetText(message)
	w.statusLabel.Show()
	w.loadingSpinner.Hide()
	w.connectButton.Enable()
	
	// Show error dialog
	dialog.ShowError(fmt.Errorf(message), w.window)
}

// Show displays the login window
func (w *LoginWindow) Show() {
	w.window.Show()
}

// Close closes the login window and stops the auth server
func (w *LoginWindow) Close() {
	_ = w.authSvc.StopServer() // Ignore error as we're closing anyway
	w.window.Close()
}
