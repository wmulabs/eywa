package ports

import "context"

// Vault persists raw attachment bytes to durable object storage.
// Implementations must return a permanent, publicly accessible URL for the stored object.
type Vault interface {
	Store(ctx context.Context, key string, data []byte, mimeType string) (url string, err error)
}
