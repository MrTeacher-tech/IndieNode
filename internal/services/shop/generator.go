package shop

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"IndieNode/db/orbitdb"
	"IndieNode/internal/models"
)

// Generator handles shop generation functionality
type Generator struct {
	templatesDir string
	orbitDB      *orbitdb.Manager
}

// NewGenerator creates a new shop generator
func NewGenerator(templatesDir string, orbitDB *orbitdb.Manager) (*Generator, error) {
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create templates directory: %w", err)
	}

	return &Generator{
		templatesDir: templatesDir,
		orbitDB:      orbitDB,
	}, nil
}

// rgbaToHex converts a color.RGBA to CSS hex format
func rgbaToHex(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// templateData represents the data passed to templates with proper color formatting
type templateData struct {
	*models.Shop
	PrimaryColor   string
	SecondaryColor string
	TertiaryColor  string
}

// GenerateShop generates a shop's files from templates and stores it in OrbitDB
func (g *Generator) GenerateShop(shop *models.Shop, outputDir string) error {
	if shop == nil {
		return fmt.Errorf("shop cannot be nil")
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy logo if it exists
	if shop.LogoPath != "" {
		if err := g.copyFile(shop.LogoPath, filepath.Join(outputDir, "logo"+filepath.Ext(shop.LogoPath))); err != nil {
			return fmt.Errorf("failed to copy logo: %w", err)
		}
	}

	// Copy item photos
	for _, item := range shop.Items {
		for _, photoPath := range item.PhotoPaths {
			if photoPath != "" {
				destPath := filepath.Join(outputDir, "images", filepath.Base(photoPath))
				if err := g.copyFile(photoPath, destPath); err != nil {
					return fmt.Errorf("failed to copy item photo: %w", err)
				}
			}
		}
	}

	// Copy web3.js file
	web3JsPath := filepath.Join(g.templatesDir, "basic", "web3.js")
	if err := g.copyFile(web3JsPath, filepath.Join(outputDir, "src", "web3.js")); err != nil {
		return fmt.Errorf("failed to copy web3.js: %w", err)
	}

	// Prepare template data with converted colors
	data := templateData{
		Shop:           shop,
		PrimaryColor:   rgbaToHex(shop.PrimaryColor),
		SecondaryColor: rgbaToHex(shop.SecondaryColor),
		TertiaryColor:  rgbaToHex(shop.TertiaryColor),
	}

	// Generate HTML from template
	htmlTmpl, err := template.ParseFiles(filepath.Join(g.templatesDir, "basic", "basic.html"))
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	outputHtmlFile, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return fmt.Errorf("failed to create HTML output file: %w", err)
	}
	defer outputHtmlFile.Close()

	if err := htmlTmpl.Execute(outputHtmlFile, data); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	// Generate CSS from template
	cssTmpl, err := template.ParseFiles(filepath.Join(g.templatesDir, "basic", "basic.css"))
	if err != nil {
		return fmt.Errorf("failed to parse CSS template: %w", err)
	}

	outputCssFile, err := os.Create(filepath.Join(outputDir, "styles.css"))
	if err != nil {
		return fmt.Errorf("failed to create CSS output file: %w", err)
	}
	defer outputCssFile.Close()

	if err := cssTmpl.Execute(outputCssFile, data); err != nil {
		return fmt.Errorf("failed to execute CSS template: %w", err)
	}

	// Store shop in OrbitDB
	if err := g.orbitDB.StoreShop(shop); err != nil {
		return fmt.Errorf("failed to store shop in OrbitDB: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func (g *Generator) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}
