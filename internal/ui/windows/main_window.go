package windows

import (
	"IndieNode/internal/models"
	"IndieNode/internal/services/auth"
	"IndieNode/internal/services/shop"
	"IndieNode/ipfs"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"

	"fyne.io/fyne/v2/widget"
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

type MainWindow struct {
	app           fyne.App
	window        fyne.Window
	shopMgr       *shop.Manager
	ipfsMgr       *ipfs.IPFSManager
	authSvc       *auth.Service
	content       *fyne.Container
	mainMenu      *fyne.MainMenu
	tabs          *container.AppTabs
	createShopTab *container.TabItem
	viewShopsTab  *container.TabItem
	settingsTab   *container.TabItem
	welcomeTab    *container.TabItem
}

func NewMainWindow(app fyne.App, shopMgr *shop.Manager, ipfsMgr *ipfs.IPFSManager, authSvc *auth.Service) *MainWindow {
	w := &MainWindow{
		app:     app,
		window:  app.NewWindow("IndieNode"),
		shopMgr: shopMgr,
		ipfsMgr: ipfsMgr,
		authSvc: authSvc,
	}

	w.createUI()
	return w
}

func (w *MainWindow) createUI() {
	w.createMainMenu()

	// Create the tabs
	w.welcomeTab = NewWelcomeTab()
	w.createShopTab = w.createShopForm()
	w.viewShopsTab = w.createShopList()
	w.settingsTab = NewSettingsTab(w.window, w.ipfsMgr)

	// Create tabs container with welcome tab first
	w.tabs = container.NewAppTabs(
		w.welcomeTab,
		w.createShopTab,
		w.viewShopsTab,
		w.settingsTab,
	)

	w.content = container.NewVBox(w.tabs)
	w.window.SetContent(w.content)
	w.window.SetMainMenu(w.mainMenu)
	w.window.Resize(fyne.NewSize(800, 600))
}

func (w *MainWindow) createShopForm() *container.TabItem {
	tabItem, shopCreator := NewShopCreatorTab(w.window, w.shopMgr, w.ipfsMgr, func(shop *models.Shop) {
		// Set the owner address from the authenticated user
		if user := w.authSvc.GetAuthenticatedUser(); user != nil {
			shop.OwnerAddress = user.Address
		}

		// Save the shop
		if err := w.shopMgr.SaveShop(shop); err != nil {
			dialog.ShowError(err, w.window)
			return
		}

		// Refresh the shop list
		w.refreshShopList()
	})

	// Load current shop data
	currentShop, err := w.shopMgr.LoadCurrentShop()
	if err != nil {
		dialog.ShowError(err, w.window)
	} else if currentShop != nil {
		shopCreator.LoadExistingShop(currentShop)
	}

	return tabItem
}

func (w *MainWindow) createShopList() *container.TabItem {
	shops, err := w.shopMgr.ListShops()
	if err != nil {
		dialog.ShowError(err, w.window)
		return nil
	}

	list := widget.NewList(
		func() int { return len(shops) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel(""),
				widget.NewButton("Publish", func() {}),
				widget.NewButton("View", func() {}),
				widget.NewButton("Edit", func() {}),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			container := obj.(*fyne.Container)
			label := container.Objects[0].(*widget.Label)
			publishBtn := container.Objects[1].(*widget.Button)
			viewBtn := container.Objects[2].(*widget.Button)
			editBtn := container.Objects[3].(*widget.Button)

			shopName := shops[id]
			label.SetText(shopName)

			// Configure publish button
			publishBtn.OnTapped = func() {
				// Get the shop's directory
				shopDir := w.shopMgr.GetShopPath(shopName)

				// Check if shop is already published
				isPublished, cid, gateway, err := w.ipfsMgr.CheckShopPublication(shopDir)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to check publication status: %w", err), w.window)
					return
				}

				if isPublished {
					// Show the existing publication info
					url := gateway + "/ipfs/" + cid
					dialog.ShowInformation("Already Published",
						"Shop is already published.\nURL: "+url, w.window)
					return
				}

				// Check if shop is generated (look for index.html)
				htmlPath := shopDir + "/src/index.html"
				shopPath := shopDir + "/shop.json"

				// Publish to IPFS
				url, err := w.ipfsMgr.Publish(htmlPath, shopPath)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to publish shop: %w", err), w.window)
					return
				}

				w.showPublishSuccessDialog(url)
			}

			// Configure view button
			viewBtn.OnTapped = func() {
				shopDir := w.shopMgr.GetShopPath(shopName)

				// Convert to absolute path
				absPath, err := filepath.Abs(shopDir)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to get absolute path: %w", err), w.window)
					return
				}

				htmlPath := filepath.Join(absPath, "src", "index.html")

				// Debug logging
				fmt.Printf("Attempting to open shop: %s\n", shopName)
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
				shop, err := w.shopMgr.LoadShop(shopName)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to load shop: %w", err), w.window)
					return
				}

				tabItem, tabContent := NewShopCreatorTab(w.window, w.shopMgr, w.ipfsMgr, func(updatedShop *models.Shop) {
					if updatedShop == nil {
						// Shop was deleted
						for i, item := range w.tabs.Items {
							if item.Text == "Edit Shop: "+shop.Name {
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
						if item.Text == "Edit Shop: "+shop.Name {
							w.tabs.Remove(item)
							// Select the view shops tab
							if len(w.tabs.Items) > i-1 {
								w.tabs.Select(w.tabs.Items[i-1])
							}
							break
						}
					}

					w.refreshShopList()
				})

				// Load the existing shop data into the tab content
				tabContent.LoadExistingShop(shop)

				// Update the tab title
				tabItem.Text = "Edit Shop: " + shop.Name

				w.tabs.Append(tabItem)
				w.tabs.Select(tabItem)
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
	// Get updated shops list
	shops, err := w.shopMgr.ListShops()
	if err != nil {
		dialog.ShowError(err, w.window)
		return
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
	list.Length = func() int { return len(shops) }
	list.UpdateItem = func(id widget.ListItemID, obj fyne.CanvasObject) {
		container := obj.(*fyne.Container)
		label := container.Objects[0].(*widget.Label)
		publishBtn := container.Objects[1].(*widget.Button)
		viewBtn := container.Objects[2].(*widget.Button)
		editBtn := container.Objects[3].(*widget.Button)

		shopName := shops[id]
		label.SetText(shopName)

		// Configure publish button
		publishBtn.OnTapped = func() {
			// Get the shop's directory
			shopDir := w.shopMgr.GetShopPath(shopName)

			// Check if shop is already published
			isPublished, cid, gateway, err := w.ipfsMgr.CheckShopPublication(shopDir)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to check publication status: %w", err), w.window)
				return
			}

			if isPublished {
				// Show the existing publication info
				url := gateway + "/ipfs/" + cid
				dialog.ShowInformation("Already Published",
					"Shop is already published.\nURL: "+url, w.window)
				return
			}

			// Check if shop is generated (look for index.html)
			htmlPath := shopDir + "/src/index.html"
			shopPath := shopDir + "/shop.json"

			// Publish to IPFS
			url, err := w.ipfsMgr.Publish(htmlPath, shopPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to publish shop: %w", err), w.window)
				return
			}

			w.showPublishSuccessDialog(url)
		}

		// Configure view button
		viewBtn.OnTapped = func() {
			shopDir := w.shopMgr.GetShopPath(shopName)

			// Convert to absolute path
			absPath, err := filepath.Abs(shopDir)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to get absolute path: %w", err), w.window)
				return
			}

			htmlPath := filepath.Join(absPath, "src", "index.html")

			// Debug logging
			fmt.Printf("Attempting to open shop: %s\n", shopName)
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
			shop, err := w.shopMgr.LoadShop(shopName)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load shop: %w", err), w.window)
				return
			}

			tabItem, tabContent := NewShopCreatorTab(w.window, w.shopMgr, w.ipfsMgr, func(updatedShop *models.Shop) {
				if updatedShop == nil {
					// Shop was deleted
					for i, item := range w.tabs.Items {
						if item.Text == "Edit Shop: "+shop.Name {
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
					if item.Text == "Edit Shop: "+shop.Name {
						w.tabs.Remove(item)
						// Select the view shops tab
						if len(w.tabs.Items) > i-1 {
							w.tabs.Select(w.tabs.Items[i-1])
						}
						break
					}
				}

				w.refreshShopList()
			})

			// Load the existing shop data into the tab content
			tabContent.LoadExistingShop(shop)

			// Update tab text and select it
			tabItem.Text = "Edit Shop: " + shop.Name
			tabItem.Content = tabContent.createEditContent()

			w.tabs.Append(tabItem)
			w.tabs.Select(tabItem)
		}
	}

	list.Refresh()
}

func (w *MainWindow) showPublishSuccessDialog(url string) {
	// Create hyperlink
	urlLink := widget.NewHyperlink("Open Shop Website", parseURL(url))

	// Create copy button
	copyBtn := widget.NewButton("Copy URL", func() {
		w.window.Clipboard().SetContent(url)
		dialog.ShowInformation("Success", "URL copied to clipboard!", w.window)
	})

	// Create container with the link and copy button
	content := container.NewVBox(
		widget.NewLabel("Shop published successfully!"),
		widget.NewLabel("Your shop is available at:"),
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

func (w *MainWindow) Close() {
	w.window.Close()
}
