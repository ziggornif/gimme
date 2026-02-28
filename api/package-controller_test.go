package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/content"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gimme-cdn/gimme/test/mocks"
	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func initObjectStorage() storage.ObjectStorageManager {
	bucketName := envOrDefault("TEST_S3_BUCKET", "gimme")
	location := envOrDefault("TEST_S3_LOCATION", "eu-west-1")
	client, err := storage.NewObjectStorageClient(&configs.Configuration{
		S3Url:        envOrDefault("TEST_S3_URL", "localhost:9000"),
		S3Key:        envOrDefault("TEST_S3_KEY", "minioadmin"),
		S3Secret:     envOrDefault("TEST_S3_SECRET", "minioadmin"),
		S3BucketName: bucketName,
		S3Location:   location,
		S3SSL:        false,
	})
	if err != nil {
		panic(err.Error())
	}
	objectStorageManager := storage.NewObjectStorageManager(client)
	if err := objectStorageManager.CreateBucket(context.Background(), bucketName, location); err != nil {
		panic(err.Error())
	}
	return objectStorageManager
}

func createPackage(t *testing.T, router http.Handler, name string, version string, filePath string, token string) *httptest.ResponseRecorder {
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	file, _ := os.Open(filePath)
	defer func(file *os.File) {
		err := file.Close()
		assert.Nil(t, err)
	}(file)

	formFile,
		_ := writer.CreateFormFile("file", filepath.Base(filePath))
	_, err := io.Copy(formFile, file)
	assert.Nil(t, err)
	err = writer.WriteField("name", name)
	assert.Nil(t, err)
	err = writer.WriteField("version", version)
	assert.Nil(t, err)
	err = writer.Close()
	assert.Nil(t, err)

	return utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})
}

func TestPackageControllerGETErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager(auth.NewMemoryTokenStore())
	mockOSManager := mocks.MockOSManagerErr{}
	service := content.NewContentService(&mockOSManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0/file.js", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestPackageControllerNotFoundURL(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager(auth.NewMemoryTokenStore())
	mockOSManager := mocks.MockOSManagerErr{}
	service := content.NewContentService(&mockOSManager, nil, 0)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetSlice_EmptyName(t *testing.T) {
	ctrl := &PackageController{}
	_, err := ctrl.getSlice("@1.0.0")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.GetHTTPCode())
	assert.Contains(t, err.Error(), "package name must not be empty")
}

func TestGetSlice_EmptyVersion(t *testing.T) {
	ctrl := &PackageController{}
	_, err := ctrl.getSlice("pkg@")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.GetHTTPCode())
	assert.Contains(t, err.Error(), "package version must not be empty")
}

func TestGetSlice_NoAtSign(t *testing.T) {
	ctrl := &PackageController{}
	_, err := ctrl.getSlice("pkg-without-at")
	assert.NotNil(t, err)
	assert.Equal(t, http.StatusBadRequest, err.GetHTTPCode())
}

func TestGetSlice_Valid(t *testing.T) {
	ctrl := &PackageController{}
	slice, err := ctrl.getSlice("mypkg@1.2.3")
	assert.Nil(t, err)
	assert.Equal(t, "mypkg", slice.Name)
	assert.Equal(t, "1.2.3", slice.Version)
}

func TestCacheControlHeader(t *testing.T) {
	tests := []struct {
		version  string
		expected string
	}{
		{"1.0.0", "public, max-age=31536000, immutable"},
		{"1.0.1", "public, max-age=31536000, immutable"},
		{"1.0", "public, max-age=300"},
		{"1", "public, max-age=300"},
		{"1.0.0-rc.1", "public, max-age=300"},
		{"1.0.0+build.1", "public, max-age=31536000, immutable"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			assert.Equal(t, tt.expected, cacheControlHeader(tt.version))
		})
	}
}
