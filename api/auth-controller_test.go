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

func newAuthRouter() *gin.Engine {
	router := gin.New()
	authManager := auth.NewAuthManager("secret", auth.NewMemoryTokenStore())
	cfg := &configs.Configuration{AdminUser: "test", AdminPassword: "test"}
	NewAuthController(router, authManager, cfg)
	return router
}

const authHeader = "Basic dGVzdDp0ZXN0"

func TestNewAuthControllerAuthErr(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestNewAuthControllerNoBody(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token", nil,
		utils.Header{Key: "Authorization", Value: authHeader})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "request body is required")
}

func TestNewAuthControllerBodyNotObject(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token", strings.NewReader(`""`),
		utils.Header{Key: "Authorization", Value: authHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "request body must be a JSON object")
}

func TestNewAuthControllerInvalidJSON(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token", strings.NewReader(`{invalid}`),
		utils.Header{Key: "Authorization", Value: authHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "request body contains invalid JSON")
}

func TestNewAuthController(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token", strings.NewReader(`{"name": "test"}`),
		utils.Header{Key: "Authorization", Value: authHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNewAuthControllerExpired(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token",
		strings.NewReader(`{"name": "test", "expirationDate": "2021-12-10"}`),
		utils.Header{Key: "Authorization", Value: authHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNewAuthControllerInvalid(t *testing.T) {
	w := utils.PerformRequest(newAuthRouter(), "POST", "/create-token", strings.NewReader(`{}`),
		utils.Header{Key: "Authorization", Value: authHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
