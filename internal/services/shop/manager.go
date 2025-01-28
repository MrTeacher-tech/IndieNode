package shop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"IndieNode/internal/models"
	"IndieNode/internal/services/auth"
	"IndieNode/ipfs"
)

// Manager handles shop-related operations
type Manager struct {
	baseDir string
	ipfsMgr *ipfs.IPFSManager
}

// NewManager creates a new shop manager
func NewManager(baseDir string, ipfsMgr *ipfs.IPFSManager) (*Manager, error) {
	// Create shops directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create shops directory: %w", err)
	}

	return &Manager{
		baseDir: baseDir,
		ipfsMgr: ipfsMgr,
	}, nil
}

// LoadCurrentShop loads the current shop from storage
func (m *Manager) LoadCurrentShop() (*models.Shop, error) {
	currentShopPath := filepath.Join(m.baseDir, "current_shop.json")

	// Try to load shop from file
	data, err := os.ReadFile(currentShopPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist, return a new shop
			return &models.Shop{
				OwnerAddress: auth.GetCurrentUser().Address,
				Items:        []models.Item{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read current shop: %w", err)
	}

	var shop models.Shop
	if err := json.Unmarshal(data, &shop); err != nil {
		return nil, fmt.Errorf("failed to parse current shop: %w", err)
	}

	return &shop, nil
}

// SaveCurrentShop saves the current shop to storage
func (m *Manager) SaveCurrentShop(shop *models.Shop) error {
	if shop == nil {
		return fmt.Errorf("shop cannot be nil")
	}

	data, err := json.MarshalIndent(shop, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	currentShopPath := filepath.Join(m.baseDir, "current_shop.json")
	if err := os.WriteFile(currentShopPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save current shop: %w", err)
	}

	return nil
}

// ClearCurrentShop creates a new empty shop
func (m *Manager) ClearCurrentShop() error {
	// Create a new empty shop with just the owner address
	newShop := &models.Shop{
		OwnerAddress: auth.GetCurrentUser().Address,
		Items:        []models.Item{},
	}
	return m.SaveCurrentShop(newShop)
}

// ListShops returns a list of all shops in the base directory
func (m *Manager) ListShops() ([]string, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read shops directory: %w", err)
	}

	var shops []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Skip the "assets" directory if it exists
			if entry.Name() != "assets" {
				shops = append(shops, entry.Name())
			}
		}
	}

	return shops, nil
}

// LoadShop loads a specific shop by name
func (m *Manager) LoadShop(shopName string) (*models.Shop, error) {
	shopPath := filepath.Join(m.baseDir, shopName, "shop.json")
	data, err := os.ReadFile(shopPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shop file: %w", err)
	}

	var shop models.Shop
	if err := json.Unmarshal(data, &shop); err != nil {
		return nil, fmt.Errorf("failed to parse shop data: %w", err)
	}

	return &shop, nil
}

// SaveShop saves a shop to its directory
func (m *Manager) SaveShop(shop *models.Shop) error {
	if shop == nil {
		return fmt.Errorf("shop cannot be nil")
	}

	if shop.Name == "" {
		return fmt.Errorf("shop name cannot be empty")
	}

	shopDir := filepath.Join(m.baseDir, shop.Name)
	if err := os.MkdirAll(shopDir, 0755); err != nil {
		return fmt.Errorf("failed to create shop directory: %w", err)
	}

	data, err := json.MarshalIndent(shop, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal shop data: %w", err)
	}

	shopPath := filepath.Join(shopDir, "shop.json")
	if err := os.WriteFile(shopPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save shop: %w", err)
	}

	return nil
}

// GetShopPath returns the absolute path to a shop's directory
func (m *Manager) GetShopPath(shopName string) string {
	return filepath.Join(m.baseDir, shopName)
}

