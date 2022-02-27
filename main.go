package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/gimme-cli/gimme/packages/auth"
	"github.com/gimme-cli/gimme/packages/storage"
	"github.com/gimme-cli/gimme/packages/upload"

	"github.com/gimme-cli/gimme/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	appConfig, err := config.NewConfig()
	if err != nil {
		log.Fatalln(err)
	}

	authManager := auth.NewAuthManager(appConfig.Secret)

	objectStorageManager, err := storage.NewObjectStorageManager(appConfig)
	if err != nil {
		log.Fatalln(err)
	}

	err = objectStorageManager.CreateBucket(appConfig.S3BucketName, appConfig.S3Location)
	if err != nil {
		log.Fatalln(err)
	}

	router := gin.Default()
	router.Use(cors.Default())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to the Gimme CDN API. Here are the available API endpoints :",
			"routes": []gin.H{
				{
					"url":         "<host>/create-token",
					"method":      "POST",
					"description": "Create access tokens",
				}, {
					"url":         "<host>/packages",
					"method":      "POST",
					"description": "Add a package to the CDN",
				}, {
					"url":         "<host>/gimme",
					"method":      "GET",
					"description": "Get a package from the CDN",
				}},
		})
	})

	router.POST("/create-token", gin.BasicAuth(gin.Accounts{
		appConfig.AdminUser: appConfig.AdminPassword,
	}), func(c *gin.Context) {
		var request auth.CreateTokenRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		token, err := authManager.CreateToken(request.Name, request.ExpirationDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	})

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

	logrus.Infof("🚀 server started and available on http://localhost:%s", appConfig.AppPort)
	err = router.Run(fmt.Sprintf(":%s", appConfig.AppPort))
	if err != nil {
		log.Fatalln(err)
	}

}
