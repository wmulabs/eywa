package registries

import (
	"fmt"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"go.uber.org/zap"
)

// selects which Spirit should process a given Pulse.
type pathfinderRegistry struct {
	pathfinders map[string]ports.Pathfinder
	mu          sync.RWMutex
	logger      *zap.SugaredLogger
}

func NewPathfinderRegistry() ports.PathfinderRegistry {
	return &pathfinderRegistry{
		pathfinders: make(map[string]ports.Pathfinder),
		logger:      dbg.GetLogger(),
	}
}

func (r *pathfinderRegistry) Register(pathfinder ports.Pathfinder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := pathfinder.GetName()
	if _, exists := r.pathfinders[name]; exists {
		r.logger.Warnw("Pathfinder already registered", "pathfinder_name", name)
		return fmt.Errorf("pathfinder already registered: %s", name)
	}

	r.pathfinders[name] = pathfinder
	r.logger.Infow("Pathfinder registered", "pathfinder_name", name)
	return nil
}

// Returns on first duplicate.
func (r *pathfinderRegistry) RegisterMultiple(pathfinders ...ports.Pathfinder) error {
	for _, pathfinder := range pathfinders {
		if err := r.Register(pathfinder); err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves a Pathfinder by name. Returns nil if not found.
func (r *pathfinderRegistry) Get(name string) ports.Pathfinder {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.pathfinders[name]
}

func (r *pathfinderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.pathfinders))
	for name := range r.pathfinders {
		names = append(names, name)
	}
	return names
}
