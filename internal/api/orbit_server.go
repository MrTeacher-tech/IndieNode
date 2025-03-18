package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"IndieNode/db/orbitdb"
	"context"
)

// Server represents the HTTP API server for OrbitDB data
type Server struct {
	orbitManager *orbitdb.Manager
	router       *mux.Router
	server       *http.Server
	port         int
	startTime    time.Time
	requestCount uint64
	isRunning    bool
}

// Response represents a standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ServerStatus contains information about the API server status
type ServerStatus struct {
	Running      bool      `json:"running"`
	Port         int       `json:"port"`
	StartTime    time.Time `json:"startTime"`
	RequestCount uint64    `json:"requestCount"`
	Uptime       string    `json:"uptime"`
}

// NewServer creates a new API server that exposes OrbitDB data
func NewServer(orbitManager *orbitdb.Manager, port int) *Server {
	router := mux.NewRouter()

	server := &Server{
		orbitManager: orbitManager,
		router:       router,
		port:         port,
		startTime:    time.Now(),
		requestCount: 0,
	}

	// Set up routes
	server.setupRoutes()

	return server
}

// GetStatus returns the current server status
func (s *Server) GetStatus() ServerStatus {
	var uptime string
	if s.isRunning {
		uptime = time.Since(s.startTime).Round(time.Second).String()
	} else {
		uptime = "Not running"
	}

	return ServerStatus{
		Running:      s.isRunning,
		Port:         s.port,
		StartTime:    s.startTime,
		RequestCount: atomic.LoadUint64(&s.requestCount),
		Uptime:       uptime,
	}
}

// setupRoutes configures all API endpoints
func (s *Server) setupRoutes() {
	// Create a middleware to count requests
	countMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&s.requestCount, 1)
			next.ServeHTTP(w, r)
		})
	}

	// Apply middleware to all routes
	s.router.Use(countMiddleware)

	// Root endpoint to check if API is running
	s.router.HandleFunc("/api", s.handleAPIStatus).Methods("GET")

	// Shop-specific endpoints
	shopRouter := s.router.PathPrefix("/api/shops").Subrouter()
	shopRouter.HandleFunc("", s.handleListShops).Methods("GET")
	shopRouter.HandleFunc("/{shopId}", s.handleGetShop).Methods("GET")
	shopRouter.HandleFunc("/{shopId}/items", s.handleGetShopItems).Methods("GET")

	log.Printf("API endpoints configured")
}

// Start begins the HTTP server
func (s *Server) Start() error {
	// Configure CORS
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins - shop websites could be accessed from various domains
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	})

	// Reset start time and request count
	s.startTime = time.Now()
	atomic.StoreUint64(&s.requestCount, 0)

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      corsMiddleware.Handler(s.router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Mark as running before starting
	s.isRunning = true

	// Start the server
	log.Printf("Starting OrbitDB API server on port %d", s.port)
	err := s.server.ListenAndServe()

	// If we get here, the server has stopped
	s.isRunning = false

	return err
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	s.isRunning = false
	return err
}

// API Handlers

// handleAPIStatus checks if the API is running
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	status := s.GetStatus()

	response := Response{
		Success: true,
		Data: map[string]interface{}{
			"status":       "API is running",
			"time":         time.Now().Format(time.RFC3339),
			"startTime":    status.StartTime,
			"uptime":       status.Uptime,
			"requestCount": status.RequestCount,
		},
	}

	respondWithJSON(w, http.StatusOK, response)
}

// handleListShops returns a list of all shops
func (s *Server) handleListShops(w http.ResponseWriter, r *http.Request) {
	shops, err := s.orbitManager.ListAllShops(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list shops: "+err.Error())
		return
	}

	response := Response{
		Success: true,
		Data:    shops,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// handleGetShop returns details for a specific shop
func (s *Server) handleGetShop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shopID := vars["shopId"]

	if shopID == "" {
		respondWithError(w, http.StatusBadRequest, "Shop ID is required")
		return
	}

	shop, err := s.orbitManager.GetShop(r.Context(), shopID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Shop not found: "+err.Error())
		return
	}

	response := Response{
		Success: true,
		Data:    shop,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// handleGetShopItems returns items for a specific shop
func (s *Server) handleGetShopItems(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shopID := vars["shopId"]

	if shopID == "" {
		respondWithError(w, http.StatusBadRequest, "Shop ID is required")
		return
	}

	// Get the shop which includes items
	shop, err := s.orbitManager.GetShop(r.Context(), shopID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Shop not found: "+err.Error())
		return
	}

	response := Response{
		Success: true,
		Data:    shop.Items,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// Helper functions

// respondWithJSON writes a JSON response
func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(payload)
}

// respondWithError writes an error response
func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	response := Response{
		Success: false,
		Error:   message,
	}
	respondWithJSON(w, statusCode, response)
}
