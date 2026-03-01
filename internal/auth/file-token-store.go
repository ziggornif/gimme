package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/hkdf"
)

// hkdfInfo is the domain-separation label used when deriving the AES key from
// the master secret. Changing this value would invalidate existing token files.
const hkdfInfo = "gimme-file-token-store-v1"

// defaultPurgeInterval is how often the background goroutine sweeps the store
// to remove expired tokens.
const defaultPurgeInterval = 5 * time.Minute

// fileTokenStoreData is the plaintext JSON structure stored (encrypted) in the
// token file. Kept separate from the in-memory maps to allow the wire format to
// evolve independently.
type fileTokenStoreData struct {
	Tokens []*TokenEntry `json:"tokens"`
}

// FileTokenStore is a persistent, file-backed implementation of TokenStore.
// It keeps an in-memory map for fast lookups and flushes the full token list to
// an AES-256-GCM encrypted JSON file on every mutation.
// The encryption key is derived from the application secret using HKDF-SHA256 so
// that no additional configuration is required.
//
// On startup the file is decrypted and loaded into memory; on every mutation the
// entire in-memory state is re-encrypted and written atomically (write to a temp
// file then rename) to avoid partial writes.
//
// A background goroutine purges expired tokens every defaultPurgeInterval and
// flushes the cleaned state to disk.
//
// FileTokenStore is safe for concurrent use.
// It is intended for single-node, zero-dependency deployments. Use RedisTokenStore
// for multi-node or production environments that require shared token state.
type FileTokenStore struct {
	mu       sync.RWMutex
	byID     map[string]*TokenEntry
	byHash   map[string]*TokenEntry
	stopCh   chan struct{}
	filePath string
	key      []byte // 32-byte AES-256 key derived from the master secret
}

// NewFileTokenStore creates a FileTokenStore that persists tokens to filePath.
// masterSecret is the application secret (config.Secret); the AES key is derived
// from it via HKDF-SHA256 so no extra configuration is needed.
// If filePath already exists it is decrypted and loaded; a missing file is created
// on the first flush.
func NewFileTokenStore(masterSecret, filePath string) (*FileTokenStore, error) {
	key, err := deriveAESKey(masterSecret)
	if err != nil {
		return nil, fmt.Errorf("file-token-store: failed to derive key: %w", err)
	}

	s := &FileTokenStore{
		byID:     make(map[string]*TokenEntry),
		byHash:   make(map[string]*TokenEntry),
		stopCh:   make(chan struct{}),
		filePath: filePath,
		key:      key,
	}

	if err := s.load(); err != nil {
		return nil, fmt.Errorf("file-token-store: failed to load %q: %w", filePath, err)
	}

	go s.purgeLoop(defaultPurgeInterval)

	logrus.Infof("[FileTokenStore] loaded %d token(s) from %q", len(s.byID), filePath)
	return s, nil
}

// deriveAESKey derives a 32-byte AES-256 key from masterSecret using HKDF-SHA256.
// The salt is nil (RFC 5869 §2.2: equivalent to a zero-filled salt of HashLen bytes).
// This is intentional: the derivation is deterministic so that the same master
// secret always produces the same AES key, allowing the token file to be decrypted
// after a process restart without storing any additional state.
// The domain-separation label (hkdfInfo) prevents key reuse with other HKDF-derived
// secrets in the application even if the salt is zero.
func deriveAESKey(masterSecret string) ([]byte, error) {
	h := hkdf.New(sha256.New, []byte(masterSecret), nil, []byte(hkdfInfo))
	key := make([]byte, 32)
	if _, err := io.ReadFull(h, key); err != nil {
		return nil, err
	}
	return key, nil
}

// load reads and decrypts the token file into the in-memory maps.
// If the file does not exist it is a no-op (fresh start).
func (s *FileTokenStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh start — file will be created on first flush
		}
		return err
	}

	plain, err := s.decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	var payload fileTokenStoreData
	if err := json.Unmarshal(plain, &payload); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	for _, e := range payload.Tokens {
		cp := *e
		s.byID[e.ID] = &cp
		s.byHash[e.TokenHash] = &cp
	}
	return nil
}

