package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gimme-cdn/gimme/packages/auth"
	"github.com/gimme-cdn/gimme/resources/tests/utils"
	"github.com/stretchr/testify/assert"

	"github.com/gimme-cdn/gimme/config"

	"github.com/gin-gonic/gin"
)

func TestNewAuthControllerAuthErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	w := utils.PerformRequest(router, "POST", "/create-token", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewAuthControllerBadRequest(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	w := utils.PerformRequest(router, "POST", "/create-token", nil, utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthController(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{"name": "test"}`

	w := utils.PerformRequest(router, "POST", "/create-token", strings.NewReader(body), utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewAuthControllerExpired(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{"name": "test", "expirationDate": "2021-12-10"}`

	w := utils.PerformRequest(router, "POST", "/create-token", strings.NewReader(body), utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthControllerInvalid(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{}`

	w := utils.PerformRequest(router, "POST", "/create-token", strings.NewReader(body), utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
