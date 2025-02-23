package main

import (
	"IndieNode/internal/models"
	"IndieNode/internal/services/shop"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TestWebhook reads shop data and IPFS info and sends it to the webhook
func DevWebhook() error {
	// Read shop.json from the Dev Test Shop
	shopPath := filepath.Join("shops", "Dev Test Shop", "shop.json")
	shopData, err := os.ReadFile(shopPath)
	if err != nil {
		return fmt.Errorf("failed to read shop.json: %w", err)
	}

	var shopInfo models.Shop
	if err := json.Unmarshal(shopData, &shopInfo); err != nil {
		return fmt.Errorf("failed to parse shop.json: %w", err)
	}

	// Read ipfs_metadata.json
	ipfsPath := filepath.Join("shops", "Dev Test Shop", "ipfs_metadata.json")
	ipfsData, err := os.ReadFile(ipfsPath)
	if err != nil {
		return fmt.Errorf("failed to read ipfs_metadata.json, probably no Dev Test Shop created: %w", err)
	}

	var ipfsInfo struct {
		Gateway string `json:"gateway"`
	}
	if err := json.Unmarshal(ipfsData, &ipfsInfo); err != nil {
		return fmt.Errorf("failed to parse ipfs_metadata.json: %w", err)
	}

	// Send webhook
	fmt.Printf("Sending webhook with shop data: %s (URL name: %s)\n", shopInfo.Name, shopInfo.URLName)
	fmt.Println("IPFS Gateway:", ipfsInfo.Gateway)

	url, err := shop.SendWebhook(&shopInfo, ipfsInfo.Gateway)
	if err != nil {
		return fmt.Errorf("webhook error: %w", err)
	}

	fmt.Println("Webhook response URL:", url)
	return nil
}

func main() {
	if err := DevWebhook(); err != nil {
		fmt.Printf("Error testing webhook: %v\n", err)
		os.Exit(1)
	}
}
