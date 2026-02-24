package api

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/errors"

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
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("access token name is required"))
	}
	return nil
}

func (ctrl *AuthController) createToken(c *gin.Context) {
	var request CreateTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		msg := "invalid request body"
		if stderrors.Is(err, io.EOF) {
			msg = "request body is required"
		} else if _, ok := err.(*json.UnmarshalTypeError); ok {
			msg = "request body must be a JSON object"
		} else if _, ok := err.(*json.SyntaxError); ok {
			msg = "request body contains invalid JSON"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	if validErr := request.validate(); validErr != nil {
		c.JSON(validErr.GetHTTPCode(), gin.H{"error": validErr.Error()})
		return
	}

	token, createErr := ctrl.authManager.CreateToken(request.Name, request.ExpirationDate)
	if createErr != nil {
		c.JSON(createErr.GetHTTPCode(), gin.H{"error": createErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"token": token})
}

// NewAuthController - Create controller
func NewAuthController(router *gin.Engine, authManager auth.AuthManager, appConfig *configs.Configuration) {
	controller := AuthController{
		authManager,
	}

	router.POST("/create-token", gin.BasicAuth(gin.Accounts{
		appConfig.AdminUser: appConfig.AdminPassword,
	}), controller.createToken)
}
