package orbitdb

import (
	"context"
	"time"

	orbitdb "berty.tech/go-orbit-db"
	"berty.tech/go-orbit-db/iface"
)

// Config holds OrbitDB configuration
type Config struct {
	Directory    string        // Base directory for OrbitDB files
	NetworkMode  string        // "public" or "private"
	Timeout      time.Duration // Connection timeout
	IPFSEndpoint string        // IPFS API endpoint
}

// Manager handles OrbitDB operations
type Manager struct {
	ctx         context.Context
	config      *Config
	db          orbitdb.OrbitDB
	shopStore   orbitdb.Store
	isConnected bool
}

// ShopData represents the shop structure in OrbitDB
type ShopData struct {
	ID          string      `json:"id"`
	Owner       string      `json:"owner"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Created     time.Time   `json:"created"`
	Updated     time.Time   `json:"updated"`
	Content     ShopContent `json:"content"`
	Assets      ShopAssets  `json:"assets"`
}

// ShopContent holds the dynamic content of a shop
type ShopContent struct {
	Items   []ItemData  `json:"items"`
	Theme   ThemeData   `json:"theme"`
	Contact ContactData `json:"contact"`
}

// ShopAssets holds references to IPFS-stored assets
type ShopAssets struct {
	LogoCID       string   `json:"logoCid"`
	ItemImageCIDs []string `json:"itemImageCids"`
}

// ItemData represents a shop item in OrbitDB
type ItemData struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Price       float64   `json:"price"`
	Description string    `json:"description"`
	ImageCIDs   []string  `json:"imageCids"`
	Created     time.Time `json:"created"`
}

// ThemeData represents shop theme configuration
type ThemeData struct {
	PrimaryColor   string `json:"primaryColor"`
	SecondaryColor string `json:"secondaryColor"`
	TertiaryColor  string `json:"tertiaryColor"`
}

// ContactData represents shop contact information
type ContactData struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Location string `json:"location"`
}

// Additional types for shop components...

// ShopInventoryData represents a shop's inventory in OrbitDB
type ShopInventoryData struct {
	ID          string          `json:"id"`
	ShopID      string          `json:"shopId"`
	OwnerID     string          `json:"ownerId"` // Ethereum address of shop owner
	Items       []ItemInventory `json:"items"`
	LastUpdated time.Time       `json:"lastUpdated"`
}

// ItemInventory represents an item's inventory data
type ItemInventory struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Price       float64   `json:"price"`
	Description string    `json:"description"`
	ImageCIDs   []string  `json:"imageCids"`
	Inventory   int64     `json:"inventory"` // -1 represents unlimited
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// InventoryManager handles inventory operations in OrbitDB
type InventoryManager struct {
	store iface.KeyValueStore
}
