package cache

import (
	"container/list"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FIFOCache implements a First In, First Out cache with TTL support.
// This cache evicts the oldest items (by insertion time) when the cache is full.
// Unlike LRU cache, accessing items does not change their position in the eviction order.
//
// FIFO cache is ideal for scenarios where:
//   - You want predictable eviction order based on insertion time
//   - Access patterns don't affect cache performance
//   - Simple caching requirements with ordered data
//
// The cache is thread-safe and includes automatic background cleanup of expired items.
type FIFOCache struct {
	cacheData  map[string]*list.Element
	list       *list.List
	maxSize    int
	defaultTtl time.Duration
	mu         sync.RWMutex
	stopChan   chan struct{}
}

// fifoItem represents an item in the FIFO cache.
// It wraps the cache data with the key for efficient list operations.
type fifoItem struct {
	key  string
	data CacheData
}

// NewFIFOCache creates a new FIFO cache with specified max size and default TTL.
// The cache will automatically start a background goroutine for cleaning up expired items.
//
// Parameters:
//   - maxSize: Maximum number of items the cache can hold
//   - defaultTtlSeconds: Default time-to-live for cache items in seconds
//
// The cache will evict the oldest items (front of the list) when capacity is reached.
// Background cleanup runs every 3 seconds to remove expired items.
func NewFIFOCache(maxSize, defaultTtlSeconds int) *FIFOCache {
	cache := &FIFOCache{
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
func (c *FIFOCache) cleanupExpiredKeys() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			expiredCount := 0

			// Iterate through list to maintain FIFO order
			for e := c.list.Front(); e != nil; {
				next := e.Next()
				item := e.Value.(*fifoItem)

				if now.After(item.data.Timeout) {
					c.list.Remove(e)
					delete(c.cacheData, item.key)
					expiredCount++
				}
				e = next
			}

			if expiredCount > 0 {
				zap.L().
					Debug("Cleaned up expired FIFO cache entries", zap.Int("count", expiredCount))
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
func (c *FIFOCache) Stop() {
	close(c.stopChan)
}

// Set adds a key-value pair to the cache with the default TTL.
// If the key already exists, the value will be updated but the position
// in the FIFO order remains unchanged.
func (c *FIFOCache) Set(key string, value any) {
	c.SetWithTTL(key, value, int(c.defaultTtl.Seconds()))
}

// SetWithTTL adds a key-value pair to the cache with a custom TTL in seconds.
// If the cache is full, the oldest item (front of the list) will be evicted.
// If the key already exists, the value and TTL will be updated but the position
// in the FIFO order remains unchanged.
func (c *FIFOCache) SetWithTTL(key string, value any, ttlSeconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if element, exists := c.cacheData[key]; exists {
		// Update existing item
		item := element.Value.(*fifoItem)
		item.data.Value = value
		item.data.Timeout = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
		return
	}

	// Check if cache is full (FIFO eviction)
	if c.list.Len() >= c.maxSize {
		// Remove oldest item (front of list)
		oldest := c.list.Front()
		if oldest != nil {
			oldestItem := oldest.Value.(*fifoItem)
			c.list.Remove(oldest)
			delete(c.cacheData, oldestItem.key)
			zap.L().Debug("FIFO cache evicted oldest item", zap.String("key", oldestItem.key))
		}
	}

	// Add new item to end of list
	item := &fifoItem{
		key: key,
		data: CacheData{
			Value:   value,
			Timeout: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
		},
	}

	element := c.list.PushBack(item)
	c.cacheData[key] = element
}

// Get retrieves a value from the cache by its key.
// Returns the value and a boolean indicating if the key exists.
// Unlike LRU cache, this operation does not change the item's position in the FIFO order.
// If the item has expired, it will be removed and false will be returned.
func (c *FIFOCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	element, exists := c.cacheData[key]
	if !exists {
		return nil, false
	}

	item := element.Value.(*fifoItem)

	// Check if expired
	if time.Now().After(item.data.Timeout) {
		go c.Delete(key)
		return item.data.Value, false
	}

	return item.data.Value, true
}

// Delete removes a key from the cache.
// This operation is thread-safe and will remove the item regardless of whether
// it has expired or not.
func (c *FIFOCache) Delete(key string) {
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
func (c *FIFOCache) GetAll() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]any)
	now := time.Now()

	for key, element := range c.cacheData {
		item := element.Value.(*fifoItem)

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
func (c *FIFOCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list.Len()
}

// MaxSize returns the maximum size of the cache.
// When the cache reaches this size, the oldest items will be evicted.
func (c *FIFOCache) MaxSize() int {
	return c.maxSize
}

// Clear removes all items from the cache.
// This operation is thread-safe and immediate.
// The background cleanup goroutine continues running.
func (c *FIFOCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.list.Init()
	c.cacheData = make(map[string]*list.Element)
}

// Keys returns all keys in the cache in FIFO order (oldest first).
// Expired items are automatically excluded from the result.
func (c *FIFOCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, c.list.Len())
	now := time.Now()

	for e := c.list.Front(); e != nil; e = e.Next() {
		item := e.Value.(*fifoItem)

		// Skip expired items
		if now.After(item.data.Timeout) {
			continue
		}

		keys = append(keys, item.key)
	}

	return keys
}

// Values returns all values in the cache in FIFO order (oldest first).
// Expired items are automatically excluded from the result.
func (c *FIFOCache) Values() []any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values := make([]any, 0, c.list.Len())
	now := time.Now()

	for e := c.list.Front(); e != nil; e = e.Next() {
		item := e.Value.(*fifoItem)

		// Skip expired items
		if now.After(item.data.Timeout) {
			continue
		}

		values = append(values, item.data.Value)
	}

	return values
}
