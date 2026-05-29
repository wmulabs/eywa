package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

var _ eywa.VigilRepository = (*VigilRepository)(nil)

type VigilRepository struct {
	client      *redis.Client
	serviceName string
	environment string
}

func NewVigilRepository(client *redis.Client, serviceName, environment string) *VigilRepository {
	return &VigilRepository{
		client:      client,
		serviceName: serviceName,
		environment: environment,
	}
}

func (r *VigilRepository) key(memoryKey string) string {
	return fmt.Sprintf("%s:%s:vigil:%s", r.serviceName, r.environment, memoryKey)
}

func (r *VigilRepository) Acquire(ctx context.Context, memoryKey, operatorID string, ttl time.Duration) error {
	now := time.Now().UTC()
	value := fmt.Sprintf("%s:%d", operatorID, now.Unix())
	ok, err := r.client.SetNX(ctx, r.key(memoryKey), value, ttl).Result()
	if err != nil {
		return fmt.Errorf("set vigil flag: %w", err)
	}
	if !ok {
		return eywa.ErrSessionHeld
	}
	return nil
}

func (r *VigilRepository) Release(ctx context.Context, memoryKey string) error {
	if err := r.client.Del(ctx, r.key(memoryKey)).Err(); err != nil {
		return fmt.Errorf("release vigil: %w", err)
	}
	return nil
}

func (r *VigilRepository) Get(ctx context.Context, memoryKey string) (*eywa.Vigil, error) {
	k := r.key(memoryKey)
	val, err := r.client.Get(ctx, k).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // no vigil = spirit mode
		}
		return nil, fmt.Errorf("get vigil: %w", err)
	}

	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("vigil: malformed value for key %s", k)
	}
	operatorID := parts[0]
	unixSec, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("vigil: invalid timestamp in key %s: %w", k, err)
	}
	seatSince := time.Unix(unixSec, 0).UTC()

	ttlDuration, err := r.client.TTL(ctx, k).Result()
	if err != nil {
		return nil, fmt.Errorf("get vigil TTL: %w", err)
	}
	expiresAt := time.Now().UTC().Add(ttlDuration)

	return &eywa.Vigil{
		MemoryKey:  memoryKey,
		OperatorID: operatorID,
		SeatSince:  seatSince,
		ExpiresAt:  expiresAt,
	}, nil
}

func (r *VigilRepository) Refresh(ctx context.Context, memoryKey string, ttl time.Duration) error {
	if err := r.client.Expire(ctx, r.key(memoryKey), ttl).Err(); err != nil {
		return fmt.Errorf("refresh vigil TTL: %w", err)
	}
	return nil
}

// ListAll scans Redis for all active vigil keys under this service/environment prefix.
func (r *VigilRepository) ListAll(ctx context.Context) ([]*eywa.Vigil, error) {
	prefix := fmt.Sprintf("%s:%s:vigil:", r.serviceName, r.environment)
	pattern := prefix + "*"

	var keys []string
	var cursor uint64
	for {
		batch, next, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("vigil: scan: %w", err)
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}

	vigils := make([]*eywa.Vigil, 0, len(keys))
	for _, k := range keys {
		memoryKey := strings.TrimPrefix(k, prefix)
		v, err := r.Get(ctx, memoryKey)
		if err != nil || v == nil {
			continue
		}
		vigils = append(vigils, v)
	}
	return vigils, nil
}
