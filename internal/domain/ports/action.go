package ports

import (
	"context"
	"github.com/wmulabs/eywa/internal/domain/entities"
)

// SessionContextKey is the context.Context key used to pass the active Memory to Actions during reasoning.
type SessionContextKey struct{}

// PulseContextKey passes the active Pulse to Actions during reasoning.
type PulseContextKey struct{}

// LinkContextKey passes the active Link configuration to Actions during reasoning.
type LinkContextKey struct{}

type ActionCategory string

const (
	// ActionDelivery marks Actions that send responses to users (WhatsApp, email, SMS).
	// The orchestrator tracks this to know whether an explicit send occurred.
	ActionDelivery     ActionCategory = "response_delivery"
	ActionRetrieval    ActionCategory = "data_retrieval"
	ActionModification ActionCategory = "data_modification"
	ActionGeneral      ActionCategory = "general"
)

// Action is a discrete capability the Spirit can invoke during a reasoning cycle.
// Each Action is described to the Oracle (name + description + parameters JSON Schema)
// so the model can decide when and how to call it.
type Action interface {
	GetName() string
	GetDescription() string
	GetParameters() map[string]any

	// Execute runs the Action and returns the result as a string fed back to the Oracle.
	// Return *ActionError with BusinessError for domain-level failures (shown to the model);
	// use InfrastructureError for technical failures (suppressed from the model).
	Execute(ctx context.Context, args map[string]any) (string, error)

	Validate(args map[string]any) error

	// IsCritical controls whether an execution failure aborts the entire Pulse processing.
	// true: the pipeline stops and surfaces the error; false: failure is logged and the cycle continues.
	IsCritical() bool

	// GetCategory classifies the Action so the orchestrator can detect whether an
	// ActionDelivery Action ran — which determines if Voice.SendResponse should be skipped.
	GetCategory() ActionCategory
}

type ActionRegistry interface {
	Register(action Action) error
	// RegisterMultiple returns an error on the first duplicate found.
	RegisterMultiple(actions ...Action) error
	Get(name string) (Action, error)
	GetMultiple(names []string) ([]Action, error)
	Execute(ctx context.Context, actionName string, args map[string]any) (string, error)
	GetActionDefinitions(ctx context.Context, actionNames []string) ([]entities.ActionDefinition, error)
	List() []Action
	IsRegistered(actionName string) bool
	ListRegistered() []string
}
