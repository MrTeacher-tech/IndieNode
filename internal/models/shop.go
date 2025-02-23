package models

import (
	"image/color"
	"regexp"
	"strings"
)

// Shop represents a shop in the IndieNode system
type Shop struct {
	ID             string
	OwnerAddress   string
	Name           string
	URLName        string    // URL-safe version of the shop name
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
	Published       bool
}

// Validate performs basic validation on the shop data
// GenerateURLName creates a URL-safe name from the shop name
func (s *Shop) GenerateURLName() {
	// Convert to lowercase
	urlName := strings.ToLower(s.Name)
	
	// Replace spaces with hyphens
	urlName = strings.ReplaceAll(urlName, " ", "-")
	
	// Remove any character that isn't alphanumeric or hyphen
	reg := regexp.MustCompile("[^a-z0-9-]+")
	urlName = reg.ReplaceAllString(urlName, "")
	
	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile("-+")
	urlName = reg.ReplaceAllString(urlName, "-")
	
	// Trim hyphens from start and end
	urlName = strings.Trim(urlName, "-")
	
	s.URLName = urlName
}

func (s *Shop) Validate() error {
	if s.Name == "" {
		return ErrEmptyShopName
	}
	if s.OwnerAddress == "" {
		return ErrEmptyOwnerAddress
	}
	if s.URLName == "" {
		s.GenerateURLName()
	}
	return nil
}
