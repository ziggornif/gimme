package auth

import "github.com/gin-gonic/gin"

// AuthProvider abstracts the authentication mechanism used to protect admin routes.
// Two implementations are available: BasicAuthProvider (default) and OIDCProvider.
//
//   - BasicAuthProvider: validates HTTP Basic Auth credentials on every request.
//   - OIDCProvider: redirects unauthenticated requests to the OIDC authorization
//     endpoint and validates a signed session cookie on subsequent requests.
type AuthProvider interface {
	// LoginMiddleware returns a Gin middleware that enforces authentication.
	// Unauthenticated requests are either rejected (Basic) or redirected (OIDC).
	LoginMiddleware() gin.HandlerFunc

	// RegisterRoutes registers any auxiliary routes required by the provider.
	// For BasicAuthProvider this is a no-op.
	// For OIDCProvider this registers GET /auth/login and GET /auth/callback.
	RegisterRoutes(router *gin.Engine)
}
