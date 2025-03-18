package windows

import (
	"IndieNode/db/orbitdb"
	"IndieNode/internal/api"
	"IndieNode/internal/services/auth"
	"IndieNode/ipfs"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Settings struct {
	window             fyne.Window
	ipfsMgr            *ipfs.IPFSManager
	orbitMgr           *orbitdb.Manager
	apiServer          *api.Server
	apiPort            int
	statusLabel        *widget.Label
	addressLabel       *widget.Label
	daemonButton       *widget.Button
	orbitDBStatusLabel *widget.Label
	dbPathLabel        *widget.Label
	networkModeLabel   *widget.Label
	shopsCountLabel    *widget.Label
	content            *fyne.Container
	pathLabel          *widget.Label
	installButton      *widget.Button
	daemonContainer    *fyne.Container
	keepDaemonCheck    *widget.Check

	// API status elements
	apiStatusLabel    *widget.Label
	apiUrlLabel       *widget.Label
	apiStartTimeLabel *widget.Label
	apiRequestsLabel  *widget.Label

	// For periodic updates
	updateTimer    *time.Ticker
	stopUpdateChan chan bool
}

func NewSettingsTab(window fyne.Window, ipfsMgr *ipfs.IPFSManager, orbitMgr *orbitdb.Manager, apiServer *api.Server, apiPort int) *container.TabItem {
	s := &Settings{
		window:             window,
		ipfsMgr:            ipfsMgr,
		orbitMgr:           orbitMgr,
		apiServer:          apiServer,
		apiPort:            apiPort,
		statusLabel:        widget.NewLabel("Checking IPFS status..."),
		addressLabel:       widget.NewLabel("Node Address: Not Running"),
		daemonButton:       widget.NewButton("Start Daemon", nil),
		orbitDBStatusLabel: widget.NewLabel("Status: Not Connected"),
		dbPathLabel:        widget.NewLabel("Database Path: Not Connected"),
		networkModeLabel:   widget.NewLabel("Network Mode: Not Connected"),
		shopsCountLabel:    widget.NewLabel("Shops in Database: 0"),
		apiStatusLabel:     widget.NewLabel("Status: Unknown"),
		apiUrlLabel:        widget.NewLabel("URL: Not available"),
		apiStartTimeLabel:  widget.NewLabel("Active Since: Not available"),
		apiRequestsLabel:   widget.NewLabel("Requests Served: 0"),
		stopUpdateChan:     make(chan bool),
	}

	// Start periodic updates
	s.startPeriodicUpdates()

	s.createUI()
	return container.NewTabItem("Settings", s.content)
}

// startPeriodicUpdates starts a ticker to update status information periodically
func (s *Settings) startPeriodicUpdates() {
	s.updateTimer = time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-s.updateTimer.C:
				// Update API status if it exists
				if s.apiServer != nil {
					s.updateAPIStatus()
				}
			case <-s.stopUpdateChan:
				s.updateTimer.Stop()
				return
			}
		}
	}()
}

// stopPeriodicUpdates stops the periodic updates
func (s *Settings) stopPeriodicUpdates() {
	if s.updateTimer != nil {
		s.stopUpdateChan <- true
	}
}

