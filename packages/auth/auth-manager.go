package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"

	"github.com/golang-jwt/jwt/v4"
)

type AuthManager struct {
	secret string
}

type CreateTokenRequest struct {
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate"`
}

// NewAuthManager create a auth manager instance
func NewAuthManager(secret string) *AuthManager {
	return &AuthManager{
		secret,
	}
}

// CreateToken create access token
func (am *AuthManager) CreateToken(name string, expirationDate string) (string, error) {
	var expiration time.Duration
	if len(expirationDate) > 0 {
		format := "2006-01-02"
		end, _ := time.Parse(format, expirationDate)
		expiration = time.Minute * time.Duration(end.Sub(time.Now()).Minutes())
	} else {
		expiration = time.Minute * 15
	}

	if expiration <= 0 {
		logrus.Error("[AuthManager] CreateToken - Expiration date must be greater than the current date.")
		return "", fmt.Errorf("Expiration date must be greater than the current date.")
	}

	claims := &jwt.RegisteredClaims{
		ID:        name,
		ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(expiration)},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(am.secret))
	if err != nil {
		logrus.Error("[AuthManager] CreateToken - Error while signing token.")
		return "", fmt.Errorf("Error while signing token.")
	}
	return signedToken, nil
}

// extractToken extract token from authentication header
func (am *AuthManager) extractToken(authHeader string) string {
	strArr := strings.Split(authHeader, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}

// decodeToken decode token from input token string
func (am *AuthManager) decodeToken(token string) (*jwt.Token, error) {
	decoded, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
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

// AuthenticateMiddleware authenticate query with a token
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

	c.Next()
}
