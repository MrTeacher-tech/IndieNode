package main

import (
	"fmt"
	"image/color"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format

	"log"

	"os"
	"path/filepath"
	"runtime"

	"strings"

	"time"

	"shopCreator/db"
	"shopCreator/db/orbitdb"
	"shopCreator/internal/models"
	"shopCreator/internal/services/auth"
	"shopCreator/internal/services/shop"
	"shopCreator/internal/ui/components"
	"shopCreator/internal/ui/windows"
	"shopCreator/ipfs"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"golang.org/x/net/context"
)

// App configuration
const (
	appWidth  = 800
	appHeight = 600
)

// Global variables for application state
var (
	shopsContainer      *fyne.Container
	mainApp             fyne.App
	mainWindow          fyne.Window
	mainIPFSStatusLabel *widget.Label
	shopManager         *shop.Manager
	shopGenerator       *shop.Generator
	ipfsManager         *ipfs.IPFSManager
	orbitDBManager      *orbitdb.Manager
)

func init() {
	var err error
	// Initialize shop manager
	shopManager, err = shop.NewManager(".")
	if err != nil {
		log.Fatalf("Failed to initialize shop manager: %v", err)
	}

	// Initialize shop generator
	shopGenerator, err = shop.NewGenerator("templates")
	if err != nil {
		log.Fatalf("Failed to initialize shop generator: %v", err)
	}
}

func updateShopsList(window fyne.Window) {
	if shopsContainer == nil {
		return
	}

	shopsContainer.Objects = nil // Clear existing items

	shops, err := shopManager.ListShops()
	if err != nil {
		dialog.ShowError(err, window)
		return
	}

	for _, shopName := range shops {
		name := shopName // Create new variable for closure
		shopBtn := widget.NewButton(name, func() {
			shop, err := shopManager.LoadShop(name)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			showShopCreator(window, shop)
		})
		shopsContainer.Add(shopBtn)
	}
}

