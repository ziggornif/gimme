package configs

import (
	"fmt"
	"os"
	"testing"

	"github.com/ziggornif/gimme/test/utils"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIMME_SECRET", "secret-for-testing-purposes-only")
	t.Setenv("GIMME_ADMIN_USER", "envadmin")
	t.Setenv("GIMME_ADMIN_PASSWORD", "envpass")
	t.Setenv("GIMME_S3_URL", "env.s3.url.cloud")
	t.Setenv("GIMME_S3_KEY", "envkey")
	t.Setenv("GIMME_S3_SECRET", "envsecret")
	t.Setenv("GIMME_S3_LOCATION", "eu-west-3")
}

func remove(src string) error {
	err := os.Remove(src)
	if err != nil {
		return err
	}
	return nil
}

var confDir = "../test/config"

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected int64
		wantErr  bool
	}{
		{name: "raw bytes", raw: "104857600", expected: 104857600},
		{name: "megabytes", raw: "100MB", expected: 104857600},
		{name: "lowercase", raw: "100mb", expected: 104857600},
		{name: "whitespace", raw: "100 MB", expected: 104857600},
		{name: "five hundred megabytes", raw: "500MB", expected: 524288000},
		{name: "gigabyte", raw: "1GB", expected: 1073741824},
		{name: "explicit binary", raw: "1MiB", expected: 1048576},
		{name: "fractional", raw: "1.5GB", expected: 1610612736},
		{name: "zero", raw: "0", expected: 0},
		{name: "empty", raw: "", wantErr: true},
		{name: "letters", raw: "abc", wantErr: true},
		{name: "unknown unit", raw: "100 apples", wantErr: true},
		{name: "missing value", raw: "MB", wantErr: true},
		{name: "overflow", raw: "9999999999GB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseSize(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), fmt.Sprintf("%q", tt.raw))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func init() {
	_ = remove("./gimme.yml")
}

func TestNewConfigNoFileNoEnv(t *testing.T) {
	viper.Reset()
	_, err := NewConfig()
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "configuration is not valid:")
	assert.Contains(t, err.Error(), "secret is not set")
	assert.Contains(t, err.Error(), "s3.url is not set")
	assert.NotContains(t, err.Error(), "unable to read the config file")
}

func TestNewConfigCompressionEnv(t *testing.T) {
	viper.Reset()
	setRequiredEnv(t)
	t.Setenv("GIMME_COMPRESSION_ENABLED", "false")
	config, err := NewConfig()
	require.Nil(t, err)
	assert.False(t, config.Compression.Enabled)
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
		Compression: CompressionConfig{Enabled: true},
		Auth: AuthConfig{
			Mode: "basic",
			OIDC: OIDCConfig{
				SecureCookies: true,
			},
		},
		TokenStore: TokenStoreConfig{
			Mode: "file",
		},
		Upload: UploadConfig{
			MaxSize:             Size{Bytes: 104857600, raw: "100MB"},
			MaxEntries:          10000,
			MaxUncompressedSize: Size{Bytes: 524288000, raw: "500MB"},
		},
	}, confObj)
	assert.Nil(t, err)
}

func TestNewConfigUploadDefaults(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "valid.yml"), "./gimme.yml")
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme.yml"))
		viper.Reset()
	})

	config, err := NewConfig()
	require.Nil(t, err)
	assert.Equal(t, int64(104857600), config.Upload.MaxSize.Bytes)
	assert.Equal(t, 10000, config.Upload.MaxEntries)
	assert.Equal(t, int64(524288000), config.Upload.MaxUncompressedSize.Bytes)
}

func TestNewConfigUploadUnits(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "upload-units.yml"), "./gimme.yml")
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme.yml"))
		viper.Reset()
	})

	config, err := NewConfig()
	require.Nil(t, err)
	assert.Equal(t, int64(104857600), config.Upload.MaxSize.Bytes)
	assert.Equal(t, 10000, config.Upload.MaxEntries)
	assert.Equal(t, int64(524288000), config.Upload.MaxUncompressedSize.Bytes)
}

func TestNewConfigUploadSizeInvalid(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "upload-size-invalid.yml"), "./gimme.yml")
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme.yml"))
		viper.Reset()
	})

	_, err := NewConfig()
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), `upload.max_size is not a valid size: "12 apples"`)
	assert.NotContains(t, err.Error(), "upload.max_size must be greater than 0")
}

