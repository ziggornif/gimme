package config

import (
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

type Configuration struct {
	AdminUser     string `json:"adminUser" mapstructure:"GIMME_ADMIN_USER"`
	AdminPassword string `json:"adminPassword" mapstructure:"GIMME_ADMIN_PASSWORD"`
	Secret        string `json:"secret" mapstructure:"GIMME_SECRET"`
	S3Url         string `json:"S3Url" mapstructure:"GIMME_SECRET"`
	S3Key         string `json:"S3Key" mapstructure:"GIMME_S3_URL"`
	S3Secret      string `json:"s3Secret" mapstructure:"GIMME_S3_SECRET"`
	S3BucketName  string `json:"s3BucketName" mapstructure:"GIMME_S3_BUCKET_NAME"`
	S3Location    string `json:"s3Location" mapstructure:"GIMME_S3_LOCATION"`
	S3SSL         bool   `json:"s3SSL" mapstructure:"GIMME_S3_SSL"`
}

func NewConfig() (*Configuration, error) {
	viper.SetConfigName("gimme")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	viper.SetDefault("s3.bucketName", "gimme")
	viper.SetDefault("s3.ssl", true)

	err := viper.ReadInConfig()
	//if err != nil {
	//	logrus.Errorf("Unable to read the config file: %s", err)
	//	return nil, err
	//}

	config := Configuration{}

	if err != nil {
		err = viper.Unmarshal(&config)
		if err != nil {
			return nil, err
		}
	} else {
		config.AdminUser = viper.GetString("admin.user")
		config.AdminPassword = viper.GetString("admin.password")
		config.Secret = viper.GetString("secret")
		config.S3Url = viper.GetString("s3.url")
		config.S3Key = viper.GetString("s3.key")
		config.S3Secret = viper.GetString("s3.secret")
		config.S3BucketName = viper.GetString("s3.bucketName")
		config.S3Location = viper.GetString("s3.location")
		config.S3SSL = viper.GetBool("s3.ssl")
	}

	err = validateConfig(&config)
	if err != nil {
		logrus.Errorf("Configuration is not valid: %s", err)
		return nil, err
	}

	return &config, nil
}

func validateConfig(config *Configuration) error {
	if len(config.AdminUser) == 0 {
		return errors.New("AdminUser is not set")
	}

	if len(config.AdminPassword) == 0 {
		return errors.New("AdminPassword is not set")
	}

	if len(config.Secret) == 0 {
		return errors.New("Secret is not set")
	}

	if len(config.S3Url) == 0 {
		return errors.New("S3Url is not set")
	}

	if len(config.S3Key) == 0 {
		return errors.New("S3Key is not set")
	}

	if len(config.S3Secret) == 0 {
		return errors.New("S3Secret is not set")
	}

	if len(config.S3Location) == 0 {
		return errors.New("S3Location is not set")
	}

	return nil
}
