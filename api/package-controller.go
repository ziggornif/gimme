package api

import (
	"net/http"
	"strings"

	"github.com/gimme-cdn/gimme/internal/gimme"

	"github.com/gimme-cdn/gimme/internal/auth"

	"github.com/gin-gonic/gin"
)

type PackageController struct {
	authManager  auth.AuthManager
	gimmeService gimme.GimmeService
}

func (ctrl *PackageController) getHTMLPackage(c *gin.Context, pkg string, name string, version string) {
	files, _ := ctrl.gimmeService.GetFiles(name, version)
	if len(files) == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"packageName": pkg,
		"files":       files,
	})
	return
}

func (ctrl *PackageController) createPackage(c *gin.Context) {
	file, _ := c.FormFile("file")
	name := c.PostForm("name")
	version := c.PostForm("version")

	uploadErr := ctrl.gimmeService.UploadPackage(name, version, file)

	if uploadErr != nil {
		c.JSON(uploadErr.GetHTTPCode(), gin.H{"error": uploadErr.String()})
		return
	}

	c.Status(http.StatusNoContent)
	return
}

func (ctrl *PackageController) getPackage(c *gin.Context) {
	file := c.Param("file")

	slice := strings.Split(c.Param("package"), "@")
	if file == "/" {
		ctrl.getHTMLPackage(c, c.Param("package"), slice[0], slice[1])
		return
	}

	object, err := ctrl.gimmeService.GetFile(slice[0], slice[1], file)
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

func (ctrl *PackageController) getPackageFolder(c *gin.Context) {
	slice := strings.Split(c.Param("package"), "@")
	ctrl.getHTMLPackage(c, c.Param("package"), slice[0], slice[1])
	return
}

// NewPackageController - Create controller
func NewPackageController(router *gin.Engine, authManager auth.AuthManager, gimmeService gimme.GimmeService) {
	controller := PackageController{
		authManager,
		gimmeService,
	}

	router.GET("/gimme/:package", controller.getPackageFolder)
	router.GET("/gimme/:package/*file", controller.getPackage)
	router.POST("/packages", authManager.AuthenticateMiddleware, controller.createPackage)
}
