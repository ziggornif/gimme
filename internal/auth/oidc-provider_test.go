package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSigningSecret = "test-signing-secret"
const testClientID = "gimme-test"

// oidcTestServer spins up an in-process mock OIDC provider.
type oidcTestServer struct {
	srv    *httptest.Server
	priv   *rsa.PrivateKey
	keyID  string
	issuer string
}

func newOIDCTestServer(t *testing.T) *oidcTestServer {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	ts := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{
			{PublicKey: priv.Public(), KeyID: "key-1", Algorithm: oidc.RS256},
		},
		Algorithms: []string{oidc.RS256},
	}

	srv := httptest.NewServer(ts)
	t.Cleanup(srv.Close)
	ts.SetIssuer(srv.URL)

	return &oidcTestServer{srv: srv, priv: priv, keyID: "key-1", issuer: srv.URL}
}

// newTestOIDCProvider creates an OIDCProvider wired to the mock OIDC server.
// secureCookies is false so tests run without HTTPS.
func newTestOIDCProvider(t *testing.T, ts *oidcTestServer) *OIDCProvider {
	t.Helper()
	p, err := NewOIDCProvider(
		context.Background(),
		ts.issuer,
		testClientID,
		"client-secret",
		"http://gimme.test/auth/callback",
		testSigningSecret,
		false, // secureCookies=false for local test environment
	)
	require.NoError(t, err)
	return p
}

// --- LoginMiddleware tests ---

func TestOIDCProvider_LoginMiddleware_NoCookie(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}

func TestOIDCProvider_LoginMiddleware_InvalidCookie(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "not-a-valid-jwt"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}

func TestOIDCProvider_LoginMiddleware_ExpiredCookie(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	// Issue a cookie that is already expired.
	claims := oidcClaims{
		Email: "user@example.com",
		Name:  "User",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSigningSecret))
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signed})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}

func TestOIDCProvider_LoginMiddleware_ValidCookie(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	// Issue a valid session cookie.
	sessionJWT, err := p.issueSessionCookie("user-1", "user@example.com", "User One")
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"sub":   c.GetString("oidc_sub"),
			"email": c.GetString("oidc_email"),
			"name":  c.GetString("oidc_name"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionJWT})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "user-1", body["sub"])
	assert.Equal(t, "user@example.com", body["email"])
	assert.Equal(t, "User One", body["name"])
}

// --- handleLogin tests ---

func TestOIDCProvider_HandleLogin_RedirectsToIdP(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	router := gin.New()
	p.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)

	location := w.Header().Get("Location")
	assert.Contains(t, location, ts.issuer)
	assert.Contains(t, location, "response_type=code")
	assert.Contains(t, location, "client_id="+testClientID)

	// State cookie must be set with correct attributes.
	cookies := w.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == stateCookieName {
			stateCookie = c
			break
		}
	}
	require.NotNil(t, stateCookie, "state cookie should be set")
	assert.NotEmpty(t, stateCookie.Value)
	assert.True(t, stateCookie.HttpOnly, "state cookie must be HttpOnly")
	assert.Equal(t, 300, stateCookie.MaxAge, "state cookie TTL must be 300s")
}

// --- handleCallback tests ---

func TestOIDCProvider_HandleCallback_MissingState(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	router := gin.New()
	p.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=somecode&state=mismatch", nil)
	// No state cookie → mismatch.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOIDCProvider_HandleCallback_StateMismatch(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	router := gin.New()
	p.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=somecode&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "correct-state"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOIDCProvider_HandleCallback_InvalidIDToken(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	// Stub an OAuth2 token endpoint that returns a tampered id_token.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"access_token": "acc",
			"token_type":   "Bearer",
			"id_token":     "not.a.valid.jwt",
			"expires_in":   strconv.Itoa(int(time.Hour.Seconds())),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	// Override the token endpoint on the provider's oauth2 config.
	p.oauth2Config.Endpoint.TokenURL = tokenServer.URL

	router := gin.New()
	p.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=code&state=mystate", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "mystate"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verification of the tampered id_token must fail.
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- issueSessionCookie / generateRandomState unit tests ---

func TestOIDCProvider_IssueSessionCookie_RoundTrip(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	signed, err := p.issueSessionCookie("sub-42", "bob@example.com", "Bob")
	require.NoError(t, err)
	assert.NotEmpty(t, signed)

	claims := &oidcClaims{}
	token, err := jwt.ParseWithClaims(signed, claims, func(t *jwt.Token) (interface{}, error) {
		// Mirror the algorithm assertion from LoginMiddleware to ensure the guard is exercised.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return p.signingSecret, nil
	})
	require.NoError(t, err)
	assert.True(t, token.Valid)
	assert.Equal(t, "sub-42", claims.Subject)
	assert.Equal(t, "bob@example.com", claims.Email)
	assert.Equal(t, "Bob", claims.Name)
	assert.True(t, claims.ExpiresAt.After(time.Now()))
}

func TestGenerateRandomState_Uniqueness(t *testing.T) {
	s1, err1 := generateRandomState()
	s2, err2 := generateRandomState()
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEmpty(t, s1)
	assert.NotEmpty(t, s2)
	assert.NotEqual(t, s1, s2)
}

// --- Additional security / edge-case tests ---

func TestOIDCProvider_LoginMiddleware_WrongSecret(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	// Cookie signed with a different secret must be rejected.
	claims := oidcClaims{
		Email: "attacker@evil.com",
		Name:  "Attacker",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "attacker",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	forged, err := token.SignedString([]byte("a-completely-different-secret"))
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: forged})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}

func TestOIDCProvider_LoginMiddleware_APIPath_Returns401JSON(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	router := gin.New()
	router.POST("/tokens", p.LoginMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	// Request without session cookie on an API path → must get 401 JSON, not a redirect.
	req := httptest.NewRequest(http.MethodPost, "/tokens", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestOIDCProvider_HandleCallback_MissingIDToken(t *testing.T) {
	ts := newOIDCTestServer(t)
	p := newTestOIDCProvider(t, ts)

	// Stub a token endpoint that returns no id_token field.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"access_token": "acc",
			"token_type":   "Bearer",
			"expires_in":   strconv.Itoa(int(time.Hour.Seconds())),
			// id_token intentionally absent
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	p.oauth2Config.Endpoint.TokenURL = tokenServer.URL

	router := gin.New()
	p.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=code&state=s", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "s"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
