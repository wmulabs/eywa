package ports

import (
	"context"
	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Pathfinder reads a Pulse and the list of candidate Spirits, then picks the best one.
// Returns an empty string when no suitable Spirit is found.
type Pathfinder interface {
	GetName() string
	// SelectSpirit receives the Spirits available for this Pulse (from the Link configuration)
	// and returns the name of the chosen Spirit, or empty string if none match.
	SelectSpirit(ctx context.Context, event *entities.Pulse, availableSpirits []string) string
}

type PathfinderRegistry interface {
	Register(pathfinder Pathfinder) error
	// RegisterMultiple returns an error on the first duplicate found.
	RegisterMultiple(pathfinders ...Pathfinder) error
	// Get returns nil if the Pathfinder is not registered.
	Get(name string) Pathfinder
	List() []string
}