// GenerateShop generates the HTML and assets for a shop
func (m *Manager) GenerateShop(shop *models.Shop) error {
	if shop == nil {
		return fmt.Errorf("shop is nil")
	}

	if shop.Name == "" {
		return fmt.Errorf("shop name is required")
	}

	// Get shop directory
	shopDir := m.GetShopPath(shop.Name)
	srcDir := filepath.Join(shopDir, "src")
	assetsDir := filepath.Join(srcDir, "assets")
	logosDir := filepath.Join(assetsDir, "logos")
	itemsDir := filepath.Join(srcDir, "items") // Keep original items dir for compatibility

	// Create required directories
	for _, dir := range []string{srcDir, assetsDir, logosDir, itemsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate index.html
	indexPath := filepath.Join(srcDir, "index.html")
	if err := m.generateHTML(shop, indexPath); err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	// Copy logo if exists
	if shop.LocalLogoPath != "" {
		ext := filepath.Ext(shop.LocalLogoPath)
		logoFileName := "logo" + ext
		targetPath := filepath.Join(logosDir, logoFileName)
		if err := m.copyFile(shop.LocalLogoPath, targetPath); err != nil {
			return fmt.Errorf("failed to copy logo: %w", err)
		}
		// Update LogoPath to be relative to index.html
		shop.LogoPath = "assets/logos/" + logoFileName
	}

	// Copy all item images
	for _, item := range shop.Items {
		for i, localPath := range item.LocalPhotoPaths {
			targetPath := filepath.Join(itemsDir, filepath.Base(localPath))
			if err := m.copyFile(localPath, targetPath); err != nil {
				return fmt.Errorf("failed to copy item image: %w", err)
			}
			// Keep using the existing items/ relative path
			item.PhotoPaths[i] = "items/" + filepath.Base(localPath)
		}
	}

	return nil
}

// DeleteShop deletes a shop and its associated files
func (m *Manager) DeleteShop(name string) error {
	shopDir := m.GetShopPath(name)

	// Try to load shop data to get CID
	shop, err := m.LoadShop(name)
	if err == nil && shop.CID != "" {
		// If shop was published, unpin from IPFS
		if err := m.ipfsMgr.UnpublishContent(shop.CID); err != nil {
			// Log the error but continue with deletion
			fmt.Printf("Warning: failed to unpin content from IPFS: %v\n", err)
		}
	}

	// Remove the shop directory and all its contents
	if err := os.RemoveAll(shopDir); err != nil {
		return fmt.Errorf("failed to delete shop directory: %w", err)
	}

	return nil
}

func (m *Manager) generateHTML(shop *models.Shop, targetPath string) error {
	// Create HTML template
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        :root {
            --primary-color: rgb(%d, %d, %d);
            --secondary-color: rgb(%d, %d, %d);
            --tertiary-color: rgb(%d, %d, %d);
        }
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: var(--secondary-color);
            color: var(--primary-color);
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: white;
            border-radius: 10px;
            box-shadow: 0 0 10px rgba(0,0,0,0.1);
        }
        .shop-header {
            text-align: center;
            margin-bottom: 30px;
        }
        .shop-logo {
            max-width: 200px;
            margin-bottom: 20px;
        }
        .items-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
            gap: 20px;
            padding: 20px;
        }
        .item-card {
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 15px;
            text-align: center;
            background-color: white;
        }
        .item-image {
            max-width: 100%%;
            height: auto;
            border-radius: 4px;
        }
        .item-name {
            font-size: 1.2em;
            margin: 10px 0;
            color: var(--primary-color);
        }
        .item-price {
            font-size: 1.1em;
            color: var(--tertiary-color);
            font-weight: bold;
        }
        .item-description {
            color: #666;
            margin: 10px 0;
        }
        .contact-info {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="shop-header">
            %s
            <h1>%s</h1>
            <p>%s</p>
            %s
        </div>
        
        <div class="items-grid">
            %s
        </div>

        <div class="contact-info">
            %s
        </div>
    </div>
</body>
</html>`,
		// Title
		shop.Name,
		// Colors
		shop.PrimaryColor.R, shop.PrimaryColor.G, shop.PrimaryColor.B,
		shop.SecondaryColor.R, shop.SecondaryColor.G, shop.SecondaryColor.B,
		shop.TertiaryColor.R, shop.TertiaryColor.G, shop.TertiaryColor.B,
		// Logo
		m.generateLogoHTML(shop),
		// Shop name and description
		shop.Name,
		shop.Description,
		// Location
		m.generateLocationHTML(shop),
		// Items
		m.generateItemsHTML(shop),
		// Contact info
		m.generateContactHTML(shop),
	)

	// Write HTML to file
	if err := os.WriteFile(targetPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	return nil
}

func (m *Manager) generateLogoHTML(shop *models.Shop) string {
	if shop.LogoPath == "" {
		return ""
	}
	return fmt.Sprintf(`<img src="%s" alt="%s Logo" class="shop-logo">`, shop.LogoPath, shop.Name)
}

func (m *Manager) generateLocationHTML(shop *models.Shop) string {
	if shop.Location == "" {
		return ""
	}
	return fmt.Sprintf(`<p><strong>Location:</strong> %s</p>`, shop.Location)
}

func (m *Manager) generateItemsHTML(shop *models.Shop) string {
	var items string
	for _, item := range shop.Items {
		var images string
		if len(item.PhotoPaths) > 0 {
			images = fmt.Sprintf(`<img src="%s" alt="%s" class="item-image">`, item.PhotoPaths[0], item.Name)
		}

		items += fmt.Sprintf(`
            <div class="item-card">
                %s
                <h3 class="item-name">%s</h3>
                <p class="item-price">$%.2f</p>
                <p class="item-description">%s</p>
            </div>
        `, images, item.Name, item.Price, item.Description)
	}
	return items
}

func (m *Manager) generateContactHTML(shop *models.Shop) string {
	var contact string
	if shop.Email != "" {
		contact += fmt.Sprintf(`<p><strong>Email:</strong> %s</p>`, shop.Email)
	}
	if shop.Phone != "" {
		contact += fmt.Sprintf(`<p><strong>Phone:</strong> %s</p>`, shop.Phone)
	}
	return contact
}

func (m *Manager) copyFile(src, dst string) error {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to destination file
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}
