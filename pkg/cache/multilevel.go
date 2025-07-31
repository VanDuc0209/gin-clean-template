package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Global singleflight group for sharing results across goroutines
var sfGroup singleflight.Group

// GetWithMultiLevelCache tries to get data from memory cache, then Redis, then API endpoint (with configurable timeouts for each layer).
// If data is found in a lower layer, it will be set back to the upper layers.
// Uses singleflight to prevent thundering herd problem - multiple goroutines requesting the same key will share the result.
//
//   - ctx: context for cancellation
//   - key: cache key
//   - memCache: in-memory cache implementing Cache interface
//   - redisClient: Redis client
//   - fetchAPI: function to fetch data from API (should be context-aware)
//   - memTTL: TTL for memory cache (seconds)
//   - redisTTL: TTL for Redis cache (seconds)
//   - redisTimeout: timeout for Redis operations (default: 50ms)
//   - apiTimeout: timeout for API operations (default: 1s)
//   - additionalData: additional data to pass to fetchAPI function
//   - syncToRedis: whether to sync data back to Redis when fetched from API (default: false)
//
// Returns: value, error (if all layers fail)
func GetWithMultiLevelCache(
	ctx context.Context,
	key string,
	memCache Cache,
	redisClient *redis.Client,
	fetchAPI func(ctx context.Context, key string, additionalData any) (any, error),
	memTTL int,
	redisTTL int,
	redisTimeout time.Duration,
	apiTimeout time.Duration,
	additionalData any,
	syncToRedis bool,
) (any, error) {

	// 1. Try memory cache first (no singleflight needed for fast path)
	if val, ok := memCache.Get(key); ok {
		return val, nil
	}

	// 2. Use singleflight to prevent multiple goroutines from fetching the same data
	result, err, _ := sfGroup.Do(key, func() (any, error) {
		// Double-check memory cache after acquiring lock
		if val, ok := memCache.Get(key); ok {
			return val, nil
		}

		// 3. Try Redis cache (with configurable timeout)
		redisCtx, cancelRedis := context.WithTimeout(ctx, redisTimeout)
		defer cancelRedis()
		redisVal, err := redisClient.Get(redisCtx, key).Result()
		if err == nil && redisVal != "" {
			// Unmarshal JSON (assume all values are stored as JSON string)
			var result any
			if err := json.Unmarshal([]byte(redisVal), &result); err == nil {
				// Set to memory cache
				memCache.SetWithTTL(key, result, memTTL)
				return result, nil
			}
		}

		// 4. Try API (with configurable timeout)
		apiCtx, cancelAPI := context.WithTimeout(ctx, apiTimeout)
		defer cancelAPI()
		result, err := fetchAPI(apiCtx, key, additionalData)
		if err != nil {
			return nil, err
		}

		// Set to memory cache
		memCache.SetWithTTL(key, result, memTTL)

		// Sync to Redis only if syncToRedis flag is true
		if syncToRedis && redisClient != nil {
			if data, err := json.Marshal(result); err == nil {
				redisClient.Set(ctx, key, data, time.Duration(redisTTL)*time.Second)
			}
		}
		return result, nil
	})

	return result, err
}

// GetWithMultiLevelCacheDefault is a convenience function that uses default timeouts.
// Redis timeout: 50ms, API timeout: 1s, syncToRedis: false
//
// This function is ideal for most use cases where you want to use the recommended
// timeout values and don't need to sync data back to Redis when fetched from API.
// It's perfect for hot data that changes frequently and doesn't need to be
// persisted across application restarts.
//
// Parameters:
//   - ctx: context for cancellation
//   - key: cache key
//   - memCache: in-memory cache implementing Cache interface
//   - redisClient: Redis client
//   - fetchAPI: function to fetch data from API (should be context-aware)
//   - memTTL: TTL for memory cache (seconds)
//   - redisTTL: TTL for Redis cache (seconds)
//   - additionalData: additional data to pass to fetchAPI function
//
// Returns: value, error (if all layers fail)
//
// Example:
//
//	user, err := GetWithMultiLevelCacheDefault(
//	    ctx,
//	    "user:123",
//	    memCache,
//	    redisClient,
//	    fetchUserAPI,
//	    60,   // memTTL: 1 minute
//	    300,  // redisTTL: 5 minutes
//	    nil,  // additionalData
//	)
func GetWithMultiLevelCacheDefault(
	ctx context.Context,
	key string,
	memCache Cache,
	redisClient *redis.Client,
	fetchAPI func(ctx context.Context, key string, additionalData any) (any, error),
	memTTL int,
	redisTTL int,
	additionalData any,
) (any, error) {
	return GetWithMultiLevelCache(
		ctx,
		key,
		memCache,
		redisClient,
		fetchAPI,
		memTTL,
		redisTTL,
		50*time.Millisecond,
		1*time.Second,
		additionalData,
		false, // syncToRedis: false by default
	)
}

// GetWithMultiLevelCacheSimple is a simplified version without additional data.
// This function is designed for simple use cases where you don't need to pass
// additional data to your fetch function. It automatically wraps your simple
// fetch function to match the required signature.
//
// The function uses default timeouts:
//   - Redis timeout: 50ms
//   - API timeout: 1s
//   - syncToRedis: false (no Redis sync)
//
// This is perfect for basic caching scenarios where your fetch function
// only needs the context and key to retrieve data.
//
// Parameters:
//   - ctx: context for cancellation
//   - key: cache key
//   - memCache: in-memory cache implementing Cache interface
//   - redisClient: Redis client
//   - fetchAPI: simple function to fetch data from API (only needs ctx and key)
//   - memTTL: TTL for memory cache (seconds)
//   - redisTTL: TTL for Redis cache (seconds)
//
// Returns: value, error (if all layers fail)
//
// Example:
//
//	// Simple fetch function
//	func fetchUserSimple(ctx context.Context, key string) (any, error) {
//	    userID := strings.TrimPrefix(key, "user:")
//	    url := fmt.Sprintf("/api/users/%s", userID)
//	    return httpClient.Get(ctx, url)
//	}
//
//	// Usage
//	user, err := GetWithMultiLevelCacheSimple(
//	    ctx,
//	    "user:123",
//	    memCache,
//	    redisClient,
//	    fetchUserSimple,
//	    60,   // memTTL: 1 minute
//	    300,  // redisTTL: 5 minutes
//	)
func GetWithMultiLevelCacheSimple(
	ctx context.Context,
	key string,
	memCache Cache,
	redisClient *redis.Client,
	fetchAPI func(ctx context.Context, key string) (any, error),
	memTTL int,
	redisTTL int,
) (any, error) {
	// Wrap the simple fetchAPI to match the signature with additionalData
	wrappedFetchAPI := func(ctx context.Context, key string, additionalData any) (any, error) {
		return fetchAPI(ctx, key)
	}

	return GetWithMultiLevelCache(
		ctx,
		key,
		memCache,
		redisClient,
		wrappedFetchAPI,
		memTTL,
		redisTTL,
		50*time.Millisecond,
		1*time.Second,
		nil,   // additionalData
		false, // syncToRedis: false by default
	)
}