func (s *Settings) createUI() {
	s.content = container.NewVBox()

	// IndieNode Settings section
	indieNodeCard := widget.NewCard("IndieNode Settings", "", nil)
	versionLabel := widget.NewLabelWithStyle(
		"Version: pre-alpha",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)
	goVersionLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("Go Version: %s", runtime.Version()),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)
	devModeLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("DEV_MODE: %v", auth.IsDevMode()),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)
	indieNodeCard.SetContent(container.NewVBox(
		versionLabel,
		goVersionLabel,
		devModeLabel,
	))
	s.content.Add(indieNodeCard)

	// IPFS Settings section
	ipfsCard := widget.NewCard("IPFS Settings", "", nil)

	// Create a more informative path label
	s.pathLabel = widget.NewLabelWithStyle(
		"IPFS Status: Not installed",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)
	s.updatePathLabel()

	s.daemonButton.OnTapped = s.handleDaemonControl

	// New Install IPFS button
	s.installButton = widget.NewButton("Install IPFS", s.handleInstallIPFS)

	// Create a container that will hold the daemon button
	s.daemonContainer = container.NewVBox()
	s.daemonContainer.Add(s.daemonButton)

	// Create the clear button
	clearPinsBtn := widget.NewButtonWithIcon("Clear All Pins", theme.ContentRemoveIcon(), func() {
		if !s.ipfsMgr.IsDaemonRunning() {
			dialog.ShowInformation("IPFS Status", "Daemon is not running", s.window)
			return
		}

		// Show confirmation dialog
		confirm := dialog.NewConfirm(
			"Clear All Pins",
			"Are you sure you want to unpin all content? This will remove all pinned data and cannot be undone.",
			func(confirmed bool) {
				if !confirmed {
					return
				}

				// Get list of pins first
				cmd := exec.Command(s.ipfsMgr.BinaryPath, "pin", "ls", "--type", "recursive")
				cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", s.ipfsMgr.DataPath))
				output, err := cmd.CombinedOutput()
				if err != nil {
					dialog.ShowError(fmt.Errorf("Error getting pins: %v", err), s.window)
					return
				}

				pins := strings.Split(strings.TrimSpace(string(output)), "\n")
				if len(pins) == 0 || (len(pins) == 1 && pins[0] == "") {
					dialog.ShowInformation("Clear Pins", "No pins found to clear", s.window)
					return
				}

				// Unpin each item
				for _, line := range pins {
					if line == "" {
						continue
					}
					// Extract CID from the line
					parts := strings.Fields(line)
					if len(parts) == 0 {
						continue
					}
					cid := parts[0]

					err := s.ipfsMgr.UnpublishContent(cid)
					if err != nil {
						dialog.ShowError(fmt.Errorf("Error unpinning %s: %v", cid, err), s.window)
						return
					}
				}

				// Show success message
				dialog.ShowInformation("Clear Pins", "Successfully unpinned all content and ran garbage collection", s.window)
			},
			s.window,
		)
		confirm.Show()
	})
	clearPinsBtn.Importance = widget.HighImportance

	showPinnedBtn := widget.NewButton("Show Pinned Content", func() {
		if !s.ipfsMgr.IsDaemonRunning() {
			dialog.ShowInformation("IPFS Status", "Daemon is not running", s.window)
			return
		}

		// Use ipfs command with proper environment
		cmd := exec.Command(s.ipfsMgr.BinaryPath, "pin", "ls", "--type", "recursive")
		cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", s.ipfsMgr.DataPath))
		output, err := cmd.CombinedOutput()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Error getting pins: %v (output: %s)", err, string(output)), s.window)
			return
		}

		// Parse the output
		pins := strings.Split(strings.TrimSpace(string(output)), "\n")
		numPins := 0
		if len(pins) > 0 && pins[0] != "" {
			numPins = len(pins)
		}

		// Create scrollable content
		var info strings.Builder
		info.WriteString(fmt.Sprintf("Number of pinned items: %d\n\n", numPins))
		for _, line := range pins {
			if line != "" {
				// Extract just the CID from the line (removes the "recursive" suffix)
				parts := strings.Fields(line)
				if len(parts) > 0 {
					cid := parts[0]
					info.WriteString(fmt.Sprintf("CID: %s\n", cid))
				}
			}
		}

		// Create a text widget with the content
		text := widget.NewTextGridFromString(info.String())

		// Put it in a scrollable container
		scroll := container.NewScroll(text)
		scroll.SetMinSize(fyne.NewSize(400, 300))

		// Create and show a custom dialog
		d := dialog.NewCustom("Pinned Content", "Close", scroll, s.window)
		d.Resize(fyne.NewSize(500, 400))
		d.Show()
	})

	// Create keep daemon running checkbox
	s.keepDaemonCheck = widget.NewCheck("Keep Daemon Running", func(checked bool) {
		// Save the preference
		if checked {
			// Keep daemon running to ensure published shops remain accessible
			dialog.ShowInformation("IPFS Daemon", "IPFS daemon will be kept running to ensure published shops remain accessible", s.window)
		}
	})
	// Default to checked for stable network configuration
	s.keepDaemonCheck.SetChecked(true)

	ipfsCard.SetContent(container.NewVBox(
		s.pathLabel,
		s.addressLabel,
		widget.NewSeparator(),
		s.statusLabel,
		s.daemonContainer,
		s.keepDaemonCheck,
		s.installButton,
		showPinnedBtn,
		clearPinsBtn,
	))
	s.content.Add(ipfsCard)

	// Add the API Server Status section
	apiCard := widget.NewCard("API Server Status", "", nil)

	// Create test API button
	testApiBtn := widget.NewButton("Test API Connection", func() {
		s.testAPIConnection()
	})

	// Refresh button
	refreshApiBtn := widget.NewButtonWithIcon("Refresh Status", theme.ViewRefreshIcon(), func() {
		s.updateAPIStatus()
	})

	// Create buttons container
	apiButtonsContainer := container.NewHBox(
		testApiBtn,
		refreshApiBtn,
	)

	apiCard.SetContent(container.NewVBox(
		s.apiStatusLabel,
		s.apiUrlLabel,
		s.apiStartTimeLabel,
		s.apiRequestsLabel,
		widget.NewSeparator(),
		apiButtonsContainer,
	))
	s.content.Add(apiCard)

	// Perform initial API status update
	s.updateAPIStatus()

	// Shop Data Storage section
	storageCard := widget.NewCard("Shop Data Storage", "", nil)

	// Update storage status labels
	if s.orbitMgr != nil && s.orbitMgr.IsConnected() {
		s.orbitDBStatusLabel.SetText("Status: Connected")
		s.dbPathLabel.SetText(fmt.Sprintf("Storage Path: %s", s.orbitMgr.GetDatabasePath()))
		s.networkModeLabel.SetText("IPFS Gateway: 127.0.0.1:8080")

		// Get shop count
		shopCount, err := s.orbitMgr.GetShopCount()
		if err == nil {
			s.shopsCountLabel.SetText(fmt.Sprintf("Shops in Database: %d", shopCount))
		}
	}

	// OrbitDB connection info
	var orbitDBInfoWidgets []fyne.CanvasObject

	if s.orbitMgr != nil && s.orbitMgr.IsOrbitDBInitialized() {
		orbitDBInfoWidgets = append(orbitDBInfoWidgets,
			widget.NewLabelWithStyle("OrbitDB: Initialized", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel(fmt.Sprintf("Directory: %s", s.orbitMgr.GetOrbitDBDirectory())),
		)

		// Add database statistics
		ctx := context.Background()
		dbStats, err := s.orbitMgr.GetDatabaseStats(ctx)
		if err == nil && dbStats != nil {
			statsContainer := container.NewVBox(
				widget.NewLabelWithStyle("Database Statistics:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(fmt.Sprintf("Total Databases: %d", dbStats.TotalDatabases)),
				widget.NewLabel(fmt.Sprintf("Loaded Databases: %d", dbStats.LoadedDatabases)),
				widget.NewLabel(fmt.Sprintf("Total Records: %d", dbStats.TotalRecords)),
				widget.NewLabel(fmt.Sprintf("Avg Records/Store: %d", dbStats.AvgRecordsPerStore)),
			)
			orbitDBInfoWidgets = append(orbitDBInfoWidgets, statsContainer)
		}

		// Show connected databases
		connectedDatabases := s.orbitMgr.GetConnectedDatabases()
		if len(connectedDatabases) > 0 {
			orbitDBInfoWidgets = append(orbitDBInfoWidgets,
				widget.NewLabelWithStyle(fmt.Sprintf("Connected Databases (%d):", len(connectedDatabases)),
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			)

			// Create a container for database entries
			dbListContainer := container.NewVBox()

			for _, dbInfo := range connectedDatabases {
				// Create a container for each database with management buttons
				dbContainer := container.NewHBox(
					widget.NewLabel(fmt.Sprintf("Shop: %s", dbInfo.ShopID)),
					container.NewHBox(
						widget.NewButtonWithIcon("", theme.VisibilityIcon(), func() {
							// Show database details
							s.showDatabaseDetails(dbInfo.ShopID)
						}),
						widget.NewButtonWithIcon("", theme.MediaReplayIcon(), func() {
							// Reload database
							s.reloadDatabase(dbInfo.ShopID)
						}),
						widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
							// Close database
							s.closeDatabase(dbInfo.ShopID)
						}),
					),
				)
				dbListContainer.Add(dbContainer)

				// Add address in a monospace font in a separate line
				dbListContainer.Add(widget.NewLabelWithStyle(
					fmt.Sprintf("Address: %s", dbInfo.Address),
					fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))

				// Add API endpoint for this shop
				if s.apiServer != nil {
					apiEndpoint := fmt.Sprintf("http://localhost:%d/api/shops/%s", s.apiPort, dbInfo.ShopID)
					apiEndpointContainer := container.NewHBox(
						widget.NewLabelWithStyle(
							fmt.Sprintf("API Endpoint: %s", apiEndpoint),
							fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}),
						widget.NewButtonWithIcon("", theme.ComputerIcon(), func() {
							// Test API endpoint for this shop
							s.testShopAPIEndpoint(dbInfo.ShopID)
						}),
					)
					dbListContainer.Add(apiEndpointContainer)

					// Add items endpoint
					itemsEndpoint := fmt.Sprintf("http://localhost:%d/api/shops/%s/items", s.apiPort, dbInfo.ShopID)
					itemsEndpointContainer := container.NewHBox(
						widget.NewLabelWithStyle(
							fmt.Sprintf("Items Endpoint: %s", itemsEndpoint),
							fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}),
						widget.NewButtonWithIcon("", theme.ComputerIcon(), func() {
							// Test items API endpoint for this shop
							s.testShopItemsAPIEndpoint(dbInfo.ShopID)
						}),
					)
					dbListContainer.Add(itemsEndpointContainer)
				}

				// Add a separator between shops
				dbListContainer.Add(widget.NewSeparator())
			}

			// Add scrollable container for databases
			dbScroll := container.NewScroll(dbListContainer)
			dbScroll.SetMinSize(fyne.NewSize(500, 200))
			orbitDBInfoWidgets = append(orbitDBInfoWidgets, dbScroll)
		} else {
			orbitDBInfoWidgets = append(orbitDBInfoWidgets,
				widget.NewLabel("No connected databases"),
			)
		}

		// Add database management buttons
		dbManagementBtns := container.NewHBox(
			widget.NewButtonWithIcon("Reload All DBs", theme.ViewRefreshIcon(), func() {
				s.reloadAllDatabases()
			}),
			widget.NewButtonWithIcon("Export All", theme.DocumentSaveIcon(), func() {
				s.exportAllShops()
			}),
		)
		orbitDBInfoWidgets = append(orbitDBInfoWidgets, dbManagementBtns)
	} else {
		orbitDBInfoWidgets = append(orbitDBInfoWidgets,
			widget.NewLabel("OrbitDB: Not initialized"),
		)
	}

	// Add refresh button for OrbitDB info
	refreshBtn := widget.NewButtonWithIcon("Refresh OrbitDB Info", theme.ViewRefreshIcon(), func() {
		s.createUI() // Recreate the UI to refresh the information
	})
	orbitDBInfoWidgets = append(orbitDBInfoWidgets, refreshBtn)

	orbitDBInfo := container.NewVBox(orbitDBInfoWidgets...)

	storageCard.SetContent(container.NewVBox(
		s.orbitDBStatusLabel,
		s.dbPathLabel,
		s.networkModeLabel,
		s.shopsCountLabel,
		widget.NewSeparator(),
		orbitDBInfo,
	))
	s.content.Add(storageCard)

	// Initial updates
	s.updateIPFSStatus()
	s.updateAddressLabel()
	s.updateInstallButtonVisibility(s.daemonButton)
}

