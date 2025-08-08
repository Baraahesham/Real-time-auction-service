package redis

import (
	"context"
	"time"

	"troffee-auction-service/internal/config"

	"github.com/redis/go-redis/v9"
)

// NewClient creates a new Redis client based on configuration
func NewClient(cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MaxRetries:   3,
	})

	return rdb
}

// PingRedis tests the Redis connection
func PingRedis(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.Ping(ctx).Err()
}
