package configs

import (
	"fmt"
	"reflect"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

type Configuration struct {
	AppPort       string
	AdminUser     string
	AdminPassword string
	Secret        string
	S3Url         string
	S3Key         string
	S3Secret      string
	S3BucketName  string
	S3Location    string
	S3SSL         bool
	EnableMetrics bool
}

func NewConfig() (*Configuration, *errors.GimmeError) {
	viper.SetConfigName("gimme")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")       // local path
	viper.AddConfigPath("/config") // docker path
	viper.AutomaticEnv()

	viper.SetDefault("port", "8080")
	viper.SetDefault("s3.bucketName", "gimme")
	viper.SetDefault("s3.ssl", true)
	viper.SetDefault("metrics", true)

	err := viper.ReadInConfig()
	if err != nil {
		logrus.Errorf("Unable to read the config file: %s", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("unable to read the config file"))
	}

	config := Configuration{}
	config.AppPort = viper.GetString("port")
	config.AdminUser = viper.GetString("admin.user")
	config.AdminPassword = viper.GetString("admin.password")
	config.Secret = viper.GetString("secret")
	config.S3Url = viper.GetString("s3.url")
	config.S3Key = viper.GetString("s3.key")
	config.S3Secret = viper.GetString("s3.secret")
	config.S3BucketName = viper.GetString("s3.bucketName")
	config.S3Location = viper.GetString("s3.location")
	config.S3SSL = viper.GetBool("s3.ssl")
	config.EnableMetrics = viper.GetBool("metrics")

	err = validateConfig(&config)
	if err != nil {
		logrus.Errorf("NewConfig - Configuration is not valid: %s", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("configuration is not valid: %s", err))
	}

	return &config, nil
}

func validateConfig(config *Configuration) error {
	var configKeys = []string{"AdminUser", "AdminPassword", "Secret", "S3Url", "S3Key", "S3Secret", "S3Location"}
	var throwableError error

	for _, key := range configKeys {
		err := assertConfigKey(config, key)
		if err != nil {
			throwableError = err
			break
		}
	}

	return throwableError
}

func assertConfigKey(config *Configuration, key string) error {
	r := reflect.ValueOf(config)
	f := reflect.Indirect(r).FieldByName(key)
	if f.Len() == 0 {
		return fmt.Errorf("%v is not set", key)
	}
	return nil
}
