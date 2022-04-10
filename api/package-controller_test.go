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

	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/gimme"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gimme-cdn/gimme/test/mocks"
	"github.com/gimme-cdn/gimme/test/utils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func initObjectStorage() storage.ObjectStorageManager {
	client, _ := storage.NewObjectStorageClient(&configs.Configuration{
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
	service := gimme.NewGimmeService(&mockOSManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0/file.js", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPackageControllerNotFoundURL(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	service := gimme.NewGimmeService(&mockOSManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerCreate(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	token, _ := authManager.CreateToken("test", "")
	service := gimme.NewGimmeService(objectStorageManager)
	NewPackageController(router, authManager, service)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	filePath := "../test/test.zip"

	file, _ := os.Open(filePath)
	defer func(file *os.File) {
		err := file.Close()
		assert.Nil(t, err)
	}(file)

	formFile,
		_ := writer.CreateFormFile("file", filepath.Base(filePath))
	_, err := io.Copy(formFile, file)
	assert.Nil(t, err)
	err = writer.WriteField("name", "awesome-lib")
	assert.Nil(t, err)
	err = writer.WriteField("version", "1.0.0")
	assert.Nil(t, err)
	err = writer.Close()
	assert.Nil(t, err)

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
	service := gimme.NewGimmeService(objectStorageManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0/awesome-lib.min.js", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "javascript")
}

func TestPackageControllerGetUI(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	router.LoadHTMLGlob("../templates/*.tmpl")
	authManager := auth.NewAuthManager("secret")
	service := gimme.NewGimmeService(objectStorageManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@1.0.0", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestPackageControllerCreateConflictErr(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	token, _ := authManager.CreateToken("test", "")
	service := gimme.NewGimmeService(objectStorageManager)
	NewPackageController(router, authManager, service)

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	filePath := "../test/test.zip"

	file, _ := os.Open(filePath)
	defer func(file *os.File) {
		err := file.Close()
		assert.Nil(t, err)
	}(file)

	formFile,
		_ := writer.CreateFormFile("file", filepath.Base(filePath))
	_, err := io.Copy(formFile, file)
	assert.Nil(t, err)
	err = writer.WriteField("name", "awesome-lib")
	assert.Nil(t, err)
	err = writer.WriteField("version", "1.0.0")
	assert.Nil(t, err)
	err = writer.Close()
	assert.Nil(t, err)

	w := utils.PerformRequest(router, "POST", "/packages", payload,
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestPackageControllerGetEmpty(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	service := gimme.NewGimmeService(objectStorageManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/awesome-lib@4.0.0", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerGetNotFound(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	service := gimme.NewGimmeService(objectStorageManager)
	NewPackageController(router, authManager, service)

	w := utils.PerformRequest(router, "GET", "/gimme/invalid@1.0.0/invalid.js", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPackageControllerPOSTEmptyFile(t *testing.T) {
	objectStorageManager := initObjectStorage()
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	token, _ := authManager.CreateToken("test", "")
	service := gimme.NewGimmeService(objectStorageManager)
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
		utils.Header{Key: "Authorization", Value: fmt.Sprintf("Bearer %s", token)},
		utils.Header{
			Key: "Content-Type", Value: writer.FormDataContentType(),
		})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
