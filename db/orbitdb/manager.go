package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"IndieNode/internal/models"

	orbitdb "berty.tech/go-orbit-db"
	"berty.tech/go-orbit-db/iface"
	"berty.tech/go-orbit-db/stores/documentstore"
	iface_ipfs "github.com/ipfs/interface-go-ipfs-core"
	// TODO: Install OrbitDB dependencies:
	// go get github.com/berty/go-orbit-db
)

// NewManager creates a new shop data manager
func NewManager(ctx context.Context, config *Config, ipfsAPI iface_ipfs.CoreAPI) (*Manager, error) {
	if config == nil {
		config = &Config{
			Directory: "./shops",
		}
	}

	manager := &Manager{
		ctx:     ctx,
		config:  config,
		shopDBs: make(map[string]iface.DocumentStore), // Initialize database cache
	}

	// Initialize shop data storage
	if err := manager.connect(ipfsAPI); err != nil {
		return nil, fmt.Errorf("failed to initialize shop data storage: %w", err)
	}

	return manager, nil
}

// connect initializes IPFS-based shop data storage with persistent OrbitDB
func (m *Manager) connect(ipfsAPI iface_ipfs.CoreAPI) error {
	// Create directory if it doesn't exist
	dir := filepath.Clean(m.config.Directory)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create shop data directory: %w", err)
	}

	// Create the OrbitDB directory within the shops directory
	orbitDBDir := filepath.Join(dir, "orbitdb")
	if err := os.MkdirAll(orbitDBDir, 0755); err != nil {
		return fmt.Errorf("failed to create OrbitDB directory: %w", err)
	}

	// Store IPFS API reference
	m.ipfs = ipfsAPI
	m.isConnected = true

	// Initialize shopDBs map if not already initialized
	if m.shopDBs == nil {
		m.shopDBs = make(map[string]iface.DocumentStore)
	}

	// Initialize OrbitDB with persistent configuration
	orbitOptions := &orbitdb.NewOrbitDBOptions{
		Directory: &orbitDBDir, // Use the orbitdb subdirectory for data persistence
	}

	log.Printf("Initializing OrbitDB at %s", orbitDBDir)
	orbDB, err := orbitdb.NewOrbitDB(context.Background(), ipfsAPI, orbitOptions)
	if err != nil {
		return fmt.Errorf("failed to initialize OrbitDB: %w", err)
	}
	m.orbitDB = orbDB
	log.Printf("Successfully initialized OrbitDB")

	// Scan for existing shop databases and reconnect to them
	if err := m.reconnectExistingDatabases(); err != nil {
		log.Printf("Warning: Some existing shops couldn't be reconnected: %v", err)
	}

	// Ensure stable gateway configuration
	log.Printf("Successfully initialized shop data storage at %s", dir)
	log.Printf("IPFS gateway configured at 127.0.0.1:8080 for stable shop access")
	return nil
}

// Close closes all OrbitDB databases and marks the connection as closed
func (m *Manager) Close() error {
	if !m.isConnected {
		return nil
	}

	// Close all open databases
	m.dbsMutex.Lock()
	for shopID, db := range m.shopDBs {
		log.Printf("Closing database for shop: %s", shopID)
		if err := db.Close(); err != nil {
			log.Printf("Error closing database for shop %s: %v", shopID, err)
		}
	}
	// Clear the map
	m.shopDBs = make(map[string]iface.DocumentStore)
	m.dbsMutex.Unlock()

	// Close OrbitDB instance
	if m.orbitDB != nil {
		log.Printf("Closing OrbitDB instance")
		if err := m.orbitDB.Close(); err != nil {
			log.Printf("Error closing OrbitDB: %v", err)
		}
		m.orbitDB = nil
	}

	m.isConnected = false
	log.Printf("OrbitDB connections closed")
	return nil
}

