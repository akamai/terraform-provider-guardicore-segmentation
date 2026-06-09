package common

import "sync"

// AssetLabelIgnoreCache is a thread-safe cache that tracks which label IDs
// should be ignored when reconciling asset labels (e.g., server-assigned labels).
type AssetLabelIgnoreCache struct {
	mu    sync.RWMutex
	items map[string]bool
}

// NewAssetLabelIgnoreCache creates a new empty AssetLabelIgnoreCache.
func NewAssetLabelIgnoreCache() *AssetLabelIgnoreCache {
	return &AssetLabelIgnoreCache{
		items: make(map[string]bool),
	}
}

// Get returns the ignored status for a label ID and whether the ID was present in the cache.
func (c *AssetLabelIgnoreCache) Get(labelID string) (bool, bool) {
	c.mu.RLock()
	ignored, ok := c.items[labelID]
	c.mu.RUnlock()
	return ignored, ok
}

// Set stores the ignored status for a label ID.
func (c *AssetLabelIgnoreCache) Set(labelID string, ignored bool) {
	c.mu.Lock()
	c.items[labelID] = ignored
	c.mu.Unlock()
}
