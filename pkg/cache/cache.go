// Package cache provides flexible in-memory caching implementations with support for
// different eviction policies (LRU, FIFO) and configurable TTL (Time To Live).
//
// The package offers a unified interface for different cache types, allowing easy
// switching between cache implementations based on configuration. Both LRU and FIFO
// caches support automatic expiration, thread-safe operations, and background cleanup.
//
// Example usage:
//
//	// Create cache from configuration
//	cache := NewCacheWithConfig()
//	defer cache.Stop()
//
//	// Add items with default TTL
//	cache.Set("key1", "value1")
//
//	// Add items with custom TTL
//	cache.SetWithTTL("key2", "value2", 300) // 5 minutes
//
//	// Retrieve items
//	value, exists := cache.Get("key1")
//
//	// Get cache statistics
//	size := cache.Size()
//	maxSize := cache.MaxSize()
package cache

import (
	"time"

	"github.com/duccv/go-clean-template/config"
)

// Cache interface defines the common methods for all cache implementations.
// All cache types (LRU, FIFO) implement this interface, allowing seamless
// switching between different eviction policies.
type Cache interface {
	// Get retrieves an item from the cache by its key.
	// Returns the value and a boolean indicating if the key exists.
	// For LRU caches, this operation also updates the access order.
	Get(key string) (any, bool)

	// Set adds a key-value pair to the cache with the default TTL.
	// If the key already exists, it will be updated with the new value.
	Set(key string, value any)

	// SetWithTTL adds a key-value pair to the cache with a custom TTL in seconds.
	// If the key already exists, it will be updated with the new value and TTL.
	SetWithTTL(key string, value any, ttlSeconds int)

	// Delete removes a key from the cache.
	// Returns immediately regardless of whether the key existed.
	Delete(key string)

	// GetAll returns all key-value pairs currently in the cache.
	// Expired items are automatically excluded from the result.
	GetAll() map[string]any

	// Size returns the current number of items in the cache.
	// This count excludes expired items that haven't been cleaned up yet.
	Size() int

	// MaxSize returns the maximum capacity of the cache.
	// When the cache reaches this size, eviction will occur based on the cache type.
	MaxSize() int

	// Clear removes all items from the cache.
	// This operation is thread-safe and immediate.
	Clear()

	// Stop gracefully shuts down the cache, including the background cleanup goroutine.
	// This should be called when the cache is no longer needed to prevent goroutine leaks.
	Stop()
}

// CacheData represents the data structure stored in cache.
// Each cache entry contains a value and an expiration timestamp.
type CacheData struct {
	// Value is the actual data stored in the cache.
	Value any
	// Timeout is the expiration timestamp for this cache entry.
	Timeout time.Time
}

// NewCache creates a new cache instance based on the provided configuration.
// The cache type is determined by the Type field in the configuration:
//   - "LRU": Creates a Least Recently Used cache
//   - "FIFO": Creates a First In, First Out cache
//   - Default: Falls back to LRU if type is not specified
//
// The capacity and default TTL are also taken from the configuration.
// The cache will start its background cleanup goroutine automatically.
//
// Example:
//
//	config := config.CacheConfig{
//	    Type:       "LRU",
//	    Capacity:   1000,
//	    DefaultTTL: 300,
//	}
//	cache := NewCache(config)
func NewCache(cfg config.CacheConfig) Cache {
	switch cfg.Type {
	case "LRU":
		return NewLRUCache(cfg.Capacity, cfg.DefaultTTL)
	case "FIFO":
		return NewFIFOCache(cfg.Capacity, cfg.DefaultTTL)
	default:
		// Default to LRU if type is not specified
		return NewLRUCache(cfg.Capacity, cfg.DefaultTTL)
	}
}

// NewCacheWithConfig creates a cache instance using the default application configuration.
// This is a convenience function that retrieves the cache configuration from the
// application environment and creates the appropriate cache type.
//
// The cache configuration is read from the application's config file or environment
// variables, allowing for easy configuration management without code changes.
//
// Example:
//
//	cache := NewCacheWithConfig()
//	defer cache.Stop()
func NewCacheWithConfig() Cache {
	env := config.GetEnv()
	return NewCache(env.CacheConfig)
}
