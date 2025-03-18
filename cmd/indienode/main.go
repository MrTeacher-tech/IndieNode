package main

import (
	"IndieNode/db/orbitdb"
	"IndieNode/internal/api"
	"IndieNode/internal/dev"
	"IndieNode/internal/services/auth"
	"IndieNode/internal/services/shop"
	"IndieNode/internal/ui/theme"
	"IndieNode/internal/ui/windows"
	"IndieNode/ipfs"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/app"
	iface_ipfs "github.com/ipfs/interface-go-ipfs-core"
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
	apiFlag := flag.Bool("api", false, "Start only the API server without the UI")
	portFlag := flag.Int("port", 8080, "Port to run development server on")
	apiPortFlag := flag.Int("api-port", 8000, "Port to run the API server on")
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

	// Initialize IPFS manager first
	ipfsMgr, err := ipfs.NewIPFSManager(&ipfs.Config{
		CustomGateways: []string{"http://localhost:5001"},
	})
	if err != nil {
		log.Fatalf("Failed to initialize IPFS manager: %v", err)
	}

	// Start IPFS daemon when app starts
	if err := ipfsMgr.StartDaemon(); err != nil {
		log.Printf("Warning: Failed to start IPFS daemon: %v", err)
	}

	// Initialize OrbitDB manager - this is needed for the API server
	orbitConfig := &orbitdb.Config{
		Directory: filepath.Join(".", "db", "orbitdb", "data"),
	}

	// Get the IPFS CoreAPI for OrbitDB
	ipfsNode, err := ipfsMgr.GetIPFSNode()
	if err != nil {
		log.Fatalf("Failed to get IPFS node: %v", err)
	}

	// Type assertion to get the CoreAPI
	ipfsCoreAPI, ok := ipfsNode.(iface_ipfs.CoreAPI)
	if !ok {
		log.Fatalf("Invalid IPFS node type: expected CoreAPI")
	}

	orbitMgr, err := orbitdb.NewManager(context.Background(), orbitConfig, ipfsCoreAPI)
	if err != nil {
		log.Fatalf("Failed to initialize OrbitDB manager: %v", err)
	}

	// Start the API server either in standalone mode or alongside the UI
	apiServer := api.NewServer(orbitMgr, *apiPortFlag)

	// If API-only mode is requested, start the API server and exit
	if *apiFlag {
		log.Printf("Starting API server on port %d...", *apiPortFlag)
		if err := apiServer.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
		return
	}

	// Otherwise, start API server in a goroutine and continue with UI
	go func() {
		log.Printf("Starting API server on port %d in background...", *apiPortFlag)
		if err := apiServer.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("API server error: %v", err)
		}
	}()

	// Initialize shop manager with IPFS manager for UI
	shopBaseDir := filepath.Join(".", "shops")
	shopMgr, err := shop.NewManager(shopBaseDir, ipfsMgr)
	if err != nil {
		log.Fatalf("Failed to initialize shop manager: %v", err)
	}

	// Continue with UI initialization
	mainApp := app.NewWithID("com.mrteacher.indienode")
	mainApp.Settings().SetTheme(theme.NewIndieNodeTheme())

	authSvc := auth.NewService()

	if auth.IsDevMode() {
		log.Printf("Running in DEV_MODE")
		// Copy dev shop template to current_shop.json
		if err := copyDevShopToCurrentShop(shopBaseDir); err != nil {
			log.Printf("Warning: Failed to copy dev shop template: %v", err)
		}

		// Skip login in dev mode
		authSvc.SetDevModeUser()
		mainWindow := windows.NewMainWindow(mainApp, shopMgr, ipfsMgr, authSvc, orbitMgr, apiServer, *apiPortFlag)
		mainWindow.SetCloseIntercept(func() {
			// Gracefully shut down API server when closing the app
			ctx, cancel := context.WithTimeout(context.Background(), 5000)
			defer cancel()
			apiServer.Stop(ctx)

			// Close the window
			mainWindow.Close()
		})
		mainWindow.Show()
	} else {
		// Create login window first
		loginWindow := windows.NewLoginWindow(mainApp, authSvc, func() {
			// This is called after successful login
			mainWindow := windows.NewMainWindow(mainApp, shopMgr, ipfsMgr, authSvc, orbitMgr, apiServer, *apiPortFlag)
			mainWindow.SetCloseIntercept(func() {
				// Gracefully shut down API server when closing the app
				ctx, cancel := context.WithTimeout(context.Background(), 5000)
				defer cancel()
				apiServer.Stop(ctx)

				// Close the window
				mainWindow.Close()
			})
			mainWindow.Show()
		})
		loginWindow.Show()
	}

	// Run the application
	mainApp.Run()
}
