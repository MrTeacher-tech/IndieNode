package models

// Item represents a product or service in a shop
type Item struct {
	ID              string
	Name            string
	Price           float64
	Description     string
	PhotoPaths      []string
	LocalPhotoPaths []string // For UI preview
}

// Validate performs basic validation on the item data
func (i *Item) Validate() error {
	if i.Name == "" {
		return ErrEmptyItemName
	}
	if i.Price < 0 {
		return ErrInvalidPrice
	}
	return nil
}
