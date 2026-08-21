package api

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/ziggornif/gimme/internal/auth"
	"github.com/ziggornif/gimme/internal/content"
	"github.com/ziggornif/gimme/test/utils"
)

func newTestAuthManager(t *testing.T) *auth.AuthManager {
	t.Helper()
	store, err := auth.NewFileTokenStore("this-is-a-32-byte-secret-for-test", filepath.Join(t.TempDir(), "tokens.enc"))
	if err != nil {
		t.Fatalf("newTestAuthManager: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return auth.NewAuthManager(store)
}

func TestPackageControllerGETInvalidUrlErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/file.js", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerGETInvalidUrlAlterErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/foo/bar.js", nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerRedirect(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme", nil)

	assert.Equal(t, http.StatusFound, w.Code)
}

func TestPackageControllerCreate(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	resp := createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)
	assert.Equal(t, http.StatusCreated, resp.Code)

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGet(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/awesome-lib.min.js", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "javascript")

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetConditional(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	_ = createPackage(t, router, "awesome-lib", "1.0.0", "../test/test.zip", rawToken)

	for _, version := range []string{"1.0.0", "1.0"} {
		t.Run(version, func(t *testing.T) {
			path := "/gimme/awesome-lib@" + version + "/awesome-lib.min.js"
			first := utils.PerformRequest(router, "GET", path, nil)

			assert.Equal(t, http.StatusOK, first.Code)
			etag := first.Header().Get("ETag")
			assert.NotEmpty(t, etag)
			assert.NotEmpty(t, first.Header().Get("Last-Modified"))

			notModified := utils.PerformRequest(router, "GET", path, nil,
				utils.Header{Key: "If-None-Match", Value: etag})
			assert.Equal(t, http.StatusNotModified, notModified.Code)
			assert.Zero(t, notModified.Body.Len())
			assert.NotEmpty(t, notModified.Header().Get("Cache-Control"))
			assert.Equal(t, etag, notModified.Header().Get("ETag"))

			stale := utils.PerformRequest(router, "GET", path, nil,
				utils.Header{Key: "If-None-Match", Value: `"nope"`})
			assert.Equal(t, http.StatusOK, stale.Code)
			assert.NotZero(t, stale.Body.Len())
		})
	}

	_ = service.DeletePackage(context.Background(), "awesome-lib", "1.0.0") //nolint:errcheck
}

func TestPackageControllerGetUI(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
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
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
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
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
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
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@4.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerGetNotFound(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/invalid@1.0.0/invalid.js", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerPOSTEmptyFile(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
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
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "DELETE", "/packages/file.js", nil,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPackageControllerDelete(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := newTestAuthManager(t)
	_, rawToken, _ := authManager.CreateToken(context.Background(), "test", "")
	service := content.NewContentService(objectStorageManager, nil, 0, content.UploadLimits{})
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "DELETE", "/packages/awesome-lib@1.0.0", nil,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", rawToken)})

	assert.Equal(t, http.StatusNoContent, w.Code)
}
