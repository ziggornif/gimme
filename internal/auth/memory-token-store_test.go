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
	assert.Empty(t, store.List())
}
