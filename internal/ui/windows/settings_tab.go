package windows

import (
	"IndieNode/internal/services/auth"
	"IndieNode/ipfs"
	"fmt"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	pathLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("IPFS Path: %s", s.ipfsMgr.BinaryPath),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	s.daemonButton.OnTapped = s.handleDaemonControl

	ipfsCard.SetContent(container.NewVBox(
		pathLabel,
		s.addressLabel,
		widget.NewSeparator(),
		s.statusLabel,
		s.daemonButton,
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
