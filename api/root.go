package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRootController - Create controller
func NewRootController(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{})
	})
}
