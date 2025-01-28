package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"IndieNode/internal/models"
	"IndieNode/ipfs"

	orbitdb "berty.tech/go-orbit-db"
	"berty.tech/go-orbit-db/iface"
	icore "github.com/ipfs/interface-go-ipfs-core"
)

// NewManager creates a new OrbitDB manager
func NewManager(ctx context.Context, config *Config, ipfsManager *ipfs.IPFSManager) (*Manager, error) {
	if config == nil {
		config = &Config{
			Directory:    "./orbitdb",
			NetworkMode:  "public",
			Timeout:      time.Minute * 1,
			IPFSEndpoint: "http://localhost:5001",
		}
	}

	manager := &Manager{
		ctx:    ctx,
		config: config,
	}

	if err := manager.connect(ipfsManager); err != nil {
		return nil, fmt.Errorf("failed to connect to OrbitDB: %w", err)
	}

	return manager, nil
}

// connect establishes connection to OrbitDB using existing IPFS node
func (m *Manager) connect(ipfsManager *ipfs.IPFSManager) error {
	if !ipfsManager.IsDaemonRunning() {
		return fmt.Errorf("IPFS daemon is not running")
	}

	// Get IPFS node from manager
	ipfsNode, err := ipfsManager.GetIPFSNode()
	if err != nil {
		return fmt.Errorf("failed to get IPFS node: %w", err)
	}

	// Type assert the ipfsNode to CoreAPI
	coreAPI, ok := ipfsNode.(icore.CoreAPI)
	if !ok {
		return fmt.Errorf("failed to convert IPFS node to CoreAPI")
	}

	// Create OrbitDB instance
	dir := m.config.Directory
	db, err := orbitdb.NewOrbitDB(context.Background(), coreAPI, &orbitdb.NewOrbitDBOptions{
		Directory: &dir,
	})
	if err != nil {
		return fmt.Errorf("failed to create OrbitDB instance: %w", err)
	}

	// Create/Open shops database
	shopStore, err := db.KeyValue(context.Background(), "shops", &orbitdb.CreateDBOptions{
		Directory: &m.config.Directory,
		Create:    &[]bool{true}[0],
	})
	if err != nil {
		return fmt.Errorf("failed to create/open shops database: %w", err)
	}

	m.db = db
	m.shopStore = shopStore
	m.isConnected = true

	log.Println("Successfully connected to OrbitDB")
	return nil
}

// Close closes the OrbitDB connection
func (m *Manager) Close() error {
	if m.db != nil {
		if err := m.db.(iface.OrbitDB).Close(); err != nil {
			return fmt.Errorf("failed to close OrbitDB: %w", err)
		}
	}
	m.isConnected = false
	return nil
}

// IsConnected returns the connection status
func (m *Manager) IsConnected() bool {
	return m.isConnected && m.db != nil
}

// GetShopStore returns the shop store instance
func (m *Manager) GetShopStore() orbitdb.Store {
	return m.shopStore
}

// VerifyOwnership checks if the given address is authorized to perform operations
func (m *Manager) VerifyOwnership(ownerAddress string) bool {
	// For now, we'll implement a basic ownership check
	// In a production environment, you might want to add more sophisticated checks
	// such as verifying signatures or checking against a smart contract

	if ownerAddress == "" {
		return false
	}

	// Add your additional ownership verification logic here if needed
	return true
}

// GetDatabasePath returns the current OrbitDB database directory path
func (m *Manager) GetDatabasePath() string {
	if !m.isConnected {
		return "Not Connected"
	}
	return m.config.Directory
}

// GetNetworkMode returns the current OrbitDB network mode
func (m *Manager) GetNetworkMode() string {
	return m.config.NetworkMode
}

// StoreShop stores a shop in OrbitDB
func (m *Manager) StoreShop(shop *models.Shop) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Use shop's Ethereum address as the key
	key := shop.OwnerAddress
	// Convert shop to JSON for storage
	shopData, err := json.Marshal(shop)
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	// Store in OrbitDB using Put since it's a KeyValue store
	kvStore := m.shopStore.(iface.KeyValueStore)
	_, err = kvStore.Put(context.Background(), key, shopData)
	if err != nil {
		return fmt.Errorf("failed to store shop in OrbitDB: %w", err)
	}

	log.Printf("Successfully stored shop for owner %s in OrbitDB", key)
	return nil
}

// GetShopCount returns the total number of shops in the database
func (m *Manager) GetShopCount() (int, error) {
	if !m.IsConnected() {
		return 0, fmt.Errorf("not connected to OrbitDB")
	}

	// Get all entries using KeyValueStore interface
	kvStore := m.shopStore.(iface.KeyValueStore)
	entries := kvStore.All()
	return len(entries), nil
}

// Additional methods will be added here...
