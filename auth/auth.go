package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/drouian-m/gimme/config"
	"github.com/golang-jwt/jwt"
)

type CreateTokenRequest struct {
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate"`
}

func CreateToken(name string, expirationDate string, config *config.Config) (string, error) {
	var expiration time.Duration
	if len(expirationDate) > 0 {
		format := "2006-01-02"
		end, _ := time.Parse(format, expirationDate)
		expiration = time.Minute * time.Duration(end.Sub(time.Now()).Minutes())
	} else {
		expiration = time.Minute * 15
	}

	if expiration <= 0 {
		return "", errors.New("Expiration date must be greater than the current date.")
	}

	claims := &jwt.StandardClaims{
		Id:        name,
		ExpiresAt: time.Now().Add(expiration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString([]byte(config.Secret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func ExtractToken(authHeader string) string {
	strArr := strings.Split(authHeader, " ")
	if len(strArr) == 2 {
		return strArr[1]
	}
	return ""
}

func ValidateToken(token string, config *config.Config) (bool, error) {
	decoded, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Secret), nil
	})

	if err != nil {
		return false, err
	}

	return decoded.Valid, nil
}
