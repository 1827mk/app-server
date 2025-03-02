package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(cfg *RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{Client: client}, nil
}

func NewRedis(redisClient *RedisClient) (*RedisClient, error) {
	if redisClient == nil || redisClient.Client == nil {
		return nil, fmt.Errorf("invalid redis database connection")
	}

	return &RedisClient{Client: redisClient.Client}, nil
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}
func (r *RedisClient) GetClient() *redis.Client {
	return r.Client
}
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, key).Result()
}
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.Client.Set(ctx, key, value, expiration).Err()
}
func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.Client.Del(ctx, key).Err()
}
func (r *RedisClient) Increment(ctx context.Context, key string) (int64, error) {
	return r.Client.Incr(ctx, key).Result()
}
func (r *RedisClient) Decrement(ctx context.Context, key string) (int64, error) {
	return r.Client.Decr(ctx, key).Result()
}
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.Client.Expire(ctx, key, expiration).Err()
}
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.Client.TTL(ctx, key).Result()
}
func (r *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.Client.Keys(ctx, pattern).Result()
}
func (r *RedisClient) FlushDB(ctx context.Context) error {
	return r.Client.FlushDB(ctx).Err()
}
func (r *RedisClient) FlushAll(ctx context.Context) error {
	return r.Client.FlushAll(ctx).Err()
}
