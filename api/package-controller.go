package api

import (
	stderrors "errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"github.com/ziggornif/gimme/internal/archive_validator"
	"github.com/ziggornif/gimme/internal/auth"
	"github.com/ziggornif/gimme/internal/content"
	"github.com/ziggornif/gimme/internal/errors"
)

type PackageController struct {
	authManager    *auth.AuthManager
	contentService content.ContentService
}

type packageSlice struct {
	Name    string
	Version string
}

func (ctrl *PackageController) getSlice(pkg string) (*packageSlice, *errors.GimmeError) {
	const invalidURLMsg = "invalid URL (valid format: /gimme/<package>@<version>/<file>)"

	slice := strings.SplitN(pkg, "@", 2)
	if len(slice) < 2 {
		return nil, errors.NewBusinessError(errors.BadRequest, stderrors.New(invalidURLMsg))
	}

	name := slice[0]
	version := slice[1]

	if name == "" {
		return nil, errors.NewBusinessError(errors.BadRequest, stderrors.New("package name must not be empty — "+invalidURLMsg))
	}
	if version == "" {
		return nil, errors.NewBusinessError(errors.BadRequest, stderrors.New("package version must not be empty — "+invalidURLMsg))
	}

	return &packageSlice{
		Name:    name,
		Version: version,
	}, nil
}

func (ctrl *PackageController) getHTMLPackage(c *gin.Context, pkg string, name string, version string) {
	files, _ := ctrl.contentService.GetFiles(c.Request.Context(), name, version)
	if len(files) == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	c.HTML(http.StatusOK, "package.tmpl", gin.H{
		"packageName": pkg,
		"files":       files,
	})
}

func (ctrl *PackageController) createPackage(c *gin.Context) {
	maxSize := ctrl.contentService.UploadLimits().MaxSize
	if maxSize > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
	}

	file, formErr := c.FormFile("file")
	if formErr != nil {
		var maxBytesErr *http.MaxBytesError
		if stderrors.As(formErr, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("upload exceeds the maximum request size of %d bytes (upload.max_size)", maxSize)})
			return
		}
		if !stderrors.Is(formErr, http.ErrMissingFile) {
			var pathErr *fs.PathError
			if stderrors.As(formErr, &pathErr) {
				logrus.Errorf("[PackageController] createPackage - failed to buffer the uploaded form: %v", formErr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process the uploaded file"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("malformed upload request: %s", formErr)})
			return
		}
	}
	name := c.PostForm("name")
	version := c.PostForm("version")

	validationErr := archive_validator.ValidateFile(file)
	if validationErr != nil {
		c.JSON(validationErr.GetHTTPCode(), gin.H{"error": validationErr.Error()})
		return
	}

	reader, openErr := file.Open()
	if openErr != nil {
		logrus.Errorf("[PackageController] createPackage - failed to open uploaded file: %v", openErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read uploaded file"})
		return
	}
	defer func(reader multipart.File) {
		err := reader.Close()
		if err != nil {
			logrus.Error("Fail to close file")
		}
	}(reader)

	uploadErr := ctrl.contentService.CreatePackage(c.Request.Context(), name, version, reader, file.Size)

	if uploadErr != nil {
		c.JSON(uploadErr.GetHTTPCode(), gin.H{"error": uploadErr.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

func cacheControlHeader(version string) string {
	if content.IsPinnedVersion(version) {
		return "public, max-age=31536000, immutable"
	}
	return "public, max-age=300"
}

// etagMatches reports whether an If-None-Match header value matches etag.
// Both sides are compared weakly: the W/ prefix is ignored.
func etagMatches(ifNoneMatch string, etag string) bool {
	if ifNoneMatch == "" || etag == "" {
		return false
	}

	etag = strings.TrimPrefix(etag, "W/")
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}

	return false
}

// notModified reports whether the request may be answered with 304.
func notModified(r *http.Request, etag string, lastModified time.Time) bool {
	if ifNoneMatch := r.Header.Get("If-None-Match"); ifNoneMatch != "" {
		return etagMatches(ifNoneMatch, etag)
	}

	ifModifiedSince := r.Header.Get("If-Modified-Since")
	if ifModifiedSince == "" || lastModified.IsZero() {
		return false
	}

	modifiedSince, err := http.ParseTime(ifModifiedSince)
	if err != nil {
		return false
	}

	return !lastModified.Truncate(time.Second).After(modifiedSince)
}

func (ctrl *PackageController) getPackage(c *gin.Context) {
	file := c.Param("file")

	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.Error()})
		return
	}

	if file == "/" {
		ctrl.getHTMLPackage(c, c.Param("package"), pkg.Name, pkg.Version)
		return
	}

	object, err := ctrl.contentService.GetFile(c.Request.Context(), pkg.Name, pkg.Version, file)
	if err != nil {
		c.Header("Cache-Control", "no-store")
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.Error()})
		return
	}
	defer func(object *minio.Object) {
		err := object.Close()
		if err != nil {
			logrus.Error("getPackage - Fail to close file")
		}
	}(object)

	infos, statErr := object.Stat()
	if statErr != nil {
		logrus.Errorf("getPackage - Fail to stat object: %v", statErr)
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusNotFound, gin.H{"error": "object not found"})
		return
	}

	etag := ""
	if infos.ETag != "" {
		etag = `"` + strings.Trim(infos.ETag, `"`) + `"`
		c.Header("ETag", etag)
	}
	if !infos.LastModified.IsZero() {
		c.Header("Last-Modified", infos.LastModified.UTC().Format(http.TimeFormat))
	}
	c.Header("Cache-Control", cacheControlHeader(pkg.Version))

	if notModified(c.Request, etag, infos.LastModified) {
		c.Status(http.StatusNotModified)
		return
	}

	c.DataFromReader(http.StatusOK, infos.Size, infos.ContentType, object, nil)
}

func (ctrl *PackageController) getPackageFolder(c *gin.Context) {
	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.Error()})
		return
	}
	ctrl.getHTMLPackage(c, c.Param("package"), pkg.Name, pkg.Version)
}

func (ctrl *PackageController) deletePackage(c *gin.Context) {
	pkg, err := ctrl.getSlice(c.Param("package"))
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.Error()})
		return
	}

	err = ctrl.contentService.DeletePackage(c.Request.Context(), pkg.Name, pkg.Version)
	if err != nil {
		c.JSON(err.GetHTTPCode(), gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// NewPackageController - Create controller
func NewPackageController(router *gin.Engine, authManager *auth.AuthManager, contentService content.ContentService) {
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
