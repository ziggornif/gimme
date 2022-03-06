package main

import (
	"fmt"
	"log"

	"github.com/gimme-cdn/gimme/errors"

	"github.com/gimme-cdn/gimme/api"

	"github.com/sirupsen/logrus"

	"github.com/gimme-cdn/gimme/packages/auth"
	"github.com/gimme-cdn/gimme/packages/storage"

	"github.com/gimme-cdn/gimme/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	var bootErr *errors.GimmeError
	appConfig, bootErr := config.NewConfig()
	if bootErr != nil {
		log.Fatalln(bootErr)
	}

	authManager := auth.NewAuthManager(appConfig.Secret)

	osmClient, bootErr := storage.NewObjectStorageClient(appConfig)
	if bootErr != nil {
		log.Fatalln(bootErr)
	}
	objectStorageManager := storage.NewObjectStorageManager(osmClient)

	bootErr = objectStorageManager.CreateBucket(appConfig.S3BucketName, appConfig.S3Location)
	if bootErr != nil {
		log.Fatalln(bootErr)
	}

	router := gin.Default()
	router.Use(cors.Default())

	api.NewRootController(router)
	api.NewAuthController(router, authManager, appConfig)
	api.NewPackageController(router, authManager, objectStorageManager)

	logrus.Infof("🚀 server started and available on http://localhost:%s", appConfig.AppPort)
	err := router.Run(fmt.Sprintf(":%s", appConfig.AppPort))
	if err != nil {
		log.Fatalln(err)
	}
}