// updateAPIStatus updates the API server status in the UI
func (s *Settings) updateAPIStatus() {
	if s.apiServer == nil {
		s.apiStatusLabel.SetText("Status: API Server not available")
		s.apiUrlLabel.SetText("URL: Not available")
		s.apiStartTimeLabel.SetText("Active Since: Not available")
		s.apiRequestsLabel.SetText("Requests Served: 0")
		return
	}

	status := s.apiServer.GetStatus()

	// Update status labels
	if status.Running {
		s.apiStatusLabel.SetText("Status: Running")
	} else {
		s.apiStatusLabel.SetText("Status: Stopped")
	}

	s.apiUrlLabel.SetText(fmt.Sprintf("URL: http://localhost:%d/api", s.apiPort))
	s.apiStartTimeLabel.SetText(fmt.Sprintf("Active Since: %s", status.StartTime.Format(time.RFC3339)))
	s.apiRequestsLabel.SetText(fmt.Sprintf("Requests Served: %d", status.RequestCount))

	// Refresh the labels
	s.apiStatusLabel.Refresh()
	s.apiUrlLabel.Refresh()
	s.apiStartTimeLabel.Refresh()
	s.apiRequestsLabel.Refresh()
}

// testAPIConnection tests the API server connection
func (s *Settings) testAPIConnection() {
	if s.apiServer == nil {
		dialog.ShowError(fmt.Errorf("API server not available"), s.window)
		return
	}

	// Make a simple request to the API status endpoint
	apiURL := fmt.Sprintf("http://localhost:%d/api", s.apiPort)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to create request: %v", err), s.window)
		return
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("API server not responding: %v", err), s.window)
		return
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		dialog.ShowError(fmt.Errorf("API server returned unexpected status: %s", resp.Status), s.window)
		return
	}

	// Show success message
	dialog.ShowInformation("API Test", "API server is responding correctly", s.window)

	// Update the status after a successful test
	s.updateAPIStatus()
}