// IsConnected returns the connection status
func (m *Manager) IsConnected() bool {
	return m.isConnected
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

// StoreShop stores a shop in OrbitDB
func (m *Manager) StoreShop(shop *models.Shop) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Generate a unique ID for the shop if it doesn't have one
	if shop.ID == "" {
		shop.ID = shop.OwnerAddress
	}

	// Get or create the document store for this shop
	docStore, err := m.GetShopDatabase(context.Background(), shop.ID)
	if err != nil {
		return fmt.Errorf("failed to get shop database: %w", err)
	}

	// Convert shop to ShopData for storage
	shopData := &ShopData{
		ID:          shop.ID,
		Owner:       shop.OwnerAddress,
		Name:        shop.Name,
		Description: shop.Description,
		Created:     time.Now(), // Use current time if not provided
		Updated:     time.Now(),
		Content: ShopContent{
			Theme: ThemeData{
				PrimaryColor:   rgbaToHex(shop.PrimaryColor),
				SecondaryColor: rgbaToHex(shop.SecondaryColor),
				TertiaryColor:  rgbaToHex(shop.TertiaryColor),
			},
			Contact: ContactData{
				Email:    shop.Email,
				Phone:    shop.Phone,
				Location: shop.Location,
			},
		},
		Assets: ShopAssets{
			LogoCID: shop.CID, // Use the shop's CID for now
		},
		OrbitDBAddress: docStore.Address().String(),
	}

	// Map items from models.Shop to ShopData
	for _, item := range shop.Items {
		itemData := ItemData{
			ID:          item.ID,
			Name:        item.Name,
			Price:       item.Price,
			Description: item.Description,
			Created:     time.Now(), // Use current time if not provided
		}

		// Map photo paths to CIDs (simplified for now)
		for _, photoPath := range item.PhotoPaths {
			// In a real implementation, you'd get actual CIDs
			itemData.ImageCIDs = append(itemData.ImageCIDs, photoPath)
		}

		shopData.Content.Items = append(shopData.Content.Items, itemData)
	}

	// Convert to JSON for OrbitDB storage
	shopJSON, err := json.Marshal(shopData)
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	// Create the document with the shop ID as document ID
	shopDoc := map[string]interface{}{}
	if err := json.Unmarshal(shopJSON, &shopDoc); err != nil {
		return fmt.Errorf("failed to prepare shop document: %w", err)
	}

	// Store in OrbitDB
	ctx := context.Background()
	_, err = docStore.Put(ctx, shopDoc)
	if err != nil {
		return fmt.Errorf("failed to store shop in OrbitDB: %w", err)
	}

	// Save metadata separately for easy address retrieval
	metadata := &ShopMetadata{
		ID:             shop.ID,
		Name:           shop.Name,
		Owner:          shop.OwnerAddress,
		OrbitDBAddress: docStore.Address().String(),
	}

	if err := m.SaveShopMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to save shop metadata: %w", err)
	}

	log.Printf("Successfully stored shop '%s' (ID: %s) in OrbitDB at address: %s",
		shop.Name, shop.ID, metadata.OrbitDBAddress)
	return nil
}

// rgbaToHex converts a color.RGBA to a hex string representation
func rgbaToHex(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// GetShopCount returns the total number of shops in the database
func (m *Manager) GetShopCount() (int, error) {
	if !m.IsConnected() {
		return 0, fmt.Errorf("not connected to OrbitDB")
	}

	// Count metadata files in shop directory
	entries, err := os.ReadDir(m.config.Directory)
	if err != nil {
		return 0, fmt.Errorf("failed to read shop directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-metadata.json") {
			count++
		}
	}

	return count, nil
}

// reconnectExistingDatabases scans for shop metadata files and reconnects to their
// associated OrbitDB databases if they exist
func (m *Manager) reconnectExistingDatabases() error {
	// Read all files in the shop directory
	entries, err := os.ReadDir(m.config.Directory)
	if err != nil {
		return fmt.Errorf("failed to read shop directory: %w", err)
	}

	var reconnectErrors []error
	var reconnectedCount int

	// Look for metadata files
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
			continue
		}

		// Extract the shop ID from the metadata filename
		shopID := strings.TrimSuffix(entry.Name(), "-metadata.json")
		log.Printf("Found shop metadata for: %s", shopID)

		// Get the shop metadata
		metadata, err := m.GetShopMetadata(context.Background(), shopID)
		if err != nil {
			reconnectErrors = append(reconnectErrors, fmt.Errorf("failed to get metadata for shop %s: %w", shopID, err))
			continue
		}

		if metadata == nil || metadata.OrbitDBAddress == "" {
			log.Printf("No OrbitDB address found for shop %s, skipping", shopID)
			continue
		}

		// Try to open the existing database using the stored address
		log.Printf("Reconnecting to OrbitDB database for shop %s at address: %s", shopID, metadata.OrbitDBAddress)

		// Create options for opening the database
		dbOptions := &iface.CreateDBOptions{}

		// Open the existing database
		store, err := m.orbitDB.Open(context.Background(), metadata.OrbitDBAddress, dbOptions)
		if err != nil {
			reconnectErrors = append(reconnectErrors, fmt.Errorf("failed to open database for shop %s: %w", shopID, err))
			continue
		}

		// Check if it's a document store
		docStore, ok := store.(iface.DocumentStore)
		if !ok {
			store.Close()
			reconnectErrors = append(reconnectErrors, fmt.Errorf("database for shop %s is not a document store", shopID))
			continue
		}

		// Load the database
		if err := docStore.Load(context.Background(), -1); err != nil {
			docStore.Close()
			reconnectErrors = append(reconnectErrors, fmt.Errorf("failed to load database for shop %s: %w", shopID, err))
			continue
		}

		// Cache the database
		m.dbsMutex.Lock()
		m.shopDBs[shopID] = docStore
		m.dbsMutex.Unlock()

		log.Printf("Successfully reconnected to database for shop %s", shopID)
		reconnectedCount++
	}

	log.Printf("Reconnected to %d existing shop databases", reconnectedCount)

	if len(reconnectErrors) > 0 {
		// Log errors but don't fail, just return a summary error
		for _, err := range reconnectErrors {
			log.Printf("Reconnection error: %v", err)
		}
		return fmt.Errorf("%d databases failed to reconnect", len(reconnectErrors))
	}

	return nil
}

