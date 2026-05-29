package redis

import (
	"context"
	"fmt"
	"time"

	redisrate "github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

// RateLimiter implements a GCRA (token bucket) rate limiter backed by Redis.
// On Redis failure it fails open — traffic is never blocked due to infrastructure issues.
type RateLimiter struct {
	limiter *redisrate.Limiter
	limit   redisrate.Limit
}

// NewLimiter creates a rate limiter that allows up to maxRequests per window per key.
// Example: NewLimiter(client, 60, time.Minute) → 60 req/min per key.
func NewLimiter(client *redis.Client, maxRequests int, window time.Duration) eywa.Limiter {
	return &RateLimiter{
		limiter: redisrate.NewLimiter(client),
		limit: redisrate.Limit{
			Rate:   maxRequests,
			Burst:  maxRequests,
			Period: window,
		},
	}
}

func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	redisKey := fmt.Sprintf("ratelimit:%s", key)

	res, err := r.limiter.Allow(ctx, redisKey, r.limit)
	if err != nil {
		// Fail open: infrastructure errors must not block legitimate traffic.
		return true, fmt.Errorf("rate limiter redis error: %w", err)
	}

	return res.Allowed > 0, nil
}
