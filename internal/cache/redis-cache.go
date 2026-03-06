package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gimme-cdn/gimme/internal/persistence"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type redisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis-backed CacheManager.
func NewRedisCache(client *persistence.RedisClient) CacheManager {
	return &redisCache{client: client.GetClient()}
}

// Get retrieves a CacheEntry by key. Returns nil, false on miss or error.
func (r *redisCache) Get(ctx context.Context, key string) (*CacheEntry, bool) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			logrus.Errorf("[RedisCache] Get - Error retrieving key %s: %s", key, err)
		}
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		logrus.Errorf("[RedisCache] Get - Error deserializing entry for key %s: %s", key, err)
		return nil, false
	}

	return &entry, true
}

// Set stores a CacheEntry under key with the given TTL.
func (r *redisCache) Set(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("error serializing cache entry: %w", err)
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("error storing cache entry for key %s: %w", key, err)
	}

	logrus.Debugf("[RedisCache] Set - Cached key %s (TTL %s)", key, ttl)
	return nil
}

// Delete removes the cache entry for the given key.
func (r *redisCache) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("error deleting cache entry for key %s: %w", key, err)
	}
	logrus.Debugf("[RedisCache] Delete - Deleted key %s", key)
	return nil
}

// DeleteByPrefix removes all cache entries whose key starts with prefix.
// Uses SCAN to avoid blocking the Redis server (safe for production).
func (r *redisCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	pattern := prefix + "*"
	var cursor uint64
	var deleted int64

	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("error scanning keys with prefix %s: %w", prefix, err)
		}

		if len(keys) > 0 {
			n, err := r.client.Del(ctx, keys...).Result()
			if err != nil {
				return fmt.Errorf("error deleting keys with prefix %s: %w", prefix, err)
			}
			deleted += n
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	logrus.Debugf("[RedisCache] DeleteByPrefix - Deleted %d keys with prefix %s", deleted, prefix)
	return nil
}

// Close closes the underlying Redis connection.
func (r *redisCache) Close() error {
	return r.client.Close()
}