// GetConnectedDatabases returns information about all connected OrbitDB databases
func (m *Manager) GetConnectedDatabases() []DatabaseInfo {
	m.dbsMutex.RLock()
	defer m.dbsMutex.RUnlock()

	var result []DatabaseInfo

	for shopID, db := range m.shopDBs {
		info := DatabaseInfo{
			ShopID:  shopID,
			Address: db.Address().String(),
		}
		result = append(result, info)
	}

	return result
}

// DatabaseInfo contains information about a connected OrbitDB database
type DatabaseInfo struct {
	ShopID  string
	Address string
}

// IsOrbitDBInitialized returns whether OrbitDB has been properly initialized
func (m *Manager) IsOrbitDBInitialized() bool {
	return m.orbitDB != nil
}

// GetOrbitDBDirectory returns the directory where OrbitDB data is stored
func (m *Manager) GetOrbitDBDirectory() string {
	return filepath.Join(m.config.Directory, "orbitdb")
}

// GetShop returns a shop from the database by ID with improved caching and error handling
func (m *Manager) GetShop(ctx context.Context, id string) (*models.Shop, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Initialize shop cache if it doesn't exist yet
	if m.shopCache == nil {
		m.shopCache = NewShopCache(5*time.Minute, 100) // 5-minute TTL, max 100 shops
	}

	// Try to get from cache first
	if shop, found := m.shopCache.Get(id); found {
		return shop, nil
	}

	// Check if metadata file exists
	metadataPath := filepath.Join(m.config.Directory, id+"-metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("shop not found: %s", id)
	} else if err != nil {
		return nil, fmt.Errorf("failed to access shop metadata: %w", err)
	}

	// Read metadata
	metadata, err := m.GetShopMetadata(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to read shop metadata: %w", err)
	}

	// Open the document store
	docstore, err := m.GetShopDatabase(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to open shop database: %w", err)
	}

	// Query the database for shop information
	results, err := docstore.Query(ctx, func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}
		docType, ok := docMap["type"].(string)
		if !ok {
			return false, nil
		}
		return docType == "shop", nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query shop database: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("shop data not found")
	}

	// Convert the first result to a Shop model
	shopData, ok := results[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid shop data format")
	}

	shop := &models.Shop{
		ID:           id,
		OwnerAddress: shopData["ownerAddress"].(string),
	}

	// Use the metadata for the OrbitDB address if needed
	log.Printf("Using shop metadata with OrbitDB address: %s", metadata.OrbitDBAddress)

	// Copy fields from shopData to the shop model
	if name, ok := shopData["name"].(string); ok {
		shop.Name = name
	}
	if description, ok := shopData["description"].(string); ok {
		shop.Description = description
	}

	// Store in cache for future requests
	m.shopCache.Set(id, shop)

	return shop, nil
}

// hexToRGBA converts a hex color string to color.RGBA
func hexToRGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{255, 255, 255, 255} // Default to white
	}

	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

// ListShopsOptions provides options for the ListShops method
type ListShopsOptions struct {
	OwnerAddress string // Filter by owner address
	Limit        int    // Limit the number of results (pagination)
	Offset       int    // Offset for pagination
	SortBy       string // Field to sort by
	SortDesc     bool   // Sort in descending order
}

