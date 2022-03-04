package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gimme-cli/gimme/config"

	"github.com/gimme-cli/gimme/packages/storage"

	"github.com/gimme-cli/gimme/packages/auth"

	"github.com/gimme-cli/gimme/resources/tests/mocks"

	"github.com/gimme-cli/gimme/resources/tests/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func initObjectStorage() storage.ObjectStorageManager {
	client, _ := storage.NewObjectStorageClient(&config.Configuration{
		S3Url:        "localhost:9000",
		S3Key:        "minioadmin",
		S3Secret:     "minioadmin",
		S3BucketName: "gimme",
		S3Location:   "eu-west-1",
		S3SSL:        false,
	})
	objectStorageManager := storage.NewObjectStorageManager(client)
	objectStorageManager.CreateBucket("gimme", "eu-west-1")
	return objectStorageManager
}

func TestPackageControllerGETErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	NewPackageController(router, authManager, &mockOSManager)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0/file.js", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPackageControllerNotFoundURL(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	NewPackageController(router, authManager, &mockOSManager)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerCreate(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	token, _ := authManager.CreateToken("test", "")
	NewPackageController(router, authManager, objectStorageManager)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	filePath := "../resources/tests/test.zip"

	file, _ := os.Open(filePath)
	defer file.Close()

	formFile,
		_ := writer.CreateFormFile("file", filepath.Base(filePath))
	io.Copy(formFile, file)
	writer.WriteField("name", "awesome-lib")
	writer.WriteField("version", "1.0.0")
	writer.Close()

	w := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPackageControllerGet(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	NewPackageController(router, authManager, objectStorageManager)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/awesome-lib.min.js", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "javascript")
}
