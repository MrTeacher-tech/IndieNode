package shop

import (
	"IndieNode/internal/models"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const webhookURL = "https://hook.us2.make.com/1d4izlejxpoim9rgxiec0yc69l6f3p1m"

type WebhookPayload struct {
	Shop    *models.Shop `json:"shop"`
	IPFSURL string       `json:"ipfs_url"`
}

type WebhookResponse struct {
	URL string `json:"url"`
}

// SendWebhook sends shop data and IPFS URL to the webhook and returns the response URL
func SendWebhook(shop *models.Shop, ipfsURL string) (string, error) {
	payload := WebhookPayload{
		Shop:    shop,
		IPFSURL: ipfsURL,
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create the request
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("webhook request failed with status: %d", resp.StatusCode)
	}

	// Parse response
	var response WebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode webhook response: %w", err)
	}

	// If no URL in response, return the original IPFS URL
	if response.URL == "" {
		return ipfsURL, nil
	}

	return response.URL, nil
}
