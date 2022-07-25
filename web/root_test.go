package web

import (
	"net/http"
	"testing"

	"github.com/gimme-cdn/gimme/test/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewRootController(t *testing.T) {
	router := gin.New()
	router.LoadHTMLGlob("../templates/*.tmpl")
	NewRootController(router)

	w := utils.PerformRequest(router, "GET", "/", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}
