package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeEntry(id, name, token string) *TokenEntry {
	return &TokenEntry{
		ID:        id,
		Name:      name,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute * 15),
	}
}

func TestMemoryTokenStore_SaveAndGetByToken(t *testing.T) {
	store := NewMemoryTokenStore()
	entry := makeEntry("id-1", "test", "jwt-abc")

	err := store.Save(entry)
	assert.Nil(t, err)

	got, ok := store.GetByToken("jwt-abc")
	assert.True(t, ok)
	assert.Equal(t, entry, got)
}

func TestMemoryTokenStore_GetByToken_NotFound(t *testing.T) {
	store := NewMemoryTokenStore()

	got, ok := store.GetByToken("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestMemoryTokenStore_List_Order(t *testing.T) {
	store := NewMemoryTokenStore()

	e1 := makeEntry("id-1", "first", "jwt-1")
	e1.CreatedAt = time.Now().Add(-2 * time.Minute)
	e2 := makeEntry("id-2", "second", "jwt-2")
	e2.CreatedAt = time.Now().Add(-1 * time.Minute)
	e3 := makeEntry("id-3", "third", "jwt-3")
	e3.CreatedAt = time.Now()

	_ = store.Save(e1)
	_ = store.Save(e2)
	_ = store.Save(e3)

	list := store.List()
	assert.Equal(t, 3, len(list))
	// newest first
	assert.Equal(t, "id-3", list[0].ID)
	assert.Equal(t, "id-2", list[1].ID)
	assert.Equal(t, "id-1", list[2].ID)
}

func TestMemoryTokenStore_Delete(t *testing.T) {
	store := NewMemoryTokenStore()
	entry := makeEntry("id-1", "test", "jwt-abc")
	_ = store.Save(entry)

	deleted := store.Delete("id-1")
	assert.True(t, deleted)

	_, ok := store.GetByToken("jwt-abc")
	assert.False(t, ok)

	assert.Empty(t, store.List())
}

func TestMemoryTokenStore_Delete_NotFound(t *testing.T) {
	store := NewMemoryTokenStore()

	deleted := store.Delete("nonexistent")
	assert.False(t, deleted)
}

func TestMemoryTokenStore_List_Empty(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()
	assert.Empty(t, store.List())
}

func TestMemoryTokenStore_PurgeExpired(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	// Already-expired token
	expired := &TokenEntry{
		ID:        "exp-1",
		Name:      "expired",
		Token:     "jwt-expired",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute), // in the past
	}
	// Still-valid token
	valid := makeEntry("valid-1", "valid", "jwt-valid")

	_ = store.Save(expired)
	_ = store.Save(valid)

	// Trigger purge directly (no need to wait for the ticker)
	store.purgeExpired()

	// Expired token must be gone
	_, ok := store.GetByToken("jwt-expired")
	assert.False(t, ok)

	// Valid token must still be present
	_, ok = store.GetByToken("jwt-valid")
	assert.True(t, ok)
}

func TestMemoryTokenStore_PurgeExpired_ZeroExpiresAt(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	// Token with zero ExpiresAt must never be purged
	noExpiry := &TokenEntry{
		ID:        "no-expiry",
		Name:      "permanent",
		Token:     "jwt-permanent",
		CreatedAt: time.Now(),
		ExpiresAt: time.Time{}, // zero value = no expiry
	}
	_ = store.Save(noExpiry)

	store.purgeExpired()

	_, ok := store.GetByToken("jwt-permanent")
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
