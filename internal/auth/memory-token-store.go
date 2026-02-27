package auth

import (
	"slices"
	"sync"
)

// MemoryTokenStore is a thread-safe in-memory implementation of TokenStore.
// It is suitable for single-instance deployments and development.
// Tokens are lost on process restart; use a persistent backend for production.
type MemoryTokenStore struct {
	mu      sync.RWMutex
	byID    map[string]*TokenEntry
	byToken map[string]*TokenEntry
}

// NewMemoryTokenStore creates a new MemoryTokenStore.
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		byID:    make(map[string]*TokenEntry),
		byToken: make(map[string]*TokenEntry),
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