func showShopCreator(window fyne.Window, existingShop *models.Shop) fyne.CanvasObject {
	var shop *models.Shop
	var err error

	if existingShop != nil {
		shop = existingShop
	} else {
		// Load or create new shop
		shop, err = shopManager.LoadCurrentShop()
		if err != nil {
			dialog.ShowError(err, window)
			return nil
		}
	}

	// Track original image paths and their intended relative paths
	type ImageMapping struct {
		OriginalPath string
		RelativePath string
	}
	var currentImages []ImageMapping

	var addItemFunc func()
	var addItemBtn *widget.Button
	var itemsList *widget.List
	var previewContainer *fyne.Container

	// Shop details
	shopName := widget.NewEntry()
	shopName.SetPlaceHolder("Shop Name")
	shopName.OnChanged = func(name string) {
		shop.Name = name
	}

	shopDescription := widget.NewMultiLineEntry()
	shopDescription.SetPlaceHolder("Shop Description")

	shopLocation := widget.NewEntry()
	shopLocation.SetPlaceHolder("Location (Optional)")

	shopEmail := widget.NewEntry()
	shopEmail.SetPlaceHolder("Email (Optional)")

	shopPhone := widget.NewEntry()
	shopPhone.SetPlaceHolder("Phone (Optional)")

	// Logo preview
	logoPreviewContainer := container.NewVBox()
	if shop.LogoPath != "" {
		img := canvas.NewImageFromFile(shop.LogoPath)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(200, 200))
		logoPreviewContainer.Add(img)
	}

	// Logo selection
	shopLogo := widget.NewEntry()
	shopLogo.SetPlaceHolder("Logo Path")
	logoBtn := widget.NewButton("Select Logo", func() {
		// Only validate shop name exists in memory
		if shop.Name == "" {
			dialog.ShowError(fmt.Errorf("please enter a shop name before adding images"), window)
			return
		}

		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				log.Printf("Error opening file dialog: %v", err)
				dialog.ShowError(err, window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			// Get and store the original source path
			sourcePath := reader.URI().Path()
			shop.LogoPath = sourcePath

			// Save current state
			if err := shopManager.SaveCurrentShop(shop); err != nil {
				dialog.ShowError(err, window)
				return
			}

			// Update preview
			logoPreviewContainer.Objects = nil
			if shop.LogoPath != "" {
				img := canvas.NewImageFromFile(shop.LogoPath)
				img.FillMode = canvas.ImageFillContain
				img.SetMinSize(fyne.NewSize(200, 200))
				logoPreviewContainer.Add(img)
			}
			logoPreviewContainer.Refresh()
		}, window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		fd.Show()
	})

	// Color pickers
	var primaryColorPicker, secondaryColorPicker, tertiaryColorPicker *components.ColorButton

	primaryColorPicker = components.NewColorButton("Primary Color", color.RGBA{0, 0, 0, 255}, window, func(c color.Color) {
		r, g, b, _ := c.RGBA()
		shop.PrimaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})
	secondaryColorPicker = components.NewColorButton("Secondary Color", color.RGBA{255, 255, 255, 255}, window, func(c color.Color) {
		r, g, b, _ := c.RGBA()
		shop.SecondaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})
	tertiaryColorPicker = components.NewColorButton("Tertiary Color", color.RGBA{128, 128, 128, 255}, window, func(c color.Color) {
		r, g, b, _ := c.RGBA()
		shop.TertiaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})

	// Optional shop details in accordion
	optionalDetails := widget.NewAccordion(
		widget.NewAccordionItem("Optional Settings", container.NewVBox(
			logoBtn,
			logoPreviewContainer,

			container.NewGridWithColumns(3,
				container.NewVBox(
					widget.NewLabel("Primary Color:"),
					primaryColorPicker,
				),
				container.NewVBox(
					widget.NewLabel("Secondary Color:"),
					secondaryColorPicker,
				),
				container.NewVBox(
					widget.NewLabel("Tertiary Color:"),
					tertiaryColorPicker,
				),
			),
		)),
	)

	shopDetailsBox := container.NewVBox(
		shopName,
		optionalDetails,
	)

	// Item details
	itemName := widget.NewEntry()
	itemName.SetPlaceHolder("Item Name")

	itemPrice := widget.NewEntry()

	priceContainer := container.NewHBox(itemPrice)
	priceContainer.Resize(fyne.NewSize(200, 40))

	itemDescription := widget.NewMultiLineEntry()
	itemDescription.SetPlaceHolder("Item Description (Optional)")

	// Image preview
	previewContainer = container.NewHBox()

	updatePreviewContainer := func() {
		previewContainer.Objects = nil
		for _, img := range currentImages {
			// Use the original path for preview
			imgObj := canvas.NewImageFromFile(img.OriginalPath)
			imgObj.SetMinSize(fyne.NewSize(100, 100))
			imgObj.Resize(fyne.NewSize(100, 100))
			previewContainer.Add(imgObj)
		}
		previewContainer.Refresh()
	}

	// Add image button
	addImageBtn := widget.NewButton("Add Image", func() {
		// Only validate shop name exists in memory
		if shop.Name == "" {
			dialog.ShowError(fmt.Errorf("please enter a shop name before adding images"), window)
			return
		}

		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				log.Printf("Error opening file dialog: %v", err)
				dialog.ShowError(err, window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			// Get and store the original source path
			sourcePath := reader.URI().Path()
			currentImages = append(currentImages, ImageMapping{
				OriginalPath: sourcePath,
				RelativePath: filepath.Join("assets", "images", filepath.Base(sourcePath)),
			})

			// Update preview using the original path
			updatePreviewContainer()
		}, window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		fd.Show()
	})

	// Add item function
	addItemFunc = func() {
		if itemName.Text == "" {
			dialog.ShowError(fmt.Errorf("item name is required"), window)
			return
		}

		price := 0.0
		if _, err := fmt.Sscanf(itemPrice.Text, "%f", &price); err != nil {
			dialog.ShowError(fmt.Errorf("invalid price format"), window)
			return
		}

		// Get the original paths for the item
		var photoPaths []string
		for _, img := range currentImages {
			photoPaths = append(photoPaths, img.OriginalPath)
		}

		newItem := models.Item{
			Name:            itemName.Text,
			Price:           price,
			Description:     itemDescription.Text,
			PhotoPaths:      photoPaths,
			LocalPhotoPaths: photoPaths,
		}

		shop.Items = append(shop.Items, newItem)

		itemName.SetText("")
		itemPrice.SetText("")
		itemDescription.SetText("")
		currentImages = nil
		updatePreviewContainer()

		itemsList.Refresh()
	}

	addItemBtn = widget.NewButton("Add Item", addItemFunc)

	// Create list for items
	itemsList = widget.NewList(
		func() int {
			return len(shop.Items)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			editBtn := widget.NewButton("Edit", nil)
			deleteBtn := widget.NewButton("Delete", nil)
			buttonBox := container.NewHBox(editBtn, deleteBtn)
			return container.NewBorder(nil, nil, nil, buttonBox, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			container := item.(*fyne.Container)
			label := container.Objects[0].(*widget.Label)
			buttonBox := container.Objects[1].(*fyne.Container)
			editBtn := buttonBox.Objects[0].(*widget.Button)
			deleteBtn := buttonBox.Objects[1].(*widget.Button)

			label.SetText(fmt.Sprintf("%s - $%.2f", shop.Items[id].Name, shop.Items[id].Price))

			editBtn.OnTapped = func() {
				itemName.SetText(shop.Items[id].Name)
				itemPrice.SetText(fmt.Sprintf("%.2f", shop.Items[id].Price))
				itemDescription.SetText(shop.Items[id].Description)
				currentImages = nil
				for _, path := range shop.Items[id].LocalPhotoPaths {
					currentImages = append(currentImages, ImageMapping{
						OriginalPath: path,
						RelativePath: filepath.Join("assets", "images", filepath.Base(path)),
					})
				}
				updatePreviewContainer()

				addItemBtn.SetText("Update Item")
				addItemBtn.OnTapped = func() {
					price := 0.0
					if _, err := fmt.Sscanf(itemPrice.Text, "%f", &price); err != nil {
						dialog.ShowError(fmt.Errorf("invalid price format"), window)
						return
					}

					// Get the original paths for the item
					var photoPaths []string
					for _, img := range currentImages {
						photoPaths = append(photoPaths, img.OriginalPath)
					}

					shop.Items[id] = models.Item{
						Name:            itemName.Text,
						Price:           price,
						Description:     itemDescription.Text,
						PhotoPaths:      photoPaths,
						LocalPhotoPaths: photoPaths,
					}

					itemName.SetText("")
					itemPrice.SetText("")
					itemDescription.SetText("")
					currentImages = nil
					updatePreviewContainer()

					addItemBtn.SetText("Add Item")
					addItemBtn.OnTapped = addItemFunc

					itemsList.Refresh()
				}
			}

			deleteBtn.OnTapped = func() {
				shop.Items = append(shop.Items[:id], shop.Items[id+1:]...)
				itemsList.Refresh()
			}
		},
	)

	// Layout for item fields
	itemFieldsBox := container.NewVBox(
		itemName,
		container.NewHBox(
			widget.NewLabel("Price: $"),
			container.NewPadded(priceContainer),
		),
	)

	// Create header with owner's Ethereum address
	ownerLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("Shop Owner: %s", auth.GetAuthenticatedUser().Address),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	content := container.NewMax(
		container.NewPadded(
			container.NewVBox(
				container.NewHBox(
					layout.NewSpacer(),
					ownerLabel,
					layout.NewSpacer(),
				),
				widget.NewSeparator(),
				shopDetailsBox,
				widget.NewSeparator(),
				widget.NewLabel("Add New Item:"),
				itemFieldsBox,
				itemDescription,
				addImageBtn,
				previewContainer,
				addItemBtn,
				widget.NewSeparator(),
				widget.NewLabel("Items:"),
				container.NewMax(itemsList),
			),
		),
	)

	// Pre-fill fields if editing existing shop
	if existingShop != nil {
		shopName.SetText(existingShop.Name)
		shopDescription.SetText(existingShop.Description)
		shopLocation.SetText(existingShop.Location)
		shopEmail.SetText(existingShop.Email)
		shopPhone.SetText(existingShop.Phone)
		shopLogo.SetText(existingShop.LogoPath)

		// Update logo preview if exists
		if existingShop.LogoPath != "" {
			logoPreviewContainer.Objects = nil
			img := canvas.NewImageFromFile(existingShop.LogoPath)
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(200, 200))
			logoPreviewContainer.Add(img)
			logoPreviewContainer.Refresh()
		}

		primaryColorPicker.SetText(fmt.Sprintf("Primary Color: #%02x%02x%02x%02x", existingShop.PrimaryColor.R, existingShop.PrimaryColor.G, existingShop.PrimaryColor.B, existingShop.PrimaryColor.A))
		secondaryColorPicker.SetText(fmt.Sprintf("Secondary Color: #%02x%02x%02x%02x", existingShop.SecondaryColor.R, existingShop.SecondaryColor.G, existingShop.SecondaryColor.B, existingShop.SecondaryColor.A))
		tertiaryColorPicker.SetText(fmt.Sprintf("Tertiary Color: #%02x%02x%02x%02x", existingShop.TertiaryColor.R, existingShop.TertiaryColor.G, existingShop.TertiaryColor.B, existingShop.TertiaryColor.A))
		itemsList.Refresh()
	}

	// Save button
	saveBtn := widget.NewButton("Save", func() {
		if err := shopManager.SaveCurrentShop(shop); err != nil {
			dialog.ShowError(err, window)
			return
		}
		dialog.ShowInformation("Success", "Shop saved successfully", window)
	})

	// Generate button
	generateBtn := widget.NewButton("Generate Shop", func() {
		outputDir := filepath.Join("shops", shop.Name)
		if err := shopGenerator.GenerateShop(shop, outputDir); err != nil {
			dialog.ShowError(err, window)
			return
		}
		dialog.ShowInformation("Success", "Shop generated successfully", window)
	})

	return container.NewVScroll(container.NewVBox(content, saveBtn, generateBtn))
}

