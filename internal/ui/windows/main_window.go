package windows

import (
	"IndieNode/internal/models"
	"IndieNode/internal/services/auth"
	"IndieNode/internal/services/shop"
	"IndieNode/ipfs"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/skip2/go-qrcode"
)

// CustomPadding implements a layout with specific padding values
type CustomPadding struct {
	horizontal float32
	vertical   float32
}

func NewCustomPadding(h, v float32) *CustomPadding {
	return &CustomPadding{horizontal: h, vertical: v}
}

func (c *CustomPadding) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for _, o := range objects {
		childSize := o.MinSize()
		w = fyne.Max(w, childSize.Width)
		h = fyne.Max(h, childSize.Height)
	}
	return fyne.NewSize(w+c.horizontal*2, h+c.vertical*2)
}

func (c *CustomPadding) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	pos := fyne.NewPos(c.horizontal, c.vertical)
	for _, o := range objects {
		o.Resize(fyne.NewSize(size.Width-c.horizontal*2, size.Height-c.vertical*2))
		o.Move(pos)
	}
}

// shopInfo stores shop data and publication status
type shopInfo struct {
	name        string
	isPublished bool
}

type MainWindow struct {
	app            fyne.App
	window         fyne.Window
	shopMgr        *shop.Manager
	ipfsMgr        *ipfs.IPFSManager
	authSvc        *auth.Service
	content        *fyne.Container
	mainMenu       *fyne.MainMenu
	tabs           *container.AppTabs
	createShopTab  *container.TabItem
	viewShopsTab   *container.TabItem
	settingsTab    *container.TabItem
	welcomeTab     *container.TabItem
	buttonMap      map[string]*widget.Button
	closeIntercept func()
	shopCreator    *ShopCreatorTab
}

func NewMainWindow(app fyne.App, shopMgr *shop.Manager, ipfsMgr *ipfs.IPFSManager, authSvc *auth.Service) *MainWindow {
	w := &MainWindow{
		app:       app,
		window:    app.NewWindow("IndieNode"), // Initialize the window
		shopMgr:   shopMgr,
		ipfsMgr:   ipfsMgr,
		authSvc:   authSvc,
		buttonMap: make(map[string]*widget.Button),
	}

	w.createUI()
	return w
}

func (w *MainWindow) createUI() {
	w.createMainMenu()

	// Create the tabs
	w.welcomeTab = NewWelcomeTab()
	content, shopCreator := NewShopCreatorTab(w.window, w.shopMgr, w.ipfsMgr, func(updatedShop *models.Shop) {
		// Set the owner address from the authenticated user
		if user := w.authSvc.GetAuthenticatedUser(); user != nil {
			updatedShop.OwnerAddress = user.Address
		}

		// Save the shop
		if err := w.shopMgr.SaveShop(updatedShop); err != nil {
			dialog.ShowError(err, w.window)
			return
		}

		// Refresh the shop list
		w.refreshShopList()
	}, func(url string) {
		// Handle publish success
		w.showPublishSuccessDialog(url)
	})
	w.shopCreator = shopCreator
	w.createShopTab = container.NewTabItem("Create Shop", content)
	w.viewShopsTab = w.createShopList()
	w.settingsTab = NewSettingsTab(w.window, w.ipfsMgr)

	// Create tabs container with welcome tab first
	w.tabs = container.NewAppTabs(
		w.welcomeTab,
		w.createShopTab,
		w.viewShopsTab,
		w.settingsTab,
	)

	// Initialize dev mode if enabled
	if auth.IsDevMode() {
		// Load the dev shop
		shop, err := w.shopMgr.LoadCurrentShop()
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load dev shop: %w", err), w.window)
		} else {
			// Load the dev shop into the UI
			w.shopCreator.LoadExistingShop(shop)
		}
	}

	w.content = container.NewVBox(w.tabs)
	w.window.SetContent(w.content)
	w.window.SetMainMenu(w.mainMenu)
	w.window.Resize(fyne.NewSize(800, 600))
}

