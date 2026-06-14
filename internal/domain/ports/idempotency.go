package ports

import (
	"context"
	"time"
)

// IdempotencyStore suppresses duplicate Pulses across redelivery.
// Webhook providers (e.g. WhatsApp via 360dialog/Twilio) may redeliver the same event;
// the store ensures a Pulse with a given IdempotencyKey is processed at most once within the TTL window.
//
// Unlike Bond, which guards against concurrent duplicates only, IdempotencyStore also
// guards against sequential redelivery after the first cycle has fully completed.
type IdempotencyStore interface {
	// Seen atomically records key and reports whether it was already present.
	// Returns true when the key was recorded by a previous call (duplicate), false when
	// this call recorded it first. Implementations must be atomic (e.g. Redis SETNX) so
	// that under concurrency exactly one caller observes false for a given key.
	//
	// On infrastructure failure, implementations should return the error and let the caller
	// decide policy; the pipeline step fails open (processes the event) to avoid dropping traffic.
	Seen(ctx context.Context, key string, ttl time.Duration) (bool, error)
}
