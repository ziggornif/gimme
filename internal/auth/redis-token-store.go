package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	// tokenKeyPrefix is the Redis key prefix for all token JSON values.
	// Full key format: "token:<uuid>"
	tokenKeyPrefix = "token:"
	// tokenIndexKey is a Redis Set that holds all known token IDs for List/GetByHash.
	tokenIndexKey = "token:__index__"
	// tokenHashPrefix is the Redis key prefix for the reverse hash index.
	// Full key format: "token:hash:<sha256hex>" → "<uuid>"
	// Allows O(1) lookup in GetByHash instead of scanning all tokens.
	tokenHashPrefix = "token:hash:"
)

// redisTokenEntry is the serialisable form of TokenEntry stored in Redis as JSON.
// Kept separate from TokenEntry to decouple the wire format from the domain model.
type redisTokenEntry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"token_hash"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	RevokedAt time.Time `json:"revoked_at"`
}

func toRedisEntry(e *TokenEntry) *redisTokenEntry {
	return &redisTokenEntry{
		ID:        e.ID,
		Name:      e.Name,
		TokenHash: e.TokenHash,
		CreatedAt: e.CreatedAt,
		ExpiresAt: e.ExpiresAt,
		RevokedAt: e.RevokedAt,
	}
}

func fromRedisEntry(r *redisTokenEntry) *TokenEntry {
	return &TokenEntry{
		ID:        r.ID,
		Name:      r.Name,
		TokenHash: r.TokenHash,
		CreatedAt: r.CreatedAt,
		ExpiresAt: r.ExpiresAt,
		RevokedAt: r.RevokedAt,
	}
}

// RedisTokenStore is a persistent, Redis-backed implementation of TokenStore.
// Tokens are stored as JSON values under keys of the form "token:<uuid>".
// A secondary Redis Set ("token:__index__") holds all known IDs for List/GetByHash.
// The TTL of each key is aligned to the token's ExpiresAt so Redis performs
// automatic eviction — no background goroutine is required.
type RedisTokenStore struct {
	client *redis.Client
}

// NewRedisTokenStore creates a RedisTokenStore connected to the given Redis URL.
// It pings Redis on startup and returns an error if the connection fails.
// Use NewRedisTokenStoreWithClient to share an existing *redis.Client.
func NewRedisTokenStore(redisURL string) (*RedisTokenStore, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis-token-store: invalid URL: %w", err)
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis-token-store: cannot reach Redis at %q: %w", opt.Addr, err)
	}

	logrus.Infof("[RedisTokenStore] connected to Redis at %s", opt.Addr)
	return &RedisTokenStore{client: client}, nil
}

