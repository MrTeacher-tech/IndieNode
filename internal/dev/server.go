package dev

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
)

// StartDevServer starts a development server to serve shop files
func StartDevServer(shopDir string, port int) error {
	// Get the src directory which contains the shop files
	srcDir := filepath.Join(shopDir, "src")

	// Create file server handler
	fs := http.FileServer(http.Dir(srcDir))

	// Handle all requests by serving files from the shop directory
	http.Handle("/", fs)

	addr := fmt.Sprintf("localhost:%d", port)
	log.Printf("Starting development server at http://%s\n", addr)
	log.Printf("Serving files from: %s\n", srcDir)

	return http.ListenAndServe(addr, nil)
}

// ServeCurrentShop starts a development server to serve the current shop files
func ServeCurrentShop(shopsBaseDir string, port int) error {
	// Get the current shop directory
	shopDir := filepath.Join(shopsBaseDir, "Dev Test Shop")
	srcDir := filepath.Join(shopDir, "src")

	// Create file server handler
	fs := http.FileServer(http.Dir(srcDir))

	// Handle all requests by serving files from the shop directory
	http.Handle("/", fs)

	addr := fmt.Sprintf("localhost:%d", port)
	log.Printf("Starting development server at http://%s\n", addr)
	log.Printf("Serving files from: %s\n", srcDir)
	log.Printf("Open your browser to http://%s to view your shop\n", addr)

	return http.ListenAndServe(addr, nil)
}
