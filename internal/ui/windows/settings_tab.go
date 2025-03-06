package windows

import (
	"IndieNode/internal/services/auth"
	"IndieNode/ipfs"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Settings struct {
	window             fyne.Window
	ipfsMgr            *ipfs.IPFSManager
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
}

func NewSettingsTab(window fyne.Window, ipfsMgr *ipfs.IPFSManager) *container.TabItem {
	s := &Settings{
		window:             window,
		ipfsMgr:            ipfsMgr,
		statusLabel:        widget.NewLabel("Checking IPFS status..."),
		addressLabel:       widget.NewLabel("Node Address: Not Running"),
		daemonButton:       widget.NewButton("Start Daemon", nil),
		orbitDBStatusLabel: widget.NewLabel("Status: Not Connected"),
		dbPathLabel:        widget.NewLabel("Database Path: Not Connected"),
		networkModeLabel:   widget.NewLabel("Network Mode: Not Connected"),
		shopsCountLabel:    widget.NewLabel("Shops in Database: 0"),
	}

	s.createUI()
	return container.NewTabItem("Settings", s.content)
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

	ipfsCard.SetContent(container.NewVBox(
		s.pathLabel,
		s.addressLabel,
		widget.NewSeparator(),
		s.statusLabel,
		s.daemonContainer,
		s.installButton,
		showPinnedBtn,
		clearPinsBtn,
	))
	s.content.Add(ipfsCard)

	// OrbitDB Settings section
	orbitDBCard := widget.NewCard("OrbitDB Settings", "", nil)
	orbitDBCard.SetContent(container.NewVBox(
		s.orbitDBStatusLabel,
		s.dbPathLabel,
		s.networkModeLabel,
		s.shopsCountLabel,
	))
	s.content.Add(orbitDBCard)

	// Initial updates
	s.updateIPFSStatus()
	s.updateAddressLabel()
	s.updateInstallButtonVisibility(s.daemonButton)
}

func (s *Settings) handleDaemonControl() {
	if s.ipfsMgr.IsDaemonRunning() {
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
