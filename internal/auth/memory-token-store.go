package auth

import (
	"slices"
	"sync"
	"time"
)

// defaultPurgeInterval is how often the background goroutine sweeps the store
// to remove expired tokens. Kept short enough to bound memory growth but long
// enough to avoid lock contention under normal load.
const defaultPurgeInterval = 5 * time.Minute

// MemoryTokenStore is a thread-safe in-memory implementation of TokenStore.
// It is suitable for single-instance deployments and development.
// Tokens are lost on process restart; use a persistent backend for production.
//
// A background goroutine purges expired tokens every defaultPurgeInterval to
// prevent unbounded memory growth in long-running processes. Call Close() to
// stop the goroutine when the store is no longer needed.
type MemoryTokenStore struct {
	mu      sync.RWMutex
	byID    map[string]*TokenEntry
	byToken map[string]*TokenEntry
	stopCh  chan struct{}
}

// NewMemoryTokenStore creates a new MemoryTokenStore and starts a background
// goroutine that periodically removes expired tokens.
func NewMemoryTokenStore() *MemoryTokenStore {
	s := &MemoryTokenStore{
		byID:    make(map[string]*TokenEntry),
		byToken: make(map[string]*TokenEntry),
		stopCh:  make(chan struct{}),
	}
	go s.purgeLoop(defaultPurgeInterval)
	return s
}

// purgeLoop runs until Close() is called, sweeping expired entries at each tick.
func (s *MemoryTokenStore) purgeLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.purgeExpired()
		case <-s.stopCh:
			return
		}
	}
}

// purgeExpired removes all entries whose ExpiresAt is in the past.
// Tokens with a zero ExpiresAt are considered non-expiring and are kept.
func (s *MemoryTokenStore) purgeExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, entry := range s.byID {
		if !entry.ExpiresAt.IsZero() && entry.ExpiresAt.Before(now) {
			delete(s.byToken, entry.Token)
			delete(s.byID, id)
		}
	}
}

// Close stops the background purge goroutine. It is safe to call Close
// multiple times; subsequent calls are no-ops.
func (s *MemoryTokenStore) Close() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
}

// Save persists a newly issued token entry.
func (s *MemoryTokenStore) Save(entry *TokenEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[entry.ID] = entry
	s.byToken[entry.Token] = entry
	return nil
}

// GetByToken returns the entry for a given raw JWT string.
func (s *MemoryTokenStore) GetByToken(token string) (*TokenEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.byToken[token]
	return e, ok
}

// List returns all stored token entries ordered by creation time (newest first).
func (s *MemoryTokenStore) List() []*TokenEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]*TokenEntry, 0, len(s.byID))
	for _, e := range s.byID {
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b *TokenEntry) int {
		return b.CreatedAt.Compare(a.CreatedAt) // newest first
	})
	return entries
}

// Delete removes the token entry with the given ID.
func (s *MemoryTokenStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.byID[id]
	if !ok {
		return false
	}
	delete(s.byID, id)
	delete(s.byToken, entry.Token)
	return true
}
