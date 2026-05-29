package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

var _ eywa.PubSub = (*RedisPubSub)(nil)

type RedisPubSub struct {
	client *redis.Client
}

func NewRedisPubSub(client *redis.Client) *RedisPubSub {
	return &RedisPubSub{client: client}
}

func (r *RedisPubSub) Publish(ctx context.Context, channel, message string) error {
	if err := r.client.Publish(ctx, channel, message).Err(); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}

// Subscribe blocks until ctx is cancelled, calling handler for each received message.
func (r *RedisPubSub) Subscribe(ctx context.Context, channel string, handler func(msg string)) error {
	sub := r.client.Subscribe(ctx, channel)
	defer sub.Close() //nolint:errcheck
	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			handler(msg.Payload)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