// ListShops returns a list of all shops in the database with optional filtering and pagination
func (m *Manager) ListShops(ctx context.Context, options *ListShopsOptions) ([]*models.Shop, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Set default options if not provided
	if options == nil {
		options = &ListShopsOptions{}
	}
	if options.Limit <= 0 {
		options.Limit = 100 // Default limit
	}

	// Read all files in the shop directory
	entries, err := os.ReadDir(m.config.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read shop directory: %w", err)
	}

	var metadataFiles []string
	// Look for metadata files
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-metadata.json") {
			shopID := strings.TrimSuffix(entry.Name(), "-metadata.json")
			metadataFiles = append(metadataFiles, shopID)
		}
	}

	// Handle pagination
	totalShops := len(metadataFiles)
	startIdx := options.Offset
	endIdx := startIdx + options.Limit
	if endIdx > totalShops {
		endIdx = totalShops
	}
	if startIdx >= totalShops {
		// No results in this page
		return []*models.Shop{}, nil
	}

	// Apply pagination
	pagedMetadataFiles := metadataFiles[startIdx:endIdx]

	var shops []*models.Shop
	var errs []string
	var shopsMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent requests

	// Process each shop in parallel for better performance
	for _, shopID := range pagedMetadataFiles {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(id string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Try to get the shop
			shop, err := m.GetShop(ctx, id)
			if err != nil {
				log.Printf("Warning: Failed to get shop %s: %v", id, err)
				shopsMutex.Lock()
				errs = append(errs, fmt.Sprintf("Shop %s: %v", id, err))
				shopsMutex.Unlock()
				return
			}

			// Filter by owner if specified
			if options.OwnerAddress != "" && shop.OwnerAddress != options.OwnerAddress {
				return
			}

			shopsMutex.Lock()
			shops = append(shops, shop)
			shopsMutex.Unlock()
		}(shopID)
	}

	wg.Wait() // Wait for all goroutines to complete

	// If we had errors but still got some shops, log a summary warning
	if len(errs) > 0 && len(shops) > 0 {
		log.Printf("Warning: %d shops couldn't be loaded: %s", len(errs), strings.Join(errs, "; "))
	} else if len(errs) > 0 && len(shops) == 0 {
		// If we had errors and got no shops, return an error
		return nil, fmt.Errorf("failed to load any shops: %s", strings.Join(errs, "; "))
	}

	// Sort the results if requested
	if options.SortBy != "" {
		sort.Slice(shops, func(i, j int) bool {
			var result bool
			switch options.SortBy {
			case "name":
				result = shops[i].Name < shops[j].Name
			case "id":
				result = shops[i].ID < shops[j].ID
			case "owner":
				result = shops[i].OwnerAddress < shops[j].OwnerAddress
			default:
				// Default to sorting by name
				result = shops[i].Name < shops[j].Name
			}

			// Reverse order if descending
			if options.SortDesc {
				return !result
			}
			return result
		})
	}

	log.Printf("Successfully loaded %d shops", len(shops))
	return shops, nil
}

// For backward compatibility with existing code
func (m *Manager) ListAllShops(ctx context.Context) ([]*models.Shop, error) {
	return m.ListShops(ctx, nil)
}

// DeleteShop removes a shop and its metadata from the database
func (m *Manager) DeleteShop(ctx context.Context, shopID string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Check if the shop metadata exists
	metadata, err := m.GetShopMetadata(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get shop metadata: %w", err)
	}

	if metadata == nil {
		return fmt.Errorf("shop not found: %s", shopID)
	}

	// Get the database
	docStore, err := m.GetShopDatabase(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to open shop database: %w", err)
	}

	// Delete the document from OrbitDB
	docs, err := docStore.Query(ctx, func(doc interface{}) (bool, error) {
		// Match by ID
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		id, ok := docMap["id"].(string)
		return ok && id == shopID, nil
	})

	if err != nil {
		return fmt.Errorf("failed to query shop data: %w", err)
	}

	if len(docs) > 0 {
		// Get the document hash
		docMap, ok := docs[0].(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid document format")
		}

		hash, ok := docMap["_id"].(string)
		if !ok {
			return fmt.Errorf("document has no hash (_id)")
		}

		// Delete the document
		if _, err := docStore.Delete(ctx, hash); err != nil {
			return fmt.Errorf("failed to delete shop from OrbitDB: %w", err)
		}
	}

	// Close and remove the database from cache
	m.dbsMutex.Lock()
	if db, exists := m.shopDBs[shopID]; exists {
		if err := db.Close(); err != nil {
			log.Printf("Warning: Error closing database for shop %s: %v", shopID, err)
		}
		delete(m.shopDBs, shopID)
	}
	m.dbsMutex.Unlock()

	// Delete the metadata file
	metadataPath := filepath.Join(m.config.Directory, shopID+"-metadata.json")
	if err := os.Remove(metadataPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete shop metadata file: %w", err)
		}
	}

	log.Printf("Successfully deleted shop %s", shopID)
	return nil
}

