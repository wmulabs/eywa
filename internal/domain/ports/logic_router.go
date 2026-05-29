package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// LogicRouter selects a sub-Spirit for an orchestrator Spirit operating in "logic" mode.
// It inspects the Pulse and returns the name of the Spirit that should handle the request.
// An empty string means no match — the orchestrator falls through to its default behavior.
type LogicRouter interface {
	GetName() string
	Route(ctx context.Context, pulse *entities.Pulse) string
}

type LogicRouterRegistry interface {
	Register(router LogicRouter) error
	Get(name string) (LogicRouter, error)
	List() []string
}