func (w *MainWindow) createShopList() *container.TabItem {
	fmt.Println("=== Starting createShopList ===")
	shops, err := w.shopMgr.ListShops()
	if err != nil {
		dialog.ShowError(err, w.window)
		return nil
	}
	fmt.Printf("Found %d shops\n", len(shops))

	// Pre-check publication status for all shops
	shopInfos := make([]shopInfo, len(shops))
	for i, shopName := range shops {
		shopDir := w.shopMgr.GetShopPath(shopName)
		fmt.Printf("Checking publication status for shop: %s\n", shopName)
		isPublished, cid, _, err := w.ipfsMgr.CheckShopPublication(shopDir)
		shopInfos[i] = shopInfo{
			name:        shopName,
			isPublished: err == nil && isPublished && cid != "", // Only consider published if we have a valid CID
		}
		fmt.Printf("Shop %s publication status: %v (cid: %s)\n", shopName, shopInfos[i].isPublished, cid)
	}

	list := widget.NewList(
		func() int {
			return len(shopInfos)
		},
		func() fyne.CanvasObject {
			fmt.Println("CreateItem called - creating template")
			label := widget.NewLabel("")
			publishBtn := widget.NewButton("", nil)
			viewBtn := widget.NewButton("View", nil)
			editBtn := widget.NewButton("Edit", nil)

			return container.NewHBox(label, publishBtn, viewBtn, editBtn)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			fmt.Printf("UpdateItem called for id: %d\n", id)
			containerObj := obj.(*fyne.Container)
			label := containerObj.Objects[0].(*widget.Label)
			publishBtn := containerObj.Objects[1].(*widget.Button)
			viewBtn := containerObj.Objects[2].(*widget.Button)
			editBtn := containerObj.Objects[3].(*widget.Button)

			info := shopInfos[id]
			fmt.Printf("Setting up shop: %s, isPublished: %v\n", info.name, info.isPublished)

			label.SetText(info.name)

			// Update publish button state using the helper
			w.updatePublishButtonState(publishBtn, info.name)

			// Configure view button
			viewBtn.OnTapped = func() {
				shopDir := w.shopMgr.GetShopPath(info.name)

				// Convert to absolute path
				absPath, err := filepath.Abs(shopDir)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to get absolute path: %w", err), w.window)
					return
				}

				htmlPath := filepath.Join(absPath, "src", "index.html")

				// Debug logging
				fmt.Printf("Attempting to open shop: %s\n", info.name)
				fmt.Printf("Shop directory: %s\n", absPath)
				fmt.Printf("HTML path: %s\n", htmlPath)

				// Check if file exists
				if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
					dialog.ShowError(fmt.Errorf("shop file not found at: %s", htmlPath), w.window)
					return
				}

				fileURL := "file://" + htmlPath
				fmt.Printf("Opening URL: %s\n", fileURL)

				if err := w.authSvc.OpenBrowser(fileURL); err != nil {
					dialog.ShowError(fmt.Errorf("failed to open shop in browser: %w", err), w.window)
					return
				}
			}

			// Configure edit button
			editBtn.OnTapped = func() {
				shop, err := w.shopMgr.LoadShop(info.name)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to load shop: %w", err), w.window)
					return
				}

				editContent, creator := NewShopCreatorTab(w.window, w.shopMgr, w.ipfsMgr, func(updatedShop *models.Shop) {
					if updatedShop == nil {
						// Shop was deleted
						for i, item := range w.tabs.Items {
							if item.Text == "Edit Shop: "+info.name {
								w.tabs.Remove(item)
								// Select the view shops tab
								if len(w.tabs.Items) > i-1 {
									w.tabs.Select(w.tabs.Items[i-1])
								}
								break
							}
						}
						w.refreshShopList()
						return
					}

					// Preserve the owner address from the existing shop
					updatedShop.OwnerAddress = shop.OwnerAddress

					if err := w.shopMgr.SaveShop(updatedShop); err != nil {
						dialog.ShowError(err, w.window)
						return
					}
					dialog.ShowInformation("Success", "Shop updated successfully", w.window)

					// Remove the edit tab after successful save
					for i, item := range w.tabs.Items {
						if item.Text == "Edit Shop: "+info.name {
							w.tabs.Remove(item)
							// Select the view shops tab
							if len(w.tabs.Items) > i-1 {
								w.tabs.Select(w.tabs.Items[i-1])
							}
							break
						}
					}

					w.refreshShopList()
				}, func(url string) {
					// Pass the main window's showPublishSuccessDialog
					w.showPublishSuccessDialog(url)
				})

				// Load the existing shop data into the tab content
				creator.LoadExistingShop(shop)

				// Create the tab item with the edit content
				tabItem := container.NewTabItemWithIcon("Edit Shop: "+shop.Name, theme.DocumentIcon(), editContent)

				w.tabs.Append(tabItem)
				w.tabs.SelectTab(tabItem)
			}
		},
	)

	// Use custom padding for the list with moderate padding values
	customPadding := NewCustomPadding(12, 8)

	// Create a scroll container to hold the list
	scrollContainer := container.NewScroll(list)
	scrollContainer.Resize(fyne.NewSize(800, 600)) // Set initial size

	// Create the padded container that fills available space
	paddedContent := container.New(customPadding,
		container.NewMax( // Max container will make its child fill available space
			scrollContainer,
		),
	)

	return container.NewTabItem("View Shops", paddedContent)
}

