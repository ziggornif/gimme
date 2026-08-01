package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/ziggornif/gimme/test/utils"
)

func TestNewRootController(t *testing.T) {
	router := gin.New()
	router.SetFuncMap(TemplateFuncs())
	router.LoadHTMLGlob("../templates/*.tmpl")
	NewRootController(router)

	w := utils.PerformRequest(router, "GET", "/", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}