func main() {
	// Initialize database
	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.DB.Close()

	mainApp = app.New()
	mainApp.Settings().SetTheme(newCustomTheme())

	// Set application icon
	icon, err := fyne.LoadResourceFromPath("IndieNode_assets/indieNode_logo.png")
	if err == nil {
		mainApp.SetIcon(icon)
	}

	// Initialize services
	shopMgr := shop.NewManager()
	authSvc := auth.NewService()

	// Set up dev mode if enabled
	if isDevMode() {
		// Create a test user in dev mode
		auth.SetCurrentUser(&auth.AuthenticatedUser{
			Address:  "0x37eA7944328DF1A4D7ffA6658A002d5C332c8113",
			LoggedIn: true,
		})
	}

	// Create the main window
	mainWindow := windows.NewMainWindow(mainApp, shopMgr, authSvc)

	// Show login window first, unless in dev mode
	if !isDevMode() {
		loginWindow := windows.NewLoginWindow(mainApp, authSvc, func() {
			// On successful login, show the main window
			mainWindow.Show()
		})
		loginWindow.Show()
	} else {
		// In dev mode, show main window directly
		mainWindow.Show()
	}

	// Start the application
	mainApp.Run()
}

func showMainWindow() {
	if mainWindow != nil {
		mainWindow.Close()
	}

	mainWindow = mainApp.NewWindow("IndieNode - Shop Manager")
	mainWindow.Resize(fyne.NewSize(appWidth, appHeight))

	// Check authentication status
	if !auth.IsAuthenticated() {
		showLoginWindow()
		return
	}

	// Create header with Ethereum address and IPFS status
	addressLabel := widget.NewLabelWithStyle(
		func() string {
			return fmt.Sprintf("Connected: %s", auth.GetAuthenticatedUser().Address)
		}(),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Initialize IPFS manager if not already initialized
	if ipfsManager == nil {
		var err error
		ipfsManager, err = ipfs.NewIPFSManager(&ipfs.Config{})
		if err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}

		// Try to connect to existing daemon
		if err = ipfsManager.InitializeExistingDaemon(); err != nil {
			log.Printf("No IPFS daemon detected: %v", err)
		} else {
			log.Printf("Successfully connected to existing IPFS daemon")
			if mainIPFSStatusLabel != nil {
				updateIPFSStatus(mainIPFSStatusLabel, nil)
			}
		}
	}

	// Create IPFS status label
	mainIPFSStatusLabel = widget.NewLabelWithStyle(
		"Checking IPFS status...",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	// Update IPFS status
	go func() {
		updateIPFSStatus(mainIPFSStatusLabel, nil) // Pass nil for the button since we don't have one in the header
	}()

	// Create scrollable containers for each tab with proper padding and expansion
	shopsScroll := container.NewMax(
		container.NewPadded(
			container.NewScroll(createShopsTab()),
		),
	)

	createShopScroll := container.NewMax(
		container.NewPadded(
			container.NewScroll(showShopCreator(mainWindow, nil)),
		),
	)

	settingsScroll := container.NewMax(
		container.NewPadded(
			container.NewScroll(createSettingsTab()),
		),
	)

	// Create tabs with expanded content
	tabs := container.NewAppTabs(
		container.NewTabItem("My Shops", shopsScroll),
		container.NewTabItem("Create Shop", createShopScroll),
		container.NewTabItem("Settings", settingsScroll),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Create main content with proper spacing
	content := container.NewBorder(
		container.NewVBox(
			container.NewHBox(
				layout.NewSpacer(),
				addressLabel,
				layout.NewSpacer(),
			),
			widget.NewSeparator(),
			mainIPFSStatusLabel,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		tabs,
	)

	mainWindow.SetContent(content)
	mainWindow.SetOnClosed(func() {
		// Stop IPFS daemon if it's running
		if ipfsManager != nil && ipfsManager.IsDaemonRunning() {
			if err := ipfsManager.StopDaemon(); err != nil {
				log.Printf("Error stopping IPFS daemon: %v", err)
			}
		}
		mainWindow = nil
	})
	mainWindow.Show()
}

func safeShowMainWindow() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in showMainWindow: %v", r)
			// Try to show an error dialog
			if mainApp != nil {
				errWin := mainApp.NewWindow("Error")
				errWin.Resize(fyne.NewSize(300, 100))
				errWin.SetContent(widget.NewLabel("An error occurred. Please restart the application."))
				errWin.Show()
			}
		}
	}()

	// Let showMainWindow handle all window management
	showMainWindow()
}

