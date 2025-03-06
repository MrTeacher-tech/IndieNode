package ipfs

import (
	"encoding/json"
	"fmt"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m *IPFSManager) AddFile(path string) (string, error) {
	if m.Shell == nil {
		return "", fmt.Errorf("IPFS shell not initialized")
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash, err := m.Shell.Add(file)
	if err != nil {
		return "", err
	}

	// Pin the content to ensure persistence
	if err := m.Shell.Pin(hash); err != nil {
		return "", fmt.Errorf("failed to pin content: %w", err)
	}

	return hash, nil
}

func (m *IPFSManager) AddDirectory(path string) (string, error) {
	if m.BinaryPath == "" {
		return "", fmt.Errorf("IPFS binary not found")
	}

	// Add directory to IPFS using command
	cmd := exec.Command(m.BinaryPath, "add", "-r", "-Q", path)
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to add directory %s to IPFS: %w", path, err)
	}
	hash := strings.TrimSpace(string(output))
	fmt.Printf("Successfully added directory to IPFS with hash: %s\n", hash)

	// Pin the content
	cmd = exec.Command(m.BinaryPath, "pin", "add", hash)
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to pin content with hash %s: %w", hash, err)
	}
	fmt.Printf("Successfully pinned content with hash: %s\n", hash)

	// Verify the pin exists
	cmd = exec.Command(m.BinaryPath, "pin", "ls", "--type", "recursive")
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error verifying pin: %w", err)
	}
	if !strings.Contains(string(output), hash) {
		return "", fmt.Errorf("pin verification failed for hash: %s", hash)
	}
	fmt.Printf("Verified pin exists for hash: %s\n", hash)

	return hash, nil
}

func (m *IPFSManager) GetGatewayURL(hash string) string {
	m.gatewayLock.Lock()
	defer m.gatewayLock.Unlock()

	fmt.Printf("[DEBUG] Getting gateway URL for hash: %s\n", hash)

	// Initialize gateways if needed
	if len(m.gateways) == 0 {
		fmt.Printf("[DEBUG] No gateways found, initializing...\n")
		gateways := DefaultGateways
		if m.Shell != nil && m.IsDaemonRunning() {
			fmt.Printf("[DEBUG] Daemon is running, adding local gateway\n")
			// Add local gateway first if daemon is running
			gateways = append([]string{"http://127.0.0.1:8080"}, gateways...)
		}

		for _, url := range gateways {
			m.gateways = append(m.gateways, GatewayStatus{
				URL:      url,
				Healthy:  true, // Assume healthy initially
				LastUsed: time.Time{},
			})
		}
	}

	// First try to find a healthy gateway
	for i := range m.gateways {
		gateway := &m.gateways[i]
		if gateway.URL == "" {
			continue
		}

		if gateway.Healthy {
			gateway.LastUsed = time.Now()
			fmt.Printf("[DEBUG] Using healthy gateway: %s\n", gateway.URL)
			// Don't add /src/index.html here - let the caller handle the full path
			return strings.TrimRight(gateway.URL, "/") + "/ipfs/" + hash
		}
	}

	// If no healthy gateway found, use ipfs.io as fallback
	fmt.Printf("[DEBUG] No healthy gateways found, using ipfs.io\n")
	return "https://ipfs.io/ipfs/" + hash
}

func (m *IPFSManager) Publish(htmlPath string, shopPath string) (string, error) {
	if !m.IsDaemonRunning() {
		return "", fmt.Errorf("IPFS daemon is not running")
	}

	// Get the shop directory path (parent of src)
	shopDir := filepath.Dir(filepath.Dir(htmlPath))
	fmt.Printf("Publishing shop from directory: %s\n", shopDir)

	// Use IPFS to add the entire shop directory
	hash, err := m.AddDirectory(shopDir)
	if err != nil {
		return "", fmt.Errorf("error adding directory to IPFS: %v", err)
	}
	fmt.Printf("Added directory to IPFS with hash: %s\n", hash)

	// Get the gateway URL
	baseURL := m.GetGatewayURL(hash)
	fmt.Printf("Base gateway URL: %s\n", baseURL)

	// List directory contents to verify structure
	cmd := exec.Command(m.BinaryPath, "ls", hash)
	output, err := cmd.Output()
	if err == nil {
		fmt.Printf("Directory contents:\n%s\n", string(output))
	}

	// Create metadata file
	metadata := struct {
		CID     string `json:"cid"`
		Gateway string `json:"gateway"`
	}{
		CID:     hash,
		Gateway: baseURL + "/src/index.html",
	}

	// Save metadata to file
	metadataPath := filepath.Join(shopDir, "ipfs_metadata.json")
	metadataJSON, err := json.MarshalIndent(metadata, "", "    ")
	if err != nil {
		return "", fmt.Errorf("error marshaling metadata: %v", err)
	}

	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return "", fmt.Errorf("error writing metadata file: %v", err)
	}
	fmt.Printf("Saved metadata to: %s\n", metadataPath)

	// Update the shop.json with the new CID
	err = updateShopCID(shopPath, hash)
	if err != nil {
		return "", fmt.Errorf("error updating shop.json: %v", err)
	}
	fmt.Printf("Updated shop.json with new CID\n")

	finalURL := baseURL + "/src/index.html"
	fmt.Printf("Final URL: %s\n", finalURL)
	return finalURL, nil
}

