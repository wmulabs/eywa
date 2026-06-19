package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

const (
	checkpointKeyPrefix = "eywa:checkpoint"
	// defaultCheckpointTTL bounds how long an orphaned checkpoint (interrupted turn never retried)
	// lingers before Redis reclaims it.
	defaultCheckpointTTL = 24 * time.Hour
)

// CheckpointStore implements eywa.CheckpointStore using Redis string keys with a TTL, persisting
// reasoning-turn snapshots so an interrupted turn can resume. Each turn is one key; Save overwrites.
type CheckpointStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewCheckpointStore creates a Redis-backed CheckpointStore with the default 24h checkpoint TTL.
func NewCheckpointStore(client *redis.Client) eywa.CheckpointStore {
	return &CheckpointStore{client: client, ttl: defaultCheckpointTTL}
}

// NewCheckpointStoreWithTTL creates a Redis-backed CheckpointStore with a custom TTL for orphan
// cleanup. A non-positive ttl falls back to the default.
func NewCheckpointStoreWithTTL(client *redis.Client, ttl time.Duration) eywa.CheckpointStore {
	if ttl <= 0 {
		ttl = defaultCheckpointTTL
	}
	return &CheckpointStore{client: client, ttl: ttl}
}

func (s *CheckpointStore) Save(ctx context.Context, turnID string, data []byte) error {
	if err := s.client.Set(ctx, s.key(turnID), data, s.ttl).Err(); err != nil {
		return fmt.Errorf("checkpoint save: %w", err)
	}
	return nil
}

func (s *CheckpointStore) Load(ctx context.Context, turnID string) ([]byte, bool, error) {
	data, err := s.client.Get(ctx, s.key(turnID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("checkpoint load: %w", err)
	}
	return data, true, nil
}

func (s *CheckpointStore) Delete(ctx context.Context, turnID string) error {
	if err := s.client.Del(ctx, s.key(turnID)).Err(); err != nil {
		return fmt.Errorf("checkpoint delete: %w", err)
	}
	return nil
}

func (s *CheckpointStore) key(turnID string) string {
	return fmt.Sprintf("%s:%s", checkpointKeyPrefix, turnID)
}
