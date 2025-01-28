package windows

import (
	"IndieNode/internal/models"
	"IndieNode/internal/services/shop"
	"IndieNode/internal/ui/components"
	"IndieNode/ipfs"
	"fmt"
	"image/color"
	"net/url"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// ImageMapping tracks original image paths and their intended relative paths
type ImageMapping struct {
	OriginalPath string
	RelativePath string
}

type ShopCreatorTab struct {
	shopMgr              *shop.Manager
	ipfsMgr              *ipfs.IPFSManager
	nameEntry            *widget.Entry
	descriptionEntry     *widget.Entry
	descriptionContainer *fyne.Container
	locationEntry        *widget.Entry
	emailEntry           *widget.Entry
	phoneEntry           *widget.Entry
	logoPath             string
	logoPreviewContainer *fyne.Container
	itemsList            *widget.List
	itemsListContainer   *fyne.Container
	itemNameEntry        *widget.Entry
	itemDescEntry        *widget.Entry
	itemPriceEntry       *widget.Entry
	itemImagesContainer  *fyne.Container
	currentItemImages    []ImageMapping
	previewContainer     *fyne.Container
	currentImages        []ImageMapping
	existingShop         *models.Shop
	onSave               func(*models.Shop)
	parent               fyne.Window
	deleteBtn            *widget.Button
}

func NewShopCreatorTab(parent fyne.Window, shopMgr *shop.Manager, ipfsMgr *ipfs.IPFSManager, onSave func(*models.Shop)) (*container.TabItem, *ShopCreatorTab) {
	tab := &ShopCreatorTab{
		shopMgr:              shopMgr,
		ipfsMgr:              ipfsMgr,
		onSave:               onSave,
		parent:               parent,
		nameEntry:            widget.NewEntry(),
		descriptionEntry:     widget.NewEntry(),
		locationEntry:        widget.NewEntry(),
		emailEntry:           widget.NewEntry(),
		phoneEntry:           widget.NewEntry(),
		logoPreviewContainer: container.NewVBox(),
		previewContainer:     container.NewVBox(),
		currentImages:        make([]ImageMapping, 0),
		itemNameEntry:        widget.NewEntry(),
		itemDescEntry:        widget.NewEntry(),
		itemPriceEntry:       widget.NewEntry(),
		itemImagesContainer:  container.NewVBox(),
		currentItemImages:    make([]ImageMapping, 0),
	}

	// Create description container with fixed size
	tab.descriptionContainer = container.NewVBox(tab.descriptionEntry)
	tab.descriptionContainer.Resize(fyne.NewSize(400, 100))

	content := tab.createContent()
	tabItem := container.NewTabItem("Create Shop", content)
	return tabItem, tab
}

func (t *ShopCreatorTab) createContent() fyne.CanvasObject {
	// Shop details
	t.nameEntry.SetPlaceHolder("Shop Name")
	if t.existingShop != nil {
		t.nameEntry.SetText(t.existingShop.Name)
	}

	t.descriptionEntry.MultiLine = true
	t.descriptionEntry.SetPlaceHolder("Shop Description")
	if t.existingShop != nil {
		t.descriptionEntry.SetText(t.existingShop.Description)
	}

	t.locationEntry.SetPlaceHolder("Location (Optional)")
	if t.existingShop != nil {
		t.locationEntry.SetText(t.existingShop.Location)
	}

	t.emailEntry.SetPlaceHolder("Email")
	if t.existingShop != nil {
		t.emailEntry.SetText(t.existingShop.Email)
	}

	t.phoneEntry.SetPlaceHolder("Phone (Optional)")
	if t.existingShop != nil {
		t.phoneEntry.SetText(t.existingShop.Phone)
	}

	// Color pickers
	primaryColorPicker := components.NewColorButton("Primary Color", color.RGBA{R: 0xff, G: 0xfc, B: 0xe9, A: 0xff}, t.parent, func(c color.Color) {
		if t.existingShop == nil {
			t.existingShop = &models.Shop{}
		}
		r, g, b, _ := c.RGBA()
		t.existingShop.PrimaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})

	secondaryColorPicker := components.NewColorButton("Secondary Color", color.RGBA{R: 0x1d, G: 0x1d, B: 0x1d, A: 0xff}, t.parent, func(c color.Color) {
		if t.existingShop == nil {
			t.existingShop = &models.Shop{}
		}
		r, g, b, _ := c.RGBA()
		t.existingShop.SecondaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})

	tertiaryColorPicker := components.NewColorButton("Tertiary Color", color.RGBA{R: 0x5a, G: 0xd9, B: 0xd5, A: 0xff}, t.parent, func(c color.Color) {
		if t.existingShop == nil {
			t.existingShop = &models.Shop{}
		}
		r, g, b, _ := c.RGBA()
		t.existingShop.TertiaryColor = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	})

	// Logo upload button
	logoUploadBtn := widget.NewButton("Upload Logo", t.handleLogoUpload)

	// Optional settings in accordion
	optionalSettings := widget.NewAccordion(
		widget.NewAccordionItem("Optional Settings", container.NewVBox(
			container.NewVBox(
				widget.NewLabel("Shop Details"),
				t.descriptionContainer,
				t.locationEntry,
				t.phoneEntry,
			),
			widget.NewSeparator(),
			container.NewVBox(
				widget.NewLabel("Logo"),
				logoUploadBtn,
				t.logoPreviewContainer,
			),
			widget.NewSeparator(),
			container.NewVBox(
				widget.NewLabel("Theme Colors"),
				container.NewGridWithColumns(3,
					container.NewVBox(
						widget.NewLabel("Primary:"),
						primaryColorPicker,
					),
					container.NewVBox(
						widget.NewLabel("Secondary:"),
						secondaryColorPicker,
					),
					container.NewVBox(
						widget.NewLabel("Tertiary:"),
						tertiaryColorPicker,
					),
				),
			),
		)),
	)

	// Items section
	t.itemNameEntry.SetPlaceHolder("Item Name")
	t.itemDescEntry.SetPlaceHolder("Item Description")
	t.itemDescEntry.MultiLine = true
	t.itemPriceEntry.SetPlaceHolder("Price (e.g. 9.99)")

	// Initialize items list
	t.itemsList = widget.NewList(
		func() int {
			if t.existingShop == nil {
				return 0
			}
			return len(t.existingShop.Items)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			editBtn := widget.NewButton("Edit", nil)
			deleteBtn := widget.NewButton("Delete", nil)
			buttonBox := container.NewHBox(editBtn, deleteBtn)
			return container.NewBorder(nil, nil, nil, buttonBox, label)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if t.existingShop == nil || id >= len(t.existingShop.Items) {
				return
			}
			container := item.(*fyne.Container)
			label := container.Objects[0].(*widget.Label)
			buttonBox := container.Objects[1].(*fyne.Container)
			editBtn := buttonBox.Objects[0].(*widget.Button)
			deleteBtn := buttonBox.Objects[1].(*widget.Button)

			label.SetText(fmt.Sprintf("%s - $%.2f", t.existingShop.Items[id].Name, t.existingShop.Items[id].Price))

			editBtn.OnTapped = func() {
				t.handleEditItem(id)
			}
			deleteBtn.OnTapped = func() {
				t.handleDeleteItem(id)
			}
		},
	)

	// Item image upload button
	itemImageBtn := widget.NewButton("Add Item Image", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, t.parent)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			sourcePath := reader.URI().Path()
			t.currentItemImages = append(t.currentItemImages, ImageMapping{
				OriginalPath: sourcePath,
				RelativePath: fmt.Sprintf("items/%s", filepath.Base(sourcePath)),
			})

			img := canvas.NewImageFromFile(sourcePath)
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(100, 100))
			t.itemImagesContainer.Add(img)
			t.itemImagesContainer.Refresh()
		}, t.parent)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		fd.Show()
	})

	clearItemBtn := widget.NewButton("Clear", func() {
		t.itemNameEntry.SetText("")
		t.itemDescEntry.SetText("")
		t.itemPriceEntry.SetText("")
		t.currentItemImages = nil
		t.itemImagesContainer.Objects = nil
		t.itemImagesContainer.Refresh()
	})

	addItemBtn := widget.NewButton("Add Item", func() {
		if t.itemNameEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("item name is required"), t.parent)
			return
		}

		price, err := strconv.ParseFloat(t.itemPriceEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("invalid price format"), t.parent)
			return
		}

		var photoPaths, localPhotoPaths []string
		for _, img := range t.currentItemImages {
			photoPaths = append(photoPaths, img.RelativePath)
			localPhotoPaths = append(localPhotoPaths, img.OriginalPath)
		}

		if t.existingShop == nil {
			t.existingShop = &models.Shop{}
		}

		t.existingShop.Items = append(t.existingShop.Items, models.Item{
			Name:            t.itemNameEntry.Text,
			Description:     t.itemDescEntry.Text,
			Price:           price,
			PhotoPaths:      photoPaths,
			LocalPhotoPaths: localPhotoPaths,
		})

		// Clear the form
		clearItemBtn.OnTapped()

		// Refresh the list
		t.itemsList.Refresh()
	})

	// Create items list container with fixed size
	t.itemsListContainer = container.NewVBox(
		widget.NewLabel("Current Items"),
		t.itemsList,
	)
	t.itemsListContainer.Resize(fyne.NewSize(400, 200))

	generateBtn := widget.NewButton("Generate Shop", func() {
		if err := t.generateShop(); err != nil {
			dialog.ShowError(err, t.parent)
		} else {
			dialog.ShowInformation("Success", "Shop generated successfully!", t.parent)
		}
	})

	publishBtn := widget.NewButton("Generate & Publish to IPFS", func() {
		if err := t.generateAndPublish(); err != nil {
			dialog.ShowError(err, t.parent)
			return
		}
	})

	submitBtn := widget.NewButton("Save Shop", t.handleSubmit)
	submitBtn.Importance = widget.HighImportance

	t.deleteBtn = widget.NewButton("Delete Shop", t.handleDeleteShop)
	t.deleteBtn.Importance = widget.DangerImportance
	// Show delete button only in edit mode
	fmt.Printf("Delete button visibility check - existingShop: %v, shop name: %s\n", t.existingShop != nil, func() string {
		if t.existingShop != nil {
			return t.existingShop.Name
		}
		return "nil"
	}())
	
	if t.existingShop != nil && t.existingShop.Name != "" {
		fmt.Println("Showing delete button")
		t.deleteBtn.Show()
	} else {
		fmt.Println("Hiding delete button")
		t.deleteBtn.Hide()
	}

	actionButtons := container.NewHBox(
		generateBtn,
		publishBtn,
		layout.NewSpacer(),
		t.deleteBtn,
		submitBtn,
	)

	mainContent := container.NewVBox(
		widget.NewLabel("Shop Name"),
		t.nameEntry,
		widget.NewLabel("Email"),
		t.emailEntry,
		actionButtons,
		optionalSettings,
		widget.NewLabel("Items"),
		t.itemsListContainer,
		container.NewHBox(
			widget.NewLabel("Add New Item"),
			layout.NewSpacer(),
			addItemBtn,
			clearItemBtn,
		),
		t.itemNameEntry,
		t.itemDescEntry,
		t.itemPriceEntry,
		itemImageBtn,
		t.itemImagesContainer,
		layout.NewSpacer(),
	)

	scroll := container.NewScroll(mainContent)
	scroll.SetMinSize(fyne.NewSize(400, 600))

	return scroll
}