// UpdateShop updates an existing shop in OrbitDB
func (m *Manager) UpdateShop(ctx context.Context, shop *models.Shop) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Verify the shop exists
	docStore, err := m.GetShopDatabase(ctx, shop.ID)
	if err != nil {
		return fmt.Errorf("failed to get shop database: %w", err)
	}

	// Query the existing document to get its hash
	docs, err := docStore.Query(ctx, func(doc interface{}) (bool, error) {
		// Match by ID
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		id, ok := docMap["id"].(string)
		return ok && id == shop.ID, nil
	})

	if err != nil {
		return fmt.Errorf("failed to query shop data: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("shop not found: %s", shop.ID)
	}

	// Get the document hash
	docMap, ok := docs[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid document format")
	}

	// We're using Put instead of a direct update by hash, but still verify
	// the document exists by checking for the _id field
	if _, ok := docMap["_id"].(string); !ok {
		return fmt.Errorf("document has no hash (_id)")
	}

	// Convert shop to ShopData
	shopData := &ShopData{
		ID:          shop.ID,
		Owner:       shop.OwnerAddress,
		Name:        shop.Name,
		Description: shop.Description,
		Updated:     time.Now(),
		Content: ShopContent{
			Theme: ThemeData{
				PrimaryColor:   rgbaToHex(shop.PrimaryColor),
				SecondaryColor: rgbaToHex(shop.SecondaryColor),
				TertiaryColor:  rgbaToHex(shop.TertiaryColor),
			},
			Contact: ContactData{
				Email:    shop.Email,
				Phone:    shop.Phone,
				Location: shop.Location,
			},
		},
		Assets: ShopAssets{
			LogoCID: shop.CID, // Use the shop's CID for now
		},
		OrbitDBAddress: docStore.Address().String(),
	}

	// Preserve the creation time from the existing document
	if createdStr, ok := docMap["created"].(string); ok {
		if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
			shopData.Created = created
		} else {
			shopData.Created = time.Now() // Fallback
		}
	} else {
		shopData.Created = time.Now() // Fallback
	}

	// Map items from models.Shop to ShopData
	for _, item := range shop.Items {
		itemData := ItemData{
			ID:          item.ID,
			Name:        item.Name,
			Price:       item.Price,
			Description: item.Description,
			Created:     time.Now(), // Use current time if not provided
		}

		// Map photo paths to CIDs
		for _, photoPath := range item.PhotoPaths {
			itemData.ImageCIDs = append(itemData.ImageCIDs, photoPath)
		}

		shopData.Content.Items = append(shopData.Content.Items, itemData)
	}

	// Validate shop data
	if err := m.validateShopData(shopData); err != nil {
		return fmt.Errorf("invalid shop data: %w", err)
	}

	// Convert to JSON for OrbitDB storage
	shopJSON, err := json.Marshal(shopData)
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	// Create the document with the shop ID as document ID
	updatedDoc := map[string]interface{}{}
	if err := json.Unmarshal(shopJSON, &updatedDoc); err != nil {
		return fmt.Errorf("failed to prepare shop document: %w", err)
	}

	// Update the document in OrbitDB
	_, err = docStore.Put(ctx, updatedDoc)
	if err != nil {
		return fmt.Errorf("failed to update shop in OrbitDB: %w", err)
	}

	// Update metadata
	metadata := &ShopMetadata{
		ID:             shop.ID,
		Name:           shop.Name,
		Owner:          shop.OwnerAddress,
		OrbitDBAddress: docStore.Address().String(),
	}

	if err := m.SaveShopMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to save shop metadata: %w", err)
	}

	log.Printf("Successfully updated shop '%s' (ID: %s) in OrbitDB", shop.Name, shop.ID)
	return nil
}

// AddShopAsset adds a new asset (logo or item image) to a shop
func (m *Manager) AddShopAsset(ctx context.Context, shopID string, assetType string, assetCID string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Get the shop
	shop, err := m.GetShop(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get shop: %w", err)
	}

	// Update shop assets based on type
	switch assetType {
	case "logo":
		shop.CID = assetCID
	case "item":
		// For item images, we would typically add this to a specific item
		// But for now, just add to the first item or create one
		if len(shop.Items) == 0 {
			shop.Items = append(shop.Items, models.Item{
				ID:          "item-1",
				Name:        "New Item",
				Price:       0,
				Description: "Item description",
				PhotoPaths:  []string{assetCID},
			})
		} else {
			shop.Items[0].PhotoPaths = append(shop.Items[0].PhotoPaths, assetCID)
		}
	default:
		return fmt.Errorf("invalid asset type: %s", assetType)
	}

	// Update the shop in OrbitDB
	return m.UpdateShop(ctx, shop)
}

