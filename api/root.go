package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

type RootController struct {
}

// NewRootController - Create controller
func NewRootController(router *gin.Engine) {
	url := ginSwagger.URL("/swagger/swagger.json") // The url pointing to API definition
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/docs/index.html")
	})
}
