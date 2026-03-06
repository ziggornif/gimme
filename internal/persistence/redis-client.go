package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates a redis client using redis url
func NewRedisClient(redisURL string) (*RedisClient, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cannot reach Redis at %q: %w", opt.Addr, err)
	}

	return &RedisClient{client}, nil
}

// GetClient return redis client instance
func (rc *RedisClient) GetClient() *redis.Client {
	return rc.client
}

// CloseConnection close redis connection
func (rc *RedisClient) CloseConnection() {
	if rc.client != nil {
		if closeErr := rc.client.Close(); closeErr != nil {
			logrus.Warnf("Error closing Redis connection: %v", closeErr)
		}
	}
}
