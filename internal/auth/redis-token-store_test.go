package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRedisStore spins up an in-process miniredis server and returns a
// RedisTokenStore wired to it. The server is stopped automatically when the
// test finishes.
func newTestRedisStore(t *testing.T) (*RedisTokenStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return NewRedisTokenStore(client), mr
}

// makeRedisEntry builds a minimal TokenEntry with a 15-minute expiry.
func makeRedisEntry(id, name, hash string) *TokenEntry {
	return &TokenEntry{
		ID:        id,
		Name:      name,
		TokenHash: hash,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
}

// TestRedisTokenStore_Close_Noop checks that Close() is a no-op: the Redis
// client is owned by the application layer and closed there, not here.
func TestRedisTokenStore_Close_Noop(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:0"})
	store := NewRedisTokenStore(client)

	assert.NotPanics(t, func() {
		store.Close()
		store.Close()
	})

	// The client must still be open — Close() must not have touched it.
	assert.NoError(t, client.Close())
}

// TestRedisTokenStore_toFromRedisEntry verifies the conversion helpers are
// inverse operations.
func TestRedisTokenStore_toFromRedisEntry(t *testing.T) {
	original := makeRedisEntry("id-1", "test", "hash-abc")
	original.RevokedAt = time.Now().UTC()

	re := toRedisEntry(original)
	back := fromRedisEntry(re)

	assert.Equal(t, original.ID, back.ID)
	assert.Equal(t, original.Name, back.Name)
	assert.Equal(t, original.TokenHash, back.TokenHash)
	assert.Equal(t, original.ExpiresAt.Unix(), back.ExpiresAt.Unix())
	assert.Equal(t, original.RevokedAt.Unix(), back.RevokedAt.Unix())
}

// TestRedisTokenStore_Save_AndGetByHash checks the basic round-trip.
func TestRedisTokenStore_Save_AndGetByHash(t *testing.T) {
	store, _ := newTestRedisStore(t)
	entry := makeRedisEntry("id-1", "test", "hash-abc")

	require.NoError(t, store.Save(context.Background(), entry))

	got, ok := store.GetByHash(context.Background(), "hash-abc")
	assert.True(t, ok)
	require.NotNil(t, got)
	assert.Equal(t, "id-1", got.ID)
	assert.Equal(t, "test", got.Name)
}

// TestRedisTokenStore_Save_AlreadyExpired checks that saving an already-expired
// token returns an error.
func TestRedisTokenStore_Save_AlreadyExpired(t *testing.T) {
	store, _ := newTestRedisStore(t)
	entry := &TokenEntry{
		ID:        "expired-id",
		Name:      "expired",
		TokenHash: "hash-expired",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute),
	}

	err := store.Save(context.Background(), entry)
	assert.Error(t, err)
}

// TestRedisTokenStore_Save_NoExpiry checks that a token with zero ExpiresAt is
// stored without a TTL (permanent).
func TestRedisTokenStore_Save_NoExpiry(t *testing.T) {
	store, _ := newTestRedisStore(t)
	entry := &TokenEntry{
		ID:        "perm-id",
		Name:      "permanent",
		TokenHash: "hash-perm",
		CreatedAt: time.Now(),
		ExpiresAt: time.Time{}, // zero = no expiry
	}

	require.NoError(t, store.Save(context.Background(), entry))

	got, ok := store.GetByHash(context.Background(), "hash-perm")
	assert.True(t, ok)
	assert.Equal(t, "perm-id", got.ID)
}

