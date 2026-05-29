package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
)

const RiteCreatedChannel = "eywa:rites"

type RequestRiteAction struct {
	riteRepo ports.RiteRepository
	pubSub   ports.PubSub // optional; nil = no SSE fanout
}

func NewRequestRiteAction(riteRepo ports.RiteRepository, pubSub ports.PubSub) ports.Action {
	return &RequestRiteAction{riteRepo: riteRepo, pubSub: pubSub}
}

func (t *RequestRiteAction) GetName() string { return "request_rite" }

func (t *RequestRiteAction) GetDescription() string {
	return "Requests operator authorization for a sensitive action. " +
		"Creates a pending rite and suspends the current flow. " +
		"The operator will approve or reject, triggering the specified event_key."
}

func (t *RequestRiteAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"event_key": map[string]any{
				"type":        "string",
				"description": "Event key to fire when the operator decides (approve or reject).",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Why authorization is being requested.",
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Key-value data forwarded to the operator and injected into the resume Pulse.",
			},
		},
		"required": []string{"event_key", "reason"},
	}
}

func (t *RequestRiteAction) Validate(args map[string]any) error {
	if eventKey, ok := args["event_key"].(string); !ok || eventKey == "" {
		return domainerrors.NewBusinessError("missing or invalid 'event_key'")
	}
	if reason, ok := args["reason"].(string); !ok || reason == "" {
		return domainerrors.NewBusinessError("missing or invalid 'reason'")
	}
	return nil
}

func (t *RequestRiteAction) IsCritical() bool                  { return true }
func (t *RequestRiteAction) GetCategory() ports.ActionCategory { return ports.ActionGeneral }

func (t *RequestRiteAction) Execute(ctx context.Context, args map[string]any) (string, error) {
	session, ok := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	if !ok || session == nil {
		return "", domainerrors.NewInfrastructureError("session not available in context")
	}

	rite := &entities.Rite{
		ID:          helpers.GenerateRandomID(),
		MemoryKey:   session.MemoryKey,
		SubjectKey:  session.SubjectKey,
		EventKey:    args["event_key"].(string),
		Reason:      args["reason"].(string),
		Context:     mapArg(args, "context"),
		Status:      entities.RitePending,
		RequestedAt: helpers.NowUTC(),
	}

	if err := t.riteRepo.Create(ctx, rite); err != nil {
		return "", fmt.Errorf("create rite: %w", err)
	}

	if t.pubSub != nil {
		if payload, err := json.Marshal(map[string]any{
			"event": "rite_created",
			"rite":  rite,
		}); err == nil {
			_ = t.pubSub.Publish(ctx, RiteCreatedChannel, string(payload))
		}
	}

	return fmt.Sprintf(`{"rite_id":"%s","status":"pending"}`, rite.ID), nil
}
