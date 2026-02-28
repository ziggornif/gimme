package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func makeEntry(id, name, hash string) *TokenEntry {
	return &TokenEntry{
		ID:        id,
		Name:      name,
		TokenHash: hash,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute * 15),
	}
}

func TestMemoryTokenStore_SaveAndGetByHash(t *testing.T) {
	store := NewMemoryTokenStore()
	entry := makeEntry("id-1", "test", "sha256-abc")

	err := store.Save(ctx, entry)
	assert.Nil(t, err)

	got, ok := store.GetByHash(ctx, "sha256-abc")
	assert.True(t, ok)
	assert.Equal(t, "id-1", got.ID)
}

func TestMemoryTokenStore_GetByHash_NotFound(t *testing.T) {
	store := NewMemoryTokenStore()

	got, ok := store.GetByHash(ctx, "nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestMemoryTokenStore_List_Order(t *testing.T) {
	store := NewMemoryTokenStore()

	e1 := makeEntry("id-1", "first", "hash-1")
	e1.CreatedAt = time.Now().Add(-2 * time.Minute)
	e2 := makeEntry("id-2", "second", "hash-2")
	e2.CreatedAt = time.Now().Add(-1 * time.Minute)
	e3 := makeEntry("id-3", "third", "hash-3")
	e3.CreatedAt = time.Now()

	_ = store.Save(ctx, e1)
	_ = store.Save(ctx, e2)
	_ = store.Save(ctx, e3)

	list := store.List(ctx)
	assert.Equal(t, 3, len(list))
	// newest first
	assert.Equal(t, "id-3", list[0].ID)
	assert.Equal(t, "id-2", list[1].ID)
	assert.Equal(t, "id-1", list[2].ID)
}

func TestMemoryTokenStore_Revoke(t *testing.T) {
	store := NewMemoryTokenStore()
	entry := makeEntry("id-1", "test", "hash-abc")
	_ = store.Save(ctx, entry)

	revoked := store.Revoke(ctx, "id-1")
	assert.True(t, revoked)

	got, ok := store.GetByHash(ctx, "hash-abc")
	assert.True(t, ok) // still present but marked revoked
	assert.True(t, got.IsRevoked())
}

func TestMemoryTokenStore_Revoke_NotFound(t *testing.T) {
	store := NewMemoryTokenStore()

	revoked := store.Revoke(ctx, "nonexistent")
	assert.False(t, revoked)
}

func TestMemoryTokenStore_Delete(t *testing.T) {
	store := NewMemoryTokenStore()
	entry := makeEntry("id-1", "test", "hash-abc")
	_ = store.Save(ctx, entry)

	deleted := store.Delete(ctx, "id-1")
	assert.True(t, deleted)

	_, ok := store.GetByHash(ctx, "hash-abc")
	assert.False(t, ok)

	assert.Empty(t, store.List(ctx))
}

func TestMemoryTokenStore_Delete_NotFound(t *testing.T) {
	store := NewMemoryTokenStore()

	deleted := store.Delete(ctx, "nonexistent")
	assert.False(t, deleted)
}

func TestMemoryTokenStore_List_Empty(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()
	assert.Empty(t, store.List(ctx))
}

func TestMemoryTokenStore_PurgeExpired(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	// Already-expired token
	expired := &TokenEntry{
		ID:        "exp-1",
		Name:      "expired",
		TokenHash: "hash-expired",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute), // in the past
	}
	// Still-valid token
	valid := makeEntry("valid-1", "valid", "hash-valid")

	_ = store.Save(ctx, expired)
	_ = store.Save(ctx, valid)

	// Trigger purge directly (no need to wait for the ticker)
	store.purgeExpired()

	// Expired token must be gone
	_, ok := store.GetByHash(ctx, "hash-expired")
	assert.False(t, ok)

	// Valid token must still be present
	_, ok = store.GetByHash(ctx, "hash-valid")
	assert.True(t, ok)
}

func TestMemoryTokenStore_PurgeExpired_ZeroExpiresAt(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	// Token with zero ExpiresAt must never be purged
	noExpiry := &TokenEntry{
		ID:        "no-expiry",
		Name:      "permanent",
		TokenHash: "hash-permanent",
		CreatedAt: time.Now(),
		ExpiresAt: time.Time{}, // zero value = no expiry
	}
	_ = store.Save(ctx, noExpiry)

	store.purgeExpired()

	_, ok := store.GetByHash(ctx, "hash-permanent")
	assert.True(t, ok)
}

func TestMemoryTokenStore_Close_Idempotent(t *testing.T) {
	store := NewMemoryTokenStore()
	// Calling Close multiple times must not panic.
	assert.NotPanics(t, func() {
		store.Close()
		store.Close()
	})
}

func TestTokenEntry_IsRevoked(t *testing.T) {
	entry := makeEntry("id-1", "test", "hash-abc")
	assert.False(t, entry.IsRevoked())

	store := NewMemoryTokenStore()
	_ = store.Save(ctx, entry)
	store.Revoke(ctx, "id-1") //nolint:errcheck
	got, _ := store.GetByHash(ctx, "hash-abc")
	assert.True(t, got.IsRevoked())
}

func TestTokenEntry_IsExpired(t *testing.T) {
	entry := makeEntry("id-1", "test", "hash-abc")
	entry.ExpiresAt = time.Now().Add(-time.Minute)
	assert.True(t, entry.IsExpired())

	entry2 := makeEntry("id-2", "test", "hash-def")
	entry2.ExpiresAt = time.Now().Add(time.Minute)
	assert.False(t, entry2.IsExpired())
}

func TestTokenEntry_IsValid(t *testing.T) {
	entry := makeEntry("id-1", "test", "hash-abc")
	assert.True(t, entry.IsValid())

	// Expired
	entry.ExpiresAt = time.Now().Add(-time.Minute)
	assert.False(t, entry.IsValid())

	// Not expired but revoked
	entry2 := makeEntry("id-2", "test", "hash-def")
	entry2.RevokedAt = time.Now()
	assert.False(t, entry2.IsValid())
}
