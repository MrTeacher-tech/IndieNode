package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"text/template"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/lusingander/colorpicker"

	"github.com/ethereum/go-ethereum/common"

	"shopCreator/auth"
	"shopCreator/db" // Import the database package
)

// App configuration
const (
	appWidth  = 800
	appHeight = 600
)

type Shop struct {
	ID             string
	OwnerAddress   string
	Name           string
	Description    string
	Location       string
	Email          string
	Phone          string
	PrimaryColor   color.RGBA
	SecondaryColor color.RGBA
	TertiaryColor  color.RGBA
	LogoPath       string
	Items          []Item
}

type Item struct {
	Name        string
	Price       float64
	Description string
	PhotoPaths  []string
}

// Global variables for shops list
var (
	shopsContainer *fyne.Container
	mainApp        fyne.App
	mainWindow     fyne.Window
)

// Global variables for authentication
var (
	currentUser *auth.AuthenticatedUser
	authMutex   sync.RWMutex
)

func updateShopsList(window fyne.Window) {
	if shopsContainer == nil {
		return
	}

	shopsContainer.Objects = nil // Clear existing items
	shops, err := os.ReadDir("shops")
	if err != nil {
		dialog.ShowError(err, window)
		return
	}

	for _, shop := range shops {
		if shop.IsDir() {
			name := shop.Name() // Create a new variable for this iteration
			row := container.NewHBox(
				widget.NewLabel(name),
				widget.NewButton("View", func() {
					shopPath := filepath.Join("shops", name, "src", "index.html")
					showShopPreview(mainApp, name, shopPath)
				}),
				widget.NewButton("Edit", func() {
					// Load existing shop data
					shop, err := loadShop(name)
					if err != nil {
						dialog.ShowError(err, window)
						return
					}

					// Create new window for editing
					editWindow := mainApp.NewWindow("Edit Shop - " + name)
					editWindow.SetContent(showShopCreator(editWindow, shop))
					editWindow.Resize(fyne.NewSize(appWidth, appHeight))
					editWindow.Show()
				}),
				widget.NewButton("Delete", func() {
					dialog.ShowConfirm("Delete Shop",
						fmt.Sprintf("Are you sure you want to delete %s?", name),
						func(delete bool) {
							if delete {
								if err := os.RemoveAll(filepath.Join("shops", name)); err != nil {
									dialog.ShowError(err, window)
									return
								}
								updateShopsList(window)
							}
						}, window)
				}),
			)
			shopsContainer.Add(row)
			shopsContainer.Add(widget.NewSeparator())
		}
	}
	shopsContainer.Refresh()
}

func showShopPreview(_ fyne.App, shopName, shopPath string) {
	// Check if the shop exists
	if _, err := os.Stat(shopPath); err != nil {
		dialog.ShowError(fmt.Errorf("shop preview not found: %w", err), mainWindow)
		return
	}

	// Create command to open default browser
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", shopPath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "file://"+shopPath)
	default:
		cmd = exec.Command("xdg-open", shopPath)
	}

	// Execute command
	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("failed to open preview: %w", err), mainWindow)
		return
	}
}