func (w *MainWindow) refreshShopList() {
	fmt.Println("=== Starting refreshShopList ===")
	// Get updated shops list
	shops, err := w.shopMgr.ListShops()
	if err != nil {
		dialog.ShowError(err, w.window)
		return
	}
	fmt.Printf("Found %d shops\n", len(shops))

	// Pre-check publication status for all shops
	shopInfos := make([]shopInfo, len(shops))
	for i, shopName := range shops {
		shopDir := w.shopMgr.GetShopPath(shopName)
		fmt.Printf("Checking publication status for shop: %s\n", shopName)
		isPublished, cid, _, err := w.ipfsMgr.CheckShopPublication(shopDir)
		shopInfos[i] = shopInfo{
			name:        shopName,
			isPublished: err == nil && isPublished && cid != "", // Only consider published if we have a valid CID
		}
		fmt.Printf("Shop %s publication status: %v (cid: %s)\n", shopName, shopInfos[i].isPublished, cid)
	}

	// Navigate through the container hierarchy to find the list
	paddedContainer := w.viewShopsTab.Content.(*fyne.Container)
	if len(paddedContainer.Objects) == 0 {
		return
	}

	maxContainer := paddedContainer.Objects[0].(*fyne.Container)
	if len(maxContainer.Objects) == 0 {
		return
	}

	scrollContainer := maxContainer.Objects[0].(*container.Scroll)
	list := scrollContainer.Content.(*widget.List)

	// Update the list data
	list.Length = func() int {
		return len(shopInfos)
	}

	list.CreateItem = func() fyne.CanvasObject {
		fmt.Println("CreateItem called - creating template")
		label := widget.NewLabel("")
		publishBtn := widget.NewButton("", nil)
		viewBtn := widget.NewButton("View", nil)
		editBtn := widget.NewButton("Edit", nil)

		return container.NewHBox(label, publishBtn, viewBtn, editBtn)
	}

	list.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		fmt.Printf("UpdateItem called for id: %d\n", id)
		containerObj := obj.(*fyne.Container)
		label := containerObj.Objects[0].(*widget.Label)
		publishBtn := containerObj.Objects[1].(*widget.Button)
		viewBtn := containerObj.Objects[2].(*widget.Button)
		editBtn := containerObj.Objects[3].(*widget.Button)

		info := shopInfos[id]
		fmt.Printf("Setting up shop: %s, isPublished: %v\n", info.name, info.isPublished)

		label.SetText(info.name)

		// Store or retrieve button from map
		if existingBtn, ok := w.buttonMap[info.name]; ok {
			fmt.Printf("Found existing button for %s with text: %s\n", info.name, existingBtn.Text)
			publishBtn.OnTapped = existingBtn.OnTapped
			publishBtn.SetText(existingBtn.Text)
		} else {
			// Double-check publication status before setting initial state
			shopDir := w.shopMgr.GetShopPath(info.name)
			isPublished, cid, _, err := w.ipfsMgr.CheckShopPublication(shopDir)
			buttonText := "Publish"
			if err == nil && isPublished && cid != "" {
				buttonText = "URL Info"
				info.isPublished = true
			} else {
				info.isPublished = false
			}
			fmt.Printf("Creating new button for %s with text: %s (isPublished: %v, cid: %s)\n",
				info.name, buttonText, info.isPublished, cid)
			publishBtn.SetText(buttonText)

			publishBtn.OnTapped = func() {
				fmt.Printf("Publish button tapped for shop: %s\n", info.name)
				shopDir := w.shopMgr.GetShopPath(info.name)

				// Always check current status when button is clicked
				isPublished, cid, gateway, err := w.ipfsMgr.CheckShopPublication(shopDir)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to check publication status: %w", err), w.window)
					return
				}

				if isPublished && cid != "" {
					url := gateway + "/ipfs/" + cid + "/src/index.html"
					fmt.Printf("Shop %s is already published at: %s\n", info.name, url)
					w.showPublishSuccessDialog(url)
					publishBtn.SetText("URL Info")
					info.isPublished = true
					w.buttonMap[info.name] = publishBtn
					return
				}

				fmt.Printf("Publishing shop: %s\n", info.name)
				htmlPath := shopDir + "/src/index.html"
				shopPath := shopDir + "/shop.json"

				url, err := w.ipfsMgr.Publish(htmlPath, shopPath)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to publish shop: %w", err), w.window)
					return
				}

				fmt.Printf("Successfully published shop %s at: %s\n", info.name, url)
				w.showPublishSuccessDialog(url)

				// Update button state
				publishBtn.SetText("URL Info")
				info.isPublished = true
				shopInfos[id] = info
				w.buttonMap[info.name] = publishBtn

				// Force refresh the list to ensure state is consistent
				w.refreshShopList()
			}

			w.buttonMap[info.name] = publishBtn
		}

		// Configure view button
		viewBtn.OnTapped = func() {
			shopDir := w.shopMgr.GetShopPath(info.name)

			// Convert to absolute path
			absPath, err := filepath.Abs(shopDir)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to get absolute path: %w", err), w.window)
				return
			}

			htmlPath := filepath.Join(absPath, "src", "index.html")

			// Debug logging
			fmt.Printf("Attempting to open shop: %s\n", info.name)
			fmt.Printf("Shop directory: %s\n", absPath)
			fmt.Printf("HTML path: %s\n", htmlPath)

			// Check if file exists
			if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
				dialog.ShowError(fmt.Errorf("shop file not found at: %s", htmlPath), w.window)
				return
			}

			fileURL := "file://" + htmlPath
			fmt.Printf("Opening URL: %s\n", fileURL)

			if err := w.authSvc.OpenBrowser(fileURL); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open shop in browser: %w", err), w.window)
				return
			}
		}

		// Configure edit button
		editBtn.OnTapped = func() {
			shop, err := w.shopMgr.LoadShop(info.name)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load shop: %w", err), w.window)
				return
			}

			content, _ := NewShopCreatorTab(w.window, w.shopMgr, w.ipfsMgr, func(updatedShop *models.Shop) {
				if updatedShop == nil {
					// Shop was deleted
					for i, item := range w.tabs.Items {
						if item.Text == "Edit Shop: "+info.name {
							w.tabs.Remove(item)
							// Select the view shops tab
							if len(w.tabs.Items) > i-1 {
								w.tabs.Select(w.tabs.Items[i-1])
							}
							break
						}
					}
					w.refreshShopList()
					return
				}

				// Preserve the owner address from the existing shop
				updatedShop.OwnerAddress = shop.OwnerAddress

				if err := w.shopMgr.SaveShop(updatedShop); err != nil {
					dialog.ShowError(err, w.window)
					return
				}
				dialog.ShowInformation("Success", "Shop updated successfully", w.window)

				// Remove the edit tab after successful save
				for i, item := range w.tabs.Items {
					if item.Text == "Edit Shop: "+info.name {
						w.tabs.Remove(item)
						// Select the view shops tab
						if len(w.tabs.Items) > i-1 {
							w.tabs.Select(w.tabs.Items[i-1])
						}
						break
					}
				}

				w.refreshShopList()
			}, func(url string) {
				// Pass the main window's showPublishSuccessDialog
				w.showPublishSuccessDialog(url)
			})

			// Create new tab item
			newTabItem := container.NewTabItemWithIcon("Edit Shop: "+shop.Name, theme.DocumentIcon(), content)

			w.tabs.Append(newTabItem)
			w.tabs.SelectTab(newTabItem)
		}
	}

	// Refresh all items
	list.Refresh()
}

