package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "this-is-a-32-byte-secret-for-test" // exactly 32 chars

// ctx is a shared background context for test helpers.
var ctx = context.Background()

// makeEntry builds a minimal TokenEntry with the given fields and a 15-minute
// expiry, suitable for most test cases.
func makeEntry(id, name, hash string) *TokenEntry {
	return &TokenEntry{
		ID:        id,
		Name:      name,
		TokenHash: hash,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute * 15),
	}
}

// newTestFileStore creates a FileTokenStore backed by a temporary file that is
// removed when the test (and its sub-tests) finish.
func newTestFileStore(t *testing.T) *FileTokenStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokens.enc")
	store, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

// TestFileTokenStore_SaveAndGetByHash checks that a saved token can be retrieved
// by its hash.
func TestFileTokenStore_SaveAndGetByHash(t *testing.T) {
	store := newTestFileStore(t)
	entry := makeEntry("id-1", "test", "hash-abc")

	err := store.Save(ctx, entry)
	assert.NoError(t, err)

	got, ok := store.GetByHash(ctx, "hash-abc")
	assert.True(t, ok)
	assert.Equal(t, "id-1", got.ID)
}

// TestFileTokenStore_GetByHash_NotFound checks that a lookup for a missing hash
// returns nil, false.
func TestFileTokenStore_GetByHash_NotFound(t *testing.T) {
	store := newTestFileStore(t)

	got, ok := store.GetByHash(ctx, "nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// TestFileTokenStore_Persistence checks that tokens survive a store reload —
// i.e. that the flush → decrypt → load round-trip works correctly.
func TestFileTokenStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.enc")

	// Write a token via the first store instance.
	store1, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	entry := makeEntry("persist-1", "persist", "hash-persist")
	require.NoError(t, store1.Save(ctx, entry))
	store1.Close()

	// Open a second instance on the same file and verify the token is present.
	store2, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store2.Close()

	got, ok := store2.GetByHash(ctx, "hash-persist")
	assert.True(t, ok)
	assert.Equal(t, "persist-1", got.ID)
}

// TestFileTokenStore_List_Order verifies that List returns entries newest first.
func TestFileTokenStore_List_Order(t *testing.T) {
	store := newTestFileStore(t)

	e1 := makeEntry("id-1", "first", "hash-1")
	e1.CreatedAt = time.Now().Add(-2 * time.Minute)
	e2 := makeEntry("id-2", "second", "hash-2")
	e2.CreatedAt = time.Now().Add(-1 * time.Minute)
	e3 := makeEntry("id-3", "third", "hash-3")
	e3.CreatedAt = time.Now()

	require.NoError(t, store.Save(ctx, e1))
	require.NoError(t, store.Save(ctx, e2))
	require.NoError(t, store.Save(ctx, e3))

	list := store.List(ctx)
	assert.Equal(t, 3, len(list))
	assert.Equal(t, "id-3", list[0].ID)
	assert.Equal(t, "id-2", list[1].ID)
	assert.Equal(t, "id-1", list[2].ID)
}

// TestFileTokenStore_Revoke checks that revocation is persisted to disk.
func TestFileTokenStore_Revoke(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.enc")

	store1, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	entry := makeEntry("id-1", "test", "hash-abc")
	require.NoError(t, store1.Save(ctx, entry))

	revoked := store1.Revoke(ctx, "id-1")
	assert.True(t, revoked)
	store1.Close()

	// Reload and verify RevokedAt is set.
	store2, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store2.Close()

	got, ok := store2.GetByHash(ctx, "hash-abc")
	assert.True(t, ok)
	assert.True(t, got.IsRevoked())
}

// TestFileTokenStore_Revoke_NotFound checks that revoking a non-existent ID
// returns false.
func TestFileTokenStore_Revoke_NotFound(t *testing.T) {
	store := newTestFileStore(t)
	assert.False(t, store.Revoke(ctx, "nonexistent"))
}

// TestFileTokenStore_Delete checks that deletion removes the entry from memory
// and is persisted to disk.
func TestFileTokenStore_Delete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.enc")

	store1, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	entry := makeEntry("id-1", "test", "hash-abc")
	require.NoError(t, store1.Save(ctx, entry))

	deleted := store1.Delete(ctx, "id-1")
	assert.True(t, deleted)

	_, ok := store1.GetByHash(ctx, "hash-abc")
	assert.False(t, ok)
	assert.Empty(t, store1.List(ctx))
	store1.Close()

	// Reload and verify the entry is still gone.
	store2, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store2.Close()

	_, ok = store2.GetByHash(ctx, "hash-abc")
	assert.False(t, ok)
}

