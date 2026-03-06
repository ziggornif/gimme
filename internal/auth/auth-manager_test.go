package auth

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gimme-cdn/gimme/test/utils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opaqueTokenPrefix is the expected prefix for all generated API keys.
const opaqueTokenPrefix = "gim_"

// newTestStore returns a FileTokenStore backed by a temporary directory.
// The store is automatically closed when the test ends.
func newTestStore(t *testing.T) *FileTokenStore {
	t.Helper()
	store, err := NewFileTokenStore(testSecret, filepath.Join(t.TempDir(), "tokens.enc"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCreateTokenError(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))

	_, _, err := authManager.CreateToken(context.Background(), "test", "2022-02-17")

	assert.Equal(t, "expiration date must be greater than the current date", err.Error())
}

func TestCreateTokenInvalidFormat(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))

	_, _, err := authManager.CreateToken(context.Background(), "test", "17/02/2022")

	assert.Equal(t, "invalid expiration date format, expected YYYY-MM-DD", err.Error())
}

func TestCreateTokenDefault(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))

	entry, rawToken, err := authManager.CreateToken(context.Background(), "test", "")

	assert.Nil(t, err)
	assert.NotEmpty(t, entry.ID)
	assert.NotEmpty(t, entry.TokenHash)
	assert.True(t, strings.HasPrefix(rawToken, opaqueTokenPrefix), "token must start with %s", opaqueTokenPrefix)
	// Raw token must not be stored — only its hash
	assert.NotEqual(t, rawToken, entry.TokenHash)
}

func TestCreateTokenCustomExp(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	dt := time.Now().Add(time.Hour * 24)
	entry, rawToken, err := authManager.CreateToken(context.Background(), "test", dt.Format("2006-01-02"))

	assert.Nil(t, err)
	assert.NotEmpty(t, rawToken)
	assert.NotEmpty(t, entry.TokenHash)
	assert.True(t, strings.HasPrefix(rawToken, opaqueTokenPrefix))
}

func TestAuthManager_AuthenticateMiddlewareErr(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareInvalid(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "invalid"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareInvalid2(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "Bearer invalid-unknown-token"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareExpired(t *testing.T) {
	store := newTestStore(t)
	authManager := NewAuthManager(store)

	// Insert a token that is already expired directly in the store.
	expired := &TokenEntry{
		ID:        "exp-1",
		Name:      "expired",
		TokenHash: hashToken("gim_expiredtoken"),
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	_ = store.Save(context.Background(), expired)

	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "Bearer gim_expiredtoken"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareOK(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareRevokedToken(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	entry, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")

	// Revoke the token before using it
	revoked := authManager.RevokeToken(context.Background(), entry.ID)
	assert.True(t, revoked)

	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_RevokeToken_NotFound(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))

	revoked := authManager.RevokeToken(context.Background(), "nonexistent-id")
	assert.False(t, revoked)
}

func TestAuthManager_ListTokens(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	_, _, _ = authManager.CreateToken(context.Background(), "token-a", "")
	_, _, _ = authManager.CreateToken(context.Background(), "token-b", "")

	tokens := authManager.ListTokens(context.Background())
	assert.Equal(t, 2, len(tokens))
}

func TestAuthManager_CreateToken_StoredEntry(t *testing.T) {
	store := newTestStore(t)
	authManager := NewAuthManager(store)

	entry, rawToken, err := authManager.CreateToken(context.Background(), "mykey", "")
	assert.Nil(t, err)
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "mykey", entry.Name)
	assert.NotEmpty(t, entry.TokenHash)

	// Raw token must have the expected prefix
	assert.True(t, strings.HasPrefix(rawToken, opaqueTokenPrefix))

	// The store must contain the entry keyed by hash (not raw token)
	expectedHash := hashToken(rawToken)
	stored, ok := store.GetByHash(context.Background(), expectedHash)
	assert.True(t, ok)
	assert.Equal(t, entry.ID, stored.ID)

	// Hash stored in entry must match the hash of the returned raw token
	assert.Equal(t, expectedHash, entry.TokenHash)
}

func TestAuthManager_CreateToken_DefaultExpiry90Days(t *testing.T) {
	authManager := NewAuthManager(newTestStore(t))
	entry, _, err := authManager.CreateToken(context.Background(), "test", "")
	assert.Nil(t, err)

	// Default expiry should be approximately 90 days from now
	expectedExpiry := time.Now().Add(90 * 24 * time.Hour)
	delta := entry.ExpiresAt.Sub(expectedExpiry)
	if delta < 0 {
		delta = -delta
	}
	assert.Less(t, delta, 5*time.Second, "default expiry should be ~90 days from now")
}
