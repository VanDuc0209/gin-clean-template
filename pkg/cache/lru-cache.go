package cache

import (
	"container/list"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LRUCache implements a Least Recently Used cache with TTL support.
// This cache evicts the least recently accessed items when the cache is full.
// Each access to an item moves it to the "most recently used" position.
//
// LRU cache is ideal for scenarios where:
//   - Frequently accessed items should stay in cache longer
//   - Access patterns affect cache performance
//   - You want to optimize for hot data
//
// The cache is thread-safe and includes automatic background cleanup of expired items.
type LRUCache struct {
	cacheData  map[string]*list.Element
	list       *list.List
	maxSize    int
	defaultTtl time.Duration
	mu         sync.RWMutex
	stopChan   chan struct{}
}

// lruItem represents an item in the LRU cache.
// It wraps the cache data with the key for efficient list operations.
type lruItem struct {
	key  string
	data CacheData
}

// NewLRUCache creates a new LRU cache with specified max size and default TTL.
// The cache will automatically start a background goroutine for cleaning up expired items.
//
// Parameters:
//   - maxSize: Maximum number of items the cache can hold
//   - defaultTtlSeconds: Default time-to-live for cache items in seconds
//
// The cache will evict the least recently used items (front of the list) when capacity is reached.
// Background cleanup runs every 3 seconds to remove expired items.
func NewLRUCache(maxSize, defaultTtlSeconds int) *LRUCache {
	cache := &LRUCache{
		cacheData:  make(map[string]*list.Element),
		list:       list.New(),
		maxSize:    maxSize,
		defaultTtl: time.Duration(defaultTtlSeconds) * time.Second,
		stopChan:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupExpiredKeys()

	return cache
}

// cleanupExpiredKeys removes expired keys from cache every 3 seconds.
// This method runs in a background goroutine and automatically stops when
// the cache is stopped via the Stop() method.
func (c *LRUCache) cleanupExpiredKeys() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			expiredCount := 0

			// Iterate through list to maintain LRU order
			for e := c.list.Front(); e != nil; {
				next := e.Next()
				item := e.Value.(*lruItem)

				if now.After(item.data.Timeout) {
					c.list.Remove(e)
					delete(c.cacheData, item.key)
					expiredCount++
				}
				e = next
			}

			if expiredCount > 0 {
				zap.L().
					Debug("Cleaned up expired LRU cache entries", zap.Int("count", expiredCount))
			}
			c.mu.Unlock()

		case <-c.stopChan:
			return
		}
	}
}

// Stop gracefully shuts down the cache and its background cleanup goroutine.
// This method should be called when the cache is no longer needed to prevent
// goroutine leaks. It is safe to call this method multiple times.
func (c *LRUCache) Stop() {
	close(c.stopChan)
}

// Set adds a key-value pair to the cache with the default TTL.
// If the key already exists, the value will be updated and the item will be moved
// to the "most recently used" position.
func (c *LRUCache) Set(key string, value any) {
	c.SetWithTTL(key, value, int(c.defaultTtl.Seconds()))
}

// SetWithTTL adds a key-value pair to the cache with a custom TTL in seconds.
// If the cache is full, the least recently used item (front of the list) will be evicted.
// If the key already exists, the value and TTL will be updated and the item will be moved
// to the "most recently used" position.
func (c *LRUCache) SetWithTTL(key string, value any, ttlSeconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if element, exists := c.cacheData[key]; exists {
		// Update existing item and move to back (most recently used)
		item := element.Value.(*lruItem)
		item.data.Value = value
		item.data.Timeout = time.Now().Add(time.Duration(ttlSeconds) * time.Second)

		// Move to back of list (most recently used)
		c.list.MoveToBack(element)
		return
	}

	// Check if cache is full (LRU eviction)
	if c.list.Len() >= c.maxSize {
		// Remove least recently used item (front of list)
		oldest := c.list.Front()
		if oldest != nil {
			oldestItem := oldest.Value.(*lruItem)
			c.list.Remove(oldest)
			delete(c.cacheData, oldestItem.key)
			zap.L().
				Debug("LRU cache evicted least recently used item", zap.String("key", oldestItem.key))
		}
	}

	// Add new item to back of list (most recently used)
	item := &lruItem{
		key: key,
		data: CacheData{
			Value:   value,
			Timeout: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
		},
	}

	element := c.list.PushBack(item)
	c.cacheData[key] = element
}

