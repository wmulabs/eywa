package ports

import (
	"context"
	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Scout goes out into the network, gathers context, and deposits findings into Pulse.Knowledge before the Spirit reasons.
// A Scout can be optional — return a nil error when the data is simply unavailable.
// Return a non-nil error only when the failure should halt processing.
type Scout interface {
	GetName() string
	// Harvest collects external data and adds it to the Pulse's Knowledge.
	// Return nil when data is unavailable but optional; return error only on critical failures.
	Harvest(ctx context.Context, event *entities.Pulse) error
	IsApplicable(event *entities.Pulse) bool
}

type ScoutRegistry interface {
	Register(scout Scout) error
	// RegisterMultiple returns an error on the first duplicate found.
	RegisterMultiple(scouts ...Scout) error
	// Harvest runs all applicable Scouts in registration order.
	Harvest(ctx context.Context, event *entities.Pulse) error
	GetScout(name string) (Scout, error)
	GetMultiple(names []string) ([]Scout, error)
	ListScouts() []string
	List() []Scout
}
