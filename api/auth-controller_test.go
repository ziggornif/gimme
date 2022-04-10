package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gimme-cdn/gimme/internal/auth"

	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/stretchr/testify/assert"

	"github.com/gimme-cdn/gimme/configs"

	"github.com/gin-gonic/gin"
)

func TestNewAuthControllerAuthErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &configs.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	w := utils.PerformRequest(router, "POST", "/create-token", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewAuthControllerBadRequest(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &configs.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	w := utils.PerformRequest(router, "POST", "/create-token", nil, utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthController(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &configs.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{"name": "test"}`

	w := utils.PerformRequest(router, "POST", "/create-token", strings.NewReader(body), utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNewAuthControllerExpired(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &configs.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{"name": "test", "expirationDate": "2021-12-10"}`

	w := utils.PerformRequest(router, "POST", "/create-token", strings.NewReader(body), utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthControllerInvalid(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &configs.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{}`

	w := utils.PerformRequest(router, "POST", "/create-token", strings.NewReader(body), utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
