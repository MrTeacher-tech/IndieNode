package models

import "time"

// Nonce represents a SIWE nonce in the database
type Nonce struct {
	Value           string
	CreatedAt       time.Time
	UsedAt          *time.Time
	EthereumAddress string
	IsUsed          bool
}
