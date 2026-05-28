package ports

import "context"

// PubSub is a publish/subscribe notifier used for cross-instance config reload.
// Subscribe blocks until ctx is cancelled, calling handler for each received message.
type PubSub interface {
	Publish(ctx context.Context, channel, message string) error
	Subscribe(ctx context.Context, channel string, handler func(msg string)) error
}
