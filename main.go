package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/drouian-m/gimme/packages/auth"
	"github.com/drouian-m/gimme/packages/storage"
	"github.com/drouian-m/gimme/packages/upload"

	"github.com/drouian-m/gimme/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	appConfig, err := config.NewConfig()
	if err != nil {
		log.Fatalln(err)
	}

	authManager := auth.NewAuthManager(appConfig)

	objectStorageManager, err := storage.NewObjectStorageManager(appConfig)
	if err != nil {
		log.Fatalln(err)
	}

	err = objectStorageManager.CreateBucket(appConfig.S3BucketName, appConfig.S3Location)
	if err != nil {
		log.Fatalln(err)
	}

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Use(cors.Default())

	router.GET("/gimme/:package/*file", func(c *gin.Context) {
		var filePath string
		file := c.Param("file")
		if len(file) > 0 {
			filePath += fmt.Sprintf("%v%v", c.Param("package"), file)
		} else {
			filePath = c.Param("package")
		}
		object, err := objectStorageManager.GetObject(filePath)
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
	})

	router.POST("/packages", authManager.AuthenticateMiddleware, func(c *gin.Context) {
		file, _ := c.FormFile("file")
		name := c.PostForm("name")
		version := c.PostForm("version")

		err = upload.ValidateFile(file)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		err = upload.ArchiveProcessor(name, version, objectStorageManager, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		c.Status(http.StatusNoContent)
		return
	})

	err = router.Run(":8080")
	if err != nil {
		log.Fatalln(err)
	}
}
