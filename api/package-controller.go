package api

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/gimme-cdn/gimme/packages/storage"

	"github.com/gimme-cdn/gimme/packages/auth"
	"github.com/gimme-cdn/gimme/packages/upload"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type PackageController struct {
	authManager          auth.AuthManager
	objectStorageManager storage.ObjectStorageManager
}

func (ctrl *PackageController) createPackage(c *gin.Context) {
	file, _ := c.FormFile("file")
	name := c.PostForm("name")
	version := c.PostForm("version")

	validationErr := upload.ValidateFile(file)
	if validationErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.String()})
		return
	}

	reader, _ := file.Open()
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {
			logrus.Error("Fail to close file")
		}
	}(reader)

	uploadErr := upload.ArchiveProcessor(name, version, ctrl.objectStorageManager, reader, file.Size)
	if uploadErr != nil {
		c.JSON(uploadErr.GetHTTPCode(), gin.H{"error": uploadErr.String()})
		return
	}

	c.Status(http.StatusNoContent)
	return
}

func (ctrl *PackageController) getPackage(c *gin.Context) {
	var filePath string
	file := c.Param("file")
	filePath += fmt.Sprintf("%v%v", c.Param("package"), file)

	object, err := ctrl.objectStorageManager.GetObject(filePath)
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}

	infos, _ := object.Stat()
	if infos.Size == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	c.DataFromReader(http.StatusOK, infos.Size, infos.ContentType, object, nil)
}

// NewPackageController - Create controller
func NewPackageController(router *gin.Engine, authManager auth.AuthManager, objectStorageManager storage.ObjectStorageManager) {
	controller := PackageController{
		authManager,
		objectStorageManager,
	}

	router.GET("/gimme/:package/*file", controller.getPackage)
	router.POST("/packages", authManager.AuthenticateMiddleware, controller.createPackage)
}
