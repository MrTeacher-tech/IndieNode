package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"image/color"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/lusingander/colorpicker"
)

// App configuration
const (
	appWidth  = 800
	appHeight = 600
)

type Shop struct {
	ID             string
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
	// Initialize Fyne application with a unique ID
	mainApp = app.NewWithID("com.indienode.shopcreator")
	mainWindow = mainApp.NewWindow("Shop Creator")
	mainWindow.Resize(fyne.NewSize(appWidth, appHeight))

	// Create shops directory if it doesn't exist
	if err := os.MkdirAll("shops", 0755); err != nil {
		dialog.ShowError(err, mainWindow)
		return
	}

	// Create tabs container
	tabs := container.NewAppTabs(
		container.NewTabItem("Create Shop", showShopCreator(mainWindow, nil)),
		container.NewTabItem("View Shops", showShopList(mainWindow, mainApp)),
	)

	mainWindow.SetContent(tabs)
	mainWindow.ShowAndRun()
}

func showShopCreator(window fyne.Window, existingShop *Shop) fyne.CanvasObject {
	shop := &Shop{}
	if existingShop != nil {
		shop = existingShop
	}

	var currentImagePaths []string
	var addItemFunc func()
	var addItemBtn *widget.Button
	var itemsList *widget.List
	var previewContainer *fyne.Container

	// Shop details
	shopName := widget.NewEntry()
	shopName.SetPlaceHolder("Shop Name")

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

			// Get the path of the selected file
			logoPath := reader.URI().Path()

			// Copy the logo to the shop's assets directory immediately
			fileName := filepath.Base(logoPath)
			shopDir := filepath.Join("shops", shop.Name)
			destDir := filepath.Join(shopDir, "assets", "logos")

			// Create the destination directory if it doesn't exist
			if err := os.MkdirAll(destDir, 0755); err != nil {
				dialog.ShowError(fmt.Errorf("failed to create logos directory: %w", err), window)
				return
			}

			destPath := filepath.Join(destDir, fileName)
			if err := copyFile(logoPath, destPath); err != nil {
				dialog.ShowError(fmt.Errorf("failed to copy logo: %w", err), window)
				return
			}

			// Store the relative path
			relativePath := filepath.Join("assets", "logos", fileName)
			shop.LogoPath = relativePath
			shopLogo.SetText(relativePath)

			// Update logo preview
			if logoPreview != nil {
				logoPreview.File = filepath.Join(shopDir, relativePath)
				logoPreview.Show()
				logoPreview.Refresh()
			}
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
		for _, path := range currentImagePaths {
			// Use the full path for preview
			fullPath := filepath.Join("shops", shop.Name, path)
			img := canvas.NewImageFromFile(fullPath)
			img.SetMinSize(fyne.NewSize(100, 100))
			img.Resize(fyne.NewSize(100, 100))
			previewContainer.Add(img)
		}
		previewContainer.Refresh()
	}

	// Add image button
	addImageBtn := widget.NewButton("Add Image", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			// Get the path of the selected file
			photoPath := reader.URI().Path()

			// Copy the image to the shop's assets directory immediately
			fileName := filepath.Base(photoPath)
			shopDir := filepath.Join("shops", shop.Name)
			destDir := filepath.Join(shopDir, "assets", "images")

			// Create the destination directory if it doesn't exist
			if err := os.MkdirAll(destDir, 0755); err != nil {
				dialog.ShowError(fmt.Errorf("failed to create images directory: %w", err), window)
				return
			}

			destPath := filepath.Join(destDir, fileName)
			if err := copyFile(photoPath, destPath); err != nil {
				dialog.ShowError(fmt.Errorf("failed to copy image: %w", err), window)
				return
			}

			// Store the relative path
			relativePath := filepath.Join("assets", "images", fileName)
			currentImagePaths = append(currentImagePaths, relativePath)

			// Update the preview using the full path
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

		newItem := Item{
			Name:        itemName.Text,
			Price:       price,
			Description: itemDescription.Text,
			PhotoPaths:  currentImagePaths,
		}

		shop.Items = append(shop.Items, newItem)

		itemName.SetText("")
		itemPrice.SetText("")
		itemDescription.SetText("")
		currentImagePaths = nil
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
				currentImagePaths = shop.Items[id].PhotoPaths
				updatePreviewContainer()

				addItemBtn.SetText("Update Item")
				addItemBtn.OnTapped = func() {
					price := 0.0
					if _, err := fmt.Sscanf(itemPrice.Text, "%f", &price); err != nil {
						dialog.ShowError(fmt.Errorf("invalid price format"), window)
						return
					}

					shop.Items[id] = Item{
						Name:        itemName.Text,
						Price:       price,
						Description: itemDescription.Text,
						PhotoPaths:  currentImagePaths,
					}

					itemName.SetText("")
					itemPrice.SetText("")
					itemDescription.SetText("")
					currentImagePaths = nil
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
		if err := generateShop(shop, shopDir); err != nil {
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

	content := container.NewVBox(
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
			logoPreview.File = filepath.Join("shops", existingShop.Name, existingShop.LogoPath)
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

func showShopList(window fyne.Window, app fyne.App) fyne.CanvasObject {
	// Initialize global shopsContainer
	shopsContainer = container.NewVBox()

	// Initial shop list load
	updateShopsList(window)

	content := container.NewVBox(
		widget.NewLabel("Your Shops"),
		shopsContainer,
	)

	return content
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
	// Create required directories
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create shop directory: %w", err)
	}

	// Create directory structure
	srcDir := filepath.Join(outputDir, "src")
	cssDir := filepath.Join(srcDir, "css")
	jsDir := filepath.Join(srcDir, "js")

	// Create all directories
	for _, dir := range []string{srcDir, cssDir, jsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy all item photos to images directory
	for i, item := range shop.Items {
		var newPaths []string
		for _, photoPath := range item.PhotoPaths {
			// The source path should be relative to the shop directory
			srcPath := filepath.Join("shops", shop.Name, photoPath)
			destPath := filepath.Join(outputDir, photoPath)

			// Create destination directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", destPath, err)
			}

			// Copy the file
			if err := copyFile(srcPath, destPath); err != nil {
				return fmt.Errorf("failed to copy photo %s: %w", filepath.Base(photoPath), err)
			}

			// Keep the same relative path
			newPaths = append(newPaths, photoPath)
		}
		shop.Items[i].PhotoPaths = newPaths
	}

	// Copy logo if exists
	if shop.LogoPath != "" {
		// The source path should be relative to the shop directory
		srcPath := filepath.Join("shops", shop.Name, shop.LogoPath)
		destPath := filepath.Join(outputDir, shop.LogoPath)

		// Create destination directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for logo: %w", err)
		}

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("failed to copy logo: %w", err)
		}
	}

	// Process CSS template
	cssTemplate, err := template.ParseFiles(filepath.Join("templates", "basic", "basic.css"))
	if err != nil {
		return fmt.Errorf("failed to parse CSS template: %w", err)
	}

	// Create CSS data with color values
	cssData := struct {
		PrimaryColor   string
		SecondaryColor string
		TertiaryColor  string
	}{
		PrimaryColor:   fmt.Sprintf("rgb(%d, %d, %d)", shop.PrimaryColor.R, shop.PrimaryColor.G, shop.PrimaryColor.B),
		SecondaryColor: fmt.Sprintf("rgb(%d, %d, %d)", shop.SecondaryColor.R, shop.SecondaryColor.G, shop.SecondaryColor.B),
		TertiaryColor:  fmt.Sprintf("rgb(%d, %d, %d)", shop.TertiaryColor.R, shop.TertiaryColor.G, shop.TertiaryColor.B),
	}

	// Create and write CSS file
	cssPath := filepath.Join(outputDir, "src", "css", "styles.css")
	cssFile, err := os.Create(cssPath)
	if err != nil {
		return fmt.Errorf("failed to create CSS file: %w", err)
	}
	defer cssFile.Close()

	if err := cssTemplate.Execute(cssFile, cssData); err != nil {
		return fmt.Errorf("failed to write CSS file: %w", err)
	}

	// Load HTML template
	htmlTemplate, err := template.ParseFiles(filepath.Join("templates", "basic", "basic.html"))
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	// Create index.html in src directory
	indexPath := filepath.Join(outputDir, "src", "index.html")
	indexFile, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index.html: %w", err)
	}
	defer indexFile.Close()

	// Execute HTML template
	if err := htmlTemplate.Execute(indexFile, shop); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Save shop data as JSON
	shopData := struct {
		Name           string    `json:"name"`
		Description    string    `json:"description"`
		Location       string    `json:"location"`
		Email          string    `json:"email"`
		Phone          string    `json:"phone"`
		PrimaryColor   string    `json:"primaryColor"`
		SecondaryColor string    `json:"secondaryColor"`
		TertiaryColor  string    `json:"tertiaryColor"`
		LogoPath       string    `json:"logoPath"`
		Items          []Item    `json:"items"`
		CreatedAt      time.Time `json:"createdAt"`
	}{
		Name:           shop.Name,
		Description:    shop.Description,
		Location:       shop.Location,
		Email:          shop.Email,
		Phone:          shop.Phone,
		PrimaryColor:   fmt.Sprintf("#%02x%02x%02x", shop.PrimaryColor.R, shop.PrimaryColor.G, shop.PrimaryColor.B),
		SecondaryColor: fmt.Sprintf("#%02x%02x%02x", shop.SecondaryColor.R, shop.SecondaryColor.G, shop.SecondaryColor.B),
		TertiaryColor:  fmt.Sprintf("#%02x%02x%02x", shop.TertiaryColor.R, shop.TertiaryColor.G, shop.TertiaryColor.B),
		LogoPath:       shop.LogoPath,
		Items:          shop.Items,
		CreatedAt:      time.Now(),
	}

	jsonData, err := json.MarshalIndent(shopData, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outputDir, "shop.json"), jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write shop data: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
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
