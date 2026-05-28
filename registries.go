package eywa

import (
	"github.com/wmulabs/eywa/internal/implementation/registries"
	"github.com/wmulabs/eywa/internal/implementation/services"
)

var (
	// NewActionRegistry creates an empty ActionRegistry. Register Action implementations
	// and pass the registry to WeaveBuilder.WithActionRegistry.
	NewActionRegistry = registries.NewActionRegistry

	// NewScoutRegistry creates an empty ScoutRegistry. Register Scout implementations
	// and pass the registry to WeaveBuilder.WithScoutRegistry.
	NewScoutRegistry = registries.NewScoutRegistry

	// NewPathfinderRegistry creates an empty PathfinderRegistry. Register Pathfinder
	// implementations and pass the registry to WeaveBuilder.WithPathfinderRegistry.
	NewPathfinderRegistry = registries.NewPathfinderRegistry

	// NewLogicRouterRegistry creates an empty LogicRouterRegistry. Register LogicRouter
	// implementations for post-processing responses before Voice delivery.
	NewLogicRouterRegistry = registries.NewLogicRouterRegistry

	// NewConduitRegistry creates an empty ConduitRegistry. Prefer passing Conduits
	// directly via WeaveBuilder.WithConduit; use this only for advanced multi-conduit setups.
	NewConduitRegistry = registries.NewConduitRegistry

	// NewRitualService creates a RitualManager backed by a RitualRepository and a Keeper.
	// Required to enable scheduled event execution via the Oracle's built-in Ritual Actions.
	NewRitualService = services.NewRitualService
)
