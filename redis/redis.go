package redis

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	redisClient *redis.Client
	basePath    string
}

func NewRedisConnection(ctx context.Context, redisUrl string, basePath string) (*RedisRepository, error) {
	debugger := newLogger()
	debugger.Infow("Redis is starting...")

	redisUrlParsed, err := url.Parse(redisUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}
	password, _ := redisUrlParsed.User.Password()

	rClient := redis.NewClient(&redis.Options{
		Addr:     redisUrlParsed.Host,
		Password: password,
	})

	if err := redisotel.InstrumentTracing(rClient); err != nil {
		debugger.Errorw("error creating telemetry for redis", "err", err)
	}

	_, err = rClient.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	debugger.Infow("Redis is connected")
	return &RedisRepository{
		redisClient: rClient,
		basePath:    basePath,
	}, nil
}

// Deprecated: Use GetClient instead.
func (r *RedisRepository) GetRedisClient() *redis.Client {
	return r.redisClient
}

func (r *RedisRepository) DisconnectRedisDB(ctx context.Context) {
	debugger := newLogger()
	if r.redisClient == nil {
		debugger.Errorw("DisconnectRedisDB called with nil client")
		return
	}
	if err := r.redisClient.Close(); err != nil {
		debugger.Warnw("error closing redis connection", "err", err)
	} else {
		debugger.Infow("disconnected from redis")
	}
}

func (r *RedisRepository) GetClient() *redis.Client {
	return r.redisClient
}

func (r *RedisRepository) CachePath(s []string) string {
	path := []string{r.basePath}
	s = append(path, s...)
	return strings.Join(s, ":")
}