func (t *ShopCreatorTab) createEditContent() fyne.CanvasObject {
	content := t.createContent()
	contentBox := content.(*fyne.Container)

	// Create a delete button
	deleteBtn := widget.NewButton("Delete Shop", func() {
		// Show confirmation dialog
		dialog.ShowConfirm("Delete Shop", "Are you sure you want to delete this shop? This action cannot be undone.", func(confirmed bool) {
			if !confirmed {
				return
			}
			if err := t.shopMgr.DeleteShop(t.existingShop.Name); err != nil {
				dialog.ShowError(err, t.parent)
				return
			}
			dialog.ShowInformation("Success", "Shop deleted successfully", t.parent)
			if t.onSave != nil {
				t.onSave(nil) // Pass nil to indicate deletion
			}
		}, t.parent)
	})
	deleteBtn.Importance = widget.DangerImportance // Make the button red

	// Add delete button at the bottom
	contentBox.Add(widget.NewSeparator())
	contentBox.Add(container.NewHBox(
		layout.NewSpacer(),
		deleteBtn,
	))

	return content
}

func (t *ShopCreatorTab) handleLogoUpload() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, t.parent)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		// Store the file path
		t.logoPath = reader.URI().Path()

		// Update preview
		t.logoPreviewContainer.Objects = nil
		img := canvas.NewImageFromFile(t.logoPath)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(200, 200))
		t.logoPreviewContainer.Add(img)
		t.logoPreviewContainer.Refresh()
	}, t.parent)

	fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
	fd.Show()
}

