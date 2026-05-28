package ports

import "context"

// Inbox buffers incoming messages per session while another processing cycle holds the lock.
// Messages accumulate in the inbox and are coalesced into a single user turn at the start of the
// next cycle, preventing message loss without duplicate processing.
type Inbox interface {
	Push(ctx context.Context, memoryKey string, message string) error
	PopAll(ctx context.Context, memoryKey string) ([]string, error)
}