// NewRedisTokenStoreWithClient creates a RedisTokenStore using an already-connected
// *redis.Client. The caller is responsible for the client lifecycle (ping, close).
// Use this to share a single Redis connection across multiple components.
func NewRedisTokenStoreWithClient(client *redis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

func (s *RedisTokenStore) key(id string) string {
	return tokenKeyPrefix + id
}

func (s *RedisTokenStore) hashKey(hash string) string {
	return tokenHashPrefix + hash
}

// Save persists a newly issued token entry in Redis.
// The key TTL is set to the token's lifetime so Redis evicts it automatically.
func (s *RedisTokenStore) Save(ctx context.Context, entry *TokenEntry) error {
	data, err := json.Marshal(toRedisEntry(entry))
	if err != nil {
		return fmt.Errorf("redis-token-store: failed to marshal entry: %w", err)
	}

	var ttl time.Duration
	if !entry.ExpiresAt.IsZero() {
		ttl = time.Until(entry.ExpiresAt)
		if ttl <= 0 {
			return fmt.Errorf("redis-token-store: token is already expired")
		}
	}

	pipe := s.client.Pipeline()
	if ttl > 0 {
		pipe.Set(ctx, s.key(entry.ID), data, ttl)
		// Reverse hash index: token:hash:<sha256hex> → <uuid>, same TTL so it is
		// evicted automatically alongside the token entry.
		pipe.Set(ctx, s.hashKey(entry.TokenHash), entry.ID, ttl)
	} else {
		pipe.Set(ctx, s.key(entry.ID), data, 0)
		pipe.Set(ctx, s.hashKey(entry.TokenHash), entry.ID, 0)
	}
	// Add to the secondary index so List() can enumerate all tokens.
	pipe.SAdd(ctx, tokenIndexKey, entry.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis-token-store: failed to save entry: %w", err)
	}

	return nil
}

// GetByHash returns the entry whose TokenHash matches the given SHA-256 hex digest.
// Uses a reverse hash index (token:hash:<sha256hex> → uuid) for an O(1) lookup
// in 2 Redis round-trips: one GET on the hash key, one GET on the token key.
// Returns nil, false if not found or if the matching entry has expired/been evicted.
func (s *RedisTokenStore) GetByHash(ctx context.Context, hash string) (*TokenEntry, bool) {
	id, err := s.client.Get(ctx, s.hashKey(hash)).Result()
	if err != nil {
		if err != redis.Nil {
			logrus.Errorf("[RedisTokenStore] GetByHash - failed to lookup hash index: %v", err)
		}
		return nil, false
	}

	return s.getByID(ctx, id)
}

// List returns all stored (non-expired) token entries ordered by creation time (newest first).
func (s *RedisTokenStore) List(ctx context.Context) []*TokenEntry {
	ids, err := s.client.SMembers(ctx, tokenIndexKey).Result()
	if err != nil {
		logrus.Errorf("[RedisTokenStore] List - failed to list index: %v", err)
		return nil
	}

	entries := make([]*TokenEntry, 0, len(ids))
	for _, id := range ids {
		entry, ok := s.getByID(ctx, id)
		if !ok {
			// Key expired in Redis — clean up the stale index entry.
			if sremErr := s.client.SRem(ctx, tokenIndexKey, id).Err(); sremErr != nil {
				logrus.Warnf("[RedisTokenStore] List - failed to remove stale index entry %q: %v", id, sremErr)
			}
			continue
		}
		entries = append(entries, entry)
	}

	slices.SortFunc(entries, func(a, b *TokenEntry) int {
		return b.CreatedAt.Compare(a.CreatedAt) // newest first
	})

	return entries
}

// Revoke marks the token entry with the given ID as revoked by setting RevokedAt.
// The entry remains in Redis (so revoked tokens can still be listed) until it expires.
// Returns false if the ID does not exist.
func (s *RedisTokenStore) Revoke(ctx context.Context, id string) bool {
	entry, ok := s.getByID(ctx, id)
	if !ok {
		return false
	}

	entry.RevokedAt = time.Now().UTC()

	data, err := json.Marshal(toRedisEntry(entry))
	if err != nil {
		logrus.Errorf("[RedisTokenStore] Revoke - failed to marshal entry: %v", err)
		return false
	}

	// Preserve the existing TTL to avoid extending (or losing) the token's expiry.
	ttl, err := s.client.TTL(ctx, s.key(id)).Result()
	if err != nil {
		logrus.Errorf("[RedisTokenStore] Revoke - failed to get TTL for %q: %v", id, err)
		return false
	}
	// Guard against the key expiring between getByID and TTL — Redis returns -2 for
	// non-existent keys. Setting a key with a negative TTL would make it persist
	// forever, so we treat this as a no-op (token already expired).
	if ttl < 0 {
		logrus.Warnf("[RedisTokenStore] Revoke - key %q expired before revocation could be written", id)
		return false
	}

	if err := s.client.Set(ctx, s.key(id), data, ttl).Err(); err != nil {
		logrus.Errorf("[RedisTokenStore] Revoke - failed to update entry: %v", err)
		return false
	}

	return true
}

// Delete removes the token entry with the given ID permanently from Redis.
// Fetches the entry first to obtain its hash so the reverse index can also be
// cleaned up. Uses a pipeline to batch Del + SRem + hash key deletion.
// Returns false if the ID does not exist.
func (s *RedisTokenStore) Delete(ctx context.Context, id string) bool {
	entry, ok := s.getByID(ctx, id)
	if !ok {
		return false
	}

	pipe := s.client.Pipeline()
	delCmd := pipe.Del(ctx, s.key(id))
	pipe.Del(ctx, s.hashKey(entry.TokenHash))
	pipe.SRem(ctx, tokenIndexKey, id)

	if _, err := pipe.Exec(ctx); err != nil {
		logrus.Errorf("[RedisTokenStore] Delete - pipeline error for %q: %v", id, err)
		return false
	}

	deleted, err := delCmd.Result()
	if err != nil {
		logrus.Errorf("[RedisTokenStore] Delete - failed to read Del result for %q: %v", id, err)
		return false
	}
	return deleted > 0
}

// Close closes the underlying Redis client connection.
func (s *RedisTokenStore) Close() {
	if err := s.client.Close(); err != nil {
		logrus.Warnf("[RedisTokenStore] Close - error closing client: %v", err)
	}
}

// getByID fetches and deserialises a single token entry by ID.
// Returns nil, false if the key does not exist (e.g. expired in Redis).
func (s *RedisTokenStore) getByID(ctx context.Context, id string) (*TokenEntry, bool) {
	data, err := s.client.Get(ctx, s.key(id)).Bytes()
	if err != nil {
		if err != redis.Nil {
			logrus.Errorf("[RedisTokenStore] getByID - error fetching key %q: %v", id, err)
		}
		return nil, false
	}

	var r redisTokenEntry
	if err := json.Unmarshal(data, &r); err != nil {
		logrus.Errorf("[RedisTokenStore] getByID - failed to unmarshal entry for key %q: %v", id, err)
		return nil, false
	}

	return fromRedisEntry(&r), true
}
