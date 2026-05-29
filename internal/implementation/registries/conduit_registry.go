package registries

import (
	"fmt"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"go.uber.org/zap"
)

type conduitRegistry struct {
	conduits map[string]ports.Conduit
	mu       sync.RWMutex
	logger   *zap.SugaredLogger
}

func NewConduitRegistry() ports.ConduitRegistry {
	return &conduitRegistry{
		conduits: make(map[string]ports.Conduit),
		logger:   dbg.GetLogger(),
	}
}

func (r *conduitRegistry) Register(conduit ports.Conduit) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := conduit.GetName()
	if _, exists := r.conduits[name]; exists {
		r.logger.Warnw("Conduit already registered", "conduit_name", name)
		return fmt.Errorf("conduit already registered: %s", name)
	}

	r.conduits[name] = conduit
	r.logger.Infow("Conduit registered", "conduit_name", name)
	return nil
}

func (r *conduitRegistry) Get(name string) (ports.Conduit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conduit, exists := r.conduits[name]
	if !exists {
		return nil, fmt.Errorf("conduit not found: %s", name)
	}
	return conduit, nil
}

func (r *conduitRegistry) ListAll() []ports.Conduit {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]ports.Conduit, 0, len(r.conduits))
	for _, c := range r.conduits {
		all = append(all, c)
	}
	return all
}
