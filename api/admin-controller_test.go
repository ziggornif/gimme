package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const adminAuthHeader = "Basic dGVzdDp0ZXN0" // test:test

func newAdminRouter() (*gin.Engine, *auth.AuthManager) {
	router := gin.New()
	store := auth.NewMemoryTokenStore()
	authManager := auth.NewAuthManager("secret", store)
	cfg := &configs.Configuration{AdminUser: "test", AdminPassword: "test"}
	NewAdminController(router, authManager, cfg)
	return router, authManager
}

func newAdminRouterWithTemplates() (*gin.Engine, *auth.AuthManager) {
	router := gin.New()
	store := auth.NewMemoryTokenStore()
	authManager := auth.NewAuthManager("secret", store)
	cfg := &configs.Configuration{AdminUser: "test", AdminPassword: "test"}
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	NewAdminController(router, authManager, cfg)
	return router, authManager
}

func TestAdminController_GetAdmin_Unauthorized(t *testing.T) {
	router, _ := newAdminRouterWithTemplates()
	w := utils.PerformRequest(router, "GET", "/admin", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminController_GetAdmin_OK(t *testing.T) {
	router, _ := newAdminRouterWithTemplates()
	w := utils.PerformRequest(router, "GET", "/admin", nil,
		utils.Header{Key: "Authorization", Value: adminAuthHeader})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestAdminController_CreateToken_Unauthorized(t *testing.T) {
	router, _ := newAdminRouter()
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(`{"name":"ci"}`),
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminController_CreateToken_NoName(t *testing.T) {
	router, _ := newAdminRouter()
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(`{}`),
		utils.Header{Key: "Authorization", Value: adminAuthHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminController_CreateToken_OK(t *testing.T) {
	router, _ := newAdminRouter()
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(`{"name":"ci"}`),
		utils.Header{Key: "Authorization", Value: adminAuthHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, err)
	assert.NotEmpty(t, resp["id"])
	assert.NotEmpty(t, resp["token"])
	assert.Equal(t, "ci", resp["name"])
}

func TestAdminController_DeleteToken_Unauthorized(t *testing.T) {
	router, _ := newAdminRouter()
	w := utils.PerformRequest(router, "DELETE", "/tokens/some-id", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminController_DeleteToken_NotFound(t *testing.T) {
	router, _ := newAdminRouter()
	w := utils.PerformRequest(router, "DELETE", "/tokens/nonexistent",
		nil,
		utils.Header{Key: "Authorization", Value: adminAuthHeader})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminController_DeleteToken_OK(t *testing.T) {
	router, authManager := newAdminRouter()

	entry, err := authManager.CreateToken("test", "")
	assert.Nil(t, err)

	w := utils.PerformRequest(router, "DELETE", "/tokens/"+entry.ID,
		nil,
		utils.Header{Key: "Authorization", Value: adminAuthHeader})
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Token should no longer be in the store
	tokens := authManager.ListTokens()
	assert.Empty(t, tokens)
}
