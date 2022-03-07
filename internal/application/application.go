package application

import (
	"fmt"
	"log"

	"github.com/gimme-cdn/gimme/api"
	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/gimme"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Application struct {
	config               *configs.Configuration
	authManager          auth.AuthManager
	objectStorageManager storage.ObjectStorageManager
	gimmeService         gimme.GimmeService
}

func NewApplication() Application {
	return Application{}
}

func (app *Application) loadConfig() {
	var err *errors.GimmeError
	app.config, err = configs.NewConfig()
	if err != nil {
		log.Fatalln(err.String())
	}
}

func (app *Application) loadModules() {
	var err *errors.GimmeError
	app.authManager = auth.NewAuthManager(app.config.Secret)

	osmClient, err := storage.NewObjectStorageClient(app.config)
	if err != nil {
		log.Fatalln(err.String())
	}
	app.objectStorageManager = storage.NewObjectStorageManager(osmClient)
	app.gimmeService = gimme.NewGimmeService(app.objectStorageManager)

	err = app.objectStorageManager.CreateBucket(app.config.S3BucketName, app.config.S3Location)
	if err != nil {
		log.Fatalln(err.String())
	}
}

func (app *Application) loadHttpServer() {
	router := gin.Default()
	router.Use(cors.Default())

	api.NewRootController(router)
	api.NewAuthController(router, app.authManager, app.config)
	api.NewPackageController(router, app.authManager, app.gimmeService)

	logrus.Infof("🚀 server started and available on http://localhost:%s", app.config.AppPort)
	err := router.Run(fmt.Sprintf(":%s", app.config.AppPort))
	if err != nil {
		log.Fatalln(err)
	}
}

func (app *Application) Run() {
	app.loadConfig()
	app.loadModules()
	app.loadHttpServer()
}