func updateShopCID(shopPath string, cid string) error {
	// Load the existing JSON
	jsonData, err := os.ReadFile(shopPath)
	if err != nil {
		return err
	}

	var shopData map[string]interface{}
	err = json.Unmarshal(jsonData, &shopData)
	if err != nil {
		return err
	}

	// Update the CID field
	shopData["CID"] = cid

	// Write the updated JSON back to the file
	updatedJSON, err := json.MarshalIndent(shopData, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(shopPath, updatedJSON, 0644)
}

func (m *IPFSManager) checkGatewayHealth(gateway string) bool {
	// For now, assume all gateways are healthy
	// We can implement more sophisticated health checks later
	return true
}

func (m *IPFSManager) AddCustomGateway(url string) {
	m.gatewayLock.Lock()
	defer m.gatewayLock.Unlock()

	// Check if gateway already exists
	for _, g := range m.gateways {
		if g.URL == url {
			return
		}
	}

	m.gateways = append(m.gateways, GatewayStatus{
		URL:      url,
		Healthy:  true,
		LastUsed: time.Time{},
	})
}

func (m *IPFSManager) CheckShopPublication(shopDir string) (isPublished bool, cid string, gateway string, err error) {
	metadataPath := filepath.Join(shopDir, "ipfs_metadata.json")

	// Check if metadata file exists
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", "", nil // Not an error, just not published
		}
		return false, "", "", fmt.Errorf("error reading metadata: %w", err)
	}

	// Parse the metadata
	var metadata struct {
		CID     string `json:"cid"`
		Gateway string `json:"gateway"`
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return false, "", "", fmt.Errorf("error parsing metadata: %w", err)
	}

	return true, metadata.CID, metadata.Gateway, nil
}

func (m *IPFSManager) RunGarbageCollection() error {
	if m.Shell == nil {
		return fmt.Errorf("IPFS shell not initialized")
	}

	// Run garbage collection using 'repo gc' command
	cmd := exec.Command(m.BinaryPath, "repo", "gc")
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run garbage collection: %w (output: %s)", err, string(output))
	}

	return nil
}

func (m *IPFSManager) UnpublishContent(cid string) error {
	if m.BinaryPath == "" {
		return fmt.Errorf("IPFS binary not found")
	}

	// Unpin the content using CLI command
	cmd := exec.Command(m.BinaryPath, "pin", "rm", cid)
	cmd.Env = append(os.Environ(), fmt.Sprintf("IPFS_PATH=%s", m.DataPath))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unpin content: %w (output: %s)", err, string(output))
	}
	fmt.Printf("Successfully unpinned content with CID: %s\n", cid)

	// Run garbage collection after unpinning
	if err := m.RunGarbageCollection(); err != nil {
		// Log the error but don't fail the unpublish operation
		fmt.Printf("Warning: failed to run garbage collection: %v\n", err)
	}

	return nil
}

func (m *IPFSManager) GetIPFSNode() (interface{}, error) {
	if !m.IsDaemonRunning() {
		return nil, fmt.Errorf("IPFS daemon is not running")
	}

	return m.Shell, nil
}

func (m *IPFSManager) GetGateways() []GatewayStatus {
	m.gatewayLock.RLock()
	defer m.gatewayLock.RUnlock()

	// Initialize gateways if needed
	if len(m.gateways) == 0 {
		m.gatewayLock.RUnlock()
		m.gatewayLock.Lock()
		for _, url := range DefaultGateways {
			m.gateways = append(m.gateways, GatewayStatus{
				URL:      url,
				Healthy:  true,
				LastUsed: time.Time{},
			})
		}
		m.gatewayLock.Unlock()
		m.gatewayLock.RLock()
	}

	// Make a copy to avoid external modifications
	gateways := make([]GatewayStatus, len(m.gateways))
	copy(gateways, m.gateways)

	// Update health status for all gateways
	for i := range gateways {
		if time.Since(gateways[i].LastUsed) > time.Minute {
			gateways[i].Healthy = m.checkGatewayHealth(gateways[i].URL)
		}
	}

	return gateways
}

func (m *IPFSManager) ReinitializeGateways() {
	m.gatewayLock.Lock()
	defer m.gatewayLock.Unlock()

	m.gateways = nil // Clear existing gateways

	// Reinitialize with current DefaultGateways
	for _, url := range DefaultGateways {
		m.gateways = append(m.gateways, GatewayStatus{
			URL:      url,
			Healthy:  true,
			LastUsed: time.Time{},
		})
	}
}