// testShopAPIEndpoint tests the API endpoint for a specific shop
func (s *Settings) testShopAPIEndpoint(shopID string) {
	if s.apiServer == nil {
		dialog.ShowError(fmt.Errorf("API server not available"), s.window)
		return
	}

	// Make a request to the shop endpoint
	apiURL := fmt.Sprintf("http://localhost:%d/api/shops/%s", s.apiPort, shopID)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to create request: %v", err), s.window)
		return
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("API server not responding: %v", err), s.window)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to read response: %v", err), s.window)
		return
	}

	// Create a text widget with the content
	text := widget.NewTextGridFromString(string(body))

	// Put it in a scrollable container
	scroll := container.NewScroll(text)
	scroll.SetMinSize(fyne.NewSize(500, 300))

	// Create and show a custom dialog
	d := dialog.NewCustom(fmt.Sprintf("API Response for Shop: %s", shopID), "Close", scroll, s.window)
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// testShopItemsAPIEndpoint tests the items API endpoint for a specific shop
func (s *Settings) testShopItemsAPIEndpoint(shopID string) {
	if s.apiServer == nil {
		dialog.ShowError(fmt.Errorf("API server not available"), s.window)
		return
	}

	// Make a request to the shop items endpoint
	apiURL := fmt.Sprintf("http://localhost:%d/api/shops/%s/items", s.apiPort, shopID)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to create request: %v", err), s.window)
		return
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		dialog.ShowError(fmt.Errorf("API server not responding: %v", err), s.window)
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to read response: %v", err), s.window)
		return
	}

	// Create a text widget with the content
	text := widget.NewTextGridFromString(string(body))

	// Put it in a scrollable container
	scroll := container.NewScroll(text)
	scroll.SetMinSize(fyne.NewSize(500, 300))

	// Create and show a custom dialog
	d := dialog.NewCustom(fmt.Sprintf("API Items Response for Shop: %s", shopID), "Close", scroll, s.window)
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// GetKeepDaemonRunning returns whether the daemon should be kept running
func (s *Settings) GetKeepDaemonRunning() bool {
	return s.keepDaemonCheck != nil && s.keepDaemonCheck.Checked
}

