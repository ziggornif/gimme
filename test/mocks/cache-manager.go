package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/gimme-cdn/gimme/internal/cache"
)

// MockCacheManager is an in-memory CacheManager for unit tests.
type MockCacheManager struct {
	mu      sync.RWMutex
	entries map[string]*cache.CacheEntry

	// Counters for asserting behaviour
	GetCalls            int
	SetCalls            int
	DeleteCalls         int
	DeleteByPrefixCalls int
}

func NewMockCacheManager() *MockCacheManager {
	return &MockCacheManager{
		entries: make(map[string]*cache.CacheEntry),
	}
}

func (m *MockCacheManager) Get(_ context.Context, key string) (*cache.CacheEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCalls++
	entry, ok := m.entries[key]
	return entry, ok
}

func (m *MockCacheManager) Set(_ context.Context, key string, entry *cache.CacheEntry, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SetCalls++
	m.entries[key] = entry
	return nil
}

func (m *MockCacheManager) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteCalls++
	delete(m.entries, key)
	return nil
}

func (m *MockCacheManager) DeleteByPrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteByPrefixCalls++
	for k := range m.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.entries, k)
		}
	}
	return nil
}

func (m *MockCacheManager) Close() error {
	return nil
}

// Seed pre-populates the mock cache with an entry.
func (m *MockCacheManager) Seed(key string, entry *cache.CacheEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = entry
}
