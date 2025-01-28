package shop

import (
	"fmt"
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

	// Generate HTML from template
	tmpl, err := template.ParseFiles(filepath.Join(g.templatesDir, "shop.html"))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	outputFile, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if err := tmpl.Execute(outputFile, shop); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
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
