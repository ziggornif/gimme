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

func initObjectStorage() storage.ObjectStorageManager {
	client, err := storage.NewObjectStorageClient(&configs.Configuration{
		S3Url:        "localhost:9000",
		S3Key:        "minioadmin",
		S3Secret:     "minioadmin",
		S3BucketName: "gimme",
		S3Location:   "eu-west-1",
		S3SSL:        false,
	})
	if err != nil {
		panic(err.Error())
	}
	objectStorageManager := storage.NewObjectStorageManager(client)
	objectStorageManager.CreateBucket(context.Background(), "gimme", "eu-west-1")
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
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	service := content.NewContentService(&mockOSManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0/file.js", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPackageControllerNotFoundURL(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	service := content.NewContentService(&mockOSManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
