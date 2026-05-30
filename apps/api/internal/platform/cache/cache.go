package cache

import (
	"sync"
	"time"
)

type cacheItem struct {
	value      any
	expiration int64
}

// MemoryCache represents an in-memory thread-safe cache with TTL support.
type MemoryCache struct {
	items map[string]*cacheItem
	mu    sync.RWMutex
}

// New creates a new MemoryCache instance.
func New() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]*cacheItem),
	}
}

// Set adds or updates an item in the cache with a specific TTL.
func (c *MemoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl).UnixNano(),
	}
}

// Get retrieves an item from the cache. Returns nil if not found or expired.
func (c *MemoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Evict expired items lazily
	if time.Now().UnixNano() > item.expiration {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	return item.value, true
}

// Evict deletes an item from the cache.
func (c *MemoryCache) Evict(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear clears all cache entries.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}