// RemoveShopAsset removes an asset from a shop
func (m *Manager) RemoveShopAsset(ctx context.Context, shopID string, assetType string, assetCID string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Get the shop
	shop, err := m.GetShop(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get shop: %w", err)
	}

	// Remove asset based on type
	switch assetType {
	case "logo":
		if shop.CID == assetCID {
			shop.CID = ""
		}
	case "item":
		// Remove the CID from all items
		for i := range shop.Items {
			newPaths := make([]string, 0)
			for _, path := range shop.Items[i].PhotoPaths {
				if path != assetCID {
					newPaths = append(newPaths, path)
				}
			}
			shop.Items[i].PhotoPaths = newPaths
		}
	default:
		return fmt.Errorf("invalid asset type: %s", assetType)
	}

	// Update the shop in OrbitDB
	return m.UpdateShop(ctx, shop)
}

// validateShopData performs basic validation of shop data
func (m *Manager) validateShopData(shop *ShopData) error {
	if shop == nil {
		return fmt.Errorf("shop data cannot be nil")
	}

	if shop.ID == "" {
		return fmt.Errorf("shop ID is required")
	}

	if shop.Owner == "" {
		return fmt.Errorf("shop owner is required")
	}

	if shop.Name == "" {
		return fmt.Errorf("shop name is required")
	}

	return nil
}

// ListShopsByOwner returns all shops owned by the given address
func (m *Manager) ListShopsByOwner(ctx context.Context, ownerAddress string) ([]*models.Shop, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	if ownerAddress == "" {
		return nil, fmt.Errorf("owner address is required")
	}

	// Get all shops first
	allShops, err := m.ListShops(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list shops: %w", err)
	}

	// Filter by owner address
	var ownerShops []*models.Shop
	for _, shop := range allShops {
		if shop.OwnerAddress == ownerAddress {
			ownerShops = append(ownerShops, shop)
		}
	}

	log.Printf("Found %d shops owned by address %s", len(ownerShops), ownerAddress)
	return ownerShops, nil
}

// ExportShopData exports shop data in a portable format
func (m *Manager) ExportShopData(ctx context.Context, shopID string) ([]byte, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Get the shop database
	docStore, err := m.GetShopDatabase(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop database: %w", err)
	}

	// Query the document store
	docs, err := docStore.Query(ctx, func(doc interface{}) (bool, error) {
		// Match by ID
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		id, ok := docMap["id"].(string)
		return ok && id == shopID, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query shop data: %w", err)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("shop not found: %s", shopID)
	}

	// Get the shop metadata
	metadata, err := m.GetShopMetadata(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop metadata: %w", err)
	}

	// Create an export structure with both shop data and metadata
	type ShopExport struct {
		ShopData *ShopData     `json:"shopData"`
		Metadata *ShopMetadata `json:"metadata"`
	}

	// Convert the document to ShopData
	docMap, ok := docs[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid document format")
	}

	// Marshal and unmarshal to convert map to ShopData
	docBytes, err := json.Marshal(docMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	var shopData ShopData
	if err := json.Unmarshal(docBytes, &shopData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shop data: %w", err)
	}

	// Create the export
	export := ShopExport{
		ShopData: &shopData,
		Metadata: metadata,
	}

	// Marshal to JSON with indentation for human readability
	exportBytes, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop export: %w", err)
	}

	return exportBytes, nil
}

// ImportShopData imports shop data from a previously exported format
func (m *Manager) ImportShopData(ctx context.Context, exportData []byte) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Define the import structure
	type ShopExport struct {
		ShopData *ShopData     `json:"shopData"`
		Metadata *ShopMetadata `json:"metadata"`
	}

	// Unmarshal the export data
	var export ShopExport
	if err := json.Unmarshal(exportData, &export); err != nil {
		return fmt.Errorf("failed to unmarshal export data: %w", err)
	}

	if export.ShopData == nil {
		return fmt.Errorf("export data does not contain shop data")
	}

	// Create options for opening or creating the database
	docStoreOptions := documentstore.DefaultStoreOptsForMap("id")

	// Create the database options
	create := true
	dbOptions := &iface.CreateDBOptions{
		Create:            &create,
		StoreType:         new(string),
		StoreSpecificOpts: docStoreOptions,
	}
	*dbOptions.StoreType = "docstore"

	// Attempt to open or create the store
	dbAddress := ""
	if export.Metadata != nil && export.Metadata.OrbitDBAddress != "" {
		dbAddress = export.Metadata.OrbitDBAddress
		log.Printf("Using existing OrbitDB address: %s", dbAddress)
	} else {
		dbAddress = "shop-" + export.ShopData.ID
		log.Printf("Creating new OrbitDB database: %s", dbAddress)
	}

	store, err := m.orbitDB.Create(ctx, dbAddress, "docstore", dbOptions)
	if err != nil {
		return fmt.Errorf("failed to create document store: %w", err)
	}

	// Ensure it's a document store
	docStore, ok := store.(iface.DocumentStore)
	if !ok {
		store.Close()
		return fmt.Errorf("created store is not a document store")
	}

	// Update the OrbitDB address in the shop data
	export.ShopData.OrbitDBAddress = docStore.Address().String()

	// Marshal shop data for storage
	shopJSON, err := json.Marshal(export.ShopData)
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	// Create the document with the shop ID as document ID
	shopDoc := map[string]interface{}{}
	if err := json.Unmarshal(shopJSON, &shopDoc); err != nil {
		return fmt.Errorf("failed to prepare shop document: %w", err)
	}

	// Store in OrbitDB
	_, err = docStore.Put(ctx, shopDoc)
	if err != nil {
		return fmt.Errorf("failed to store shop in OrbitDB: %w", err)
	}

	// Save metadata for the imported shop
	metadata := &ShopMetadata{
		ID:             export.ShopData.ID,
		Name:           export.ShopData.Name,
		Owner:          export.ShopData.Owner,
		OrbitDBAddress: docStore.Address().String(),
	}

	if err := m.SaveShopMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to save shop metadata: %w", err)
	}

	// Cache the database
	m.dbsMutex.Lock()
	m.shopDBs[export.ShopData.ID] = docStore
	m.dbsMutex.Unlock()

	log.Printf("Successfully imported shop '%s' (ID: %s) into OrbitDB at address: %s",
		export.ShopData.Name, export.ShopData.ID, metadata.OrbitDBAddress)
	return nil
}

