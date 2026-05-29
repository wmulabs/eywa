package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

const inboxKeyPrefix = "eywa:inbox"

// InboxManager implements eywa.Inbox using Redis lists.
// Messages are appended via RPUSH with a sliding TTL. PopAll reads and deletes
// the list atomically in a single pipeline to avoid partial reads.
type InboxManager struct {
	client *redis.Client
	ttl    time.Duration
}

// NewInbox creates a new inbox backed by the provided Redis client.
// ttl controls how long buffered messages are retained (recommended: 10s).
func NewInbox(client *redis.Client, ttl time.Duration) eywa.Inbox {
	return &InboxManager{client: client, ttl: ttl}
}

// Push appends message to the memory inbox and refreshes the TTL.
func (r *InboxManager) Push(ctx context.Context, memoryKey string, message string) error {
	key := r.key(memoryKey)
	pipe := r.client.Pipeline()
	pipe.RPush(ctx, key, message)
	pipe.Expire(ctx, key, r.ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("execute inbox pipeline: %w", err)
	}
	return nil
}

// PopAll atomically reads all messages and deletes the inbox key.
// Returns an empty slice when the inbox is empty or the key does not exist.
func (r *InboxManager) PopAll(ctx context.Context, memoryKey string) ([]string, error) {
	key := r.key(memoryKey)
	pipe := r.client.Pipeline()
	lrange := pipe.LRange(ctx, key, 0, -1)
	pipe.Del(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		newLogger().Warnw("inbox pipeline partial failure",
			"memory_key", memoryKey,
			"error", err)
		return nil, fmt.Errorf("execute inbox pipeline: %w", err)
	}
	return lrange.Val(), nil
}

func (r *InboxManager) key(memoryKey string) string {
	return fmt.Sprintf("%s:%s", inboxKeyPrefix, memoryKey)
}
