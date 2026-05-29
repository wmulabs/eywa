package fiber

import (
	"context"
	"encoding/json"
	"time"

	eywa "github.com/wmulabs/eywa"
)

// SSE channel constants. All published messages are JSON strings.
const (
	channelRites = "eywa:rites"
	channelVigil = "eywa:vigil"
	// per-session echo channel: channelEchoPrefix + memoryKey
	channelEchoPrefix = "eywa:echoes:"
)

// EventBus wraps eywa.PubSub and publishes typed management events.
// All methods are no-ops when bus is nil — callers never need to nil-check.
type EventBus struct {
	pubSub eywa.PubSub
}

func NewEventBus(pubSub eywa.PubSub) *EventBus {
	if pubSub == nil {
		return nil
	}
	return &EventBus{pubSub: pubSub}
}

func (b *EventBus) PublishRiteDecided(ctx context.Context, riteID string, status eywa.RiteStatus) {
	if b == nil {
		return
	}
	b.publish(ctx, channelRites, map[string]any{
		"event":   "rite_decided",
		"rite_id": riteID,
		"status":  string(status),
	})
}

func (b *EventBus) PublishVigilAcquired(ctx context.Context, memoryKey, operatorID string) {
	if b == nil {
		return
	}
	b.publish(ctx, channelVigil, map[string]any{
		"event":       "vigil_acquired",
		"memory_key":  memoryKey,
		"operator_id": operatorID,
		"seat_since":  time.Now().UTC(),
	})
	b.publish(ctx, channelEchoPrefix+memoryKey, map[string]any{
		"event":       "vigil_acquired",
		"operator_id": operatorID,
	})
}

func (b *EventBus) PublishVigilReleased(ctx context.Context, memoryKey string) {
	if b == nil {
		return
	}
	b.publish(ctx, channelVigil, map[string]any{
		"event":      "vigil_released",
		"memory_key": memoryKey,
	})
	b.publish(ctx, channelEchoPrefix+memoryKey, map[string]any{
		"event": "vigil_released",
	})
}

func (b *EventBus) PublishEchoAdded(ctx context.Context, memoryKey string, echo eywa.Echo) {
	if b == nil {
		return
	}
	b.publish(ctx, channelEchoPrefix+memoryKey, map[string]any{
		"event":   "message_added",
		"message": echo,
	})
}

func (b *EventBus) publish(ctx context.Context, channel string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = b.pubSub.Publish(ctx, channel, string(data))
}