func TestNewConfigUploadSizeEnv(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "valid.yml"), "./gimme.yml")
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme.yml"))
		viper.Reset()
	})
	t.Setenv("GIMME_UPLOAD_MAX_SIZE", "250MB")

	config, err := NewConfig()
	require.Nil(t, err)
	assert.Equal(t, int64(262144000), config.Upload.MaxSize.Bytes)
}

func TestNewConfigUploadLimitsInvalid(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "upload-invalid.yml"), "./gimme.yml")
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme.yml"))
		viper.Reset()
	})

	_, err := NewConfig()
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "\n  - upload.max_size must be greater than 0")
	assert.Contains(t, err.Error(), "\n  - upload.max_entries must be greater than 0")
	assert.Contains(t, err.Error(), "\n  - upload.max_uncompressed_size must be greater than 0")
}

func TestNewConfigUploadLimits(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "upload-valid.yml"), "./gimme.yml")
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme.yml"))
		viper.Reset()
	})

	config, err := NewConfig()
	require.Nil(t, err)
	assert.Equal(t, int64(123456), config.Upload.MaxSize.Bytes)
	assert.Equal(t, 321, config.Upload.MaxEntries)
	assert.Equal(t, int64(654321), config.Upload.MaxUncompressedSize.Bytes)
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

	assert.Equal(t, `configuration is not valid: tokenStore.mode "database" is not supported (supported: "file", "redis", "postgres")`, err.Error())
}

