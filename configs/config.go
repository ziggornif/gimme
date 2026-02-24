package configs

import (
	"fmt"

	"github.com/gimme-cdn/gimme/internal/errors"

	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

type CacheConfig struct {
	Enabled  bool
	Type     string // "redis" ; "memory" reserved for future use
	TTL      int    // seconds
	RedisURL string
}

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
	Cache         CacheConfig
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
	viper.SetDefault("cache.enabled", false)
	viper.SetDefault("cache.type", "redis")
	viper.SetDefault("cache.ttl", 3600)
	viper.SetDefault("cache.redis_url", "redis://localhost:6379")

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
	config.Cache = CacheConfig{
		Enabled:  viper.GetBool("cache.enabled"),
		Type:     viper.GetString("cache.type"),
		TTL:      viper.GetInt("cache.ttl"),
		RedisURL: viper.GetString("cache.redis_url"),
	}

	if err := validateConfig(&config); err != nil {
		logrus.Errorf("NewConfig - Configuration is not valid: %s", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("configuration is not valid: %s", err))
	}

	return &config, nil
}

func validateConfig(config *Configuration) error {
	if config.AdminUser == "" {
		return fmt.Errorf("AdminUser is not set")
	}
	if config.AdminPassword == "" {
		return fmt.Errorf("AdminPassword is not set")
	}
	if config.Secret == "" {
		return fmt.Errorf("Secret is not set")
	}
	if config.S3Url == "" {
		return fmt.Errorf("S3Url is not set")
	}
	if config.S3Key == "" {
		return fmt.Errorf("S3Key is not set")
	}
	if config.S3Secret == "" {
		return fmt.Errorf("S3Secret is not set")
	}
	if config.S3Location == "" {
		return fmt.Errorf("S3Location is not set")
	}
	if config.Cache.Enabled {
		if config.Cache.Type != "redis" {
			return fmt.Errorf("cache.type %q is not supported (supported: \"redis\")", config.Cache.Type)
		}
		if config.Cache.RedisURL == "" {
			return fmt.Errorf("cache.redis_url is required when cache is enabled")
		}
	}
	return nil
}
