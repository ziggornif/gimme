package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// basicAuthHeader returns a valid Basic Auth header value for user:password.
func basicAuthHeader(user, password string) string {
	creds := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
	return "Basic " + creds
}

// TestBasicAuthProvider_LoginMiddleware_ValidCredentials checks that a request
// with correct credentials passes through to the handler.
func TestBasicAuthProvider_LoginMiddleware_ValidCredentials(t *testing.T) {
	p := NewBasicAuthProvider("admin", "secret")
	require.NotNil(t, p)

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", basicAuthHeader("admin", "secret"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBasicAuthProvider_LoginMiddleware_InvalidCredentials checks that a request
// with wrong credentials is rejected with 401.
func TestBasicAuthProvider_LoginMiddleware_InvalidCredentials(t *testing.T) {
	p := NewBasicAuthProvider("admin", "secret")

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", basicAuthHeader("admin", "wrongpassword"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestBasicAuthProvider_LoginMiddleware_MissingCredentials checks that a request
// without any Authorization header is rejected with 401.
func TestBasicAuthProvider_LoginMiddleware_MissingCredentials(t *testing.T) {
	p := NewBasicAuthProvider("admin", "secret")

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestBasicAuthProvider_RegisterRoutes checks that RegisterRoutes is a no-op
// (does not panic and does not add extra routes).
func TestBasicAuthProvider_RegisterRoutes(t *testing.T) {
	p := NewBasicAuthProvider("admin", "secret")
	router := gin.New()

	assert.NotPanics(t, func() {
		p.RegisterRoutes(router)
	})

	// No extra routes should have been added.
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
