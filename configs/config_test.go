package configs

import (
	"fmt"
	"os"
	"testing"

	"github.com/gimme-cdn/gimme/test/utils"

	"github.com/stretchr/testify/assert"
)

func remove(src string) error {
	err := os.Remove(src)
	if err != nil {
		return err
	}
	return nil
}

var confDir = "../test/config"

func init() {
	_ = remove("./gimme.yml")
}

func TestNewConfigFileErr(t *testing.T) {
	_, err := NewConfig()
	assert.Equal(t, "unable to read the config file", err.Error())
}

func TestNewConfig(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "valid.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	confObj, err := NewConfig()
	assert.Equal(t, &Configuration{
		AdminUser:          "test",
		AdminPassword:      "test",
		AppPort:            "8080",
		Secret:             "secret-for-testing-purposes-only",
		S3Url:              "test.s3.url.cloud",
		S3Key:              "s3key",
		S3Secret:           "s3secret",
		S3BucketName:       "gimme",
		S3Location:         "eu-west-1",
		S3SSL:              true,
		EnableMetrics:      true,
		CORSAllowedOrigins: []string{},
		RedisURL:           "",
		TokenFile:          "/tmp/gimme-tokens.enc",
		Cache: CacheConfig{
			Enabled: false,
			Type:    "redis",
			TTL:     3600,
		},
		Auth: AuthConfig{
			Mode: "basic",
			OIDC: OIDCConfig{
				SecureCookies: true,
			},
		},
		TokenStore: TokenStoreConfig{
			Mode: "file",
		},
	}, confObj)
	assert.Nil(t, err)
}

func TestNewConfigValidationErrAdmUsr(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-adm-usr.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: admin.user is not set", err.Error())
}

func TestNewConfigValidationErrAdmPass(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-adm-pass.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: admin.password is not set", err.Error())
}

func TestNewConfigValidationErrSecret(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-secret.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: secret is not set", err.Error())
}

func TestNewConfigValidationErrS3Url(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-url.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: s3.url is not set", err.Error())
}

func TestNewConfigValidationErrS3Key(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-key.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: s3.key is not set", err.Error())
}

func TestNewConfigValidationErrS3Secret(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-secret.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: s3.secret is not set", err.Error())
}

func TestNewConfigValidationErrS3Location(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-location.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: s3.location is not set", err.Error())
}

func TestNewConfigValidationErrCacheInvalidType(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "cache-invalid-type.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: cache.type "memory" is not supported (supported: "redis")`, err.Error())
}

func TestNewConfigValidationErrCacheNoRedisURL(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "cache-no-redis-url.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, "configuration is not valid: redis_url is required when cache.enabled is true", err.Error())
}

func TestNewConfigValidationErrAuthInvalidMode(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "auth-invalid-mode.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: auth.mode "saml" is not supported (supported: "basic", "oidc")`, err.Error())
}

func TestNewConfigValidationErrOIDCNoIssuer(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "oidc-no-issuer.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: auth.oidc.issuer is required when auth.mode is "oidc"`, err.Error())
}

func TestNewConfigValidationErrOIDCNoClientID(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "oidc-no-client-id.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: auth.oidc.client_id is required when auth.mode is "oidc"`, err.Error())
}

func TestNewConfigValidationErrOIDCNoRedirectURL(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "oidc-no-redirect-url.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: auth.oidc.redirect_url is required when auth.mode is "oidc"`, err.Error())
}

func TestNewConfigValidationErrTokenStoreRedisNoRedisURL(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "tokenstore-redis-no-redis-url.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: redis_url is required when tokenStore.mode is "redis"`, err.Error())
}

func TestNewConfigValidationErrTokenStoreInvalidMode(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "tokenstore-invalid-mode.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: tokenStore.mode "database" is not supported (supported: "file", "redis")`, err.Error())
}

// TestNewConfigOIDCValid asserts that a valid OIDC config does not require
// admin credentials (AdminUser / AdminPassword are unused in oidc mode).
func TestNewConfigOIDCValid(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "oidc-valid.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	confObj, err := NewConfig()
	assert.Nil(t, err)
	assert.Equal(t, "oidc", confObj.Auth.Mode)
	assert.Empty(t, confObj.AdminUser)
	assert.Empty(t, confObj.AdminPassword)
	assert.Equal(t, "https://keycloak.example.com/realms/gimme", confObj.Auth.OIDC.Issuer)
	assert.Equal(t, "gimme", confObj.Auth.OIDC.ClientID)
	assert.Equal(t, "https://gimme.example.com/auth/callback", confObj.Auth.OIDC.RedirectURL)
}
