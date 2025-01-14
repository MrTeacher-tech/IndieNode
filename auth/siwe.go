package auth

import (
	"crypto/rand"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"shopCreator/db"
)

// SIWEMessage represents the structure of a Sign-In with Ethereum message
type SIWEMessage struct {
	Domain    string
	Address   string
	Statement string
	URI       string
	Version   string
	ChainId   int
	Nonce     string
	IssuedAt  time.Time
}

// AuthenticatedUser represents a user who has successfully authenticated with SIWE
type AuthenticatedUser struct {
	Address         string
	SignedMessage   string
	Signature       string
	AuthenticatedAt time.Time
}

// generateNonce creates a unique nonce using timestamp and random bytes
func generateNonce() (string, error) {
	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		// Generate a nonce using timestamp and random bytes
		timestamp := time.Now().Unix()
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}
		nonce := fmt.Sprintf("%d-%x", timestamp, randomBytes)

		// Check if this nonce exists in the database
		exists, err := checkNonceExists(nonce)
		if err != nil {
			return "", fmt.Errorf("failed to check nonce existence: %w", err)
		}

		// If nonce doesn't exist, save it and return
		if !exists {
			if err := db.SaveNonce(nonce, ""); err != nil {
				return "", fmt.Errorf("failed to save nonce: %w", err)
			}
			return nonce, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique nonce after %d attempts", maxAttempts)
}

func checkNonceExists(nonce string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM nonces WHERE nonce = ?)"
	err := db.DB.QueryRow(query, nonce).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CreateSIWEMessage generates a formatted SIWE message for signing
func CreateSIWEMessage(address string) *SIWEMessage {
	now := time.Now()
	nonce, err := generateNonce()
	if err != nil {
		// Log the error but use a fallback nonce
		fmt.Printf("Warning: Failed to generate secure nonce: %v\n", err)
		nonce = fmt.Sprintf("%d-fallback", now.Unix())
	}

	return &SIWEMessage{
		Domain:    "indienode.local",
		Address:   address,
		Statement: "Sign in with Ethereum to IndieNode",
		URI:       "indienode://login",
		Version:   "1",
		ChainId:   1, // Mainnet
		Nonce:     nonce,
		IssuedAt:  now,
	}
}

// FormatMessage formats the SIWE message for signing
func (msg *SIWEMessage) FormatMessage() string {
	return fmt.Sprintf(`%s wants you to sign in with your Ethereum account:
%s

%s

URI: %s
Version: %s
Chain ID: %d
Nonce: %s
Issued At: %s`,
		msg.Domain,
		msg.Address,
		msg.Statement,
		msg.URI,
		msg.Version,
		msg.ChainId,
		msg.Nonce,
		msg.IssuedAt.Format(time.RFC3339))
}

// VerifySignature verifies a SIWE message signature
func VerifySignature(msg *SIWEMessage, signature string, address string) bool {
	// Format the message for verification

	// In a real implementation, this would verify the signature using SIWE
	// For development, we'll simulate successful verification
	return true
}
