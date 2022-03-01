package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RootController struct {
}

func (ctrl *RootController) root(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Welcome to the Gimme CDN API. Here are the available API endpoints :",
		"routes": []gin.H{
			{
				"url":         "<host>/create-token",
				"method":      "POST",
				"description": "Create access tokens",
			}, {
				"url":         "<host>/packages",
				"method":      "POST",
				"description": "Add a package to the CDN",
			}, {
				"url":         "<host>/gimme",
				"method":      "GET",
				"description": "Get a package from the CDN",
			}},
	})
}

// NewRootController - Create controller
func NewRootController(router *gin.Engine) {
	controller := RootController{}

	router.GET("/", controller.root)
}
