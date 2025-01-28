package main

import (
	"IndieNode/internal/services/auth"
	"IndieNode/internal/services/shop"
	"IndieNode/internal/ui/theme"
	"IndieNode/internal/ui/windows"
	"IndieNode/ipfs"
	"log"
	"path/filepath"

	"fyne.io/fyne/v2/app"
)

func main() {
	mainApp := app.NewWithID("com.mrteacher.indienode")
	mainApp.Settings().SetTheme(theme.NewIndieNodeTheme())

	// Initialize services
	shopBaseDir := filepath.Join(".", "shops")

	// Initialize IPFS manager first
	ipfsMgr, err := ipfs.NewIPFSManager(&ipfs.Config{
		CustomGateways: []string{"http://localhost:5001"},
	})
	if err != nil {
		log.Fatalf("Failed to initialize IPFS manager: %v", err)
	}

	// Initialize shop manager with IPFS manager
	shopMgr, err := shop.NewManager(shopBaseDir, ipfsMgr)
	if err != nil {
		log.Fatalf("Failed to initialize shop manager: %v", err)
	}

	authSvc := auth.NewService()

	if auth.IsDevMode() {
		// Skip login in dev mode
		authSvc.SetDevModeUser()
		mainWindow := windows.NewMainWindow(mainApp, shopMgr, ipfsMgr, authSvc)
		mainWindow.Show()
	} else {
		// Create login window first
		loginWindow := windows.NewLoginWindow(mainApp, authSvc, func() {
			// This is called after successful login
			mainWindow := windows.NewMainWindow(mainApp, shopMgr, ipfsMgr, authSvc)
			mainWindow.Show()
		})
		loginWindow.Show()
	}

	// Run the application
	mainApp.Run()
}
