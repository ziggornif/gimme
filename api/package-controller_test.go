package api

import (
	"net/http"
	"testing"

	"github.com/gimme-cli/gimme/packages/auth"

	"github.com/gimme-cli/gimme/resources/tests/mocks"

	"github.com/gimme-cli/gimme/resources/tests/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPackageControllerGETErr(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	NewPackageController(router, authManager, &mockOSManager)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0/file.js", "")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPackageControllerNotFoundURL(t *testing.T) {
	router := gin.New()
	authManager := auth.NewAuthManager("secret")
	mockOSManager := mocks.MockOSManagerErr{}
	NewPackageController(router, authManager, &mockOSManager)

	w := utils.PerformRequest(router, "GET", "/gimme/test@1.0.0", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

//TODO : add object storage tests here
