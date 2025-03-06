package ipfs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	shell "github.com/ipfs/go-ipfs-api"
)

func NewIPFSManager(config *Config) (*IPFSManager, error) {
	// Try to find system IPFS first
	systemPath, err := exec.LookPath("ipfs")
	if err == nil {
		// Found system IPFS, check version
		cmd := exec.Command(systemPath, "--version")
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			fmt.Printf("IPFS version output: %s\n", version)
			parts := strings.Fields(version)
			if len(parts) >= 3 {
				version = "v" + parts[2]
				fmt.Printf("Parsed version: %s (min: %s)\n", version, MinCompatibleVersion)
				if version >= MinCompatibleVersion {
					dataPath, err := detectIPFSPath(systemPath)
					if err != nil {
						dataPath = filepath.Join(os.Getenv("HOME"), ".ipfs")
					}
					return &IPFSManager{
						BinaryPath: systemPath,
						DataPath:   dataPath,
						Mode:       SystemIPFS,
					}, nil
				}
			}
		}
	} else {
		fmt.Printf("No system IPFS found: %v\n", err)
	}

	// If system IPFS not found or not compatible, set up app-specific IPFS
	fmt.Printf("Falling back to app-specific IPFS\n")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(homeDir, "indie_node_ipfs")
	wrapperPath := filepath.Join(basePath, "ipfs")
	if runtime.GOOS == "windows" {
		wrapperPath += ".bat"
	}

	// Only set BinaryPath if IPFS actually exists
	binaryPath := ""
	if _, err := os.Stat(wrapperPath); err == nil {
		binaryPath = wrapperPath
	}

	manager := &IPFSManager{
		BinaryPath: binaryPath,                          // Empty if IPFS not found
		DataPath:   filepath.Join(basePath, "ipfs-data"), // Use app-specific data path
		Mode:       AppSpecificIPFS,
		gateways:   make([]GatewayStatus, 0),
	}

	// Initialize the IPFS shell
	manager.Shell = shell.NewShell("localhost:5001")

	if runtime.GOOS == "windows" {
		manager.BinaryPath += ".exe"
	}

	// Only allow custom binary path override, not custom data path
	if config != nil {
		if config.CustomBinaryPath != "" {
			manager.BinaryPath = config.CustomBinaryPath
		}
		// Remove custom data path override to ensure consistency
		if len(config.CustomGateways) > 0 {
			for _, gateway := range config.CustomGateways {
				manager.AddCustomGateway(gateway)
			}
		}
	}

	return manager, nil
}