func (s *Settings) handleDaemonControl() {
	if s.ipfsMgr.IsDaemonRunning() {
		if s.keepDaemonCheck.Checked {
			dialog.ShowInformation("IPFS Daemon", "Cannot stop daemon while 'Keep Daemon Running' is enabled. This ensures published shops remain accessible.", s.window)
			return
		}
		if err := s.ipfsMgr.StopDaemon(); err != nil {
			dialog.ShowError(err, s.window)
			return
		}
	} else {
		if err := s.ipfsMgr.StartDaemon(); err != nil {
			dialog.ShowError(err, s.window)
			return
		}
	}
	s.updateIPFSStatus()
	s.updateAddressLabel()
}

func (s *Settings) updateIPFSStatus() {
	if s.ipfsMgr.IsDaemonRunning() {
		s.statusLabel.SetText("Status: Running")
		s.daemonButton.SetText("Stop Daemon")
	} else {
		s.statusLabel.SetText("Status: Stopped")
		s.daemonButton.SetText("Start Daemon")
	}
	s.statusLabel.Refresh()
	s.daemonButton.Refresh()
}

func (s *Settings) updatePathLabel() {
	if s.ipfsMgr.BinaryPath == "" {
		s.pathLabel.SetText("IPFS Status: Not installed")
	} else {
		s.pathLabel.SetText(fmt.Sprintf("IPFS Path: %s", s.ipfsMgr.BinaryPath))
	}
	s.pathLabel.Refresh()
}