func createShopsTab() fyne.CanvasObject {
	content := container.NewVBox()

	// Initialize shops container
	shopsContainer = container.NewVBox()

	// Initial shop list load
	updateShopsList(mainWindow)

	content.Add(widget.NewLabel("Your Shops"))
	content.Add(shopsContainer)

	return content
}

func createSettingsTab() fyne.CanvasObject {
	content := container.NewVBox()

	// Create IndieNode Settings section
	indieNodeCard := widget.NewCard("IndieNode Settings", "", nil)

	// Create version label
	versionLabel := widget.NewLabelWithStyle(
		"Version: pre-alpha",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Create Go version label
	goVersionLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("Go Version: %s", runtime.Version()),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Create DEV_MODE label
	devModeLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("DEV_MODE: %v", isDevMode()),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	indieNodeCard.SetContent(container.NewVBox(
		versionLabel,
		goVersionLabel,
		devModeLabel,
	))

	content.Add(indieNodeCard)

	// Create IPFS Settings section
	ipfsCard := widget.NewCard("IPFS Settings", "", nil)

	// Create status label for settings
	statusLabel := widget.NewLabelWithStyle(
		"Checking IPFS status...",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	// Add path label
	pathLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("IPFS Path: %s", ipfsManager.BinaryPath),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Add address label
	addressLabel := widget.NewLabelWithStyle(
		"Node Address: Not Running",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Function to update address label
	updateAddressLabel := func() {
		if ipfsManager.IsDaemonRunning() {
			nodeID, addrs, err := ipfsManager.GetNodeInfo()
			if err == nil && len(addrs) > 0 {
				// Look for the local API address first
				for _, addr := range addrs {
					if strings.Contains(addr, "127.0.0.1") && strings.Contains(addr, "5001") {
						addressLabel.SetText(fmt.Sprintf("Node Address: %s\nNode ID: %s", addr, nodeID))
						return
					}
				}
				// If no local address found, use the first available address
				addressLabel.SetText(fmt.Sprintf("Node Address: %s\nNode ID: %s", addrs[0], nodeID))
			} else {
				addressLabel.SetText("Node Address: Error getting address")
			}
		} else {
			addressLabel.SetText("Node Address: Not Running")
		}
		addressLabel.Refresh()
	}

	// Create daemon control button
	var daemonButton *widget.Button
	daemonButton = widget.NewButton("Start Daemon", func() {
		if ipfsManager.IsDaemonRunning() {
			// Stop OrbitDB first if it's running
			if orbitDBManager != nil {
				if err := orbitDBManager.Close(); err != nil {
					dialog.ShowError(fmt.Errorf("failed to close OrbitDB: %w", err), mainWindow)
				}
				orbitDBManager = nil
			}

			if err := ipfsManager.StopDaemon(); err != nil {
				dialog.ShowError(err, mainWindow)
				return
			}
		} else {
			if err := ipfsManager.StartDaemon(); err != nil {
				dialog.ShowError(err, mainWindow)
				return
			}

			// Initialize OrbitDB
			config := &orbitdb.Config{
				Directory:    "./orbitdb",
				NetworkMode:  "public",
				Timeout:      time.Minute * 1,
				IPFSEndpoint: "http://localhost:5001",
			}

			var err error
			orbitDBManager, err = orbitdb.NewManager(context.Background(), config, ipfsManager)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to initialize OrbitDB: %w", err), mainWindow)
				return
			}
		}

		updateIPFSStatus(statusLabel, daemonButton)
		updateIPFSStatus(mainIPFSStatusLabel, nil)
		updateAddressLabel()
	})

	// Set initial states
	updateIPFSStatus(statusLabel, daemonButton)
	updateAddressLabel()

	ipfsCard.SetContent(container.NewVBox(
		pathLabel,
		addressLabel,
		widget.NewSeparator(),
		statusLabel,
		daemonButton,
	))

	content.Add(ipfsCard)

	// Create OrbitDB Settings section
	orbitDBCard := widget.NewCard("OrbitDB Settings", "", nil)

	// Create status label for OrbitDB
	orbitDBStatusLabel := widget.NewLabelWithStyle(
		"Status: Not Connected",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	// Add database path label
	dbPathLabel := widget.NewLabelWithStyle(
		"Database Path: Not Connected",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Add network mode label
	networkModeLabel := widget.NewLabelWithStyle(
		"Network Mode: Not Connected",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Add shops count label
	shopsCountLabel := widget.NewLabelWithStyle(
		"Shops in Database: 0",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// Function to update OrbitDB status
	updateOrbitDBStatus := func() {
		if orbitDBManager != nil && orbitDBManager.IsConnected() {
			orbitDBStatusLabel.SetText("Status: Connected")
			dbPathLabel.SetText(fmt.Sprintf("Database Path: %s", orbitDBManager.GetDatabasePath()))
			networkModeLabel.SetText(fmt.Sprintf("Network Mode: %s", orbitDBManager.GetNetworkMode()))

			// Get all shops count
			if shops, err := orbitDBManager.ListShops(context.Background(), ""); err == nil {
				shopsCountLabel.SetText(fmt.Sprintf("Shops in Database: %d", len(shops)))
			} else {
				shopsCountLabel.SetText("Shops in Database: Error counting shops")
			}
		} else {
			orbitDBStatusLabel.SetText("Status: Not Connected")
			dbPathLabel.SetText("Database Path: Not Connected")
			networkModeLabel.SetText("Network Mode: Not Connected")
			shopsCountLabel.SetText("Shops in Database: Not Connected")
		}

		// Refresh all labels
		orbitDBStatusLabel.Refresh()
		dbPathLabel.Refresh()
		networkModeLabel.Refresh()
		shopsCountLabel.Refresh()
	}

	// Initial update
	updateOrbitDBStatus()

	// Create refresh button
	refreshButton := widget.NewButton("Refresh Status", func() {
		updateOrbitDBStatus()
	})

	// Layout with all status information and refresh button
	orbitDBCard.SetContent(container.NewVBox(
		orbitDBStatusLabel,
		widget.NewSeparator(),
		dbPathLabel,
		networkModeLabel,
		shopsCountLabel,
		widget.NewSeparator(),
		refreshButton,
	))

	content.Add(orbitDBCard)
	return content
}

func showLoginWindow() {
	loginWindow := mainApp.NewWindow("Login")
	loginWindow.Resize(fyne.NewSize(400, 300))

	addressEntry := widget.NewEntry()
	addressEntry.SetPlaceHolder("Enter Ethereum Address")

	messageEntry := widget.NewEntry()
	messageEntry.SetPlaceHolder("Enter Message")

	signatureEntry := widget.NewEntry()
	signatureEntry.SetPlaceHolder("Enter Signature")

	loginBtn := widget.NewButton("Login", func() {
		err := auth.AuthenticateWithEthereum(addressEntry.Text, messageEntry.Text, signatureEntry.Text)
		if err != nil {
			dialog.ShowError(err, loginWindow)
			return
		}
		loginWindow.Close()
		showMainWindow()
	})

	content := container.NewVBox(
		addressEntry,
		messageEntry,
		signatureEntry,
		loginBtn,
	)

	loginWindow.SetContent(content)
	loginWindow.Show()
}

func updateIPFSStatus(ipfsStatusLabel *widget.Label, daemonButton *widget.Button) {
	installed, version := ipfsManager.IsIPFSDownloaded()
	if installed {
		status := "Not Connected"
		if ipfsManager.IsDaemonRunning() {
			status = "Connected"
			if daemonButton != nil {
				daemonButton.SetText("Stop Daemon")
				// Add additional info in settings tab
				daemonType := "External (started from command line)"
				if ipfsManager.Daemon != nil {
					daemonType = "Internal (started by IndieNode)"
				}
				ipfsStatusLabel.SetText(fmt.Sprintf("IPFS %s | Daemon: %s\n%s",
					strings.TrimSpace(version), status, daemonType))
				return
			}
		} else {
			if daemonButton != nil {
				daemonButton.SetText("Start Daemon")
			}
		}

		// For main status bar or other locations
		ipfsStatusLabel.SetText(fmt.Sprintf("IPFS %s | Daemon: %s",
			strings.TrimSpace(version), status))
	} else {
		ipfsStatusLabel.SetText("IPFS Status: Not Installed")
		if daemonButton != nil {
			daemonButton.Disable()
		}
	}
}

type CustomTheme struct {
	fyne.Theme
}

func newCustomTheme() *CustomTheme {
	return &CustomTheme{Theme: theme.DefaultTheme()}
}

func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0xff, G: 0xfc, B: 0xe9, A: 0xff} // #fffce9
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0x1d, G: 0x1d, B: 0x1d, A: 0xff} // #1d1d1d
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x5a, G: 0xd9, B: 0xd5, A: 0xff} // #5ad9d5
	default:
		return t.Theme.Color(name, variant)
	}
}

func isDevMode() bool {
	return os.Getenv("DEV_MODE") == "1"
}

func init() {
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}
}
