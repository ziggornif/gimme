package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// tokenPrefix is prepended to every opaque token so that it is immediately
// recognisable as a Gimme API key (similar to GitHub's "ghp_" prefix).
const tokenPrefix = "gim_"

// tokenRawBytes is the number of random bytes used to build the token body.
// 32 random bytes → 64 hex chars → total token length 68 chars (prefix + hex).
// Hex encoding is used instead of base62 to avoid modulo bias.
const tokenRawBytes = 32

// AuthManager manages opaque API token lifecycle.
// It no longer relies on JWT: tokens are random strings whose SHA-256 hash is
// stored in the backing TokenStore (Redis). Authentication consists of hashing
// the presented token and looking it up in the store.
type AuthManager struct {
	store TokenStore
}

// NewAuthManager creates an auth manager instance.
func NewAuthManager(store TokenStore) *AuthManager {
	return &AuthManager{
		store: store,
	}
}

// generateOpaqueToken returns a cryptographically random opaque token of the
// form "gim_<hex>" (68 chars total: 4-char prefix + 64 hex chars).
// Hex encoding is used to avoid modulo bias that would arise from base62.
// It also returns the SHA-256 hex hash that must be stored instead of the token.
func generateOpaqueToken() (rawToken, tokenHash string, err error) {
	buf := make([]byte, tokenRawBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	rawToken = tokenPrefix + hex.EncodeToString(buf)
	tokenHash = hashToken(rawToken)
	return rawToken, tokenHash, nil
}

// hashToken returns the SHA-256 hex digest of the raw opaque token.
// This is what is stored in the backing store — never the raw token itself.
func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// CreateToken generates an opaque API token, persists its hash in the store and
// returns the entry along with the raw token (returned once, never stored again).
func (am *AuthManager) CreateToken(ctx context.Context, name string, expirationDate string) (*TokenEntry, string, *errors.GimmeError) {
	var expiresAt time.Time

	if len(expirationDate) > 0 {
		format := "2006-01-02"
		end, parseErr := time.Parse(format, expirationDate)
		if parseErr != nil {
			logrus.Errorf("[AuthManager] CreateToken - Invalid expiration date format: %s", expirationDate)
			return nil, "", errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid expiration date format, expected YYYY-MM-DD"))
		}
		if time.Until(end) <= 0 {
			logrus.Error("[AuthManager] CreateToken - Expiration date must be greater than the current date")
			return nil, "", errors.NewBusinessError(errors.BadRequest, fmt.Errorf("expiration date must be greater than the current date"))
		}
		expiresAt = end
	} else {
		// Default expiry: 90 days — a sensible default for an API key
		// (much longer than the previous 15-minute default which was designed for JWTs).
		expiresAt = time.Now().Add(90 * 24 * time.Hour)
	}

	rawToken, tokenHash, genErr := generateOpaqueToken()
	if genErr != nil {
		logrus.Errorf("[AuthManager] CreateToken - Error generating token: %v", genErr)
		return nil, "", errors.NewBusinessError(errors.InternalError, fmt.Errorf("error generating token"))
	}

	entry := &TokenEntry{
		ID:        uuid.New().String(),
		Name:      name,
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if saveErr := am.store.Save(ctx, entry); saveErr != nil {
		logrus.Errorf("[AuthManager] CreateToken - Error while saving token: %v", saveErr)
		return nil, "", errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while saving token"))
	}

	// Return rawToken separately — it will not be stored anywhere and must be
	// shown to the user exactly once.
	return entry, rawToken, nil
}

// ListTokens returns all stored token entries (newest first).
func (am *AuthManager) ListTokens(ctx context.Context) []*TokenEntry {
	return am.store.List(ctx)
}

// RevokeToken marks the token with the given ID as revoked.
// Returns false if the ID does not exist.
func (am *AuthManager) RevokeToken(ctx context.Context, id string) bool {
	return am.store.Revoke(ctx, id)
}

// extractToken extracts the raw Bearer token from the Authorization header.
func (am *AuthManager) extractToken(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

// AuthenticateMiddleware validates an opaque Bearer token.
// It computes the SHA-256 hash of the presented token, looks it up in the store,
// then checks that the entry is neither revoked nor expired.
func (am *AuthManager) AuthenticateMiddleware(c *gin.Context) {
	rawToken := am.extractToken(c.GetHeader("Authorization"))
	if rawToken == "" {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return
	}

	hash := hashToken(rawToken)
	entry, ok := am.store.GetByHash(c.Request.Context(), hash)
	if !ok {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return
	}

	if !entry.IsValid() {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return
	}

	c.Next()
}
