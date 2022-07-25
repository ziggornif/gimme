package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRootWebController - Create controller
func NewRootWebController(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{})
	})
}
