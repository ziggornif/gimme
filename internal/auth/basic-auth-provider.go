package auth

import (
	"github.com/gin-gonic/gin"
)

// BasicAuthProvider implements AuthProvider using HTTP Basic Auth.
// This is the default mode and preserves the existing behaviour:
// every request to a protected route must carry valid Basic Auth credentials.
type BasicAuthProvider struct {
	accounts gin.Accounts
}

// NewBasicAuthProvider creates a BasicAuthProvider with the given credentials.
func NewBasicAuthProvider(user, password string) *BasicAuthProvider {
	return &BasicAuthProvider{
		accounts: gin.Accounts{user: password},
	}
}

// LoginMiddleware returns Gin's built-in BasicAuth middleware.
func (p *BasicAuthProvider) LoginMiddleware() gin.HandlerFunc {
	return gin.BasicAuth(p.accounts)
}

// RegisterRoutes is a no-op for Basic Auth: no auxiliary routes are needed.
func (p *BasicAuthProvider) RegisterRoutes(_ *gin.Engine) {}