func detectIPFSPath(binaryPath string) (string, error) {
	if envPath := os.Getenv("IPFS_PATH"); envPath != "" {
		fmt.Printf("Using IPFS_PATH from environment: %s\n", envPath)
		return envPath, nil
	}

	cmd := exec.Command(binaryPath, "config", "show")
	_, err := cmd.Output()
	if err == nil {
		if dir := cmd.Dir; dir != "" {
			fmt.Printf("Using IPFS path from config command: %s\n", dir)
			return dir, nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %v", err)
	}
	defaultPath := filepath.Join(homeDir, ".ipfs")

	if _, err := os.Stat(filepath.Join(defaultPath, "config")); err == nil {
		fmt.Printf("Using default IPFS path: %s\n", defaultPath)
		return defaultPath, nil
	}

	return "", fmt.Errorf("could not detect IPFS repository location")
}

func (m *IPFSManager) EnsureInstalled() error {
	if _, err := os.Stat(m.BinaryPath); os.IsNotExist(err) {
		return m.downloadIPFS()
	}
	return nil
}

func (m *IPFSManager) GetIPFSVersion() (string, error) {
	// Try to get the IPFS path
	ipfsPath, err := exec.LookPath("ipfs")
	if err != nil {
		// If not in PATH, use our custom location
		ipfsPath = m.BinaryPath
	}

	cmd := exec.Command(ipfsPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Parse output to get just the version number
	version := string(output)
	if len(version) > 12 { // Strip "ipfs version " prefix
		version = version[12:]
	}
	return strings.TrimSpace(version), nil
}

func (m *IPFSManager) IsIPFSDownloaded() (bool, string) {
	// Check if binary actually exists at the path
	if _, err := os.Stat(m.BinaryPath); err == nil {
		version, err := m.GetIPFSVersion()
		if err == nil {
			return true, version
		}
		return false, ""
	}
	return false, ""
}

func (m *IPFSManager) downloadIPFS() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Setup paths
	basePath := filepath.Join(homeDir, "indie_node_ipfs")
	binaryPath := filepath.Join(basePath, "ipfs-bin") // Actual binary
	wrapperPath := filepath.Join(basePath, "ipfs")    // Wrapper script
	dataPath := filepath.Join(basePath, "ipfs-data")  // IPFS data directory
	
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
		wrapperPath += ".bat"
	}

	// Create the directories
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create IPFS directory: %v", err)
	}
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return fmt.Errorf("failed to create IPFS data directory: %v", err)
	}

	// Update manager's data path
	m.DataPath = dataPath

	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	var osName string
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	url := fmt.Sprintf("https://dist.ipfs.tech/kubo/%s/kubo_%s_%s-%s.tar.gz",
		IPFSVersion, IPFSVersion, osName, arch)

	fmt.Printf("Downloading IPFS from: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download IPFS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download IPFS: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "ipfs-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	fmt.Printf("Downloading to temporary file: %s\n", tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write download to temporary file: %v", err)
	}
	tmpFile.Close()

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "ipfs_install")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp dir when done

	fmt.Printf("Extracting to temporary directory: %s\n", tempDir)

	// Use platform-specific extraction
	var extractErr error
	if runtime.GOOS == "windows" {
		// For Windows, we'll need a different approach since tar might not be available
		extractErr = extractTarGzWindows(tmpFile.Name(), tempDir)
	} else {
		cmd := exec.Command("tar", "-xzf", tmpFile.Name(), "-C", tempDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			extractErr = fmt.Errorf("tar extraction failed: %v\nOutput: %s", err, string(output))
		}
	}

	if extractErr != nil {
		return extractErr
	}

	// Find the extracted binary
	extractedPath := filepath.Join(tempDir, "kubo", "ipfs")
	if runtime.GOOS == "windows" {
		extractedPath += ".exe"
	}

	// Create the target directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		return fmt.Errorf("failed to create binary directory: %v", err)
	}

	// Remove existing binaries if they exist
	_ = os.Remove(binaryPath)
	_ = os.Remove(wrapperPath)

	// Copy the binary instead of moving (moving across devices can fail)
	src, err := os.Open(extractedPath)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %v", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(binaryPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination binary: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy binary: %v", err)
	}

	// Create wrapper script
	var wrapperContent string
	if runtime.GOOS == "windows" {
		wrapperContent = fmt.Sprintf(`@echo off
set IPFS_PATH=%s
"%s" %%*`, dataPath, binaryPath)
	} else {
		wrapperContent = fmt.Sprintf(`#!/bin/sh
IPFS_PATH=%s exec "%s" "$@"`, dataPath, binaryPath)
	}

	if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to create wrapper script: %v", err)
	}

	// Update binary path to point to wrapper
	m.BinaryPath = wrapperPath

	if err := os.Chmod(m.BinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %v", err)
	}

	fmt.Printf("IPFS installation completed successfully\n")
	return nil
}

// extractTarGzWindows handles tar.gz extraction for Windows systems
func extractTarGzWindows(tarGzPath, destPath string) error {
	// This is a placeholder - you'll need to implement Windows-specific extraction
	// You might want to use a Go-native tar/gzip implementation or a third-party library
	return fmt.Errorf("Windows tar.gz extraction not yet implemented")
}
