package models

import "errors"

var (
	// ErrEmptyShopName is returned when a shop name is empty
	ErrEmptyShopName = errors.New("shop name cannot be empty")
	
	// ErrEmptyOwnerAddress is returned when owner address is empty
	ErrEmptyOwnerAddress = errors.New("owner address cannot be empty")
	
	// ErrEmptyItemName is returned when an item name is empty
	ErrEmptyItemName = errors.New("item name cannot be empty")
	
	// ErrInvalidPrice is returned when an item price is negative
	ErrInvalidPrice = errors.New("item price cannot be negative")
)
