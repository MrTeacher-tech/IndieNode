package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

// InitDB initializes the SQLite database
func InitDB() error {
	// Create data directory if it doesn't exist
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open database connection
	dbPath := filepath.Join(dataDir, "nonces.db")
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create nonces table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS nonces (
		nonce TEXT PRIMARY KEY,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		used_at TIMESTAMP,
		ethereum_address TEXT,
		is_used BOOLEAN DEFAULT FALSE
	);
	`

	if _, err := DB.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create nonces table: %w", err)
	}

	return nil
}

// SaveNonce stores a new nonce in the database
func SaveNonce(nonce, address string) error {
	query := `
	INSERT INTO nonces (nonce, ethereum_address)
	VALUES (?, ?);
	`

	_, err := DB.Exec(query, nonce, address)
	if err != nil {
		return fmt.Errorf("failed to save nonce: %w", err)
	}

	return nil
}

// VerifyAndUseNonce checks if a nonce exists and hasn't been used, then marks it as used
func VerifyAndUseNonce(nonce string) (bool, error) {
	tx, err := DB.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if nonce exists and is unused
	var isUsed bool
	err = tx.QueryRow("SELECT is_used FROM nonces WHERE nonce = ?", nonce).Scan(&isUsed)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query nonce: %w", err)
	}

	if isUsed {
		return false, nil
	}

	// Mark nonce as used
	_, err = tx.Exec("UPDATE nonces SET is_used = TRUE, used_at = CURRENT_TIMESTAMP WHERE nonce = ?", nonce)
	if err != nil {
		return false, fmt.Errorf("failed to mark nonce as used: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return true, nil
}

// CleanupOldNonces removes nonces older than the specified duration
func CleanupOldNonces(age time.Duration) error {
	cutoff := time.Now().Add(-age)

	query := `
	DELETE FROM nonces
	WHERE created_at < ?
	AND (is_used = TRUE OR created_at < ?);
	`

	_, err := DB.Exec(query, cutoff, time.Now().Add(-24*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to cleanup old nonces: %w", err)
	}

	return nil
}