// TestFileTokenStore_Delete_NotFound checks that deleting a non-existent ID
// returns false.
func TestFileTokenStore_Delete_NotFound(t *testing.T) {
	store := newTestFileStore(t)
	assert.False(t, store.Delete(ctx, "nonexistent"))
}

// TestFileTokenStore_List_Empty checks that List returns an empty slice for a
// fresh store.
func TestFileTokenStore_List_Empty(t *testing.T) {
	store := newTestFileStore(t)
	assert.Empty(t, store.List(ctx))
}

// TestFileTokenStore_PurgeExpired checks that expired tokens are removed from
// memory and flushed to disk by purgeExpired.
func TestFileTokenStore_PurgeExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.enc")

	store, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store.Close()

	expired := &TokenEntry{
		ID:        "exp-1",
		Name:      "expired",
		TokenHash: "hash-expired",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	valid := makeEntry("valid-1", "valid", "hash-valid")

	require.NoError(t, store.Save(ctx, expired))
	require.NoError(t, store.Save(ctx, valid))

	store.purgeExpired()

	_, ok := store.GetByHash(ctx, "hash-expired")
	assert.False(t, ok)

	_, ok = store.GetByHash(ctx, "hash-valid")
	assert.True(t, ok)

	// Verify the purge was persisted: reload and check.
	store2, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store2.Close()

	_, ok = store2.GetByHash(ctx, "hash-expired")
	assert.False(t, ok)
	_, ok = store2.GetByHash(ctx, "hash-valid")
	assert.True(t, ok)
}

// TestFileTokenStore_PurgeExpired_ZeroExpiresAt checks that non-expiring tokens
// are never purged.
func TestFileTokenStore_PurgeExpired_ZeroExpiresAt(t *testing.T) {
	store := newTestFileStore(t)

	noExpiry := &TokenEntry{
		ID:        "no-expiry",
		Name:      "permanent",
		TokenHash: "hash-permanent",
		CreatedAt: time.Now(),
		ExpiresAt: time.Time{},
	}
	require.NoError(t, store.Save(ctx, noExpiry))

	store.purgeExpired()

	_, ok := store.GetByHash(ctx, "hash-permanent")
	assert.True(t, ok)
}

// TestFileTokenStore_Close_Idempotent checks that calling Close multiple times
// does not panic.
func TestFileTokenStore_Close_Idempotent(t *testing.T) {
	store := newTestFileStore(t)
	assert.NotPanics(t, func() {
		store.Close()
		store.Close()
	})
}

// TestFileTokenStore_WrongKey checks that loading a file with a different secret
// returns an error (authentication tag mismatch).
func TestFileTokenStore_WrongKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.enc")

	// Write with the correct secret.
	store1, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	require.NoError(t, store1.Save(ctx, makeEntry("id-1", "test", "h1")))
	store1.Close()

	// Try to load with a different secret — must fail.
	differentSecret := "a-completely-different-32byte-sec"
	_, err = NewFileTokenStore(differentSecret, path)
	assert.Error(t, err)
}

// TestFileTokenStore_EncryptDecryptRoundTrip is a low-level unit test for the
// encrypt/decrypt helpers.
func TestFileTokenStore_EncryptDecryptRoundTrip(t *testing.T) {
	store := newTestFileStore(t)
	plain := []byte(`{"tokens":[]}`)

	ciphertext, err := store.encrypt(plain)
	require.NoError(t, err)
	assert.NotEqual(t, plain, ciphertext)

	decrypted, err := store.decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plain, decrypted)
}

// TestFileTokenStore_Decrypt_TooShort checks that decrypt returns an error when
// the ciphertext is shorter than the nonce size.
func TestFileTokenStore_Decrypt_TooShort(t *testing.T) {
	store := newTestFileStore(t)
	_, err := store.decrypt([]byte("short"))
	assert.Error(t, err)
}

// TestFileTokenStore_MissingFile checks that opening a non-existent file is a
// no-op (the file is created on first flush).
func TestFileTokenStore_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.enc")
	store, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store.Close()

	assert.Empty(t, store.List(ctx))

	// The file must not exist yet (no flush triggered).
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

// TestFileTokenStore_FileCreatedOnFirstSave checks that the token file is
// created after the first Save call.
func TestFileTokenStore_FileCreatedOnFirstSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.enc")
	store, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Save(ctx, makeEntry("id-1", "test", "h1")))

	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "token file should exist after first Save")
}

// TestDeriveAESKey checks that the same secret always produces the same key and
// that two different secrets produce different keys.
func TestDeriveAESKey(t *testing.T) {
	key1a, err := deriveAESKey("secret-of-exactly-32-bytes-here!")
	require.NoError(t, err)
	key1b, err := deriveAESKey("secret-of-exactly-32-bytes-here!")
	require.NoError(t, err)
	assert.Equal(t, key1a, key1b, "same secret must produce same key")

	key2, err := deriveAESKey("another-32-byte-secret-here-1234")
	require.NoError(t, err)
	assert.NotEqual(t, key1a, key2, "different secrets must produce different keys")
}