func (w *MainWindow) showPublishSuccessDialog(url string) {
	// Sanitize the URL first
	sanitizedURL := sanitizeIPFSURL(url)

	// Generate QR code with sanitized URL
	qr, err := qrcode.New(sanitizedURL, qrcode.Medium)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to generate QR code: %w", err), w.window)
		return
	}

	// Convert QR code to PNG bytes
	pngBytes, err := qr.PNG(256)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to generate QR code image: %w", err), w.window)
		return
	}

	// Create image from bytes
	img := canvas.NewImageFromReader(bytes.NewReader(pngBytes), "QR Code")
	img.FillMode = canvas.ImageFillOriginal
	img.SetMinSize(fyne.NewSize(256, 256))

	// Create hyperlink with sanitized URL
	urlLink := widget.NewHyperlink("Open Shop Website", parseURL(url))

	// Create copy button with sanitized URL
	copyBtn := widget.NewButton("Copy URL", func() {
		w.window.Clipboard().SetContent(sanitizedURL)
		dialog.ShowInformation("Success", "URL copied to clipboard!", w.window)
	})

	// Create container with the link and copy button
	content := container.NewVBox(
		widget.NewLabel("Shop published successfully!"),
		widget.NewLabel("Scan QR code or use URL below:"),
		img,
		widget.NewLabel(sanitizedURL),
		urlLink,
		copyBtn,
	)

	// Show custom dialog
	d := dialog.NewCustom("Success", "Close", content, w.window)
	d.Show()
}

