package config

import (
	"errors"
	"os"
)

//TODO: improve this later
type Config struct {
	AdminUser     string
	AdminPassword string
	Secret        string
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

	return &Config{
		adminUser,
		adminPassword,
		secret,
	}, nil
}