func (s *Settings) updateAddressLabel() {
	if s.ipfsMgr.IsDaemonRunning() {
		nodeID, addrs, err := s.ipfsMgr.GetNodeInfo()
		if err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				if strings.Contains(addr, "127.0.0.1") && strings.Contains(addr, "5001") {
					s.addressLabel.SetText(fmt.Sprintf("Node Address: %s\nNode ID: %s", addr, nodeID))
					return
				}
			}
			s.addressLabel.SetText(fmt.Sprintf("Node Address: %s\nNode ID: %s", addrs[0], nodeID))
		} else {
			s.addressLabel.SetText("Node Address: Error getting address")
		}
	} else {
		s.addressLabel.SetText("Node Address: Not Running")
	}
	s.addressLabel.Refresh()
}

func (s *Settings) updateInstallButtonVisibility(daemonButton *widget.Button) {
	if s.ipfsMgr.BinaryPath != "" {
		s.installButton.Hide()
		s.daemonContainer.Show()
	} else {
		s.installButton.Show()
		s.daemonContainer.Hide()
	}
}

func (s *Settings) handleInstallIPFS() {
	err := s.ipfsMgr.EnsureInstalled()
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	s.updatePathLabel()
	s.updateInstallButtonVisibility(s.daemonButton)
	s.updateIPFSStatus()

	if err := s.ipfsMgr.StartDaemon(); err != nil {
		dialog.ShowError(err, s.window)
		return
	}
}

// showDatabaseDetails shows detailed information about a specific database
func (s *Settings) showDatabaseDetails(shopID string) {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	ctx := context.Background()
	dbStatus, err := s.orbitMgr.GetDatabaseStatus(ctx, shopID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to get database status: %w", err), s.window)
		return
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Shop: %s", dbStatus.ShopID),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(fmt.Sprintf("Address: %s", dbStatus.Address),
			fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}),
		widget.NewLabel(fmt.Sprintf("Loaded: %t", dbStatus.IsLoaded)),
		widget.NewLabel(fmt.Sprintf("Record Count: %d", dbStatus.RecordCount)),
	)

	// Add repair button if database is loaded
	if dbStatus.IsLoaded {
		repairBtn := widget.NewButtonWithIcon("Repair Database", theme.MediaReplayIcon(), func() {
			s.repairDatabase(shopID)
		})
		content.Add(repairBtn)
	} else {
		loadBtn := widget.NewButtonWithIcon("Load Database", theme.MailForwardIcon(), func() {
			s.loadDatabase(shopID)
		})
		content.Add(loadBtn)
	}

	dialog.ShowCustom("Database Details", "Close", content, s.window)
}

// reloadDatabase reloads a specific database
func (s *Settings) reloadDatabase(shopID string) {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	ctx := context.Background()

	// Close and remove from cache
	if err := s.orbitMgr.CloseShopDatabase(shopID); err != nil {
		dialog.ShowError(fmt.Errorf("failed to close database: %w", err), s.window)
		return
	}

	// Reopen the database
	_, err := s.orbitMgr.GetShopDatabase(ctx, shopID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to reload database: %w", err), s.window)
		return
	}

	dialog.ShowInformation("Success", fmt.Sprintf("Database for shop %s reloaded successfully", shopID), s.window)
	s.createUI() // Refresh UI
}

