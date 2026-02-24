package api

import (
	"net/http"

	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gin-gonic/gin"
)

type HealthController struct {
	storageManager storage.ObjectStorageManager
}

func (ctrl *HealthController) liveness(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (ctrl *HealthController) readiness(c *gin.Context) {
	if err := ctrl.storageManager.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// NewHealthController - Create health controller
func NewHealthController(router *gin.Engine, storageManager storage.ObjectStorageManager) {
	controller := HealthController{storageManager: storageManager}
	router.GET("/healthz", controller.liveness)
	router.GET("/readyz", controller.readiness)
}
