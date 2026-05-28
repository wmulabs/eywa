package actions

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type UpdateSubjectTool struct{}

func NewUpdateSubjectAction() ports.Action {
	return &UpdateSubjectTool{}
}

func (t *UpdateSubjectTool) GetName() string {
	return "update_subject"
}

func (t *UpdateSubjectTool) GetDescription() string {
	return "Updates the current subject and optionally stores facts about it. " +
		"Use this when you identify what business entity the user is referring to (e.g. a specific shipment, ticket, or order). " +
		"The subject_key will be associated with all subsequent messages in this memory."
}

func (t *UpdateSubjectTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject_key": map[string]any{
				"type":        "string",
				"description": "The business topic identifier (e.g. 'shipment:123', 'ticket:456'). Use the format 'entity_type:id'.",
			},
			"facts": map[string]any{
				"type":        "object",
				"description": "Optional key-value data to accumulate about this topic. These facts will be available to the LLM in future interactions without replaying the full conversation history.",
			},
		},
		"required": []string{"subject_key"},
	}
}

func (t *UpdateSubjectTool) Validate(args map[string]any) error {
	topicKey, ok := args["subject_key"].(string)
	if !ok || topicKey == "" {
		return errors.NewBusinessError("missing or invalid 'subject_key' argument")
	}
	return nil
}

func (t *UpdateSubjectTool) IsCritical() bool {
	return false
}

func (t *UpdateSubjectTool) GetCategory() ports.ActionCategory {
	return ports.ActionGeneral
}

func (t *UpdateSubjectTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	session, ok := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	if !ok || session == nil {
		return "", errors.NewInfrastructureError("session not available in context")
	}

	next := args["subject_key"].(string)
	previous := session.SubjectKey
	switched := t.isTopicSwitch(previous, next)

	t.applyTopicUpdate(session, next, switched, args)

	return t.formatUpdateMessage(previous, next, switched), nil
}

func (t *UpdateSubjectTool) isTopicSwitch(previous, next string) bool {
	return previous != "" && previous != next
}

func (t *UpdateSubjectTool) applyTopicUpdate(session *entities.Memory, next string, switched bool, args map[string]any) {
	session.SubjectKey = next

	if switched {
		session.TopicFacts = make(map[string]any)
		session.Summary = ""
	}

	if facts, ok := args["facts"].(map[string]any); ok {
		if session.TopicFacts == nil {
			session.TopicFacts = make(map[string]any)
		}
		for k, v := range facts {
			session.TopicFacts[k] = v
		}
	}
}

func (t *UpdateSubjectTool) formatUpdateMessage(previous, next string, switched bool) string {
	if switched {
		return fmt.Sprintf("subject switched from '%s' to '%s'", previous, next)
	}
	return fmt.Sprintf("subject updated to '%s'", next)
}
