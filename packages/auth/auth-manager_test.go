package auth

import (
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var jwtRegex = `^[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*$`

func TestCreateTokenError(t *testing.T) {
	authManager := NewAuthManager("secret")

	_, err := authManager.CreateToken("test", "2022-02-17")

	assert.Equal(t, "CreateToken - Expiration date must be greater than the current date.", err.Error())
}

func TestCreateTokenDefault(t *testing.T) {
	authManager := NewAuthManager("secret")

	token, err := authManager.CreateToken("test", "")

	assert.Regexp(t, regexp.MustCompile(jwtRegex), token)
	assert.Nil(t, err)
}

func TestCreateTokenCustomExp(t *testing.T) {
	authManager := NewAuthManager("secret")
	dt := time.Now().Add(time.Hour * 24)
	token, err := authManager.CreateToken("test", dt.Format("2006-01-02"))

	assert.Regexp(t, regexp.MustCompile(jwtRegex), token)
	assert.Nil(t, err)
}
