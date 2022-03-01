package api

import (
	"net/http"
	"testing"

	"github.com/gimme-cli/gimme/packages/auth"
	"github.com/gimme-cli/gimme/resources/tests/utils"
	"github.com/stretchr/testify/assert"

	"github.com/gimme-cli/gimme/config"

	"github.com/gin-gonic/gin"
)

func TestNewAuthControllerAuthErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	w := utils.PerformRequest(router, "POST", "/create-token", "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewAuthControllerBadRequest(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	w := utils.PerformRequest(router, "POST", "/create-token", "", utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthController(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{"name": "test"}`

	w := utils.PerformRequest(router, "POST", "/create-token", body, utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewAuthControllerExpired(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewAuthController(router, authManager, &config.Configuration{
		AdminUser: "test", AdminPassword: "test",
	})

	body := `{"name": "test", "expirationDate": "2021-12-10"}`

	w := utils.PerformRequest(router, "POST", "/create-token", body, utils.Header{Key: "Authorization", Value: "Basic dGVzdDp0ZXN0"})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
