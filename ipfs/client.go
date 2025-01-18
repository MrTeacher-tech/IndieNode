package ipfs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	if m.Shell == nil {
		return "", fmt.Errorf("IPFS shell not initialized")
	}

	// Add directory to IPFS
	hash, err := m.Shell.AddDir(path)
	if err != nil {
		return "", err
	}

	// Pin the content to ensure persistence
	if err := m.Shell.Pin(hash); err != nil {
		return "", fmt.Errorf("failed to pin content: %w", err)
	}

	return hash, nil
}

func (m *IPFSManager) GetGatewayURL(hash string) string {
	m.gatewayLock.Lock()
	defer m.gatewayLock.Unlock()

	// Initialize gateways if needed
	if len(m.gateways) == 0 {
		gateways := DefaultGateways
		if m.Shell != nil && m.IsDaemonRunning() {
			// Add local gateway first if daemon is running
			gateways = append([]string{"http://localhost:8080"}, gateways...)
		}

		for _, url := range gateways {
			m.gateways = append(m.gateways, GatewayStatus{
				URL:      url,
				Healthy:  true, // Assume healthy initially
				LastUsed: time.Time{},
			})
		}
	}

	// Try to find a healthy gateway
	for i := range m.gateways {
		gateway := &m.gateways[i]

		// Check health if it hasn't been used recently
		if time.Since(gateway.LastUsed) > time.Minute {
			gateway.Healthy = m.checkGatewayHealth(gateway.URL)
		}

		if gateway.Healthy {
			gateway.LastUsed = time.Now()
			return gateway.URL + "/ipfs/" + hash + "/src/index.html"
		}
	}

	// If no healthy gateway found, return the first one (with warning)
	fmt.Printf("Warning: No healthy IPFS gateways found, using first available\n")
	return m.gateways[0].URL + "/ipfs/" + hash + "/src/index.html"
}

func (m *IPFSManager) checkGatewayHealth(gateway string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", gateway+"/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG/readme", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
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

func (m *IPFSManager) Publish(htmlPath string, shopPath string) (string, error) {
	// Use IPFS to add the existing HTML file
	hash, err := m.AddFile(htmlPath)
	if err != nil {
		return "", fmt.Errorf("error adding file to IPFS: %v", err)
	}

	// Get the gateway URL using the hash returned from AddFile
	url := m.GetGatewayURL(hash)

	// Update the shop.json with the new CID
	err = updateShopCID(shopPath, hash)
	if err != nil {
		return "", fmt.Errorf("error updating shop.json: %v", err)
	}

	return url, nil
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

// CheckShopPublication checks if a shop has been published by looking for ipfs_metadata.json
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

// UnpublishContent unpins content from IPFS using its CID
func (m *IPFSManager) UnpublishContent(cid string) error {
	if m.Shell == nil {
		return fmt.Errorf("IPFS shell not initialized")
	}

	// Unpin the content
	if err := m.Shell.Unpin(cid); err != nil {
		return fmt.Errorf("failed to unpin content: %w", err)
	}

	return nil
}
