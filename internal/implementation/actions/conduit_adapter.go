package actions

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// ConduitActionAdapter wraps a single MCP tool as an eywa Action.
// Name format: "<conduit_name>__<tool_name>" to avoid registry collisions.
type ConduitActionAdapter struct {
	conduit    ports.Conduit
	definition entities.ActionDefinition
	toolName   string
}

func NewConduitActionAdapter(conduit ports.Conduit, tool entities.ActionDefinition) *ConduitActionAdapter {
	return &ConduitActionAdapter{
		conduit:    conduit,
		definition: tool,
		toolName:   tool.Name,
	}
}

func (a *ConduitActionAdapter) GetName() string {
	return fmt.Sprintf("%s__%s", a.conduit.GetName(), a.toolName)
}

func (a *ConduitActionAdapter) GetDescription() string {
	return a.definition.Description
}

func (a *ConduitActionAdapter) GetParameters() map[string]any {
	return a.definition.Parameters
}

func (a *ConduitActionAdapter) GetCategory() ports.ActionCategory { return ports.ActionGeneral }
func (a *ConduitActionAdapter) IsCritical() bool                  { return false }

func (a *ConduitActionAdapter) Validate(args map[string]any) error { return nil }

func (a *ConduitActionAdapter) Execute(ctx context.Context, args map[string]any) (string, error) {
	result, err := a.conduit.Call(ctx, a.toolName, args)
	if err != nil {
		return "", domainerrors.NewInfrastructureError(
			fmt.Sprintf("conduit %q tool %q call failed", a.conduit.GetName(), a.toolName),
			err,
		)
	}
	return result, nil
}