func (t *ShopCreatorTab) handleSubmit() {
	if t.existingShop == nil {
		t.existingShop = &models.Shop{}
	}

	t.existingShop.Name = t.nameEntry.Text
	t.existingShop.Description = t.descriptionEntry.Text
	t.existingShop.Location = t.locationEntry.Text
	t.existingShop.Email = t.emailEntry.Text
	t.existingShop.Phone = t.phoneEntry.Text
    
    // Set both logo paths
    if t.logoPath != "" {
        t.existingShop.LocalLogoPath = t.logoPath
        ext := filepath.Ext(t.logoPath)
        t.existingShop.LogoPath = "assets/logos/logo" + ext
    }

	if err := t.existingShop.Validate(); err != nil {
		dialog.ShowError(err, t.parent)
		return
	}

	if t.onSave != nil {
		t.onSave(t.existingShop)
	}

	// Clear the form
	t.nameEntry.SetText("")
	t.descriptionEntry.SetText("")
	t.locationEntry.SetText("")
	t.emailEntry.SetText("")
	t.phoneEntry.SetText("")
	t.logoPath = ""
	t.logoPreviewContainer.Objects = nil
	t.logoPreviewContainer.Refresh()
	t.existingShop = nil
	t.itemsList.Refresh()
}

func (t *ShopCreatorTab) handleEditItem(id widget.ListItemID) {
	if t.existingShop == nil || id >= len(t.existingShop.Items) {
		return
	}

	item := t.existingShop.Items[id]

	// Create dialog for editing
	nameEntry := widget.NewEntry()
	nameEntry.SetText(item.Name)
	nameEntry.SetPlaceHolder("Item Name")

	descEntry := widget.NewEntry()
	descEntry.MultiLine = true
	descEntry.SetText(item.Description)
	descEntry.SetPlaceHolder("Item Description")

	priceEntry := widget.NewEntry()
	priceEntry.SetText(fmt.Sprintf("%.2f", item.Price))
	priceEntry.SetPlaceHolder("Price")

	var itemImages []ImageMapping
	imagePreview := container.NewVBox()

	// Add existing images
	for _, path := range item.LocalPhotoPaths {
		img := canvas.NewImageFromFile(path)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(100, 100))
		imagePreview.Add(img)
		itemImages = append(itemImages, ImageMapping{
			OriginalPath: path,
			RelativePath: fmt.Sprintf("items/%s", filepath.Base(path)),
		})
	}

	selectImageBtn := widget.NewButton("Add Image", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, t.parent)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			sourcePath := reader.URI().Path()
			itemImages = append(itemImages, ImageMapping{
				OriginalPath: sourcePath,
				RelativePath: fmt.Sprintf("items/%s", filepath.Base(sourcePath)),
			})

			img := canvas.NewImageFromFile(sourcePath)
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(100, 100))
			imagePreview.Add(img)
			imagePreview.Refresh()
		}, t.parent)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		fd.Show()
	})

	content := container.NewVBox(
		nameEntry,
		descEntry,
		priceEntry,
		selectImageBtn,
		imagePreview,
	)

	dialog.ShowCustomConfirm("Edit Item", "Save", "Cancel", content, func(save bool) {
		if save {
			price, err := strconv.ParseFloat(priceEntry.Text, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("invalid price: must be a number"), t.parent)
				return
			}

			var photoPaths, localPhotoPaths []string
			for _, img := range itemImages {
				photoPaths = append(photoPaths, img.RelativePath)
				localPhotoPaths = append(localPhotoPaths, img.OriginalPath)
			}

			t.existingShop.Items[id] = models.Item{
				Name:            nameEntry.Text,
				Description:     descEntry.Text,
				Price:           price,
				PhotoPaths:      photoPaths,
				LocalPhotoPaths: localPhotoPaths,
			}

			t.itemsList.Refresh()
		}
	}, t.parent)
}

