package auth

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/gimme-cli/gimme/resources/tests/utils"

	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/assert"
)

var jwtRegex = `^[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_.+/=]*$`

func TestCreateTokenError(t *testing.T) {
	authManager := NewAuthManager("secret")

	_, err := authManager.CreateToken("test", "2022-02-17")

	assert.Equal(t, "Expiration date must be greater than the current date.", err.Error())
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

func TestAuthManager_AuthenticateMiddlewareErr(t *testing.T) {
	authManager := NewAuthManager("secret")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareInvalid(t *testing.T) {
	authManager := NewAuthManager("secret")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "invalid"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareInvalid2(t *testing.T) {
	authManager := NewAuthManager("secret")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "Bearer invalid"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareExpired(t *testing.T) {
	authManager := NewAuthManager("secret")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2NDU5ODcwNTIsImp0aSI6InppZyJ9.q9NbUVV6egGlZBLMbvRBO_-VnWy_edDT4VNU6g8GIxQ"})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthManager_AuthenticateMiddlewareOK(t *testing.T) {
	authManager := NewAuthManager("secret")
	token, _ := authManager.CreateToken("test", "")
	router := gin.New()
	router.GET("/", authManager.AuthenticateMiddleware, func(c *gin.Context) {
	})

	w := utils.PerformRequest(router, "GET", "/", nil, utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)})

	assert.Equal(t, http.StatusOK, w.Code)
}
