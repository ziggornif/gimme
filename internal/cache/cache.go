package cache

import (
	"context"
	"time"
)

// CacheEntry holds the fully-resolved S3 object key for a partial version request.
// e.g. key "pkg@1.0/file.js" → ObjectPath "pkg@1.0.3/file.js"
type CacheEntry struct {
	ObjectPath string
}

// CacheManager is the interface for the internal cache layer.
// Implementations must be safe for concurrent use.
type CacheManager interface {
	// Get retrieves a cache entry by key.
	// Returns nil, false if the key is not found or has expired.
	Get(ctx context.Context, key string) (*CacheEntry, bool)

	// Set stores a cache entry for the given key with the configured TTL.
	Set(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error

	// Delete removes the cache entry for the given key.
	Delete(ctx context.Context, key string) error

	// DeleteByPrefix removes all cache entries whose key starts with prefix.
	DeleteByPrefix(ctx context.Context, prefix string) error

	// Close releases any resources held by the cache backend.
	Close() error
}
