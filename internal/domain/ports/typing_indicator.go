package ports

import "context"

type TypingIndicator interface {
	StartTyping(ctx context.Context, phone string) error
	StopTyping(ctx context.Context, phone string) error
}
