package configs

import (
	"fmt"
	"strings"

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

// OIDCConfig holds the configuration for the OIDC provider.
// Only used when AuthConfig.Mode is "oidc".
type OIDCConfig struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	SecureCookies bool // set to true when Gimme is served over HTTPS (default: true)
}

// AuthConfig controls the authentication mode for the admin interface.
// Mode "basic" (default) uses HTTP Basic Auth with admin credentials.
// Mode "oidc" delegates authentication to an external OIDC provider.
type AuthConfig struct {
	Mode string // "basic" (default) | "oidc"
	OIDC OIDCConfig
}

type Configuration struct {
	AppPort            string
	AdminUser          string
	AdminPassword      string
	Secret             string
	S3Url              string
	S3Key              string
	S3Secret           string
	S3BucketName       string
	S3Location         string
	S3SSL              bool
	EnableMetrics      bool
	CORSAllowedOrigins []string
	Cache              CacheConfig
	Auth               AuthConfig
}

func NewConfig() (*Configuration, *errors.GimmeError) {
	viper.SetConfigName("gimme")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")       // local path
	viper.AddConfigPath("/config") // docker path
	// Enable env var overrides with GIMME_ prefix.
	// e.g. GIMME_SECRET overrides config key "secret",
	//      GIMME_ADMIN_USER overrides "admin.user",
	//      GIMME_S3_KEY overrides "s3.key", etc.
	viper.SetEnvPrefix("GIMME")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("port", "8080")
	viper.SetDefault("s3.bucketName", "gimme")
	viper.SetDefault("s3.ssl", true)
	viper.SetDefault("metrics", true)
	viper.SetDefault("cors.allowed_origins", []string{})
	viper.SetDefault("cache.enabled", false)
	viper.SetDefault("cache.type", "redis")
	viper.SetDefault("cache.ttl", 3600)
	viper.SetDefault("cache.redis_url", "redis://localhost:6379")
	viper.SetDefault("auth.mode", "basic")
	viper.SetDefault("auth.oidc.secure_cookies", true)

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
	config.CORSAllowedOrigins = viper.GetStringSlice("cors.allowed_origins")
	config.Cache = CacheConfig{
		Enabled:  viper.GetBool("cache.enabled"),
		Type:     viper.GetString("cache.type"),
		TTL:      viper.GetInt("cache.ttl"),
		RedisURL: viper.GetString("cache.redis_url"),
	}
	config.Auth = AuthConfig{
		Mode: viper.GetString("auth.mode"),
		OIDC: OIDCConfig{
			Issuer:        viper.GetString("auth.oidc.issuer"),
			ClientID:      viper.GetString("auth.oidc.client_id"),
			ClientSecret:  viper.GetString("auth.oidc.client_secret"),
			RedirectURL:   viper.GetString("auth.oidc.redirect_url"),
			SecureCookies: viper.GetBool("auth.oidc.secure_cookies"),
		},
	}

	if err := validateConfig(&config); err != nil {
		logrus.Errorf("NewConfig - Configuration is not valid: %s", err)
		return nil, errors.NewBusinessError(errors.InternalError, fmt.Errorf("configuration is not valid: %s", err))
	}

	return &config, nil
}

func validateConfig(config *Configuration) error {
	// Admin credentials are only required in "basic" mode.
	// In "oidc" mode, authentication is fully delegated to the OIDC provider
	// and admin credentials are not used.
	if config.Auth.Mode == "basic" {
		if config.AdminUser == "" {
			return fmt.Errorf("AdminUser is not set")
		}
		if config.AdminPassword == "" {
			return fmt.Errorf("AdminPassword is not set")
		}
	}
	if config.Secret == "" {
		return fmt.Errorf("Secret is not set")
	}
	if len(config.Secret) < 32 {
		return fmt.Errorf("Secret must be at least 32 characters long (got %d)", len(config.Secret))
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
	// Redis is always required for token persistence (opaque tokens stored in Redis).
	// A blank URL would cause a fatal error at startup, so we validate it here to
	// give a clear config-level error message rather than a cryptic connection failure.
	if config.Cache.RedisURL == "" {
		return fmt.Errorf("cache.redis_url is required (Redis is used for persistent token storage)")
	}
	if config.Cache.Enabled {
		if config.Cache.Type != "redis" {
			return fmt.Errorf("cache.type %q is not supported (supported: \"redis\")", config.Cache.Type)
		}
	}
	switch config.Auth.Mode {
	case "basic":
		// no additional fields required
	case "oidc":
		if config.Auth.OIDC.Issuer == "" {
			return fmt.Errorf("auth.oidc.issuer is required when auth.mode is \"oidc\"")
		}
		if config.Auth.OIDC.ClientID == "" {
			return fmt.Errorf("auth.oidc.client_id is required when auth.mode is \"oidc\"")
		}
		if config.Auth.OIDC.RedirectURL == "" {
			return fmt.Errorf("auth.oidc.redirect_url is required when auth.mode is \"oidc\"")
		}
		if config.Auth.OIDC.ClientSecret == "" {
			logrus.Warn("auth.oidc.client_secret is empty — token exchange may fail with confidential clients")
		}
	default:
		return fmt.Errorf("auth.mode %q is not supported (supported: \"basic\", \"oidc\")", config.Auth.Mode)
	}
	return nil
}
