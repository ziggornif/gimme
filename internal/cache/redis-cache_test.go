package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gimme-cdn/gimme/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRedisCache spins up an in-process miniredis server and returns a
// redisCache wired to it. The server is stopped automatically when the test
// finishes.
func newTestRedisCache(t *testing.T) (CacheManager, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, _ := persistence.NewRedisClient("redis://" + mr.Addr())
	t.Cleanup(func() { client.CloseConnection() })
	return NewRedisCache(client), mr
}

// TestRedisCache_NewRedisCacheWithClient checks that the constructor wiring a
// pre-existing client works and the cache is usable immediately.
func TestRedisCache_NewRedisCacheWithClient(t *testing.T) {
	cache, _ := newTestRedisCache(t)
	require.NotNil(t, cache)

	// Simple round-trip to confirm the cache is functional.
	entry := &CacheEntry{ObjectPath: "pkg@1.0.0/file.js"}
	require.NoError(t, cache.Set(context.Background(), "key1", entry, time.Minute))

	got, ok := cache.Get(context.Background(), "key1")
	assert.True(t, ok)
	assert.Equal(t, "pkg@1.0.0/file.js", got.ObjectPath)
}

// TestRedisCache_Get_Miss checks that a cache miss returns nil, false.
func TestRedisCache_Get_Miss(t *testing.T) {
	cache, _ := newTestRedisCache(t)

	got, ok := cache.Get(context.Background(), "missing-key")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// TestRedisCache_Get_Hit checks that a stored entry is returned correctly.
func TestRedisCache_Get_Hit(t *testing.T) {
	cache, _ := newTestRedisCache(t)
	entry := &CacheEntry{ObjectPath: "lib@2.1.3/lib.min.js"}
	require.NoError(t, cache.Set(context.Background(), "lib-key", entry, time.Minute))

	got, ok := cache.Get(context.Background(), "lib-key")
	assert.True(t, ok)
	require.NotNil(t, got)
	assert.Equal(t, "lib@2.1.3/lib.min.js", got.ObjectPath)
}

// TestRedisCache_Get_AfterExpiry checks that an expired entry is no longer returned.
func TestRedisCache_Get_AfterExpiry(t *testing.T) {
	cache, mr := newTestRedisCache(t)
	entry := &CacheEntry{ObjectPath: "pkg@1.0.0/file.js"}
	require.NoError(t, cache.Set(context.Background(), "ttl-key", entry, time.Second))

	// Fast-forward past the TTL.
	mr.FastForward(2 * time.Second)

	got, ok := cache.Get(context.Background(), "ttl-key")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// TestRedisCache_Set_OverwritesExisting checks that Set updates an existing key.
func TestRedisCache_Set_OverwritesExisting(t *testing.T) {
	cache, _ := newTestRedisCache(t)

	require.NoError(t, cache.Set(context.Background(), "k", &CacheEntry{ObjectPath: "v1"}, time.Minute))
	require.NoError(t, cache.Set(context.Background(), "k", &CacheEntry{ObjectPath: "v2"}, time.Minute))

	got, ok := cache.Get(context.Background(), "k")
	assert.True(t, ok)
	assert.Equal(t, "v2", got.ObjectPath)
}

// TestRedisCache_Delete checks that Delete removes an existing key.
func TestRedisCache_Delete(t *testing.T) {
	cache, _ := newTestRedisCache(t)
	require.NoError(t, cache.Set(context.Background(), "del-key", &CacheEntry{ObjectPath: "v"}, time.Minute))

	require.NoError(t, cache.Delete(context.Background(), "del-key"))

	got, ok := cache.Get(context.Background(), "del-key")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// TestRedisCache_Delete_MissingKey checks that Delete on a non-existent key
// does not return an error.
func TestRedisCache_Delete_MissingKey(t *testing.T) {
	cache, _ := newTestRedisCache(t)
	assert.NoError(t, cache.Delete(context.Background(), "ghost-key"))
}

// TestRedisCache_DeleteByPrefix checks that DeleteByPrefix removes all matching
// keys and leaves the others intact.
func TestRedisCache_DeleteByPrefix(t *testing.T) {
	cache, _ := newTestRedisCache(t)

	require.NoError(t, cache.Set(context.Background(), "pkg@1/a.js", &CacheEntry{ObjectPath: "a"}, time.Minute))
	require.NoError(t, cache.Set(context.Background(), "pkg@1/b.js", &CacheEntry{ObjectPath: "b"}, time.Minute))
	require.NoError(t, cache.Set(context.Background(), "other@2/c.js", &CacheEntry{ObjectPath: "c"}, time.Minute))

	require.NoError(t, cache.DeleteByPrefix(context.Background(), "pkg@1"))

	_, ok1 := cache.Get(context.Background(), "pkg@1/a.js")
	_, ok2 := cache.Get(context.Background(), "pkg@1/b.js")
	_, ok3 := cache.Get(context.Background(), "other@2/c.js")

	assert.False(t, ok1, "pkg@1/a.js should have been deleted")
	assert.False(t, ok2, "pkg@1/b.js should have been deleted")
	assert.True(t, ok3, "other@2/c.js should still exist")
}

// TestRedisCache_DeleteByPrefix_NoMatch checks that DeleteByPrefix with no
// matching keys is a no-op (no error).
func TestRedisCache_DeleteByPrefix_NoMatch(t *testing.T) {
	cache, _ := newTestRedisCache(t)
	assert.NoError(t, cache.DeleteByPrefix(context.Background(), "nonexistent-prefix"))
}

// TestRedisCache_Close checks that Close succeeds without panicking.
func TestRedisCache_Close(t *testing.T) {
	cache, _ := newTestRedisCache(t)
	assert.NoError(t, cache.Close())
}
