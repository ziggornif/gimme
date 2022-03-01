package api

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/gimme-cli/gimme/packages/storage"

	"github.com/gimme-cli/gimme/packages/auth"
	"github.com/gimme-cli/gimme/packages/upload"
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

	err := upload.ValidateFile(file)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	reader, _ := file.Open()
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {
			logrus.Error("Fail to close file")
		}
	}(reader)
	err = upload.ArchiveProcessor(name, version, ctrl.objectStorageManager, reader, file.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	c.Status(http.StatusNoContent)
	return
}

func (ctrl *PackageController) getPackage(c *gin.Context) {
	var filePath string
	file := c.Param("file")
	if len(file) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	filePath += fmt.Sprintf("%v%v", c.Param("package"), file)

	object, err := ctrl.objectStorageManager.GetObject(filePath)
	if err != nil {
		fmt.Println(err)
		c.Status(http.StatusInternalServerError)
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