func TestNewConfigValidationErrTokenStorePostgresNoDSN(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "tokenstore-postgres-no-dsn.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	assert.Equal(t, `configuration is not valid: tokenStore.pg_url is required when tokenStore.mode is "postgres"`, err.Error())
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

// TestNewConfigValidationReportsEveryInvalidField asserts that a configuration
// with several problems names all of them in a single error, so first-run setup
// is one pass instead of one restart per missing field.
func TestNewConfigValidationReportsEveryInvalidField(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "multiple-invalid.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	require.NotNil(t, err)
	assert.Equal(t, "configuration is not valid:\n"+
		"  - secret must be at least 32 bytes long (got 8)\n"+
		"  - s3.url is not set\n"+
		"  - s3.key is not set\n"+
		"  - s3.secret is not set", err.Error())
}

// TestNewConfigValidationSkipsFieldsThatDoNotApply asserts the mode-dependent
// checks still hold when several problems are reported: in oidc mode the
// missing admin credentials are not part of the list.
func TestNewConfigValidationSkipsFieldsThatDoNotApply(t *testing.T) {
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "oidc-multiple-invalid.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	require.NotNil(t, err)
	assert.Equal(t, "configuration is not valid:\n"+
		"  - auth.oidc.issuer is required when auth.mode is \"oidc\"\n"+
		"  - auth.oidc.client_id is required when auth.mode is \"oidc\"\n"+
		"  - auth.oidc.redirect_url is required when auth.mode is \"oidc\"", err.Error())
	assert.NotContains(t, err.Error(), "admin.")
}

// TestNewConfigShippedExampleIsRejected asserts that the installation template
// shipped in every release archive refuses to start as-is, so that no instance
// runs with the placeholder secret published here.
func TestNewConfigShippedExampleIsRejected(t *testing.T) {
	utils.CopyFile("../gimme.example.yml", "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	_, err := NewConfig()

	// The wording is not asserted: #64 will change it.
	require.NotNil(t, err, "the shipped example must not be a runnable configuration")
	assert.Contains(t, err.Error(), "secret")
}

// TestNewConfigManagedS3ExampleIsAccepted asserts the opposite for a
// demonstration stack: it must start as shipped, since the user is asked to
// supply storage credentials and nothing else.
func TestNewConfigManagedS3ExampleIsAccepted(t *testing.T) {
	utils.CopyFile("../examples/deployment/docker-compose/with-managed-s3/gimme.yml", "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	confObj, err := NewConfig()

	require.Nil(t, err, "the demonstration stack must start as shipped")
	assert.GreaterOrEqual(t, len(confObj.Secret), 32)
}

func TestNewConfigEnvOnly(t *testing.T) {
	viper.Reset()
	setRequiredEnv(t)

	confObj, err := NewConfig()

	require.Nil(t, err, "an env-only configuration must be accepted")
	assert.Equal(t, "envadmin", confObj.AdminUser)
	assert.Equal(t, "envpass", confObj.AdminPassword)
	assert.Equal(t, "secret-for-testing-purposes-only", confObj.Secret)
	assert.Equal(t, "env.s3.url.cloud", confObj.S3Url)
	assert.Equal(t, "envkey", confObj.S3Key)
	assert.Equal(t, "envsecret", confObj.S3Secret)
	assert.Equal(t, "eu-west-3", confObj.S3Location)
	assert.Equal(t, "8080", confObj.AppPort)
	assert.Equal(t, "gimme", confObj.S3BucketName)
	assert.Equal(t, "file", confObj.TokenStore.Mode)
}

// The compiled binary is named gimme and lands next to the config search path,
// so an extension-less file of that name must never be read as configuration.
func TestNewConfigEnvOnlyIgnoresExtensionlessFile(t *testing.T) {
	viper.Reset()
	require.NoError(t, os.WriteFile("./gimme", []byte("\x7fELF\x02\x01\x01\x00\xff\xfe binary, not yaml"), 0o600))
	t.Cleanup(func() {
		assert.NoError(t, remove("./gimme"))
		viper.Reset()
	})
	setRequiredEnv(t)

	confObj, err := NewConfig()

	require.Nil(t, err, "a file named gimme with no extension is not a config file")
	assert.Equal(t, "envadmin", confObj.AdminUser)
}

func TestNewConfigEnvOnlyMissingField(t *testing.T) {
	viper.Reset()
	setRequiredEnv(t)
	t.Setenv("GIMME_S3_LOCATION", "")

	_, err := NewConfig()

	require.NotNil(t, err)
	assert.Equal(t, "configuration is not valid: s3.location is not set", err.Error())
}

func TestNewConfigMalformedFileErr(t *testing.T) {
	viper.Reset()
	setRequiredEnv(t)
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "malformed.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()

	_, err := NewConfig()

	require.NotNil(t, err, "a malformed config file must not be silently ignored")
	assert.Equal(t, "unable to read the config file", err.Error())
}

func TestNewConfigCORSOriginsFromEnv(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"comma", "https://a.example.com,https://b.example.com", []string{"https://a.example.com", "https://b.example.com"}},
		{"comma and space", "https://a.example.com, https://b.example.com", []string{"https://a.example.com", "https://b.example.com"}},
		{"space separated is split upstream by viper", "https://a.example.com https://b.example.com", []string{"https://a.example.com", "https://b.example.com"}},
		{"single", "https://a.example.com", []string{"https://a.example.com"}},
		{"trailing and doubled separators", "https://a.example.com,,https://b.example.com,", []string{"https://a.example.com", "https://b.example.com"}},
		{"wildcard", "*", []string{"*"}},
		{"empty", "", []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			setRequiredEnv(t)
			t.Setenv("GIMME_CORS_ALLOWED_ORIGINS", tc.raw)

			confObj, err := NewConfig()

			require.Nil(t, err)
			assert.Equal(t, tc.want, confObj.CORSAllowedOrigins)
			assert.NotNil(t, confObj.CORSAllowedOrigins)
		})
	}
}

func TestNewConfigCORSOriginsFromFile(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "cors-origins.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()

	confObj, err := NewConfig()

	require.Nil(t, err)
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, confObj.CORSAllowedOrigins)
}

func TestNewConfigCORSOriginsEnvOverridesFile(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "cors-origins.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	t.Setenv("GIMME_CORS_ALLOWED_ORIGINS", "https://only.example.com")

	confObj, err := NewConfig()

	require.Nil(t, err)
	assert.Equal(t, []string{"https://only.example.com"}, confObj.CORSAllowedOrigins)
}

func TestNewConfigEnvOverridesFile(t *testing.T) {
	viper.Reset()
	utils.CopyFile(fmt.Sprintf("%v/%v", confDir, "valid.yml"), "./gimme.yml")
	defer func() {
		err := remove("./gimme.yml")
		assert.Nil(t, err)
	}()
	t.Setenv("GIMME_S3_LOCATION", "eu-west-3")
	t.Setenv("GIMME_ADMIN_USER", "envadmin")

	confObj, err := NewConfig()

	require.Nil(t, err)
	assert.Equal(t, "eu-west-3", confObj.S3Location)
	assert.Equal(t, "envadmin", confObj.AdminUser)
	assert.Equal(t, "test.s3.url.cloud", confObj.S3Url)
}
