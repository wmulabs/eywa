package actions

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// SendToContactAction delivers a message to the contact associated with the active Pulse
// via the Voice declared on the Link. The contact_key parameter is optional —
// it defaults to Pulse.ContactPhone when omitted.
//
// Use this on executor Spirits that need to notify a contact after completing a task
// without relying on channel-specific send actions (send_whatsapp_message, etc.).
type SendToContactAction struct {
	voiceRegistry ports.VoiceRegistry
}

func NewSendToContactAction(voiceRegistry ports.VoiceRegistry) *SendToContactAction {
	return &SendToContactAction{voiceRegistry: voiceRegistry}
}

func (a *SendToContactAction) GetName() string { return "send_to_contact" }

func (a *SendToContactAction) GetDescription() string {
	return "Sends a message to the contact associated with this event via the configured response channel. " +
		"Use this to deliver the final result of a task to the user. " +
		"contact_key is optional and defaults to the event's contact phone number."
}

func (a *SendToContactAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message to send to the contact.",
			},
			"contact_key": map[string]any{
				"type":        "string",
				"description": "Optional. The contact phone number (E.164 format). Defaults to the event's ContactPhone.",
			},
		},
		"required": []string{"message"},
	}
}

func (a *SendToContactAction) GetCategory() ports.ActionCategory { return ports.ActionDelivery }
func (a *SendToContactAction) IsCritical() bool                  { return true }

func (a *SendToContactAction) Validate(args map[string]any) error {
	msg, ok := args["message"].(string)
	if !ok || msg == "" {
		return domainerrors.NewBusinessError("message is required and must be a non-empty string")
	}
	return nil
}

func (a *SendToContactAction) Execute(ctx context.Context, args map[string]any) (string, error) {
	message, _ := args["message"].(string)

	link, _ := ctx.Value(ports.LinkContextKey{}).(*entities.Link)
	pulse, _ := ctx.Value(ports.PulseContextKey{}).(*entities.Pulse)

	if link == nil {
		return "", domainerrors.NewInfrastructureError("send_to_contact: link context not available", nil)
	}

	voiceName := link.VoiceName
	if voiceName == "" {
		return "", domainerrors.NewBusinessError("no response channel configured on this Link")
	}

	voice, err := a.voiceRegistry.Get(voiceName)
	if err != nil {
		return "", domainerrors.NewInfrastructureError(fmt.Sprintf("voice %q not found", voiceName), err)
	}

	target := pulse
	if overrideKey, ok := args["contact_key"].(string); ok && overrideKey != "" && pulse != nil {
		copy := *pulse
		copy.ContactPhone = overrideKey
		target = &copy
	}

	if target == nil {
		return "", domainerrors.NewBusinessError("no pulse context available for delivery")
	}

	if err := voice.SendResponse(ctx, target, message); err != nil {
		return "", domainerrors.NewInfrastructureError("failed to send message to contact", err)
	}

	return fmt.Sprintf("Message delivered to %s", target.ContactPhone), nil
}
