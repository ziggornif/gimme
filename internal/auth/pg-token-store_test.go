package auth

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestPGStore creates a pgxmock pool and a PGTokenStore wired to it.
// The mock pool is configured to expect the CREATE TABLE statement.
func newTestPGStore(t *testing.T) (*PGTokenStore, pgxmock.PgxPoolIface) {
	t.Helper()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	// Expect the CREATE TABLE statement from NewPGTokenStoreWithPool.
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS gimme_tokens").
		WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))

	store, storeErr := NewPGTokenStore(mock)
	require.NoError(t, storeErr)
	t.Cleanup(store.Close)

	return store, mock
}

// makePGEntry builds a minimal TokenEntry with a 15-minute expiry.
func makePGEntry(id, name, hash string) *TokenEntry {
	return &TokenEntry{
		ID:        id,
		Name:      name,
		TokenHash: hash,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
}

func TestPGTokenStore_Save_AndGetByHash(t *testing.T) {
	store, mock := newTestPGStore(t)
	entry := makePGEntry("id-1", "test", "hash-abc")

	mock.ExpectExec("INSERT INTO gimme_tokens").
		WithArgs(entry.ID, entry.Name, entry.TokenHash, entry.CreatedAt, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	require.NoError(t, store.Save(context.Background(), entry))

	mock.ExpectQuery("SELECT .+ FROM gimme_tokens WHERE token_hash").
		WithArgs("hash-abc").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "token_hash", "created_at", "expires_at", "revoked_at"}).
			AddRow(entry.ID, entry.Name, entry.TokenHash, entry.CreatedAt, &entry.ExpiresAt, nil))

	got, ok := store.GetByHash(context.Background(), "hash-abc")
	assert.True(t, ok)
	require.NotNil(t, got)
	assert.Equal(t, "id-1", got.ID)
	assert.Equal(t, "test", got.Name)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_GetByHash_NotFound(t *testing.T) {
	store, mock := newTestPGStore(t)

	mock.ExpectQuery("SELECT .+ FROM gimme_tokens WHERE token_hash").
		WithArgs("nonexistent").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "token_hash", "created_at", "expires_at", "revoked_at"}))

	got, ok := store.GetByHash(context.Background(), "nonexistent")
	assert.False(t, ok)
	assert.Nil(t, got)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_List(t *testing.T) {
	store, mock := newTestPGStore(t)

	now := time.Now().UTC()
	expires := now.Add(15 * time.Minute)

	mock.ExpectQuery("SELECT .+ FROM gimme_tokens ORDER BY created_at DESC").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "token_hash", "created_at", "expires_at", "revoked_at"}).
			AddRow("id-3", "third", "hash-3", now, &expires, nil).
			AddRow("id-2", "second", "hash-2", now.Add(-time.Minute), &expires, nil).
			AddRow("id-1", "first", "hash-1", now.Add(-2*time.Minute), &expires, nil))

	list := store.List(context.Background())
	assert.Len(t, list, 3)
	assert.Equal(t, "id-3", list[0].ID)
	assert.Equal(t, "id-2", list[1].ID)
	assert.Equal(t, "id-1", list[2].ID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_List_Empty(t *testing.T) {
	store, mock := newTestPGStore(t)

	mock.ExpectQuery("SELECT .+ FROM gimme_tokens ORDER BY created_at DESC").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "token_hash", "created_at", "expires_at", "revoked_at"}))

	list := store.List(context.Background())
	assert.Empty(t, list)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_Revoke(t *testing.T) {
	store, mock := newTestPGStore(t)

	mock.ExpectExec("UPDATE gimme_tokens SET revoked_at").
		WithArgs(pgxmock.AnyArg(), "id-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	assert.True(t, store.Revoke(context.Background(), "id-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_Revoke_NotFound(t *testing.T) {
	store, mock := newTestPGStore(t)

	mock.ExpectExec("UPDATE gimme_tokens SET revoked_at").
		WithArgs(pgxmock.AnyArg(), "nonexistent").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	assert.False(t, store.Revoke(context.Background(), "nonexistent"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_Delete(t *testing.T) {
	store, mock := newTestPGStore(t)

	mock.ExpectExec("DELETE FROM gimme_tokens WHERE id").
		WithArgs("id-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	assert.True(t, store.Delete(context.Background(), "id-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_Delete_NotFound(t *testing.T) {
	store, mock := newTestPGStore(t)

	mock.ExpectExec("DELETE FROM gimme_tokens WHERE id").
		WithArgs("nonexistent").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	assert.False(t, store.Delete(context.Background(), "nonexistent"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPGTokenStore_Close_Idempotent(t *testing.T) {
	store, mock := newTestPGStore(t)

	assert.NotPanics(t, func() {
		store.Close()
		store.Close()
	})

	_ = mock
}