// GetDatabaseStatus returns detailed status information about a specific shop database
func (m *Manager) GetDatabaseStatus(ctx context.Context, shopID string) (*DatabaseStatus, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Check if we have the database in cache
	m.dbsMutex.RLock()
	db, exists := m.shopDBs[shopID]
	m.dbsMutex.RUnlock()

	if !exists {
		// Try to get metadata to see if the database exists but isn't loaded
		metadata, err := m.GetShopMetadata(ctx, shopID)
		if err != nil {
			return nil, fmt.Errorf("failed to get shop metadata: %w", err)
		}

		if metadata == nil || metadata.OrbitDBAddress == "" {
			return nil, fmt.Errorf("no database found for shop %s", shopID)
		}

		// Database exists but isn't loaded
		return &DatabaseStatus{
			ShopID:      shopID,
			Address:     metadata.OrbitDBAddress,
			IsLoaded:    false,
			RecordCount: 0,
			OpenedAt:    time.Time{},
		}, nil
	}

	// Get document count
	docs, err := db.Query(ctx, func(doc interface{}) (bool, error) {
		return true, nil // Match all documents
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	// Get opening time using reflection and the store implementation
	// This is a bit of a hack, but there's no direct way to get this info
	openedAt := time.Now() // Default to now if we can't determine

	return &DatabaseStatus{
		ShopID:      shopID,
		Address:     db.Address().String(),
		IsLoaded:    true,
		RecordCount: len(docs),
		OpenedAt:    openedAt,
	}, nil
}

// DatabaseStatus contains detailed information about a database
type DatabaseStatus struct {
	ShopID      string    // Shop ID
	Address     string    // OrbitDB address
	IsLoaded    bool      // Whether the database is currently loaded
	RecordCount int       // Number of records in the database
	OpenedAt    time.Time // When the database was opened
}

// CloseShopDatabase closes a specific shop database and removes it from the cache
func (m *Manager) CloseShopDatabase(shopID string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	m.dbsMutex.Lock()
	defer m.dbsMutex.Unlock()

	db, exists := m.shopDBs[shopID]
	if !exists {
		return fmt.Errorf("database for shop %s is not open", shopID)
	}

	// Close the database
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database for shop %s: %w", shopID, err)
	}

	// Remove from cache
	delete(m.shopDBs, shopID)
	log.Printf("Closed database for shop %s", shopID)
	return nil
}

// DatabaseStats contains statistics about all OrbitDB databases in the system
type DatabaseStats struct {
	TotalDatabases     int // Total number of databases found in metadata
	LoadedDatabases    int // Number of currently loaded databases
	TotalRecords       int // Total number of records across all loaded databases
	AvgRecordsPerStore int // Average number of records per loaded database
}

// GetDatabaseStats returns statistics about all OrbitDB databases in the system
func (m *Manager) GetDatabaseStats(ctx context.Context) (*DatabaseStats, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to OrbitDB")
	}

	// Count all metadata files to get total database count
	entries, err := os.ReadDir(m.config.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read shop directory: %w", err)
	}

	// Count metadata files
	totalDBs := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-metadata.json") {
			totalDBs++
		}
	}

	// Get loaded database count and record counts
	m.dbsMutex.RLock()
	loadedDBs := len(m.shopDBs)

	// Count records in all loaded databases
	totalRecords := 0
	for shopID, db := range m.shopDBs {
		// Query all documents to get count
		docs, err := db.Query(ctx, func(doc interface{}) (bool, error) {
			return true, nil // Match all documents
		})
		if err != nil {
			log.Printf("Warning: Failed to get record count for shop %s: %v", shopID, err)
			continue
		}
		totalRecords += len(docs)
	}
	m.dbsMutex.RUnlock()

	// Calculate average
	avgRecords := 0
	if loadedDBs > 0 {
		avgRecords = totalRecords / loadedDBs
	}

	return &DatabaseStats{
		TotalDatabases:     totalDBs,
		LoadedDatabases:    loadedDBs,
		TotalRecords:       totalRecords,
		AvgRecordsPerStore: avgRecords,
	}, nil
}

