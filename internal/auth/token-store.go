package auth

import "time"

// TokenEntry holds metadata for an issued API token.
type TokenEntry struct {
	// ID is the unique identifier for this token (UUID v4).
	ID string
	// Name is the human-readable label provided at creation time.
	Name string
	// Token is the signed JWT string.
	Token string
	// CreatedAt is when the token was issued.
	CreatedAt time.Time
	// ExpiresAt is the token expiration time (zero value means no explicit expiry).
	ExpiresAt time.Time
}

// TokenStore manages the lifecycle of issued tokens.
// Implementations must be safe for concurrent use.
type TokenStore interface {
	// Save persists a newly issued token entry.
	Save(entry *TokenEntry) error

	// GetByToken returns the entry for a given raw JWT string.
	// Returns nil, false if not found.
	GetByToken(token string) (*TokenEntry, bool)

	// List returns all stored token entries ordered by creation time (newest first).
	List() []*TokenEntry

	// Delete removes the token entry with the given ID.
	// Returns false if the ID does not exist.
	Delete(id string) bool
}
