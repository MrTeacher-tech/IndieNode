package shop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

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
	var shopPath string
	if auth.IsDevMode() {
		// In dev mode, load from the dev template in internal/dev
		// Get the project root directory (parent of shops directory)
		projectRoot := filepath.Dir(m.baseDir)
		shopPath = filepath.Join(projectRoot, "internal", "dev", "dev_shop.json")
	} else {
		shopPath = filepath.Join(m.baseDir, "current_shop.json")
		
		// When not in dev mode, check if current shop matches dev shop
		if _, err := os.Stat(shopPath); err == nil {
			// Current shop exists, check if it matches dev shop
			projectRoot := filepath.Dir(m.baseDir)
			devShopPath := filepath.Join(projectRoot, "internal", "dev", "dev_shop.json")
			
			// Load both shops
			currentData, err := os.ReadFile(shopPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read current shop: %w", err)
			}
			
			devData, err := os.ReadFile(devShopPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read dev shop: %w", err)
			}
			
			// Unmarshal both shops
			var currentShop, devShop models.Shop
			if err := json.Unmarshal(currentData, &currentShop); err != nil {
				return nil, fmt.Errorf("failed to parse current shop: %w", err)
			}
			if err := json.Unmarshal(devData, &devShop); err != nil {
				return nil, fmt.Errorf("failed to parse dev shop: %w", err)
			}
			
			// Compare shops (ignoring owner address)
			currentShop.OwnerAddress = ""
			devShop.OwnerAddress = ""
			currentJSON, _ := json.Marshal(currentShop)
			devJSON, _ := json.Marshal(devShop)
			
			if string(currentJSON) == string(devJSON) {
				// Current shop matches dev shop, clear it
				if err := m.ClearCurrentShop(); err != nil {
					return nil, fmt.Errorf("failed to clear dev shop data: %w", err)
				}
			}
		}
	}

	// Try to load shop from file
	data, err := os.ReadFile(shopPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist, return a new shop
			return &models.Shop{
				OwnerAddress: auth.GetCurrentUser().Address,
				Items:        []models.Item{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read shop file: %w", err)
	}

	var shop models.Shop
	if err := json.Unmarshal(data, &shop); err != nil {
		return nil, fmt.Errorf("failed to parse shop data: %w", err)
	}

	// In dev mode, always set the owner address to the current user
	if auth.IsDevMode() {
		shop.OwnerAddress = auth.GetCurrentUser().Address
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

	// Copy web3.js file
	web3JsPath := filepath.Join(filepath.Dir(m.baseDir), "templates", "basic", "web3.js")
	if err := m.copyFile(web3JsPath, filepath.Join(srcDir, "web3.js")); err != nil {
		return fmt.Errorf("failed to copy web3.js: %w", err)
	}

	// Generate CSS
	cssPath := filepath.Join(srcDir, "styles.css")
	if err := m.generateCSS(shop, cssPath); err != nil {
		return fmt.Errorf("failed to generate CSS: %w", err)
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

// generateCSS generates the CSS file from the template
func (m *Manager) generateCSS(shop *models.Shop, targetPath string) error {
	// Read the CSS template
	cssTemplate, err := os.ReadFile(filepath.Join("templates", "basic", "basic.css"))
	if err != nil {
		return fmt.Errorf("failed to read CSS template: %w", err)
	}

	// Create a template with the CSS content
	tmpl, err := template.New("css").Parse(string(cssTemplate))
	if err != nil {
		return fmt.Errorf("failed to parse CSS template: %w", err)
	}

	// Create the target CSS file
	cssFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create CSS file: %w", err)
	}
	defer cssFile.Close()

	// Prepare template data with RGB colors
	data := struct {
		PrimaryColor   string
		SecondaryColor string
		TertiaryColor  string
	}{
		PrimaryColor:   fmt.Sprintf("rgb(%d, %d, %d)", shop.PrimaryColor.R, shop.PrimaryColor.G, shop.PrimaryColor.B),
		SecondaryColor: fmt.Sprintf("rgb(%d, %d, %d)", shop.SecondaryColor.R, shop.SecondaryColor.G, shop.SecondaryColor.B),
		TertiaryColor:  fmt.Sprintf("rgb(%d, %d, %d)", shop.TertiaryColor.R, shop.TertiaryColor.G, shop.TertiaryColor.B),
	}

	// Execute the template with the color data
	if err := tmpl.Execute(cssFile, data); err != nil {
		return fmt.Errorf("failed to generate CSS: %w", err)
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
    <script src="https://cdn.jsdelivr.net/npm/web3@1.5.2/dist/web3.min.js"></script>
    <script src="web3.js"></script>
    <link rel="stylesheet" href="styles.css">
</head>
<body>
    <div class="shop-header">
        %s
        <h1>%s</h1>
        <p>%s</p>
        <div class="shop-info">
            %s
            %s
        </div>
    </div>
    <div class="items-grid">
        %s
    </div>
</body>
</html>`,
		shop.Name,
		m.generateLogoHTML(shop),
		shop.Name,
		shop.Description,
		m.generateLocationHTML(shop),
		m.generateContactHTML(shop),
		m.generateItemsHTML(shop),
	)

	// Write the HTML file
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
                <button class="eth-buy-button" data-item-id="%s" data-item-price="%.2f">Buy with MetaMask Wallet</button>
            </div>
        `, images, item.Name, item.Price, item.Description, item.Name, item.Price)
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
