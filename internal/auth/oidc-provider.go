package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	sessionCookieName = "gimme_session"
	stateCookieName   = "gimme_oidc_state"
	sessionCookieTTL  = 8 * time.Hour
)

// oidcClaims holds the user identity extracted from the OIDC ID token.
// It is embedded in the signed session cookie.
// Sub is carried via jwt.RegisteredClaims.Subject ("sub" JSON key) to avoid
// having two "sub" fields in the token payload.
type oidcClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// OIDCProvider implements AuthProvider using the OAuth2 authorization code flow.
//
// Flow:
//  1. LoginMiddleware detects an absent/invalid session cookie → redirect to /auth/login
//     (API paths under /tokens return 401 JSON instead of a redirect).
//  2. GET /auth/login  → generates a random state, stores it in a short-lived cookie,
//     redirects the browser to the IdP authorization endpoint.
//  3. GET /auth/callback → validates state cookie, exchanges authorization code for tokens,
//     verifies the OIDC ID token, issues a signed session cookie (HS256 JWT), redirects to /admin.
type OIDCProvider struct {
	oauth2Config  oauth2.Config
	verifier      *oidc.IDTokenVerifier
	signingSecret []byte
	secureCookies bool // should be true in production (HTTPS); false only for local dev
}

// NewOIDCProvider initialises the OIDC provider.
// It performs OIDC discovery against the issuer URL at startup — if the issuer is
// unreachable the application will fail to start.
// secureCookies must be true whenever Gimme is served over HTTPS.
//
// In environments where the server-side issuer URL differs from the browser-facing
// one (e.g. Docker Compose where the IdP is reachable as "keycloak:8180" internally
// but "localhost:8180" from the browser), pass the internal URL as issuer and set
// KC_HOSTNAME_STRICT=false on Keycloak so it reflects the Host header — which means
// the discovery document will also return the internal URL as issuer, satisfying the
// strict issuer check. Alternatively, use a shared hostname via Docker network aliases.
func NewOIDCProvider(ctx context.Context, issuer, clientID, clientSecret, redirectURL, signingSecret string, secureCookies bool) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for issuer %q: %w", issuer, err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	oauth2Cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &OIDCProvider{
		oauth2Config:  oauth2Cfg,
		verifier:      verifier,
		signingSecret: []byte(signingSecret),
		secureCookies: secureCookies,
	}, nil
}

// LoginMiddleware returns a Gin middleware that checks for a valid session cookie.
// - Browser routes (GET /admin): unauthenticated requests are redirected to /auth/login.
// - API routes (POST|DELETE /tokens*): unauthenticated requests receive 401 JSON.
func (p *OIDCProvider) LoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil {
			logrus.Debugf("[OIDCProvider] LoginMiddleware - no session cookie")
			p.rejectUnauthenticated(c)
			return
		}

		claims := &oidcClaims{}
		token, err := jwt.ParseWithClaims(cookie, claims, func(t *jwt.Token) (interface{}, error) {
			// Explicitly assert HS256 to guard against algorithm-confusion attacks.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return p.signingSecret, nil
		})
		if err != nil || !token.Valid {
			logrus.Debugf("[OIDCProvider] LoginMiddleware - invalid session cookie: %v", err)
			p.rejectUnauthenticated(c)
			return
		}

		// Expose identity to downstream handlers via Gin context.
		c.Set("oidc_sub", claims.Subject)
		c.Set("oidc_email", claims.Email)
		c.Set("oidc_name", claims.Name)
		c.Next()
	}
}

// rejectUnauthenticated sends the appropriate rejection response based on the request path
// and Accept header:
// - API paths (/tokens*) or JSON-accepting clients → 401 JSON
// - Browser paths (/admin) → 302 redirect to /auth/login
func (p *OIDCProvider) rejectUnauthenticated(c *gin.Context) {
	isAPIPath := strings.HasPrefix(c.Request.URL.Path, "/tokens")
	acceptsJSON := strings.Contains(c.GetHeader("Accept"), "application/json")
	if isAPIPath || acceptsJSON {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		c.Abort()
		return
	}
	c.Redirect(http.StatusFound, "/auth/login")
	c.Abort()
}

