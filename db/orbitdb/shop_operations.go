package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"berty.tech/go-orbit-db/iface"
	"github.com/ethereum/go-ethereum/common"
)

// CreateShop creates a new shop in OrbitDB
func (m *Manager) CreateShop(ctx context.Context, shop *ShopData) error {
	if !m.isConnected {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Verify ownership
	if !m.VerifyOwnership(shop.Owner) {
		return fmt.Errorf("unauthorized: only shop owner can create shop")
	}

	// Validate shop data
	if err := validateShopData(shop); err != nil {
		return fmt.Errorf("invalid shop data: %w", err)
	}

	// Set timestamps
	now := time.Now()
	shop.Created = now
	shop.Updated = now

	// Convert to JSON
	shopJSON, err := json.Marshal(shop)
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	// Store the shop data in OrbitDB
	if _, err := m.shopStore.(iface.KeyValueStore).Put(ctx, shop.ID, shopJSON); err != nil {
		return fmt.Errorf("failed to store shop data: %w", err)
	}

	return nil
}

// GetShop retrieves a shop from OrbitDB
func (m *Manager) GetShop(ctx context.Context, shopID string) (*ShopData, error) {
	if !m.isConnected {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Get shop data from store
	shopJSON, err := m.shopStore.(iface.KeyValueStore).Get(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop data: %w", err)
	}

	if shopJSON == nil {
		return nil, fmt.Errorf("shop not found: %s", shopID)
	}

	// Parse JSON data
	var shop ShopData
	if err := json.Unmarshal(shopJSON, &shop); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shop data: %w", err)
	}

	return &shop, nil
}

// UpdateShop updates an existing shop in OrbitDB
func (m *Manager) UpdateShop(ctx context.Context, shop *ShopData) error {
	if !m.isConnected {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Verify ownership
	if !m.VerifyOwnership(shop.Owner) {
		return fmt.Errorf("unauthorized: only shop owner can update shop")
	}

	// Check if shop exists
	existing, err := m.GetShop(ctx, shop.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing shop: %w", err)
	}

	// Preserve creation time
	shop.Created = existing.Created
	shop.Updated = time.Now()

	// Validate shop data
	if err := validateShopData(shop); err != nil {
		return fmt.Errorf("invalid shop data: %w", err)
	}

	// Convert to JSON
	shopJSON, err := json.Marshal(shop)
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	// Update in OrbitDB
	if _, err := m.shopStore.(iface.KeyValueStore).Put(ctx, shop.ID, shopJSON); err != nil {
		return fmt.Errorf("failed to update shop data: %w", err)
	}

	return nil
}

// DeleteShop removes a shop from OrbitDB
func (m *Manager) DeleteShop(ctx context.Context, shopID, ownerAddress string) error {
	if !m.isConnected {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Verify ownership
	if !m.VerifyOwnership(ownerAddress) {
		return fmt.Errorf("unauthorized: only shop owner can delete shop")
	}

	// Check if shop exists and is owned by the requester
	existing, err := m.GetShop(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get existing shop: %w", err)
	}

	if existing.Owner != ownerAddress {
		return fmt.Errorf("unauthorized: only shop owner can delete shop")
	}

	// Delete from OrbitDB
	if _, err := m.shopStore.(iface.KeyValueStore).Delete(ctx, shopID); err != nil {
		return fmt.Errorf("failed to delete shop: %w", err)
	}

	return nil
}

// ListShops returns all shops owned by the given address
func (m *Manager) ListShops(ctx context.Context, ownerAddress string) ([]*ShopData, error) {
	if !m.isConnected {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	if !common.IsHexAddress(ownerAddress) {
		return nil, fmt.Errorf("invalid ethereum address")
	}

	// Get all entries from the store
	entries := m.shopStore.(iface.KeyValueStore).All()

	var shops []*ShopData
	// Iterate through all shops
	for _, value := range entries {
		var shop ShopData
		if err := json.Unmarshal(value, &shop); err != nil {
			return nil, fmt.Errorf("failed to unmarshal shop data: %w", err)
		}

		// Only include shops owned by the specified address
		if shop.Owner == ownerAddress {
			shops = append(shops, &shop)
		}
	}

	return shops, nil
}

// AddShopAsset adds a new asset (logo or item image) to IPFS and updates the shop
func (m *Manager) AddShopAsset(ctx context.Context, shopID string, assetType string, assetCID string) error {
	if !m.isConnected {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Get existing shop
	shop, err := m.GetShop(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get shop: %w", err)
	}

	// Verify ownership
	if !m.VerifyOwnership(shop.Owner) {
		return fmt.Errorf("unauthorized: only shop owner can add assets")
	}

	// Update shop assets based on type
	switch assetType {
	case "logo":
		shop.Assets.LogoCID = assetCID
	case "item":
		shop.Assets.ItemImageCIDs = append(shop.Assets.ItemImageCIDs, assetCID)
	default:
		return fmt.Errorf("invalid asset type: %s", assetType)
	}

	// Update shop in OrbitDB
	return m.UpdateShop(ctx, shop)
}

// RemoveShopAsset removes an asset from the shop's assets
func (m *Manager) RemoveShopAsset(ctx context.Context, shopID string, assetType string, assetCID string) error {
	if !m.isConnected {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Get existing shop
	shop, err := m.GetShop(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get shop: %w", err)
	}

	// Verify ownership
	if !m.VerifyOwnership(shop.Owner) {
		return fmt.Errorf("unauthorized: only shop owner can remove assets")
	}

	// Remove asset based on type
	switch assetType {
	case "logo":
		if shop.Assets.LogoCID == assetCID {
			shop.Assets.LogoCID = ""
		}
	case "item":
		// Filter out the specified CID
		newCIDs := make([]string, 0)
		for _, cid := range shop.Assets.ItemImageCIDs {
			if cid != assetCID {
				newCIDs = append(newCIDs, cid)
			}
		}
		shop.Assets.ItemImageCIDs = newCIDs
	default:
		return fmt.Errorf("invalid asset type: %s", assetType)
	}

	// Update shop in OrbitDB
	return m.UpdateShop(ctx, shop)
}

// validateShopData performs basic validation of shop data
func validateShopData(shop *ShopData) error {
	if shop == nil {
		return fmt.Errorf("shop data cannot be nil")
	}

	if shop.ID == "" {
		return fmt.Errorf("shop ID is required")
	}

	if !common.IsHexAddress(shop.Owner) {
		return fmt.Errorf("invalid ethereum address for shop owner")
	}

	if shop.Name == "" {
		return fmt.Errorf("shop name is required")
	}

	return nil
}
