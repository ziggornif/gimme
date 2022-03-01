package config

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func copyFile(src string, dst string) (int64, error) {
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func remove(src string) error {
	err := os.Remove(src)
	if err != nil {
		return err
	}
	return nil
}

var confDir = "../resources/tests/config"

func init() {
	remove("./gimme.yml")
}

func TestNewConfigFileErr(t *testing.T) {
	_, err := NewConfig()
	assert.Equal(t, "Unable to read the config file", err.Error())
}

func TestNewConfig(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "valid.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	confObj, err := NewConfig()
	assert.Equal(t, &Configuration{
		AdminUser:     "test",
		AdminPassword: "test",
		AppPort:       "8080",
		Secret:        "secret",
		S3Url:         "test.s3.url.cloud",
		S3Key:         "s3key",
		S3Secret:      "s3secret",
		S3BucketName:  "gimme",
		S3Location:    "eu-west-1",
		S3SSL:         true,
	}, confObj)
	assert.Nil(t, err)
}

func TestNewConfigValidationErrAdmUsr(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-adm-usr.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: AdminUser is not set", err.Error())
}

func TestNewConfigValidationErrAdmPass(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-adm-pass.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: AdminPassword is not set", err.Error())
}

func TestNewConfigValidationErrSecret(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-secret.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: Secret is not set", err.Error())
}

func TestNewConfigValidationErrS3Url(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-url.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: S3Url is not set", err.Error())
}

func TestNewConfigValidationErrS3Key(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-key.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: S3Key is not set", err.Error())
}

func TestNewConfigValidationErrS3Secret(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-secret.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: S3Secret is not set", err.Error())
}

func TestNewConfigValidationErrS3Location(t *testing.T) {
	copyFile(fmt.Sprintf("%v/%v", confDir, "no-s3-location.yml"), "./gimme.yml")
	defer remove("./gimme.yml")
	_, err := NewConfig()

	assert.Equal(t, "Configuration is not valid: S3Location is not set", err.Error())
}
