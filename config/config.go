package config

import (
	"errors"
	"os"
	"strconv"
)

//TODO: improve this later
type Config struct {
	AdminUser     string
	AdminPassword string
	Secret        string
	S3Url         string
	S3Key         string
	S3Secret      string
	S3BucketName  string
	S3Location    string
	S3SSL         bool
}

func NewConfig() (*Config, error) {
	adminUser, ok := os.LookupEnv("GIMME_ADMIN_USER")
	if !ok {
		return nil, errors.New("GIMME_ADMIN_USER is not set")
	}

	adminPassword, ok := os.LookupEnv("GIMME_ADMIN_PASSWORD")
	if !ok {
		return nil, errors.New("GIMME_ADMIN_PASSWORD is not set")
	}

	secret, ok := os.LookupEnv("GIMME_SECRET")
	if !ok {
		return nil, errors.New("GIMME_SECRET is not set")
	}

	s3Url, ok := os.LookupEnv("GIMME_S3_URL")
	if !ok {
		return nil, errors.New("GIMME_S3_URL is not set")
	}

	s3key, ok := os.LookupEnv("GIMME_S3_KEY")
	if !ok {
		return nil, errors.New("GIMME_S3_KEY is not set")
	}

	s3Secret, ok := os.LookupEnv("GIMME_S3_SECRET")
	if !ok {
		return nil, errors.New("GIMME_S3_SECRET is not set")
	}

	var s3SSL bool
	strS3SSL, ok := os.LookupEnv("GIMME_S3_SSL")
	if !ok {
		s3SSL = true
	}
	s3SSL, err := strconv.ParseBool(strS3SSL)
	if err != nil {
		return nil, errors.New("Invalid GIMME_S3_SSL value (boolean needed)")
	}

	s3BucketName, ok := os.LookupEnv("GIMME_S3_BUCKET_NAME")
	if !ok {
		s3BucketName = "gimme"
	}

	s3Location, ok := os.LookupEnv("GIMME_S3_LOCATION")
	if !ok {
		return nil, errors.New("GIMME_S3_LOCATION is not set")
	}

	return &Config{
		adminUser,
		adminPassword,
		secret,
		s3Url,
		s3key,
		s3Secret,
		s3BucketName,
		s3Location,
		s3SSL,
	}, nil
}