// TestRedisTokenStore_GetByHash_NotFound checks that a missing hash returns
// nil, false.
func TestRedisTokenStore_GetByHash_NotFound(t *testing.T) {
	store, _ := newTestRedisStore(t)

	got, ok := store.GetByHash(context.Background(), "nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// TestRedisTokenStore_GetByHash_StaleIndex checks that when the token key has
// expired but the hash index remains, GetByHash cleans up the stale entry and
// returns nil, false.
func TestRedisTokenStore_GetByHash_StaleIndex(t *testing.T) {
	store, mr := newTestRedisStore(t)
	entry := makeRedisEntry("stale-id", "stale", "hash-stale")

	require.NoError(t, store.Save(context.Background(), entry))

	// Fast-forward miniredis time so the token key TTL expires.
	mr.FastForward(20 * time.Minute)

	got, ok := store.GetByHash(context.Background(), "hash-stale")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// TestRedisTokenStore_List checks that List returns all stored entries sorted
// newest-first.
func TestRedisTokenStore_List(t *testing.T) {
	store, _ := newTestRedisStore(t)

	e1 := makeRedisEntry("id-1", "first", "hash-1")
	e1.CreatedAt = time.Now().Add(-2 * time.Minute)
	e2 := makeRedisEntry("id-2", "second", "hash-2")
	e2.CreatedAt = time.Now().Add(-time.Minute)
	e3 := makeRedisEntry("id-3", "third", "hash-3")
	e3.CreatedAt = time.Now()

	require.NoError(t, store.Save(context.Background(), e1))
	require.NoError(t, store.Save(context.Background(), e2))
	require.NoError(t, store.Save(context.Background(), e3))

	list := store.List(context.Background())
	assert.Len(t, list, 3)
	assert.Equal(t, "id-3", list[0].ID)
	assert.Equal(t, "id-2", list[1].ID)
	assert.Equal(t, "id-1", list[2].ID)
}

// TestRedisTokenStore_List_Empty checks that List on an empty store returns nil
// (not an error, not a non-nil empty slice).
func TestRedisTokenStore_List_Empty(t *testing.T) {
	store, _ := newTestRedisStore(t)
	list := store.List(context.Background())
	assert.Empty(t, list)
}

// TestRedisTokenStore_List_PrunesStaleEntries checks that List removes index
// entries whose underlying token key has expired.
func TestRedisTokenStore_List_PrunesStaleEntries(t *testing.T) {
	store, mr := newTestRedisStore(t)
	entry := makeRedisEntry("stale-id", "stale", "hash-stale")

	require.NoError(t, store.Save(context.Background(), entry))

	// Expire the token.
	mr.FastForward(20 * time.Minute)

	list := store.List(context.Background())
	assert.Empty(t, list)
}

// TestRedisTokenStore_Revoke checks that revoking a token sets RevokedAt.
func TestRedisTokenStore_Revoke(t *testing.T) {
	store, _ := newTestRedisStore(t)
	entry := makeRedisEntry("id-1", "test", "hash-abc")
	require.NoError(t, store.Save(context.Background(), entry))

	revoked := store.Revoke(context.Background(), "id-1")
	assert.True(t, revoked)

	got, ok := store.GetByHash(context.Background(), "hash-abc")
	require.True(t, ok)
	assert.True(t, got.IsRevoked())
}

// TestRedisTokenStore_Revoke_NotFound checks that revoking a non-existent ID
// returns false.
func TestRedisTokenStore_Revoke_NotFound(t *testing.T) {
	store, _ := newTestRedisStore(t)
	assert.False(t, store.Revoke(context.Background(), "nonexistent"))
}

// TestRedisTokenStore_Revoke_ExpiredKey checks that revoking an entry whose key
// has expired returns false.
func TestRedisTokenStore_Revoke_ExpiredKey(t *testing.T) {
	store, mr := newTestRedisStore(t)
	entry := makeRedisEntry("id-1", "test", "hash-abc")
	require.NoError(t, store.Save(context.Background(), entry))

	// Expire the token.
	mr.FastForward(20 * time.Minute)

	assert.False(t, store.Revoke(context.Background(), "id-1"))
}

// TestRedisTokenStore_Delete checks that deleting a token removes it from the
// store completely.
func TestRedisTokenStore_Delete(t *testing.T) {
	store, _ := newTestRedisStore(t)
	entry := makeRedisEntry("id-1", "test", "hash-abc")
	require.NoError(t, store.Save(context.Background(), entry))

	deleted := store.Delete(context.Background(), "id-1")
	assert.True(t, deleted)

	got, ok := store.GetByHash(context.Background(), "hash-abc")
	assert.False(t, ok)
	assert.Nil(t, got)

	assert.Empty(t, store.List(context.Background()))
}

// TestRedisTokenStore_Delete_NotFound checks that deleting a non-existent ID
// returns false.
func TestRedisTokenStore_Delete_NotFound(t *testing.T) {
	store, _ := newTestRedisStore(t)
	assert.False(t, store.Delete(context.Background(), "nonexistent"))
}

// TestRedisTokenStore_keyHelpers checks that key() and hashKey() produce the
// expected prefixes.
func TestRedisTokenStore_keyHelpers(t *testing.T) {
	store, _ := newTestRedisStore(t)
	assert.Equal(t, "token:abc", store.key("abc"))
	assert.Equal(t, "token:hash:xyz", store.hashKey("xyz"))
}
