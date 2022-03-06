package main

import (
	"fmt"
	"log"

	"github.com/gimme-cdn/gimme/api"

	"github.com/sirupsen/logrus"

	"github.com/gimme-cdn/gimme/packages/auth"
	"github.com/gimme-cdn/gimme/packages/storage"

	"github.com/gimme-cdn/gimme/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	appConfig, err := config.NewConfig()
	if err != nil {
		log.Fatalln(err)
	}

	authManager := auth.NewAuthManager(appConfig.Secret)

	osmClient, err := storage.NewObjectStorageClient(appConfig)
	if err != nil {
		log.Fatalln(err)
	}

	objectStorageManager := storage.NewObjectStorageManager(osmClient)

	err = objectStorageManager.CreateBucket(appConfig.S3BucketName, appConfig.S3Location)
	if err != nil {
		log.Fatalln(err)
	}

	router := gin.Default()
	router.Use(cors.Default())

	api.NewRootController(router)
	api.NewAuthController(router, authManager, appConfig)
	api.NewPackageController(router, authManager, objectStorageManager)

	logrus.Infof("🚀 server started and available on http://localhost:%s", appConfig.AppPort)
	err = router.Run(fmt.Sprintf(":%s", appConfig.AppPort))
	if err != nil {
		log.Fatalln(err)
	}
}
