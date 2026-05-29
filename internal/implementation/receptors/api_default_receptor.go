package receptors

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
)

// Required fields: memory_key, message. Optional: source, contact_phone, idempotency_key, context, metadata.
type APIDefaultReceptor struct{}

func NewAPIDefaultReceptor() ports.Receptor {
	return &APIDefaultReceptor{}
}

func (c *APIDefaultReceptor) GetName() string {
	return "api_default"
}

func (c *APIDefaultReceptor) Convert(ctx context.Context, eventType string, raw map[string]any) ([]*entities.Pulse, error) {
	if err := c.validateRequiredFields(raw); err != nil {
		return nil, err
	}

	memoryKey := helpers.GetStringOrDefault(raw, "memory_key", helpers.GenerateRandomID())
	message, _ := helpers.GetString(raw, "message")

	event := c.buildEvent(eventType, memoryKey, message, raw)
	return []*entities.Pulse{event}, nil
}

func (c *APIDefaultReceptor) validateRequiredFields(raw map[string]any) error {
	message, ok := helpers.GetString(raw, "message")
	if !ok {
		return fmt.Errorf("missing 'message' field: must be a string")
	}
	if message == "" {
		return fmt.Errorf("invalid 'message' field: cannot be empty")
	}
	return nil
}

func (c *APIDefaultReceptor) buildEvent(eventType, memoryKey, message string, raw map[string]any) *entities.Pulse {
	source := helpers.GetStringOrDefault(raw, "source", "api")
	contactPhone := helpers.GetStringOrDefault(raw, "contact_phone", "")
	idempotencyKey := helpers.GetStringOrDefault(raw, "idempotency_key", "")

	builder := entities.NewPulse(entities.MemoryKey{
		Channel: "api",
		User:    memoryKey,
	}).
		WithEventType(eventType).
		WithUserMessage(message).
		WithSource(source).
		WithPayload(raw)

	if contactPhone != "" {
		builder = builder.WithContactPhone(contactPhone)
	}
	if idempotencyKey != "" {
		builder = builder.WithIdempotencyKey(idempotencyKey)
	}

	if userContext, ok := helpers.GetMap(raw, "context"); ok && len(userContext) > 0 {
		builder.MergeKnowledge(userContext)
	}
	if userMetadata, ok := helpers.GetMap(raw, "metadata"); ok && len(userMetadata) > 0 {
		builder.MergeMetadata(userMetadata)
	}

	return builder.Build()
}
