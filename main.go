package main

import (
	"fmt"
	"log"

	"github.com/gimme-cdn/gimme/internal/gimme"

	"github.com/gimme-cdn/gimme/api"
	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	var bootErr *errors.GimmeError
	appConfig, bootErr := configs.NewConfig()
	if bootErr != nil {
		log.Fatalln(bootErr)
	}

	authManager := auth.NewAuthManager(appConfig.Secret)

	osmClient, bootErr := storage.NewObjectStorageClient(appConfig)
	if bootErr != nil {
		log.Fatalln(bootErr)
	}
	objectStorageManager := storage.NewObjectStorageManager(osmClient)
	gimmeService := gimme.NewGimmeService(objectStorageManager)

	bootErr = objectStorageManager.CreateBucket(appConfig.S3BucketName, appConfig.S3Location)
	if bootErr != nil {
		log.Fatalln(bootErr)
	}

	router := gin.Default()
	router.Use(cors.Default())

	api.NewRootController(router)
	api.NewAuthController(router, authManager, appConfig)
	api.NewPackageController(router, authManager, gimmeService)

	logrus.Infof("🚀 server started and available on http://localhost:%s", appConfig.AppPort)
	err := router.Run(fmt.Sprintf(":%s", appConfig.AppPort))
	if err != nil {
		log.Fatalln(err)
	}
}
