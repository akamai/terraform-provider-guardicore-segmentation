package provider

import "sync"

type assetLabelIgnoreCache struct {
	mu    sync.RWMutex
	items map[string]bool
}

func newAssetLabelIgnoreCache() *assetLabelIgnoreCache {
	return &assetLabelIgnoreCache{
		items: make(map[string]bool),
	}
}

func (c *assetLabelIgnoreCache) Get(labelID string) (bool, bool) {
	c.mu.RLock()
	ignored, ok := c.items[labelID]
	c.mu.RUnlock()
	return ignored, ok
}

func (c *assetLabelIgnoreCache) Set(labelID string, ignored bool) {
	c.mu.Lock()
	c.items[labelID] = ignored
	c.mu.Unlock()
}
