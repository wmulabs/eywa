package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Conduit connects Eywa to an external MCP (Model Context Protocol) server,
// discovers its tools, and delegates calls to it.
type Conduit interface {
	GetName() string
	Connect(ctx context.Context) error
	ListTools(ctx context.Context) ([]entities.ActionDefinition, error)
	Call(ctx context.Context, toolName string, args map[string]any) (string, error)
	Close() error
}

type ConduitRegistry interface {
	Register(conduit Conduit) error
	Get(name string) (Conduit, error)
	ListAll() []Conduit
}
