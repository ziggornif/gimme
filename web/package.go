package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gimme-cdn/gimme/internal/domain/gimmecdn/api"

	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gin-gonic/gin"
)

type PackageWebController struct {
	cdnEngine api.CDNEngine
}

type packageSlice struct {
	Name    string
	Version string
}

func (ctrl *PackageWebController) getSlice(pkg string) (*packageSlice, *errors.GimmeError) {
	slice := strings.Split(pkg, "@")
	if len(slice) <= 1 {
		return nil, errors.NewBusinessError(errors.BadRequest, fmt.Errorf("Invalid URL (valid format : GET /gimme/<package>@<version>/<file>)"))

	}

	return &packageSlice{
		Name:    slice[0],
		Version: slice[1],
	}, nil
}

func (ctrl *PackageWebController) getHTMLPackage(c *gin.Context, pkg string, name string, version string) {
	files := ctrl.cdnEngine.GetPackageFiles(name, version)
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

func (ctrl *PackageWebController) getPackageFolder(c *gin.Context) {
	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.String()})
		return
	}
	ctrl.getHTMLPackage(c, c.Param("package"), pkg.Name, pkg.Version)
	return
}

// NewPackageWebController - Create web controller
func NewPackageWebController(router *gin.Engine, cdnEngine api.CDNEngine) {
	controller := PackageWebController{
		cdnEngine,
	}

	router.GET("/gimme/:package", controller.getPackageFolder)
}