func (t *ShopCreatorTab) handleDeleteItem(id widget.ListItemID) {
	if t.existingShop == nil || id >= len(t.existingShop.Items) {
		return
	}

	dialog.ShowConfirm("Delete Item", "Are you sure you want to delete this item?", func(delete bool) {
		if delete {
			t.existingShop.Items = append(t.existingShop.Items[:id], t.existingShop.Items[id+1:]...)
			t.itemsList.Refresh()
		}
	}, t.parent)
}

func (t *ShopCreatorTab) generateShop() error {
	if t.existingShop == nil {
		return fmt.Errorf("no shop data available")
	}

	if t.nameEntry.Text == "" {
		return fmt.Errorf("shop name is required")
	}

	// Update shop data
	t.existingShop.Name = t.nameEntry.Text
	t.existingShop.Description = t.descriptionEntry.Text
	t.existingShop.Location = t.locationEntry.Text
	t.existingShop.Email = t.emailEntry.Text
	t.existingShop.Phone = t.phoneEntry.Text

    // Set logo paths
    if t.logoPath != "" {
        t.existingShop.LocalLogoPath = t.logoPath
        ext := filepath.Ext(t.logoPath)
        t.existingShop.LogoPath = "assets/logos/logo" + ext
    }

	// Generate the shop
	if err := t.shopMgr.GenerateShop(t.existingShop); err != nil {
		return fmt.Errorf("failed to generate shop: %w", err)
	}

	// Call onSave to refresh the shop list
	if t.onSave != nil {
		t.onSave(t.existingShop)
	}

	return nil
}