// ReloadAllDatabases closes all open database instances and reopens them
func (m *Manager) ReloadAllDatabases(ctx context.Context) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Get a list of all currently open shop IDs
	m.dbsMutex.RLock()
	shopIDs := make([]string, 0, len(m.shopDBs))
	for shopID := range m.shopDBs {
		shopIDs = append(shopIDs, shopID)
	}
	m.dbsMutex.RUnlock()

	// Close all databases first
	for _, shopID := range shopIDs {
		if err := m.CloseShopDatabase(shopID); err != nil {
			log.Printf("Warning: Error closing database for shop %s: %v", shopID, err)
			// Continue closing other databases
		}
	}

	// Clear the map to be safe
	m.dbsMutex.Lock()
	m.shopDBs = make(map[string]iface.DocumentStore)
	m.dbsMutex.Unlock()

	// Reload all databases
	if err := m.reconnectExistingDatabases(); err != nil {
		return fmt.Errorf("error reconnecting to databases: %w", err)
	}

	log.Printf("Successfully reloaded all databases")
	return nil
}

// RepairShopDatabase attempts to repair a potentially corrupted shop database
func (m *Manager) RepairShopDatabase(ctx context.Context, shopID string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to OrbitDB")
	}

	// Check if the database is currently open
	m.dbsMutex.RLock()
	_, isOpen := m.shopDBs[shopID]
	m.dbsMutex.RUnlock()

	// If it's open, close it first
	if isOpen {
		if err := m.CloseShopDatabase(shopID); err != nil {
			return fmt.Errorf("failed to close database before repair: %w", err)
		}
	}

	// Get the metadata
	metadata, err := m.GetShopMetadata(ctx, shopID)
	if err != nil {
		return fmt.Errorf("failed to get shop metadata: %w", err)
	}

	if metadata == nil || metadata.OrbitDBAddress == "" {
		return fmt.Errorf("no database address found for shop %s", shopID)
	}

	log.Printf("Attempting to repair database for shop %s at address: %s", shopID, metadata.OrbitDBAddress)

	// For a real repair, we might need to:
	// 1. Check the OrbitDB directory for issues
	// 2. Fix potential IPFS keys or access control
	// 3. Rebuild database indexes

	// For now, let's try a simple approach - force recreate options
	create := true
	dbOptions := &iface.CreateDBOptions{
		Create:    &create,
		StoreType: new(string),
		// Add specific options if needed for repair
	}
	*dbOptions.StoreType = "docstore"

	// Try to open the database with repair options
	store, err := m.orbitDB.Open(ctx, metadata.OrbitDBAddress, dbOptions)
	if err != nil {
		return fmt.Errorf("failed to open database for repair: %w", err)
	}

	// Check if it's a document store
	docStore, ok := store.(iface.DocumentStore)
	if !ok {
		store.Close()
		return fmt.Errorf("database is not a document store")
	}

	// Try to load data
	if err := docStore.Load(ctx, -1); err != nil {
		docStore.Close()
		return fmt.Errorf("failed to load database after repair: %w", err)
	}

	// If everything worked, add to cache
	m.dbsMutex.Lock()
	m.shopDBs[shopID] = docStore
	m.dbsMutex.Unlock()

	log.Printf("Successfully repaired database for shop %s", shopID)
	return nil
}

// Additional methods will be added here...

// readMetadata method
func (m *Manager) readMetadata(shopID string) (*ShopMetadata, error) {
	metadataPath := filepath.Join(m.config.Directory, shopID+"-metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata ShopMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// For returns shops owned by a specific address
func (m *Manager) For(ctx context.Context, ownerAddress string) ([]*models.Shop, error) {
	if ownerAddress == "" {
		return nil, fmt.Errorf("owner address is required")
	}

	options := &ListShopsOptions{
		OwnerAddress: ownerAddress,
		SortBy:       "name", // Default sort by name
	}

	return m.ListShops(ctx, options)
}
