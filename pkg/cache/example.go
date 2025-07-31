package cache

import (
	"fmt"
	"time"

	"github.com/duccv/go-clean-template/config"
)

// ExampleUsage demonstrates basic usage of the flexible cache system.
// This example shows how to create a cache from configuration, add items,
// retrieve items, and get cache statistics.
//
// The example uses the application's configuration to determine cache type,
// capacity, and default TTL, making it easy to switch between cache types
// without code changes.
func ExampleUsage() {
	// Get configuration
	env := config.GetEnv()

	// Create cache based on configuration
	cache := NewCache(env.CacheConfig)
	defer cache.Stop()

	fmt.Printf("Using %s cache with capacity %d and TTL %d seconds\n",
		env.CacheConfig.Type, env.CacheConfig.Capacity, env.CacheConfig.DefaultTTL)

	// Add some items
	cache.Set("user:1", map[string]interface{}{
		"id":    1,
		"name":  "John Doe",
		"email": "john@example.com",
	})

	cache.SetWithTTL("session:abc123", "active", 300) // 5 minutes TTL

	// Get items
	if user, exists := cache.Get("user:1"); exists {
		fmt.Printf("Found user: %v\n", user)
	}

	if session, exists := cache.Get("session:abc123"); exists {
		fmt.Printf("Session status: %v\n", session)
	}

	// Get cache statistics
	fmt.Printf("Cache size: %d/%d\n", cache.Size(), cache.MaxSize())

	// Get all items
	allItems := cache.GetAll()
	fmt.Printf("Total items in cache: %d\n", len(allItems))
}

// ExampleCacheSwitching demonstrates how to create and use different cache types
// dynamically. This example shows how the same interface can be used with
// different cache implementations (LRU and FIFO).
//
// The example highlights the differences between cache types:
// - LRU: Accessing items affects their position in the eviction order
// - FIFO: Accessing items does not change their position
func ExampleCacheSwitching() {
	// Create LRU cache
	lruConfig := config.CacheConfig{
		Type:       "LRU",
		Capacity:   100,
		DefaultTTL: 60,
	}
	lruCache := NewCache(lruConfig)
	defer lruCache.Stop()

	// Create FIFO cache
	fifoConfig := config.CacheConfig{
		Type:       "FIFO",
		Capacity:   100,
		DefaultTTL: 60,
	}
	fifoCache := NewCache(fifoConfig)
	defer fifoCache.Stop()

	// Both caches implement the same interface
	caches := []Cache{lruCache, fifoCache}

	for i, cache := range caches {
		cacheType := "LRU"
		if i == 1 {
			cacheType = "FIFO"
		}

		// Add items
		for j := 0; j < 5; j++ {
			cache.Set(fmt.Sprintf("key%d", j), fmt.Sprintf("value%d", j))
		}

		// Access some items (this affects LRU order)
		cache.Get("key0")
		cache.Get("key2")

		fmt.Printf("%s Cache - Size: %d\n",
			cacheType, cache.Size())
	}
}

// ExampleCustomTTL demonstrates how to use custom TTL (Time To Live) values
// for different cache items. This example shows how items with different
// expiration times behave in the cache.
//
// The example adds items with different TTLs and demonstrates:
// - How expired items are automatically removed
// - How cache size changes as items expire
// - How the background cleanup process works
func ExampleCustomTTL() {
	cache := NewCacheWithConfig()
	defer cache.Stop()

	// Add items with different TTLs
	cache.SetWithTTL("short-lived", "expires in 10 seconds", 10)
	cache.SetWithTTL("medium-lived", "expires in 60 seconds", 60)
	cache.SetWithTTL("long-lived", "expires in 300 seconds", 300)

	fmt.Println("Added items with different TTLs")
	fmt.Printf("Cache size: %d\n", cache.Size())

	// Wait for some items to expire
	time.Sleep(15 * time.Second)

	// Check which items are still there
	if _, exists := cache.Get("short-lived"); !exists {
		fmt.Println("Short-lived item expired")
	}

	if _, exists := cache.Get("medium-lived"); exists {
		fmt.Println("Medium-lived item still exists")
	}

	fmt.Printf("Cache size after expiration: %d\n", cache.Size())
}
