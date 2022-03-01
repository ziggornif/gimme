package api

import (
	"net/http"

	"github.com/gimme-cli/gimme/config"

	"github.com/gimme-cli/gimme/packages/auth"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authManager auth.AuthManager
}

func (ctrl *AuthController) createToken(c *gin.Context) {
	var request auth.CreateTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := ctrl.authManager.CreateToken(request.Name, request.ExpirationDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// NewAuthController - Create controller
func NewAuthController(router *gin.Engine, authManager auth.AuthManager, appConfig *config.Configuration) {
	controller := AuthController{
		authManager,
	}

	router.POST("/create-token", gin.BasicAuth(gin.Accounts{
		appConfig.AdminUser: appConfig.AdminPassword,
	}), controller.createToken)
}