// flush serialises the current in-memory state, encrypts it and writes it to
// the token file atomically (temp file + rename).
// The caller must hold s.mu (read or write lock is sufficient since we only read
// the maps here, but in practice callers hold the write lock after a mutation).
func (s *FileTokenStore) flush() error {
	entries := make([]*TokenEntry, 0, len(s.byID))
	for _, e := range s.byID {
		cp := *e
		entries = append(entries, &cp)
	}

	plain, err := json.Marshal(fileTokenStoreData{Tokens: entries})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	ciphertext, err := s.encrypt(plain)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Write to a temp file in the same directory then rename for atomicity.
	// os.CreateTemp creates the file with mode 0600 (Go 1.16+), restricting
	// access to the owning user only. The mode is preserved after os.Rename.
	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), ".gimme-tokens-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(ciphertext); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	// Sync before close to guarantee data durability: without Sync the OS may
	// buffer the write and lose it on a power failure even after Rename succeeds.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.filePath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Wire format: nonce (12 bytes) || ciphertext+tag.
func (s *FileTokenStore) encrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// decrypt decrypts data produced by encrypt.
func (s *FileTokenStore) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, data[:ns], data[ns:], nil)
}

// purgeLoop runs until Close() is called, sweeping expired entries at each tick.
func (s *FileTokenStore) purgeLoop(interval time.Duration) {
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

// purgeExpired removes expired entries from memory and flushes the updated state
// to disk. Only entries with a non-zero ExpiresAt in the past are removed.
func (s *FileTokenStore) purgeExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := false
	for id, entry := range s.byID {
		if !entry.ExpiresAt.IsZero() && entry.ExpiresAt.Before(now) {
			delete(s.byHash, entry.TokenHash)
			delete(s.byID, id)
			removed = true
		}
	}

	if removed {
		if err := s.flush(); err != nil {
			logrus.Errorf("[FileTokenStore] purgeExpired - flush error: %v", err)
		}
	}
}

// Close stops the background purge goroutine.
// It is safe to call Close multiple times; subsequent calls are no-ops.
func (s *FileTokenStore) Close() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
}

// Save persists a newly issued token entry to memory and flushes to disk.
func (s *FileTokenStore) Save(_ context.Context, entry *TokenEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *entry
	s.byID[entry.ID] = &cp
	s.byHash[entry.TokenHash] = &cp

	if err := s.flush(); err != nil {
		return fmt.Errorf("file-token-store: Save flush: %w", err)
	}
	return nil
}

// GetByHash returns a copy of the entry for a given SHA-256 hex hash.
func (s *FileTokenStore) GetByHash(_ context.Context, hash string) (*TokenEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.byHash[hash]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// List returns all stored token entries ordered by creation time (newest first).
func (s *FileTokenStore) List(_ context.Context) []*TokenEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*TokenEntry, 0, len(s.byID))
	for _, e := range s.byID {
		cp := *e
		entries = append(entries, &cp)
	}
	slices.SortFunc(entries, func(a, b *TokenEntry) int {
		return b.CreatedAt.Compare(a.CreatedAt) // newest first
	})
	return entries
}

// Revoke marks the token entry with the given ID as revoked and flushes to disk.
// The in-memory mutation is applied first; if the subsequent flush fails the
// change is visible in memory for the lifetime of the process but will be lost
// on restart. The flush error is logged but not propagated because the TokenStore
// interface does not allow Revoke to return an error.
func (s *FileTokenStore) Revoke(_ context.Context, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.byID[id]
	if !ok {
		return false
	}
	entry.RevokedAt = time.Now().UTC()
	// Save stores a single *TokenEntry pointer in both byID and byHash, so
	// mutating via byID automatically updates the byHash view as well.

	if err := s.flush(); err != nil {
		logrus.Errorf("[FileTokenStore] Revoke - flush error: %v", err)
	}
	return true
}

// Delete removes the token entry with the given ID permanently and flushes to disk.
// See Revoke for the flush-failure semantics.
func (s *FileTokenStore) Delete(_ context.Context, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.byID[id]
	if !ok {
		return false
	}
	delete(s.byID, id)
	delete(s.byHash, entry.TokenHash)

	if err := s.flush(); err != nil {
		logrus.Errorf("[FileTokenStore] Delete - flush error: %v", err)
	}
	return true
}