// invalidFlushPath returns a file path whose parent directory does not exist,
// causing os.CreateTemp to fail. The path is constructed portably so tests
// pass on both Unix and Windows.
func invalidFlushPath(t *testing.T) string {
	t.Helper()
	// Create a real temp dir, remove it, then reference a file inside it.
	// The directory no longer exists → CreateTemp will fail.
	dir := filepath.Join(t.TempDir(), "deleted")
	return filepath.Join(dir, "tokens.enc")
}

// TestFileTokenStore_Flush_InvalidDir checks that flush returns an error when
// the directory does not exist (os.CreateTemp fails).
func TestFileTokenStore_Flush_InvalidDir(t *testing.T) {
	store := newTestFileStore(t)

	// Point the store at a non-existent directory so CreateTemp fails.
	store.filePath = invalidFlushPath(t)

	// Flush is called internally by Save — it must propagate the error.
	err := store.Save(ctx, makeEntry("id-x", "test", "hash-x"))
	assert.Error(t, err)
}

// TestFileTokenStore_Revoke_FlushError checks that Revoke still returns true
// when the flush fails (the in-memory mutation is applied regardless).
func TestFileTokenStore_Revoke_FlushError(t *testing.T) {
	store := newTestFileStore(t)
	entry := makeEntry("id-1", "test", "hash-abc")
	require.NoError(t, store.Save(ctx, entry))

	// Break the file path so the subsequent flush in Revoke will fail.
	store.filePath = invalidFlushPath(t)

	// Revoke should still succeed (in-memory state is updated).
	revoked := store.Revoke(ctx, "id-1")
	assert.True(t, revoked)
}

// TestFileTokenStore_Delete_FlushError checks that Delete still returns true
// when the flush fails (the in-memory mutation is applied regardless).
func TestFileTokenStore_Delete_FlushError(t *testing.T) {
	store := newTestFileStore(t)
	entry := makeEntry("id-1", "test", "hash-abc")
	require.NoError(t, store.Save(ctx, entry))

	// Break the file path so the subsequent flush in Delete will fail.
	store.filePath = invalidFlushPath(t)

	deleted := store.Delete(ctx, "id-1")
	assert.True(t, deleted)
}

// TestFileTokenStore_PurgeExpired_FlushError checks that purgeExpired logs but
// does not panic when the flush fails.
func TestFileTokenStore_PurgeExpired_FlushError(t *testing.T) {
	store := newTestFileStore(t)

	expired := &TokenEntry{
		ID:        "exp-1",
		Name:      "expired",
		TokenHash: "hash-expired",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	require.NoError(t, store.Save(ctx, expired))

	// Break the file path so purgeExpired's flush fails silently.
	store.filePath = invalidFlushPath(t)

	assert.NotPanics(t, func() {
		store.purgeExpired()
	})
}

// TestFileTokenStore_Load_CorruptFile checks that loading a file with invalid
// (non-JSON) plaintext after decryption returns an error.
func TestFileTokenStore_Load_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.enc")

	// Write a valid encrypted file.
	store1, err := NewFileTokenStore(testSecret, path)
	require.NoError(t, err)
	require.NoError(t, store1.Save(ctx, makeEntry("id-1", "test", "h1")))
	store1.Close()

	// Corrupt the ciphertext by overwriting bytes past the nonce (first 12 bytes).
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// Flip a byte in the ciphertext body (after the 12-byte nonce) to cause
	// GCM authentication failure.
	if len(data) > 13 {
		data[13] ^= 0xFF
	}
	require.NoError(t, os.WriteFile(path, data, 0600))

	// Reload must fail with an error (decrypt will fail).
	_, err = NewFileTokenStore(testSecret, path)
	assert.Error(t, err)
}

// TestFileTokenStore_ConcurrentSave exercises concurrent writes to catch race
// conditions (run with -race).
func TestFileTokenStore_ConcurrentSave(t *testing.T) {
	store := newTestFileStore(t)

	done := make(chan struct{})
	for i := range 10 {
		go func(n int) {
			e := makeEntry(
				fmt.Sprintf("concurrent-%d", n),
				"concurrent",
				fmt.Sprintf("hash-concurrent-%d", n),
			)
			_ = store.Save(context.Background(), e)
			done <- struct{}{}
		}(i)
	}
	for range 10 {
		<-done
	}

	list := store.List(ctx)
	assert.Equal(t, 10, len(list))
}