// closeDatabase closes a specific database and removes it from cache
func (s *Settings) closeDatabase(shopID string) {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	if err := s.orbitMgr.CloseShopDatabase(shopID); err != nil {
		dialog.ShowError(fmt.Errorf("failed to close database: %w", err), s.window)
		return
	}

	dialog.ShowInformation("Success", fmt.Sprintf("Database for shop %s closed successfully", shopID), s.window)
	s.createUI() // Refresh UI
}

// loadDatabase loads a database that isn't currently loaded
func (s *Settings) loadDatabase(shopID string) {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	ctx := context.Background()
	_, err := s.orbitMgr.GetShopDatabase(ctx, shopID)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to load database: %w", err), s.window)
		return
	}

	dialog.ShowInformation("Success", fmt.Sprintf("Database for shop %s loaded successfully", shopID), s.window)
	s.createUI() // Refresh UI
}

// repairDatabase attempts to repair a database
func (s *Settings) repairDatabase(shopID string) {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	// Confirm with the user
	dialog.ShowConfirm("Repair Database",
		fmt.Sprintf("Are you sure you want to attempt to repair the database for shop %s?", shopID),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			ctx := context.Background()
			if err := s.orbitMgr.RepairShopDatabase(ctx, shopID); err != nil {
				dialog.ShowError(fmt.Errorf("failed to repair database: %w", err), s.window)
				return
			}

			dialog.ShowInformation("Success",
				fmt.Sprintf("Database for shop %s repaired successfully", shopID),
				s.window)
			s.createUI() // Refresh UI
		},
		s.window)
}

// reloadAllDatabases reloads all databases
func (s *Settings) reloadAllDatabases() {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	// Confirm with the user
	dialog.ShowConfirm("Reload All Databases",
		"Are you sure you want to reload all databases? This will close and reopen all connections.",
		func(confirmed bool) {
			if !confirmed {
				return
			}

			ctx := context.Background()
			if err := s.orbitMgr.ReloadAllDatabases(ctx); err != nil {
				dialog.ShowError(fmt.Errorf("failed to reload databases: %w", err), s.window)
				return
			}

			dialog.ShowInformation("Success", "All databases reloaded successfully", s.window)
			s.createUI() // Refresh UI
		},
		s.window)
}

// exportAllShops exports all shops to a directory
func (s *Settings) exportAllShops() {
	if s.orbitMgr == nil || !s.orbitMgr.IsConnected() {
		dialog.ShowError(fmt.Errorf("not connected to OrbitDB"), s.window)
		return
	}

	// Get all connected databases
	connectedDBs := s.orbitMgr.GetConnectedDatabases()
	if len(connectedDBs) == 0 {
		dialog.ShowInformation("Export", "No databases to export", s.window)
		return
	}

	// Create export directory
	exportDir := filepath.Join(s.orbitMgr.GetDatabasePath(), "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		dialog.ShowError(fmt.Errorf("failed to create export directory: %w", err), s.window)
		return
	}

	var exportErrors []string
	var exportCount int

	ctx := context.Background()
	for _, dbInfo := range connectedDBs {
		exportData, err := s.orbitMgr.ExportShopData(ctx, dbInfo.ShopID)
		if err != nil {
			exportErrors = append(exportErrors, fmt.Sprintf("Failed to export shop %s: %v", dbInfo.ShopID, err))
			continue
		}

		// Write to file
		exportPath := filepath.Join(exportDir, dbInfo.ShopID+"-export.json")
		if err := os.WriteFile(exportPath, exportData, 0644); err != nil {
			exportErrors = append(exportErrors, fmt.Sprintf("Failed to write export for shop %s: %v", dbInfo.ShopID, err))
			continue
		}

		exportCount++
	}

	// Show result
	if len(exportErrors) > 0 {
		errorMsg := strings.Join(exportErrors, "\n")
		dialog.ShowError(fmt.Errorf("Export completed with errors:\n%s", errorMsg), s.window)
	} else {
		dialog.ShowInformation("Export Successful",
			fmt.Sprintf("Successfully exported %d shops to %s", exportCount, exportDir),
			s.window)
	}
}
