package auth

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// AuthenticatedUser represents a user that has been authenticated with Ethereum
type AuthenticatedUser struct {
	Address  string
	Message  string
	LoggedIn bool
}

var (
	currentUser *AuthenticatedUser
	authMutex   sync.RWMutex
)

const devModeAddress = "0x37eA7944328DF1A4D7ffA6658A002d5C332cTEST"

// Service handles authentication-related operations
type Service struct {
	server      *http.Server
	serverMutex sync.Mutex
	isRunning   bool
}

// NewService creates a new authentication service
func NewService() *Service {
	return &Service{}
}

// GetAuthenticatedUser returns the current authenticated user
func (s *Service) GetAuthenticatedUser() *AuthenticatedUser {
	authMutex.RLock()
	defer authMutex.RUnlock()
	return currentUser
}

// GetCurrentUser returns the current authenticated user
func GetCurrentUser() *AuthenticatedUser {
	authMutex.RLock()
	defer authMutex.RUnlock()
	return currentUser
}

// SetCurrentUser sets the current authenticated user
func SetCurrentUser(user *AuthenticatedUser) {
	authMutex.Lock()
	defer authMutex.Unlock()
	currentUser = user
}

// AuthenticateWithEthereum handles the Ethereum authentication process
func (s *Service) AuthenticateWithEthereum(address, message, signature string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in AuthenticateWithEthereum: %v", r)
		}
	}()

	// Basic validation
	if address == "" || message == "" || signature == "" {
		return fmt.Errorf("address, message, and signature are required")
	}

	// Validate Ethereum address
	if !common.IsHexAddress(address) {
		return fmt.Errorf("invalid Ethereum address format")
	}

	// Create new authenticated user
	user := &AuthenticatedUser{
		Address:  address,
		Message:  message,
		LoggedIn: true,
	}

	// Update the current user
	SetCurrentUser(user)

	return nil
}

// ClearCurrentUser clears the current authenticated user
func ClearCurrentUser() {
	SetCurrentUser(nil)
}

// IsAuthenticated returns true if there is a currently authenticated user
func IsAuthenticated() bool {
	user := GetCurrentUser()
	return user != nil && user.LoggedIn
}

// IsServerRunning returns true if the authentication server is running
func (s *Service) IsServerRunning() bool {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()
	return s.isRunning
}

// SetDevModeUser sets a mock authenticated user for development
func (s *Service) SetDevModeUser() {
	user := &AuthenticatedUser{
		Address:  devModeAddress,
		Message:  "Dev Mode Authentication",
		LoggedIn: true,
	}
	SetCurrentUser(user)
}

// IsDevMode returns true if DEV_MODE environment variable is set to "TRUE"
func IsDevMode() bool {
	return os.Getenv("DEV_MODE") == "TRUE"
}
