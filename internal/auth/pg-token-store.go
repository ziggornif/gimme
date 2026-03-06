package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirupsen/logrus"
)

// PGPool is the subset of *pgxpool.Pool methods used by PGTokenStore.
// Extracted as an interface to allow mocking in tests.
type PGPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Ping(ctx context.Context) error
}

// PGTokenStore is a persistent, PostgreSQL-backed implementation of TokenStore.
// Tokens are stored in a "gimme_tokens" table that is auto-created on startup.
// A background goroutine periodically purges expired tokens.
type PGTokenStore struct {
	pool   PGPool
	stopCh chan struct{}
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS gimme_tokens (
    id         TEXT        PRIMARY KEY,
    name       TEXT        NOT NULL,
    token_hash TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_gimme_tokens_hash ON gimme_tokens (token_hash);
`

// NewPGTokenStore creates a PGTokenStore using a PostgreSQL connection.
// Auto-creates the table and starts the purge goroutine.
func NewPGTokenStore(pool PGPool) (*PGTokenStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		return nil, fmt.Errorf("pg-token-store: failed to create table: %w", err)
	}

	store := &PGTokenStore{
		pool:   pool,
		stopCh: make(chan struct{}),
	}

	go store.purgeLoop()

	return store, nil
}

// Save persists a newly issued token entry in PostgreSQL.
func (s *PGTokenStore) Save(ctx context.Context, entry *TokenEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO gimme_tokens (id, name, token_hash, created_at, expires_at, revoked_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		entry.ID,
		entry.Name,
		entry.TokenHash,
		entry.CreatedAt,
		nullableTime(entry.ExpiresAt),
		nullableTime(entry.RevokedAt),
	)
	if err != nil {
		return fmt.Errorf("pg-token-store: failed to save entry: %w", err)
	}
	return nil
}

// GetByHash returns the entry whose TokenHash matches the given SHA-256 hex digest.
// Returns nil, false if not found.
func (s *PGTokenStore) GetByHash(ctx context.Context, hash string) (*TokenEntry, bool) {
	entry, err := s.scanRow(s.pool.QueryRow(ctx,
		`SELECT id, name, token_hash, created_at, expires_at, revoked_at FROM gimme_tokens WHERE token_hash = $1`, hash))
	if err != nil {
		if err != pgx.ErrNoRows {
			logrus.Errorf("[PGTokenStore] GetByHash - query error: %v", err)
		}
		return nil, false
	}
	return entry, true
}

// List returns all stored token entries ordered by creation time (newest first).
func (s *PGTokenStore) List(ctx context.Context) []*TokenEntry {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, token_hash, created_at, expires_at, revoked_at FROM gimme_tokens ORDER BY created_at DESC`)
	if err != nil {
		logrus.Errorf("[PGTokenStore] List - query error: %v", err)
		return nil
	}
	defer rows.Close()

	var entries []*TokenEntry
	for rows.Next() {
		entry, scanErr := s.scanRows(rows)
		if scanErr != nil {
			logrus.Errorf("[PGTokenStore] List - scan error: %v", scanErr)
			return nil
		}
		entries = append(entries, entry)
	}
	return entries
}

// Revoke marks the token entry with the given ID as revoked by setting RevokedAt.
// Returns false if the ID does not exist.
func (s *PGTokenStore) Revoke(ctx context.Context, id string) bool {
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx,
		`UPDATE gimme_tokens SET revoked_at = $1 WHERE id = $2`, now, id)
	if err != nil {
		logrus.Errorf("[PGTokenStore] Revoke - update error for %q: %v", id, err)
		return false
	}
	return tag.RowsAffected() > 0
}

// Delete removes the token entry with the given ID permanently.
// Returns false if the ID does not exist.
func (s *PGTokenStore) Delete(ctx context.Context, id string) bool {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM gimme_tokens WHERE id = $1`, id)
	if err != nil {
		logrus.Errorf("[PGTokenStore] Delete - error for %q: %v", id, err)
		return false
	}
	return tag.RowsAffected() > 0
}

// Close stops the background purge goroutine. It does NOT close the pool
// (owned by the application layer).
func (s *PGTokenStore) Close() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
}

// purgeLoop runs periodically to delete expired tokens.
func (s *PGTokenStore) purgeLoop() {
	ticker := time.NewTicker(defaultPurgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.purgeExpired()
		}
	}
}

func (s *PGTokenStore) purgeExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tag, err := s.pool.Exec(ctx,
		`DELETE FROM gimme_tokens WHERE expires_at IS NOT NULL AND expires_at < NOW()`)
	if err != nil {
		logrus.Errorf("[PGTokenStore] purgeExpired - error: %v", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		logrus.Infof("[PGTokenStore] purgeExpired - removed %d expired token(s)", n)
	}
}

// scanRow scans a single pgx.Row into a TokenEntry.
func (s *PGTokenStore) scanRow(row pgx.Row) (*TokenEntry, error) {
	var (
		entry     TokenEntry
		expiresAt *time.Time
		revokedAt *time.Time
	)
	if err := row.Scan(&entry.ID, &entry.Name, &entry.TokenHash, &entry.CreatedAt, &expiresAt, &revokedAt); err != nil {
		return nil, err
	}
	if expiresAt != nil {
		entry.ExpiresAt = *expiresAt
	}
	if revokedAt != nil {
		entry.RevokedAt = *revokedAt
	}
	return &entry, nil
}

// scanRows scans the current row from pgx.Rows into a TokenEntry.
func (s *PGTokenStore) scanRows(rows pgx.Rows) (*TokenEntry, error) {
	var (
		entry     TokenEntry
		expiresAt *time.Time
		revokedAt *time.Time
	)
	if err := rows.Scan(&entry.ID, &entry.Name, &entry.TokenHash, &entry.CreatedAt, &expiresAt, &revokedAt); err != nil {
		return nil, err
	}
	if expiresAt != nil {
		entry.ExpiresAt = *expiresAt
	}
	if revokedAt != nil {
		entry.RevokedAt = *revokedAt
	}
	return &entry, nil
}

// nullableTime converts a zero time.Time to nil (SQL NULL), otherwise returns a pointer.
func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
