package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gin-gonic/gin"
)

// AdminController handles the admin UI and token management API.
type AdminController struct {
	authManager *auth.AuthManager
}

// createTokenRequest is the JSON body for token creation.
type createTokenRequest struct {
	Name           string `json:"name"`
	ExpirationDate string `json:"expirationDate"`
}

const maxTokenNameLength = 255

func (req *createTokenRequest) validate() *errors.GimmeError {
	if len(req.Name) == 0 {
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("access token name is required"))
	}
	if len(req.Name) > maxTokenNameLength {
		return errors.NewBusinessError(errors.BadRequest, fmt.Errorf("access token name must not exceed %d characters", maxTokenNameLength))
	}
	return nil
}

// tokenResponse is the JSON shape returned when a token is created.
// The Token field carries the raw opaque token value, shown exactly once.
type tokenResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (ctrl *AdminController) getAdmin(c *gin.Context) {
	tokens := ctrl.authManager.ListTokens(c.Request.Context())
	c.HTML(http.StatusOK, "admin.tmpl", gin.H{
		"tokens": tokens,
	})
}

func (ctrl *AdminController) createToken(c *gin.Context) {
	var request createTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if validErr := request.validate(); validErr != nil {
		c.JSON(validErr.GetHTTPCode(), gin.H{"error": validErr.Error()})
		return
	}

	entry, rawToken, createErr := ctrl.authManager.CreateToken(c.Request.Context(), request.Name, request.ExpirationDate)
	if createErr != nil {
		c.JSON(createErr.GetHTTPCode(), gin.H{"error": createErr.Error()})
		return
	}

	// rawToken is returned once here and never stored — the client must save it.
	c.JSON(http.StatusCreated, tokenResponse{
		ID:        entry.ID,
		Name:      entry.Name,
		Token:     rawToken,
		CreatedAt: entry.CreatedAt,
		ExpiresAt: entry.ExpiresAt,
	})
}

func (ctrl *AdminController) deleteToken(c *gin.Context) {
	id := c.Param("id")

	if revoked := ctrl.authManager.RevokeToken(c.Request.Context(), id); !revoked {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// NewAdminController registers admin routes protected by the given AuthProvider.
func NewAdminController(router *gin.Engine, authManager *auth.AuthManager, authProvider auth.AuthProvider) {
	controller := AdminController{
		authManager: authManager,
	}

	loginMW := authProvider.LoginMiddleware()

	router.GET("/admin", loginMW, controller.getAdmin)
	router.POST("/tokens", loginMW, controller.createToken)
	router.DELETE("/tokens/:id", loginMW, controller.deleteToken)
}
