package orbitdb

import (
	"sync"
	"time"

	"IndieNode/internal/models"
)

// ShopCache implements a simple in-memory cache for shop data
type ShopCache struct {
	shops    map[string]*models.Shop
	expiry   map[string]time.Time
	mutex    sync.RWMutex
	ttl      time.Duration
	maxSize  int
	lastUsed map[string]time.Time
}

// NewShopCache creates a new shop cache with the specified TTL and max size
func NewShopCache(ttl time.Duration, maxSize int) *ShopCache {
	return &ShopCache{
		shops:    make(map[string]*models.Shop),
		expiry:   make(map[string]time.Time),
		lastUsed: make(map[string]time.Time),
		ttl:      ttl,
		maxSize:  maxSize,
	}
}

// cloneShop makes a deep copy of a Shop
func cloneShop(s *models.Shop) *models.Shop {
	if s == nil {
		return nil
	}

	// Create a copy of the shop
	clone := &models.Shop{
		ID:             s.ID,
		OwnerAddress:   s.OwnerAddress,
		Name:           s.Name,
		Description:    s.Description,
		Location:       s.Location,
		Email:          s.Email,
		Phone:          s.Phone,
		CID:            s.CID,
		URLName:        s.URLName,
		Published:      s.Published,
		PrimaryColor:   s.PrimaryColor,
		SecondaryColor: s.SecondaryColor,
		TertiaryColor:  s.TertiaryColor,
	}

	// Deep copy items
	if len(s.Items) > 0 {
		clone.Items = make([]models.Item, len(s.Items))
		copy(clone.Items, s.Items)
	}

	return clone
}

// Get retrieves a shop from the cache by ID
func (c *ShopCache) Get(id string) (*models.Shop, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	shop, exists := c.shops[id]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(c.expiry[id]) {
		return nil, false
	}

	// Update last used time
	c.lastUsed[id] = time.Now()

	// Return a deep copy to prevent mutation
	return cloneShop(shop), true
}

// Set stores a shop in the cache
func (c *ShopCache) Set(id string, shop *models.Shop) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If cache is full, evict least recently used entry
	if len(c.shops) >= c.maxSize {
		var oldestID string
		var oldestTime time.Time

		// Find the oldest entry
		for id, t := range c.lastUsed {
			if oldestID == "" || t.Before(oldestTime) {
				oldestID = id
				oldestTime = t
			}
		}

		// Evict the oldest entry
		if oldestID != "" {
			delete(c.shops, oldestID)
			delete(c.expiry, oldestID)
			delete(c.lastUsed, oldestID)
		}
	}

	// Store a deep copy to prevent mutation
	c.shops[id] = cloneShop(shop)
	c.expiry[id] = time.Now().Add(c.ttl)
	c.lastUsed[id] = time.Now()
}

// Clear empties the cache
func (c *ShopCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.shops = make(map[string]*models.Shop)
	c.expiry = make(map[string]time.Time)
	c.lastUsed = make(map[string]time.Time)
}

// Size returns the number of items in the cache
func (c *ShopCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.shops)
}
