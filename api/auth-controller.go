package api

import (
	"fmt"
	"net/http"

	"github.com/gimme-cdn/gimme/errors"

	"github.com/gimme-cdn/gimme/config"

	"github.com/gimme-cdn/gimme/packages/auth"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authManager auth.AuthManager
}

type CreateTokenRequest struct {
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate"`
}

func (req *CreateTokenRequest) validate() *errors.GimmeError {
	if len(req.Name) == 0 {
		return errors.NewError(errors.BadRequest, fmt.Errorf("access token name is required"))
	}
	return nil
}

func (ctrl *AuthController) createToken(c *gin.Context) {
	var request CreateTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if validErr := request.validate(); validErr != nil {
		c.JSON(validErr.GetHTTPCode(), gin.H{"error": validErr.String()})
	}

	token, createErr := ctrl.authManager.CreateToken(request.Name, request.ExpirationDate)
	if createErr != nil {
		c.JSON(createErr.GetHTTPCode(), gin.H{"error": createErr.String()})
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
