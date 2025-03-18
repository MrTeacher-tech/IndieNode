package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"berty.tech/go-orbit-db/iface"
	"berty.tech/go-orbit-db/stores/documentstore"
)

// ShopMetadata contains essential metadata about a shop including its OrbitDB address
type ShopMetadata struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Owner          string `json:"owner"`
	OrbitDBAddress string `json:"orbitDbAddress"`
}

// SaveShopMetadata saves just the shop metadata including the OrbitDB address
func (m *Manager) SaveShopMetadata(ctx context.Context, metadata *ShopMetadata) error {
	if !m.isConnected {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Validate
	if metadata.ID == "" {
		return fmt.Errorf("shop ID is required")
	}

	// Store metadata in a separate file from the shop data
	metadataPath := filepath.Join(m.config.Directory, metadata.ID+"-metadata.json")
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal shop metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save shop metadata: %w", err)
	}

	return nil
}

// GetShopMetadata retrieves shop metadata by shop ID
func (m *Manager) GetShopMetadata(ctx context.Context, shopID string) (*ShopMetadata, error) {
	if !m.isConnected {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Read metadata file
	metadataPath := filepath.Join(m.config.Directory, shopID+"-metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return nil without error if metadata doesn't exist yet
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read shop metadata: %w", err)
	}

	// Parse metadata
	var metadata ShopMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shop metadata: %w", err)
	}

	return &metadata, nil
}

// GetShopDatabase retrieves or creates an OrbitDB database for a shop
func (m *Manager) GetShopDatabase(ctx context.Context, shopID string) (iface.DocumentStore, error) {
	// Check if we have a cached database
	m.dbsMutex.RLock()
	if db, exists := m.shopDBs[shopID]; exists {
		m.dbsMutex.RUnlock()
		return db, nil
	}
	m.dbsMutex.RUnlock()

	// Get the shop metadata to check if there's an existing database
	metadata, err := m.GetShopMetadata(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop metadata: %w", err)
	}

	var db iface.DocumentStore

	if metadata != nil && metadata.OrbitDBAddress != "" {
		// We have an existing database, reopen it
		log.Printf("Reopening existing database for shop %s: %s", shopID, metadata.OrbitDBAddress)

		// Create options for opening an existing database
		dbOptions := &iface.CreateDBOptions{}

		// Open the existing database
		store, err := m.orbitDB.Open(ctx, metadata.OrbitDBAddress, dbOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to open database for shop %s: %w", shopID, err)
		}

		// Ensure it's a document store
		var ok bool
		db, ok = store.(iface.DocumentStore)
		if !ok {
			store.Close()
			return nil, fmt.Errorf("database for shop %s is not a document store", shopID)
		}
	} else {
		// Create a new database
		log.Printf("Creating new database for shop %s", shopID)

		// Get document store options from the documentstore package
		docStoreOptions := documentstore.DefaultStoreOptsForMap("id")

		// Create the database options
		create := true
		dbOptions := &iface.CreateDBOptions{
			Create:            &create,
			StoreType:         new(string),
			StoreSpecificOpts: docStoreOptions,
		}
		*dbOptions.StoreType = "docstore"

		// Create the new document store
		store, err := m.orbitDB.Create(ctx, "shop-"+shopID, "docstore", dbOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create document store for shop %s: %w", shopID, err)
		}

		// Ensure it's a document store
		var ok bool
		db, ok = store.(iface.DocumentStore)
		if !ok {
			store.Close()
			return nil, fmt.Errorf("created store for shop %s is not a document store", shopID)
		}

		// Save the address for future use
		if metadata == nil {
			metadata = &ShopMetadata{
				ID: shopID,
			}
		}

		metadata.OrbitDBAddress = store.Address().String()

		// Save the updated metadata
		if err := m.SaveShopMetadata(ctx, metadata); err != nil {
			return nil, fmt.Errorf("failed to save shop metadata: %w", err)
		}

		log.Printf("Created new database for shop %s at address: %s", shopID, metadata.OrbitDBAddress)
	}

	// Load the data
	if err := db.Load(ctx, -1); err != nil {
		return nil, fmt.Errorf("failed to load database for shop %s: %w", shopID, err)
	}

	// Cache the database
	m.dbsMutex.Lock()
	m.shopDBs[shopID] = db
	m.dbsMutex.Unlock()

	return db, nil
}
