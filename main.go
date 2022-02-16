package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/drouian-m/gimme/auth"
	"github.com/drouian-m/gimme/config"
	"github.com/drouian-m/gimme/storage"
	"github.com/drouian-m/gimme/upload"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	appConfig, err := config.NewConfig()
	if err != nil {
		log.Fatalln(err)
	}

	authManager := auth.NewAuthManager(appConfig)

	objectStorageManager, err := storage.NewObjectStorageManager(storage.MinioConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "test",
		SecretAccessKey: "golangtest",
		UseSSL:          false,
	})
	if err != nil {
		log.Fatalln(err)
	}

	err = objectStorageManager.CreateBucket("gimme", "eu-west-1")
	if err != nil {
		log.Fatalln(err)
	}

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Use(cors.Default())

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

	err = router.Run(":8080")
	if err != nil {
		log.Fatalln(err)
	}
}
