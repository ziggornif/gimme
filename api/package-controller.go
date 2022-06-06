package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/content"
	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

type PackageController struct {
	authManager    auth.AuthManager
	contentService content.ContentService
}

type packageSlice struct {
	Name    string
	Version string
}

func (ctrl *PackageController) getSlice(pkg string) (*packageSlice, *errors.GimmeError) {
	slice := strings.Split(pkg, "@")
	if len(slice) <= 1 {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("Invalid URL (valid format : GET /gimme/<package>@<version>/<file>)"))

	}

	return &packageSlice{
		Name:    slice[0],
		Version: slice[1],
	}, nil
}

func (ctrl *PackageController) getHTMLPackage(c *gin.Context, pkg string, name string, version string) {
	files, _ := ctrl.contentService.GetFiles(name, version)
	if len(files) == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	c.HTML(http.StatusOK, "package.tmpl", gin.H{
		"packageName": pkg,
		"files":       files,
	})
	return
}

func (ctrl *PackageController) createPackage(c *gin.Context) {
	file, _ := c.FormFile("file")
	name := c.PostForm("name")
	version := c.PostForm("version")

	uploadErr := ctrl.contentService.UploadPackage(name, version, file)

	if uploadErr != nil {
		c.JSON(uploadErr.GetHTTPCode(), gin.H{"error": uploadErr.String()})
		return
	}

	c.Status(http.StatusCreated)
	return
}

func (ctrl *PackageController) getPackage(c *gin.Context) {
	file := c.Param("file")

	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}

	if file == "/" {
		ctrl.getHTMLPackage(c, c.Param("package"), pkg.Name, pkg.Version)
		return
	}

	object, err := ctrl.contentService.GetFile(pkg.Name, pkg.Version, file)
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}
	defer func(object *minio.Object) {
		err := object.Close()
		if err != nil {
			logrus.Error("getPackage - Fail to close file")
		}
	}(object)

	infos, _ := object.Stat()
	if infos.Size == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	c.DataFromReader(http.StatusOK, infos.Size, infos.ContentType, object, nil)
}

func (ctrl *PackageController) getPackageFolder(c *gin.Context) {
	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}
	ctrl.getHTMLPackage(c, c.Param("package"), pkg.Name, pkg.Version)
	return
}

func (ctrl *PackageController) deletePackage(c *gin.Context) {
	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}

	err = ctrl.contentService.DeletePackage(pkg.Name, pkg.Version)
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}

	c.Status(http.StatusNoContent)
	return
}

// NewPackageController - Create controller
func NewPackageController(router *gin.Engine, authManager auth.AuthManager, contentService content.ContentService) {
	controller := PackageController{
		authManager,
		contentService,
	}

	router.GET("/gimme", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})
	router.GET("/gimme/:package", controller.getPackageFolder)
	router.GET("/gimme/:package/*file", controller.getPackage)
	router.POST("/packages", authManager.AuthenticateMiddleware, controller.createPackage)
	router.DELETE("/packages/:package", authManager.AuthenticateMiddleware, controller.deletePackage)
}
