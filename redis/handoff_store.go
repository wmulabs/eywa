package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

const handoffKeyPrefix = "eywa:handoff"

// HandoffStore implements eywa.HandoffStore using a Redis string per session key. It records which
// Spirit owns a conversation after a handoff so subsequent Pulses route to it across instances.
type HandoffStore struct {
	client *redis.Client
	ttl    time.Duration // 0 = no expiry (the pin persists until cleared or overwritten)
}

// NewHandoffStore creates a Redis-backed HandoffStore. ttl bounds how long a pin survives without a new
// turn; pass 0 to keep it until explicitly cleared. Size ttl at or above the session/memory TTL.
func NewHandoffStore(client *redis.Client, ttl time.Duration) eywa.HandoffStore {
	return &HandoffStore{client: client, ttl: ttl}
}

func (s *HandoffStore) GetActiveSpirit(ctx context.Context, sessionKey string) (string, error) {
	v, err := s.client.Get(ctx, s.key(sessionKey)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("handoff get: %w", err)
	}
	return v, nil
}

func (s *HandoffStore) SetActiveSpirit(ctx context.Context, sessionKey, spiritName string) error {
	if err := s.client.Set(ctx, s.key(sessionKey), spiritName, s.ttl).Err(); err != nil {
		return fmt.Errorf("handoff set: %w", err)
	}
	return nil
}

func (s *HandoffStore) ClearActiveSpirit(ctx context.Context, sessionKey string) error {
	if err := s.client.Del(ctx, s.key(sessionKey)).Err(); err != nil {
		return fmt.Errorf("handoff del: %w", err)
	}
	return nil
}

func (s *HandoffStore) key(sessionKey string) string {
	return fmt.Sprintf("%s:%s", handoffKeyPrefix, sessionKey)
}
