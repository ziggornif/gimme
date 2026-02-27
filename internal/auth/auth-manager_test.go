package auth

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/gimme-cdn/gimme/test/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

var jwtRegex = `^[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*$`

func TestCreateTokenError(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())

	_, err := authManager.CreateToken("test", "2022-02-17")

	assert.Equal(t, "expiration date must be greater than the current date", err.Error())
}

func TestCreateTokenInvalidFormat(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())

	_, err := authManager.CreateToken("test", "17/02/2022")

	assert.Equal(t, "invalid expiration date format, expected YYYY-MM-DD", err.Error())
}

func TestCreateTokenDefault(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())

	entry, err := authManager.CreateToken("test", "")

	assert.Regexp(t, regexp.MustCompile(jwtRegex), entry.Token)
	assert.Nil(t, err)
}

func TestCreateTokenCustomExp(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	dt := time.Now().Add(time.Hour * 24)
	entry, err := authManager.CreateToken("test", dt.Format("2006-01-02"))

	assert.Regexp(t, regexp.MustCompile(jwtRegex), entry.Token)
	assert.Nil(t, err)
}

func TestAuthManager_AuthenticateMiddlewareErr(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareInvalid(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "invalid"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareInvalid2(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "Bearer invalid"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareExpired(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2NDU5ODcwNTIsImp0aSI6InppZyJ9.q9NbUVV6egGlZBLMbvRBO_-VnWy_edDT4VNU6g8GIxQ"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareOK(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	entry, _ := authManager.CreateToken("test", "")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", entry.Token)})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareNoExp(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())

	// Build a valid token signed with the right secret but without an exp claim
	claims := jwt.MapClaims{"jti": "test"}
	rawToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := rawToken.SignedString([]byte("secret"))

	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", signed)})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareRevokedToken(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	entry, _ := authManager.CreateToken("test", "")

	// Revoke the token before using it
	revoked := authManager.RevokeToken(entry.ID)
	assert.True(t, revoked)

	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", entry.Token)})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_RevokeToken_NotFound(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())

	revoked := authManager.RevokeToken("nonexistent-id")
	assert.False(t, revoked)
}

func TestAuthManager_ListTokens(t *testing.T) {
	authManager := NewAuthManager("secret", NewMemoryTokenStore())
	_, _ = authManager.CreateToken("token-a", "")
	_, _ = authManager.CreateToken("token-b", "")

	tokens := authManager.ListTokens()
	assert.Equal(t, 2, len(tokens))
}

func TestAuthManager_CreateToken_StoredEntry(t *testing.T) {
	store := NewMemoryTokenStore()
	authManager := NewAuthManager("secret", store)

	entry, err := authManager.CreateToken("mykey", "")
	assert.Nil(t, err)
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "mykey", entry.Name)
	assert.NotEmpty(t, entry.Token)

	// Verify it's retrievable from the store
	stored, ok := store.GetByToken(entry.Token)
	assert.True(t, ok)
	assert.Equal(t, entry.ID, stored.ID)
}
