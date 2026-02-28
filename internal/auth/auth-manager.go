package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type AuthManager struct {
	secret string
	store  TokenStore
}

// NewAuthManager create an auth manager instance
func NewAuthManager(secret string, store TokenStore) *AuthManager {
	return &AuthManager{
		secret: secret,
		store:  store,
	}
}

// CreateToken creates an access token, persists it in the store and returns the entry.
func (am *AuthManager) CreateToken(name string, expirationDate string) (*TokenEntry, *errors.GimmeError) {
	var expiration time.Duration
	var expiresAt time.Time

	if len(expirationDate) > 0 {
		format := "2006-01-02"
		end, parseErr := time.Parse(format, expirationDate)
		if parseErr != nil {
			logrus.Errorf("[AuthManager] CreateToken - Invalid expiration date format: %s", expirationDate)
			return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("invalid expiration date format, expected YYYY-MM-DD"))
		}
		expiration = time.Until(end)
		expiresAt = end
	} else {
		expiration = time.Minute * 15
		expiresAt = time.Now().Add(expiration)
	}

	if expiration <= 0 {
		logrus.Error("[AuthManager] CreateToken - Expiration date must be greater than the current date")
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("expiration date must be greater than the current date"))
	}

	id := uuid.New().String()

	claims := &jwt.RegisteredClaims{
		ID:        id,
		ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(expiration)},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(am.secret))
	if err != nil {
		logrus.Error("[AuthManager] CreateToken - Error while signing token")
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while signing token"))
	}

	entry := &TokenEntry{
		ID:        id,
		Name:      name,
		Token:     signedToken,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if saveErr := am.store.Save(entry); saveErr != nil {
		logrus.Errorf("[AuthManager] CreateToken - Error while saving token: %v", saveErr)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("error while saving token"))
	}

	return entry, nil
}

// ListTokens returns all stored token entries (newest first).
func (am *AuthManager) ListTokens() []*TokenEntry {
	return am.store.List()
}

// RevokeToken deletes the token with the given ID from the store.
// Returns false if the ID does not exist.
func (am *AuthManager) RevokeToken(id string) bool {
	return am.store.Delete(id)
}

// extractToken extract token from authentication header
func (am *AuthManager) extractToken(authHeader string) string {
	strArr := strings.Split(authHeader, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}

// decodeToken decode token from input token string.
// It explicitly verifies that the signing method is HMAC (HS256) to prevent
// algorithm-confusion attacks (e.g. accepting an "alg: none" or RS256 token).
func (am *AuthManager) decodeToken(token string) (*jwt.Token, error) {
	decoded, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(am.secret), nil
	})

	if err != nil {
		return nil, err
	}

	return decoded, nil
}

// getClaimsFromJWT extract claims from token
func (am *AuthManager) getClaimsFromJWT(token *jwt.Token) jwt.MapClaims {
	claims := jwt.MapClaims{}
	for key, value := range token.Claims.(jwt.MapClaims) {
		claims[key] = value
	}

	return claims
}

// AuthenticateMiddleware authenticate query with a token.
// It validates the JWT signature and expiry, then checks the token is still
// present in the store (whitelist — catches revoked tokens).
func (am *AuthManager) AuthenticateMiddleware(c *gin.Context) {
	tokenString := am.extractToken(c.GetHeader("Authorization"))
	token, err := am.decodeToken(tokenString)
	if err != nil || !token.Valid {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return
	}

	tokenClaims := am.getClaimsFromJWT(token)
	if tokenClaims["exp"] == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing exp field"})
		c.Abort()
		return
	}

	// Whitelist check: reject tokens that have been revoked via DELETE /tokens/:id.
	if _, ok := am.store.GetByToken(tokenString); !ok {
		c.Status(http.StatusUnauthorized)
		c.Abort()
		return
	}

	c.Next()
}
