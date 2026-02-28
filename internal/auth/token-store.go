package auth

import (
	"context"
	"time"
)

// TokenEntry holds metadata for an issued opaque API token.
// The raw token is never persisted — only its SHA-256 hash is stored.
type TokenEntry struct {
	// ID is the unique identifier for this token (UUID v4).
	ID string
	// Name is the human-readable label provided at creation time.
	Name string
	// TokenHash is the SHA-256 hex digest of the raw opaque token.
	// The raw token is returned once at creation and never stored.
	TokenHash string
	// CreatedAt is when the token was issued.
	CreatedAt time.Time
	// ExpiresAt is the token expiration time (zero value means no explicit expiry).
	ExpiresAt time.Time
	// RevokedAt is set when the token is explicitly revoked via DELETE /tokens/:id.
	// A non-zero RevokedAt means the token is invalid even if ExpiresAt is in the future.
	RevokedAt time.Time
}

// IsRevoked reports whether the token has been explicitly revoked.
func (e *TokenEntry) IsRevoked() bool {
	return !e.RevokedAt.IsZero()
}

// IsExpired reports whether the token's expiry time has passed.
func (e *TokenEntry) IsExpired() bool {
	return !e.ExpiresAt.IsZero() && e.ExpiresAt.Before(time.Now())
}

// IsValid reports whether the token is neither revoked nor expired.
func (e *TokenEntry) IsValid() bool {
	return !e.IsRevoked() && !e.IsExpired()
}

// TokenStore manages the lifecycle of issued opaque tokens.
// Implementations must be safe for concurrent use.
// All methods accept a context.Context so that HTTP request cancellation and
// deadlines are propagated to the underlying storage backend.
type TokenStore interface {
	// Save persists a newly issued token entry.
	Save(ctx context.Context, entry *TokenEntry) error

	// GetByHash returns the entry for a given SHA-256 hex hash of the raw token.
	// Returns nil, false if not found.
	GetByHash(ctx context.Context, hash string) (*TokenEntry, bool)

	// List returns all stored token entries ordered by creation time (newest first).
	List(ctx context.Context) []*TokenEntry

	// Revoke marks the token entry with the given ID as revoked by setting RevokedAt.
	// Returns false if the ID does not exist.
	Revoke(ctx context.Context, id string) bool

	// Delete removes the token entry with the given ID permanently.
	// Returns false if the ID does not exist.
	Delete(ctx context.Context, id string) bool

	// Close releases any resources held by the store (e.g. background goroutines,
	// network connections). It is safe to call Close multiple times.
	Close()
}
