package models

import (
	"image/color"
)

// Shop represents a shop in the IndieNode system
type Shop struct {
	ID             string
	OwnerAddress   string
	Name           string
	Description    string
	Location       string
	Email          string
	Phone          string
	PrimaryColor   color.RGBA
	SecondaryColor color.RGBA
	TertiaryColor  color.RGBA
	LogoPath       string
	LocalLogoPath  string // For UI preview
	Items          []Item
	CID            string // IPFS Content Identifier
}

// Validate performs basic validation on the shop data
func (s *Shop) Validate() error {
	if s.Name == "" {
		return ErrEmptyShopName
	}
	if s.OwnerAddress == "" {
		return ErrEmptyOwnerAddress
	}
	return nil
}
