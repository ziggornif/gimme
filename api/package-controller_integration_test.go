package api

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/content"
	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestAuthManager() *auth.AuthManager {
	return auth.NewAuthManager(auth.NewMemoryTokenStore())
}

func TestPackageControllerGETInvalidUrlErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/file.js", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerGETInvalidUrlAlterErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/foo/bar.js", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerRedirect(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme", nil)

	assert.Equal(t, http.StatusFound, w.Code)
}

func TestPackageControllerCreate(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	resp := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusCreated, resp.Code)

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGet(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/awesome-lib.min.js", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "javascript")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetUI(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetUIAlter(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerCreateConflictErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	resp := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusCreated, resp.Code)

	resp2 := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusConflict, resp2.Code)

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetEmpty(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@4.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerGetNotFound(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/invalid@1.0.0/invalid.js", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerPOSTEmptyFile(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	err := writer.WriteField("name", "awesome-lib")
	assert.Nil(t, err)
	err = writer.WriteField("version", "1.0.0")
	assert.Nil(t, err)
	err = writer.Close()
	assert.Nil(t, err)

	w := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerDeleteInvalidUrlErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "DELETE", "/packages/file.js", nil,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerDelete(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager()
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "DELETE", "/packages/awesome-lib@1.0.0", nil,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusNoContent, w.Code)
}