func main() {
	// Initialize database
	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.DB.Close()

	mainApp = app.New()

	// Set application icon
	icon, err := fyne.LoadResourceFromPath("IndieNode_assets/indieNode_logo.png")
	if err == nil {
		mainApp.SetIcon(icon)
	}

	// Create sign-in window
	signInWindow := mainApp.NewWindow("IndieNode - Sign In")
	signInWindow.Resize(fyne.NewSize(400, 500)) // Made taller to accommodate logo

	// Load and create logo image
	logo := canvas.NewImageFromFile("IndieNode_assets/indieNode_logo.png")
	logo.SetMinSize(fyne.NewSize(200, 200)) // Set appropriate size for logo
	logo.FillMode = canvas.ImageFillContain

	// Create centered welcome text
	welcomeText := widget.NewLabelWithStyle(
		"Welcome to IndieNode",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	address := widget.NewEntry()
	address.SetPlaceHolder("Enter Ethereum Address")

	signInBtn := widget.NewButton("Sign In with Ethereum", func() {
		if address.Text == "" {
			dialog.ShowError(fmt.Errorf("please enter an Ethereum address"), signInWindow)
			return
		}

		// Create SIWE message
		siweMsg := auth.CreateSIWEMessage(address.Text)
		formattedMsg := siweMsg.FormatMessage()

		// Show message for signing
		dialog.ShowCustomConfirm("Sign Message", "Sign", "Cancel",
			widget.NewTextGridFromString(formattedMsg),
			func(sign bool) {
				if !sign {
					return
				}

				// For development, we'll simulate a successful signature
				// In production, this would interact with a wallet
				simulatedSignature := "0x123..." // Placeholder signature

				// Verify the signature
				verified := auth.VerifySignature(siweMsg, simulatedSignature, address.Text)

				if !verified {
					log.Printf("Signature verification failed")
					dialog.ShowError(fmt.Errorf("invalid signature"), signInWindow)
					return
				}

				log.Printf("Signature verified successfully")

				// Set the current user
				currentUser = &auth.AuthenticatedUser{
					Address:         address.Text,
					SignedMessage:   formattedMsg,
					Signature:       simulatedSignature,
					AuthenticatedAt: time.Now(),
				}

				// Close the sign-in window
				signInWindow.Close()

				// Show the main window
				log.Printf("Opening main window...")
				showMainWindow()
			},
			signInWindow,
		)
	})

	content := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), logo, layout.NewSpacer()),
		welcomeText,
		widget.NewLabel("Enter your Ethereum address:"),
		address,
		signInBtn,
	)

	signInWindow.SetContent(content)
	signInWindow.Show()
	mainApp.Run()
}

// showMainWindow creates and shows the main shop management window
func showMainWindow() {
	mainWindow = mainApp.NewWindow("IndieNode - Shop Manager")
	mainWindow.Resize(fyne.NewSize(appWidth, appHeight))

	// Create header with Ethereum address
	addressLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("Connected: %s", currentUser.Address),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Create tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("My Shops", createShopsTab()),
		container.NewTabItem("Create Shop", showShopCreator(mainWindow, nil)),
	)

	// Create main content
	content := container.NewVBox(
		container.NewHBox(
			layout.NewSpacer(),
			addressLabel,
			layout.NewSpacer(),
		),
		widget.NewSeparator(),
		tabs,
	)

	mainWindow.SetContent(content)
	mainWindow.Show()
}

// createShopsTab creates the tab that shows the list of shops
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