// Get retrieves a value from the cache by its key and updates its position.
// Returns the value and a boolean indicating if the key exists.
// This operation moves the accessed item to the "most recently used" position.
// If the item has expired, it will be removed and false will be returned.
func (c *LRUCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, exists := c.cacheData[key]
	if !exists {
		return nil, false
	}

	item := element.Value.(*lruItem)

	// Check if expired
	if time.Now().After(item.data.Timeout) {
		c.list.Remove(element)
		delete(c.cacheData, key)
		return item.data.Value, false
	}

	// Move to back of list (most recently used)
	c.list.MoveToBack(element)

	return item.data.Value, true
}

// Delete removes a key from the cache.
// This operation is thread-safe and will remove the item regardless of whether
// it has expired or not.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.cacheData[key]; exists {
		c.list.Remove(element)
		delete(c.cacheData, key)
	}
}

// GetAll returns all key-value pairs currently in the cache.
// Expired items are automatically excluded from the result.
// The returned map is a copy and modifications won't affect the cache.
func (c *LRUCache) GetAll() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]any)
	now := time.Now()

	for key, element := range c.cacheData {
		item := element.Value.(*lruItem)

		// Skip expired items
		if now.After(item.data.Timeout) {
			continue
		}

		result[key] = item.data.Value
	}

	return result
}

// Size returns the current number of items in the cache.
// This count excludes expired items that haven't been cleaned up yet.
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list.Len()
}

// MaxSize returns the maximum size of the cache.
// When the cache reaches this size, the least recently used items will be evicted.
func (c *LRUCache) MaxSize() int {
	return c.maxSize
}

// Clear removes all items from the cache.
// This operation is thread-safe and immediate.
// The background cleanup goroutine continues running.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.list.Init()
	c.cacheData = make(map[string]*list.Element)
}

// Keys returns all keys in the cache in LRU order (least recently used first).
// Expired items are automatically excluded from the result.
func (c *LRUCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, c.list.Len())
	now := time.Now()

	for e := c.list.Front(); e != nil; e = e.Next() {
		item := e.Value.(*lruItem)

		// Skip expired items
		if now.After(item.data.Timeout) {
			continue
		}

		keys = append(keys, item.key)
	}

	return keys
}

// Values returns all values in the cache in LRU order (least recently used first).
// Expired items are automatically excluded from the result.
func (c *LRUCache) Values() []any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values := make([]any, 0, c.list.Len())
	now := time.Now()

	for e := c.list.Front(); e != nil; e = e.Next() {
		item := e.Value.(*lruItem)

		// Skip expired items
		if now.After(item.data.Timeout) {
			continue
		}

		values = append(values, item.data.Value)
	}

	return values
}

// GetLRU returns the least recently used key without updating its position.
// Returns the key, value, and a boolean indicating if the key exists.
// This method is useful for monitoring cache behavior without affecting access order.
func (c *LRUCache) GetLRU() (string, any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.list.Len() == 0 {
		return "", nil, false
	}

	oldest := c.list.Front()
	item := oldest.Value.(*lruItem)

	// Check if expired
	if time.Now().After(item.data.Timeout) {
		return "", nil, false
	}

	return item.key, item.data.Value, true
}

// GetMRU returns the most recently used key without updating its position.
// Returns the key, value, and a boolean indicating if the key exists.
// This method is useful for monitoring cache behavior without affecting access order.
func (c *LRUCache) GetMRU() (string, any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.list.Len() == 0 {
		return "", nil, false
	}

	newest := c.list.Back()
	item := newest.Value.(*lruItem)

	// Check if expired
	if time.Now().After(item.data.Timeout) {
		return "", nil, false
	}

	return item.key, item.data.Value, true
}

// Touch updates the access time of a key by moving it to the most recently used position.
// Returns a boolean indicating if the key exists and was successfully updated.
// This method is useful for keeping items in cache without retrieving their values.
func (c *LRUCache) Touch(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, exists := c.cacheData[key]
	if !exists {
		return false
	}

	item := element.Value.(*lruItem)

	// Check if expired
	if time.Now().After(item.data.Timeout) {
		c.list.Remove(element)
		delete(c.cacheData, key)
		return false
	}

	// Move to back of list (most recently used)
	c.list.MoveToBack(element)
	return true
}