// RegisterRoutes registers GET /auth/login and GET /auth/callback on the router.
func (p *OIDCProvider) RegisterRoutes(router *gin.Engine) {
	router.GET("/auth/login", p.handleLogin)
	router.GET("/auth/callback", p.handleCallback)
}

// handleLogin initiates the authorization code flow.
func (p *OIDCProvider) handleLogin(c *gin.Context) {
	state, err := generateRandomState()
	if err != nil {
		logrus.Errorf("[OIDCProvider] handleLogin - failed to generate state: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Store state in a short-lived HttpOnly cookie to validate on callback.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookieName, state, 300, "/", "", p.secureCookies, true)

	authURL := p.oauth2Config.AuthCodeURL(state)
	c.Redirect(http.StatusFound, authURL)
}

// handleCallback processes the IdP redirect, verifies the ID token, and issues a session cookie.
func (p *OIDCProvider) handleCallback(c *gin.Context) {
	// Validate state to prevent CSRF.
	stateCookie, err := c.Cookie(stateCookieName)
	if err != nil || stateCookie != c.Query("state") {
		logrus.Warnf("[OIDCProvider] handleCallback - state mismatch (cookie: %v, query: %q)", err, c.Query("state"))
		// Clear the stale state cookie to avoid it persisting until its TTL expires.
		c.SetCookie(stateCookieName, "", -1, "/", "", p.secureCookies, true)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	// Exchange authorization code for OAuth2 tokens.
	oauth2Token, err := p.oauth2Config.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		logrus.Errorf("[OIDCProvider] handleCallback - code exchange failed: %v", err)
		// Clear stale state cookie only after a definitive failure (not retryable here).
		c.SetCookie(stateCookieName, "", -1, "/", "", p.secureCookies, true)
		c.JSON(http.StatusBadGateway, gin.H{"error": "token exchange failed"})
		return
	}
	// Clear state cookie only once the exchange has succeeded.
	c.SetCookie(stateCookieName, "", -1, "/", "", p.secureCookies, true)

	// Extract and verify the OIDC ID token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		logrus.Errorf("[OIDCProvider] handleCallback - id_token missing from token response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "id_token missing"})
		return
	}

	idToken, err := p.verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		logrus.Errorf("[OIDCProvider] handleCallback - ID token verification failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "id_token verification failed"})
		return
	}

	// Extract standard claims from the ID token.
	var idClaims oidcClaims
	if err := idToken.Claims(&idClaims); err != nil {
		logrus.Errorf("[OIDCProvider] handleCallback - failed to extract ID token claims: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Issue a signed session cookie (HS256 JWT) valid for sessionCookieTTL.
	sessionJWT, err := p.issueSessionCookie(idClaims.Subject, idClaims.Email, idClaims.Name)
	if err != nil {
		logrus.Errorf("[OIDCProvider] handleCallback - failed to sign session cookie: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ttlSeconds := int(sessionCookieTTL.Seconds())
	// SameSite=Lax (not Strict) so the browser sends the cookie on the redirect
	// from the IdP callback back to /admin (a top-level cross-site navigation).
	// Strict would silently drop the cookie on that first redirect, causing an
	// infinite redirect loop: /admin → /auth/login → IdP → /auth/callback → /admin → …
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, sessionJWT, ttlSeconds, "/", "", p.secureCookies, true)

	logrus.Infof("[OIDCProvider] handleCallback - session issued for sub=%q email=%q", idClaims.Subject, idClaims.Email)
	c.Redirect(http.StatusFound, "/admin")
}

// issueSessionCookie creates a signed HS256 JWT encoding the user's identity.
func (p *OIDCProvider) issueSessionCookie(sub, email, name string) (string, error) {
	claims := oidcClaims{
		Email: email,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sessionCookieTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.signingSecret)
}

// generateRandomState produces a URL-safe random string for CSRF protection.
func generateRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
