package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubConduit struct {
	name   string
	result string
	err    error
}

var _ ports.Conduit = (*stubConduit)(nil)

func (c *stubConduit) GetName() string                 { return c.name }
func (c *stubConduit) Connect(_ context.Context) error { return nil }
func (c *stubConduit) ListTools(_ context.Context) ([]entities.ActionDefinition, error) {
	panic("not implemented")
}
func (c *stubConduit) Call(_ context.Context, _ string, _ map[string]any) (string, error) {
	return c.result, c.err
}
func (c *stubConduit) Close() error { return nil }

func TestConduitActionAdapter_GetName_PrefixesConduitAndToolName(t *testing.T) {
	conduit := &stubConduit{name: "my_mcp"}
	def := entities.ActionDefinition{Name: "search"}
	adapter := NewConduitActionAdapter(conduit, def)

	if adapter.GetName() != "my_mcp__search" {
		t.Errorf("expected 'my_mcp__search', got '%s'", adapter.GetName())
	}
}

func TestConduitActionAdapter_GetDescription_ReturnsDefinitionDescription(t *testing.T) {
	conduit := &stubConduit{name: "mcp"}
	def := entities.ActionDefinition{Name: "search", Description: "search the web"}
	adapter := NewConduitActionAdapter(conduit, def)

	if adapter.GetDescription() != "search the web" {
		t.Errorf("expected 'search the web', got '%s'", adapter.GetDescription())
	}
}

func TestConduitActionAdapter_GetParameters_ReturnsDefinitionParameters(t *testing.T) {
	params := map[string]any{"type": "object"}
	conduit := &stubConduit{name: "mcp"}
	def := entities.ActionDefinition{Name: "search", Parameters: params}
	adapter := NewConduitActionAdapter(conduit, def)

	if adapter.GetParameters()["type"] != "object" {
		t.Errorf("expected parameters from definition, got %v", adapter.GetParameters())
	}
}

func TestConduitActionAdapter_GetCategory_ReturnsActionGeneral(t *testing.T) {
	adapter := NewConduitActionAdapter(&stubConduit{name: "mcp"}, entities.ActionDefinition{Name: "tool"})
	if adapter.GetCategory() != ports.ActionGeneral {
		t.Errorf("expected ActionGeneral, got %v", adapter.GetCategory())
	}
}

func TestConduitActionAdapter_IsCritical_ReturnsFalse(t *testing.T) {
	adapter := NewConduitActionAdapter(&stubConduit{name: "mcp"}, entities.ActionDefinition{Name: "tool"})
	if adapter.IsCritical() {
		t.Error("expected IsCritical to be false")
	}
}

func TestConduitActionAdapter_Validate_AlwaysReturnsNil(t *testing.T) {
	adapter := NewConduitActionAdapter(&stubConduit{name: "mcp"}, entities.ActionDefinition{Name: "tool"})
	if err := adapter.Validate(map[string]any{"any": "args"}); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestConduitActionAdapter_Execute_ReturnsConduitResult(t *testing.T) {
	conduit := &stubConduit{name: "mcp", result: "found 3 results"}
	adapter := NewConduitActionAdapter(conduit, entities.ActionDefinition{Name: "search"})

	result, err := adapter.Execute(context.Background(), map[string]any{"q": "golang"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "found 3 results" {
		t.Errorf("expected 'found 3 results', got '%s'", result)
	}
}

func TestConduitActionAdapter_Execute_ConduitCallError_ReturnsInfraError(t *testing.T) {
	conduit := &stubConduit{name: "mcp", err: errors.New("mcp timeout")}
	adapter := NewConduitActionAdapter(conduit, entities.ActionDefinition{Name: "search"})

	_, err := adapter.Execute(context.Background(), map[string]any{"q": "golang"})
	if err == nil {
		t.Error("expected error when conduit.Call fails, got nil")
	}
}