func showShopCreator(window fyne.Window, existingShop *Shop) fyne.CanvasObject {
	var shop Shop
	if existingShop != nil {
		shop = *existingShop
	} else {
		shop = Shop{
			ID:           fmt.Sprintf("shop_%d", time.Now().Unix()),
			OwnerAddress: currentUser.Address, // Set owner address for new shops
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
	logoPreview := canvas.NewImageFromFile("")
	logoPreview.FillMode = canvas.ImageFillContain
	logoPreview.SetMinSize(fyne.NewSize(200, 200))
	logoPreview.Hide()

	// Logo selection
	shopLogo := widget.NewEntry()
	shopLogo.SetPlaceHolder("Logo Path")
	logoBtn := widget.NewButton("Select Logo", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			// Store the original source path
			sourcePath := reader.URI().Path()
			shop.LogoPath = sourcePath // Store the source path directly

			// Update logo preview using source path
			if logoPreview != nil {
				logoPreview.File = sourcePath
				logoPreview.Show()
				logoPreview.Refresh()
			}

			shopLogo.SetText(sourcePath)
		}, window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		fd.Show()
	})

	// Color pickers
	var primaryColorPicker, secondaryColorPicker, tertiaryColorPicker *ColorButton

	primaryColorPicker = NewColorButton("", color.RGBA{0, 0, 0, 255}, window, func(c color.Color) {
		r, g, b, _ := c.RGBA()
		shop.PrimaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})
	secondaryColorPicker = NewColorButton("", color.RGBA{255, 255, 255, 255}, window, func(c color.Color) {
		r, g, b, _ := c.RGBA()
		shop.SecondaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})
	tertiaryColorPicker = NewColorButton("", color.RGBA{128, 128, 128, 255}, window, func(c color.Color) {
		r, g, b, _ := c.RGBA()
		shop.TertiaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})

	// Optional shop details in accordion
	optionalDetails := widget.NewAccordion(
		widget.NewAccordionItem("Optional Settings", container.NewVBox(
			logoBtn,
			logoPreview,
			shopLocation,
			shopEmail,
			shopPhone,

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

		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
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

		newItem := Item{
			Name:        itemName.Text,
			Price:       price,
			Description: itemDescription.Text,
			PhotoPaths:  photoPaths,
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
				for _, path := range shop.Items[id].PhotoPaths {
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

					shop.Items[id] = Item{
						Name:        itemName.Text,
						Price:       price,
						Description: itemDescription.Text,
						PhotoPaths:  photoPaths,
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

	// Generate button
	generateBtn := widget.NewButton("Generate Shop", func() {
		if shopName.Text == "" {
			dialog.ShowError(fmt.Errorf("shop name is required"), window)
			return
		}

		shop.Name = shopName.Text
		shop.Description = shopDescription.Text
		shop.Location = shopLocation.Text
		shop.Email = shopEmail.Text
		shop.Phone = shopPhone.Text
		shop.LogoPath = shopLogo.Text

		// Create shop directory and generate shop
		shopDir := filepath.Join("shops", shop.Name)
		if err := generateShop(&shop, shopDir); err != nil {
			dialog.ShowError(err, window)
			return
		}

		dialog.ShowInformation("Success", "Shop has been generated!", window)
		updateShopsList(window)
	})

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
		fmt.Sprintf("Shop Owner: %s", shop.OwnerAddress),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	content := container.NewVBox(
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
		itemsList,
		generateBtn,
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
			logoPreview.File = existingShop.LogoPath
			logoPreview.Show()
			logoPreview.Refresh()
		}

		primaryColorPicker.currentColor = existingShop.PrimaryColor
		secondaryColorPicker.currentColor = existingShop.SecondaryColor
		tertiaryColorPicker.currentColor = existingShop.TertiaryColor
		primaryColorPicker.SetText(fmt.Sprintf("Primary Color: #%02x%02x%02x%02x", existingShop.PrimaryColor.R, existingShop.PrimaryColor.G, existingShop.PrimaryColor.B, existingShop.PrimaryColor.A))
		secondaryColorPicker.SetText(fmt.Sprintf("Secondary Color: #%02x%02x%02x%02x", existingShop.SecondaryColor.R, existingShop.SecondaryColor.G, existingShop.SecondaryColor.B, existingShop.SecondaryColor.A))
		tertiaryColorPicker.SetText(fmt.Sprintf("Tertiary Color: #%02x%02x%02x%02x", existingShop.TertiaryColor.R, existingShop.TertiaryColor.G, existingShop.TertiaryColor.B, existingShop.TertiaryColor.A))
		primaryColorPicker.Refresh()
		secondaryColorPicker.Refresh()
		tertiaryColorPicker.Refresh()
		itemsList.Refresh()
	}

	return container.NewVScroll(content)
}

func loadShop(shopName string) (*Shop, error) {
	shopFile := filepath.Join("shops", shopName, "shop.json")
	data, err := os.ReadFile(shopFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read shop file: %w", err)
	}

	var shop Shop
	if err := json.Unmarshal(data, &shop); err != nil {
		return nil, fmt.Errorf("failed to parse shop data: %w", err)
	}

	// No need to modify paths as they are already stored correctly
	return &shop, nil
}

func hexToColor(hex string) color.RGBA {
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{r, g, b, 255}
}

func generateShop(shop *Shop, outputDir string) error {
	// Create necessary directories
	if err := os.MkdirAll(filepath.Join(outputDir, "src"), 0755); err != nil {
		return err
	}

	// Create index.html template
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Name}}</title>
    <style>
        :root {
            --primary-color: {{.PrimaryColorHex}};
            --secondary-color: {{.SecondaryColorHex}};
            --tertiary-color: {{.TertiaryColorHex}};
        }
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: var(--tertiary-color);
        }
        .header {
            background-color: var(--primary-color);
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .owner-address {
            font-family: monospace;
            background-color: var(--secondary-color);
            color: white;
            padding: 10px;
            border-radius: 4px;
            margin-top: 10px;
            font-size: 0.9em;
        }
        .shop-info {
            background-color: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .items {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 20px;
        }
        .item {
            background-color: white;
            padding: 20px;
            border-radius: 8px;
        }
        .item img {
            max-width: 100%;
            height: auto;
            border-radius: 4px;
        }
        .logo {
            max-width: 200px;
            height: auto;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="header">
        {{if .LogoPath}}
        <img src="{{.LogoPath}}" alt="{{.Name}} Logo" class="logo">
        {{end}}
        <h1>{{.Name}}</h1>
        <div class="owner-address">Owner: {{.OwnerAddress}}</div>
    </div>
    
    <div class="shop-info">
        <h2>About Us</h2>
        <p>{{.Description}}</p>
        <p><strong>Location:</strong> {{.Location}}</p>
        <p><strong>Contact:</strong> {{.Email}} | {{.Phone}}</p>
    </div>

    <h2>Our Items</h2>
    <div class="items">
        {{range .Items}}
        <div class="item">
            <h3>{{.Name}}</h3>
            {{range .PhotoPaths}}
            <img src="{{.}}" alt="{{$.Name}}">
            {{end}}
            <p>{{.Description}}</p>
            <p><strong>Price:</strong> ${{.Price}}</p>
        </div>
        {{end}}
    </div>
</body>
</html>`

	// Create template data with hex color values
	type TemplateData struct {
		*Shop
		PrimaryColorHex   string
		SecondaryColorHex string
		TertiaryColorHex  string
	}

	data := TemplateData{
		Shop:              shop,
		PrimaryColorHex:   fmt.Sprintf("#%02x%02x%02x", shop.PrimaryColor.R, shop.PrimaryColor.G, shop.PrimaryColor.B),
		SecondaryColorHex: fmt.Sprintf("#%02x%02x%02x", shop.SecondaryColor.R, shop.SecondaryColor.G, shop.SecondaryColor.B),
		TertiaryColorHex:  fmt.Sprintf("#%02x%02x%02x", shop.TertiaryColor.R, shop.TertiaryColor.G, shop.TertiaryColor.B),
	}

	// Create index.html file
	indexFile, err := os.Create(filepath.Join(outputDir, "src", "index.html"))
	if err != nil {
		return fmt.Errorf("failed to create index.html file: %w", err)
	}
	defer indexFile.Close()

	// Parse and execute the template
	t, err := template.New("shop").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := t.Execute(indexFile, data); err != nil {
		return fmt.Errorf("failed to write index.html file: %w", err)
	}

	// Step 2: Copy logo if it exists (from source path)
	if shop.LogoPath != "" {
		fileName := filepath.Base(shop.LogoPath)
		destPath := filepath.Join(outputDir, "assets", "logos", fileName)

		if err := copyFile(shop.LogoPath, destPath); err != nil {
			return fmt.Errorf("failed to copy logo: %w", err)
		}

		// Update logo path to be relative
		shop.LogoPath = filepath.Join("assets", "logos", fileName)
	}

	// Step 3: Copy all item photos (from source paths)
	for i, item := range shop.Items {
		var newPaths []string
		for _, photoPath := range item.PhotoPaths {
			fileName := filepath.Base(photoPath)
			destPath := filepath.Join(outputDir, "assets", "images", fileName)

			if err := copyFile(photoPath, destPath); err != nil {
				return fmt.Errorf("failed to copy photo %s: %w", fileName, err)
			}

			// Store the relative path
			newPaths = append(newPaths, filepath.Join("assets", "images", fileName))
		}
		shop.Items[i].PhotoPaths = newPaths
	}

	// Step 4: Save shop data as JSON
	shopData, err := json.MarshalIndent(shop, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outputDir, "shop.json"), shopData, 0644); err != nil {
		return fmt.Errorf("failed to save shop data: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	log.Printf("Copying file from %s to %s", src, dst)

	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		log.Printf("Failed to open source file: %v", err)
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		log.Printf("Failed to create destination file: %v", err)
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			log.Printf("Error closing destination file: %v", err)
		}
	}()

	// Copy the contents
	written, err := io.Copy(destFile, sourceFile)
	if err != nil {
		log.Printf("Failed to copy file contents: %v", err)
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Verify file size
	sourceInfo, err := os.Stat(src)
	if err != nil {
		log.Printf("Failed to get source file info: %v", err)
		return fmt.Errorf("failed to verify file size: %w", err)
	}

	if written != sourceInfo.Size() {
		log.Printf("File size mismatch: wrote %d bytes, expected %d bytes", written, sourceInfo.Size())
		return fmt.Errorf("incomplete file copy: wrote %d bytes, expected %d bytes", written, sourceInfo.Size())
	}

	log.Printf("Successfully copied %d bytes", written)
	return nil
}

type ColorButton struct {
	widget.Button
	currentColor color.RGBA
	window       fyne.Window
	onSelected   func(color.Color)
	background   *canvas.Rectangle
}

func NewColorButton(label string, initialColor color.RGBA, window fyne.Window, onSelected func(color.Color)) *ColorButton {
	btn := &ColorButton{
		currentColor: initialColor,
		window:       window,
		onSelected:   onSelected,
		background:   canvas.NewRectangle(initialColor),
	}
	btn.ExtendBaseWidget(btn)
	btn.Importance = widget.MediumImportance
	btn.SetText(label + "Click to Change")
	return btn
}

func (c *ColorButton) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(c.currentColor)

	// Create text with initial color based on background
	text := canvas.NewText(c.Text, color.Black) // Default to black, will be updated based on background
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Bold: true}

	// Calculate initial text color
	r, g, b := float32(c.currentColor.R)/255, float32(c.currentColor.G)/255, float32(c.currentColor.B)/255
	luminance := 0.2126*r + 0.7152*g + 0.0722*b
	if luminance > 0.5 {
		text.Color = color.Black
	} else {
		text.Color = color.White
	}

	objects := []fyne.CanvasObject{background, text}

	return &colorButtonRenderer{
		button:     c,
		background: background,
		text:       text,
		objects:    objects,
	}
}

type colorButtonRenderer struct {
	button     *ColorButton
	background *canvas.Rectangle
	text       *canvas.Text
	objects    []fyne.CanvasObject
}

func (r *colorButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(150, 40) // Set a reasonable minimum size
}

func (r *colorButtonRenderer) Layout(size fyne.Size) {
	// Fill the entire button with the background
	r.background.Resize(size)
	r.background.Move(fyne.NewPos(0, 0))

	// Center the text
	textSize := r.text.MinSize()
	r.text.Resize(textSize)
	r.text.Move(fyne.NewPos(
		(size.Width-textSize.Width)/2,
		(size.Height-textSize.Height)/2,
	))
}

func (r *colorButtonRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *colorButtonRenderer) Refresh() {
	// Update background color
	r.background.FillColor = r.button.currentColor
	r.background.Refresh()

	// Update text content
	r.text.Text = r.button.Text

	// Update text color based on background brightness
	red := float32(r.button.currentColor.R) / 255
	green := float32(r.button.currentColor.G) / 255
	blue := float32(r.button.currentColor.B) / 255
	luminance := 0.2126*red + 0.7152*green + 0.0722*blue
	if luminance > 0.5 {
		r.text.Color = color.Black
	} else {
		r.text.Color = color.White
	}

	r.text.Refresh()
}

func (r *colorButtonRenderer) Destroy() {}

func (c *ColorButton) Tapped(_ *fyne.PointEvent) {
	// Create color picker with hue style
	picker := colorpicker.New(200, colorpicker.StyleHue)

	// Convert RGBA to NRGBA for the picker
	nrgba := color.NRGBA{
		R: c.currentColor.R,
		G: c.currentColor.G,
		B: c.currentColor.B,
		A: c.currentColor.A,
	}
	picker.SetColor(nrgba)

	// Create preview rectangle
	preview := canvas.NewRectangle(nrgba)
	preview.Resize(fyne.NewSize(50, 50))

	// Create RGB entry fields
	rEntry := widget.NewEntry()
	gEntry := widget.NewEntry()
	bEntry := widget.NewEntry()
	aEntry := widget.NewEntry()

	rEntry.SetText(fmt.Sprintf("%d", c.currentColor.R))
	gEntry.SetText(fmt.Sprintf("%d", c.currentColor.G))
	bEntry.SetText(fmt.Sprintf("%d", c.currentColor.B))
	aEntry.SetText(fmt.Sprintf("%d", c.currentColor.A))

	var selectedColor color.Color

	// Update both picker and entries when either changes
	updateFromPicker := func(clr color.Color) {
		selectedColor = clr

		// Convert to NRGBA which is what the picker uses
		nrgba, ok := clr.(color.NRGBA)
		if !ok {
			fmt.Printf("Error: color is not NRGBA\n")
			return
		}

		preview.FillColor = nrgba
		preview.Refresh()

		// Update RGB entry fields with the new color values
		rEntry.SetText(fmt.Sprintf("%d", nrgba.R))
		gEntry.SetText(fmt.Sprintf("%d", nrgba.G))
		bEntry.SetText(fmt.Sprintf("%d", nrgba.B))
		aEntry.SetText(fmt.Sprintf("%d", nrgba.A))
	}

	updateFromEntry := func(string) {
		r, _ := strconv.Atoi(rEntry.Text)
		g, _ := strconv.Atoi(gEntry.Text)
		b, _ := strconv.Atoi(bEntry.Text)
		a, _ := strconv.Atoi(aEntry.Text)

		// Clamp values between 0 and 255
		r = clamp(r, 0, 255)
		g = clamp(g, 0, 255)
		b = clamp(b, 0, 255)
		a = clamp(a, 0, 255)

		newColor := color.NRGBA{
			R: uint8(r),
			G: uint8(g),
			B: uint8(b),
			A: uint8(a),
		}
		picker.SetColor(newColor)
		preview.FillColor = newColor
		preview.Refresh()
		selectedColor = newColor
	}

	picker.SetOnChanged(updateFromPicker)
	rEntry.OnChanged = updateFromEntry
	gEntry.OnChanged = updateFromEntry
	bEntry.OnChanged = updateFromEntry
	aEntry.OnChanged = updateFromEntry

	// Create form for RGB inputs
	form := widget.NewForm(
		widget.NewFormItem("R", rEntry),
		widget.NewFormItem("G", gEntry),
		widget.NewFormItem("B", bEntry),
		widget.NewFormItem("A", aEntry),
	)

	// Create container with preview and form
	rightContainer := container.NewVBox(
		preview,
		form,
	)

	// Create main container
	content := container.NewHBox(
		picker,
		rightContainer,
	)

	// Show dialog
	dialog.ShowCustomConfirm("Select Color", "Apply", "Cancel", content, func(apply bool) {
		if apply && selectedColor != nil {
			// Convert the selected color to RGBA
			nrgba := selectedColor.(color.NRGBA)
			c.currentColor = color.RGBA{
				R: nrgba.R,
				G: nrgba.G,
				B: nrgba.B,
				A: nrgba.A,
			}
			if c.onSelected != nil {
				c.onSelected(c.currentColor)
			}
			c.Refresh()
		}
	}, c.window)
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// Authentication related functions
func isUserAuthenticated() bool {
	authMutex.RLock()
	defer authMutex.RUnlock()
	return currentUser != nil
}

func authenticateWithEthereum(address, message, signature string) {
	if !common.IsHexAddress(address) {
		dialog.ShowError(fmt.Errorf("invalid Ethereum address"), mainWindow)
		return
	}

	// Create SIWE message
	siweMsg := auth.CreateSIWEMessage(address)

	// Verify the signature
	if !auth.VerifySignature(siweMsg, signature, address) {
		dialog.ShowError(fmt.Errorf("invalid signature"), mainWindow)
		return
	}

	// Set authenticated user
	authMutex.Lock()
	currentUser = &auth.AuthenticatedUser{
		Address:         address,
		SignedMessage:   message,
		Signature:       signature,
		AuthenticatedAt: time.Now(),
	}
	authMutex.Unlock()

	dialog.ShowInformation("Success", "Successfully authenticated with Ethereum", mainWindow)
}
