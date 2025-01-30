package main

import (
	"IndieNode/internal/dev"
	"IndieNode/internal/services/auth"
	"IndieNode/internal/services/shop"
	"IndieNode/internal/ui/theme"
	"IndieNode/internal/ui/windows"
	"IndieNode/ipfs"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/app"
)

// copyDevShopToCurrentShop copies the dev shop template to current_shop.json
func copyDevShopToCurrentShop(shopBaseDir string) error {
	// Convert shopBaseDir to absolute path
	absShopBaseDir, err := filepath.Abs(shopBaseDir)
	if err != nil {
		return err
	}
	
	// Get project root (parent of shops directory)
	projectRoot := filepath.Dir(absShopBaseDir)
	
	// Source and destination paths
	srcPath := filepath.Join(projectRoot, "internal", "dev", "dev_shop.json")
	dstPath := filepath.Join(absShopBaseDir, "current_shop.json")

	log.Printf("Copying dev shop from: %s", srcPath)
	log.Printf("Copying dev shop to: %s", dstPath)

	// Ensure shops directory exists
	if err := os.MkdirAll(absShopBaseDir, 0755); err != nil {
		return err
	}

	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the contents
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	log.Printf("Successfully copied dev shop template")
	return nil
}

func main() {
	// Add command line flags
	serveFlag := flag.Bool("serve", false, "Start development server for the current shop")
	portFlag := flag.Int("port", 8080, "Port to run development server on")
	flag.Parse()

	// If serve flag is set, start the development server
	if *serveFlag {
		shopBaseDir := filepath.Join(".", "shops")
		log.Printf("Starting development server...")
		if err := dev.ServeCurrentShop(shopBaseDir, *portFlag); err != nil {
			log.Fatalf("Failed to start development server: %v", err)
		}
		return
	}

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
		log.Printf("Running in DEV_MODE")
		// Copy dev shop template to current_shop.json
		if err := copyDevShopToCurrentShop(shopBaseDir); err != nil {
			log.Printf("Warning: Failed to copy dev shop template: %v", err)
		}
		
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