func (t *ShopCreatorTab) generateAndPublish() error {
	// First generate the shop
	if err := t.generateShop(); err != nil {
		return err
	}

	// Make sure the shop is saved
	if err := t.shopMgr.SaveShop(t.existingShop); err != nil {
		return fmt.Errorf("failed to save shop: %w", err)
	}

	// Show progress dialog
	progress := dialog.NewProgress("Publishing", "Publishing shop to IPFS...", t.parent)
	progress.Show()
	defer progress.Hide()

	// Get shop path
	shopPath := t.shopMgr.GetShopPath(t.existingShop.Name)
	htmlPath := filepath.Join(shopPath, "src", "index.html")

	// Publish to IPFS
	cid, err := t.ipfsMgr.Publish(htmlPath, shopPath)
	if err != nil {
		return fmt.Errorf("failed to publish to IPFS: %w", err)
	}

	// Get gateway URL
	gatewayURL := t.ipfsMgr.GetGatewayURL(cid)

	// Show success dialog with the gateway URL
	t.showPublishSuccessDialog(gatewayURL)

	return nil
}

// showPublishSuccessDialog shows a custom dialog with clickable URL and copy button
func (t *ShopCreatorTab) showPublishSuccessDialog(gatewayURL string) {
	// Create hyperlink
	urlLink := widget.NewHyperlink("Open Shop Website", parseURL(gatewayURL))
	
	// Create copy button
	copyBtn := widget.NewButton("Copy URL", func() {
		t.parent.Clipboard().SetContent(gatewayURL)
		dialog.ShowInformation("Success", "URL copied to clipboard!", t.parent)
	})

	// Create container with the link and copy button
	content := container.NewVBox(
		widget.NewLabel("Shop published successfully!"),
		widget.NewLabel("Your shop is available at:"),
		urlLink,
		copyBtn,
	)

	// Show custom dialog
	d := dialog.NewCustom("Success", "Close", content, t.parent)
	d.Show()
}