func (w *MainWindow) createMainMenu() {
	w.mainMenu = fyne.NewMainMenu(
		fyne.NewMenu("File",
			fyne.NewMenuItem("Quit", func() {
				w.window.Close()
			}),
		),
	)
}

func (w *MainWindow) Show() {
	w.window.Show()
}

func (w *MainWindow) SetCloseIntercept(callback func()) {
	w.closeIntercept = callback
	w.window.SetCloseIntercept(callback)
}

func (w *MainWindow) Close() {
	w.window.Close()
}

// updatePublishButtonState updates the state and behavior of a publish button based on shop publication status
func (w *MainWindow) updatePublishButtonState(publishBtn *widget.Button, shopName string) {
	shopDir := w.shopMgr.GetShopPath(shopName)
	isPublished, cid, gateway, _ := w.ipfsMgr.CheckShopPublication(shopDir)

	if isPublished && cid != "" {
		publishBtn.SetText("URL Info")
		publishBtn.OnTapped = func() {
			url := gateway + "/ipfs/" + cid + "/src/index.html"
			w.showPublishSuccessDialog(url)
		}
	} else {
		publishBtn.SetText("Publish")
		publishBtn.OnTapped = func() {
			// Existing publish logic
			htmlPath := filepath.Join(shopDir, "src", "index.html")
			shopPath := filepath.Join(shopDir, "shop.json")

			url, err := w.ipfsMgr.Publish(htmlPath, shopPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to publish shop: %w", err), w.window)
				return
			}

			w.showPublishSuccessDialog(url)
			w.updatePublishButtonState(publishBtn, shopName) // Update button state after publishing
		}
	}
	w.buttonMap[shopName] = publishBtn
}
