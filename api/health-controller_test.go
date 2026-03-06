package api

import (
	"net/http"
	"testing"

	"github.com/gimme-cdn/gimme/test/mocks"
	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthControllerLiveness(t *testing.T) {
	router := gin.New()
	NewHealthController(router, &mocks.MockOSManager{})

	w := utils.PerformRequest(router, "GET", "/healthz", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthControllerReadinessOK(t *testing.T) {
	router := gin.New()
	NewHealthController(router, &mocks.MockOSManager{})

	w := utils.PerformRequest(router, "GET", "/readyz", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthControllerReadinessStorageErr(t *testing.T) {
	router := gin.New()
	NewHealthController(router, &mocks.MockOSManagerErr{})

	w := utils.PerformRequest(router, "GET", "/readyz", nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
