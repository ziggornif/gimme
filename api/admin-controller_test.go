package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adminAuthHeader = "Basic dGVzdDp0ZXN0" // test:test

// newAdminTokenStore creates a FileTokenStore in a temp dir for controller tests.
func newAdminTokenStore(t *testing.T) *auth.FileTokenStore {
	t.Helper()
	store, err := auth.NewFileTokenStore("this-is-a-32-byte-secret-for-test", filepath.Join(t.TempDir(), "tokens.enc"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func newAdminRouter(t *testing.T) (*gin.Engine, *auth.AuthManager) {
	router := gin.New()
	authManager := auth.NewAuthManager(newAdminTokenStore(t))
	provider := auth.NewBasicAuthProvider("test", "test")
	NewAdminController(router, authManager, provider)
	return router, authManager
}

func newAdminRouterWithTemplates(t *testing.T) (*gin.Engine, *auth.AuthManager) {
	router := gin.New()
	authManager := auth.NewAuthManager(newAdminTokenStore(t))
	provider := auth.NewBasicAuthProvider("test", "test")
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	NewAdminController(router, authManager, provider)
	return router, authManager
}

func TestAdminController_GetAdmin_Unauthorized(t *testing.T) {
	router, _ := newAdminRouterWithTemplates(t)
	w := utils.PerformRequest(router, "GET", "/admin", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminController_GetAdmin_OK(t *testing.T) {
	router, _ := newAdminRouterWithTemplates(t)
	w := utils.PerformRequest(router, "GET", "/admin", nil,
		utils.Header{Key: "Authorization", Value: adminAuthHeader})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestAdminController_CreateToken_Unauthorized(t *testing.T) {
	router, _ := newAdminRouter(t)
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(`{"name":"ci"}`),
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminController_CreateToken_NoName(t *testing.T) {
	router, _ := newAdminRouter(t)
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(`{}`),
		utils.Header{Key: "Authorization", Value: adminAuthHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminController_CreateToken_OK(t *testing.T) {
	router, _ := newAdminRouter(t)
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(`{"name":"ci"}`),
		utils.Header{Key: "Authorization", Value: adminAuthHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, err)
	assert.NotEmpty(t, resp["id"])
	// Token must be returned in the response (opaque, shown once)
	token, ok := resp["token"].(string)
	assert.True(t, ok)
	assert.True(t, strings.HasPrefix(token, "gim_"), "token must start with 'gim_'")
	assert.Equal(t, "ci", resp["name"])
}

func TestAdminController_DeleteToken_Unauthorized(t *testing.T) {
	router, _ := newAdminRouter(t)
	w := utils.PerformRequest(router, "DELETE", "/tokens/some-id", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminController_DeleteToken_NotFound(t *testing.T) {
	router, _ := newAdminRouter(t)
	w := utils.PerformRequest(router, "DELETE", "/tokens/nonexistent",
		nil,
		utils.Header{Key: "Authorization", Value: adminAuthHeader})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminController_CreateToken_NameTooLong(t *testing.T) {
	router, _ := newAdminRouter(t)
	longName := strings.Repeat("a", maxTokenNameLength+1)
	body := fmt.Sprintf(`{"name":%q}`, longName)
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(body),
		utils.Header{Key: "Authorization", Value: adminAuthHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "must not exceed")
}

func TestAdminController_CreateToken_NameAtMaxLength(t *testing.T) {
	router, _ := newAdminRouter(t)
	exactName := strings.Repeat("a", maxTokenNameLength)
	body := fmt.Sprintf(`{"name":%q}`, exactName)
	w := utils.PerformRequest(router, "POST", "/tokens", strings.NewReader(body),
		utils.Header{Key: "Authorization", Value: adminAuthHeader},
		utils.Header{Key: "Content-Type", Value: "application/json"})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminController_DeleteToken_OK(t *testing.T) {
	router, authManager := newAdminRouter(t)

	entry, _, err := authManager.CreateToken(context.Background(), "test", "")
	assert.Nil(t, err)

	w := utils.PerformRequest(router, "DELETE", "/tokens/"+entry.ID,
		nil,
		utils.Header{Key: "Authorization", Value: adminAuthHeader})
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Token should be marked as revoked (still in list but revoked)
	tokens := authManager.ListTokens(context.Background())
	assert.Equal(t, 1, len(tokens))
	assert.True(t, tokens[0].IsRevoked())
}