// helper function to safely parse URL
func parseURL(urlStr string) *url.URL {
	u, err := url.Parse(urlStr)
	if err != nil {
		return &url.URL{Path: urlStr}
	}
	return u
}

// handleDeleteShop handles the deletion of the current shop
func (t *ShopCreatorTab) handleDeleteShop() {
	if t.existingShop == nil || t.existingShop.Name == "" {
		dialog.ShowError(fmt.Errorf("no shop selected to delete"), t.parent)
		return
	}

	// Show confirmation dialog
	dialog.ShowConfirm("Delete Shop",
		fmt.Sprintf("Are you sure you want to delete the shop '%s'? This action cannot be undone.", t.existingShop.Name),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Delete the shop
			if err := t.shopMgr.DeleteShop(t.existingShop.Name); err != nil {
				dialog.ShowError(fmt.Errorf("failed to delete shop: %w", err), t.parent)
				return
			}

			// Call onSave with nil to indicate deletion
			if t.onSave != nil {
				t.onSave(nil)
			}
		},
		t.parent,
	)
}

// LoadExistingShop loads an existing shop's data into the UI
func (t *ShopCreatorTab) LoadExistingShop(shop *models.Shop) {
    fmt.Printf("LoadExistingShop called with shop: %v, name: %s\n", shop != nil, shop.Name)
	t.existingShop = shop

	// Load basic shop details
	t.nameEntry.SetText(shop.Name)
	t.descriptionEntry.SetText(shop.Description)
	t.locationEntry.SetText(shop.Location)
	t.emailEntry.SetText(shop.Email)
	t.phoneEntry.SetText(shop.Phone)

	// Update delete button visibility
	if t.deleteBtn != nil {
		fmt.Println("Showing delete button in LoadExistingShop")
		t.deleteBtn.Show()
	}

	// Load logo if exists
	if shop.LogoPath != "" {
		t.logoPath = shop.LogoPath
		// Get the full shop path and join with the relative logo path
		shopPath := t.shopMgr.GetShopPath(shop.Name)
		srcPath := filepath.Join(shopPath, "src")
		fullLogoPath := filepath.Join(srcPath, shop.LogoPath)
		img := canvas.NewImageFromFile(fullLogoPath)
		img.SetMinSize(fyne.NewSize(100, 100))
		img.Resize(fyne.NewSize(100, 100))
		t.logoPreviewContainer.Objects = nil // Clear existing content
		t.logoPreviewContainer.Add(img)
		t.logoPreviewContainer.Refresh()
	}

	// Refresh items list if it exists
	if t.itemsList != nil {
		t.itemsList.Refresh()
	}
}
