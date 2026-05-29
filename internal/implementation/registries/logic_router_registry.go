package registries

import (
	"fmt"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"go.uber.org/zap"
)

type logicRouterRegistry struct {
	routers map[string]ports.LogicRouter
	mu      sync.RWMutex
	logger  *zap.SugaredLogger
}

func NewLogicRouterRegistry() ports.LogicRouterRegistry {
	return &logicRouterRegistry{
		routers: make(map[string]ports.LogicRouter),
		logger:  dbg.GetLogger(),
	}
}

func (r *logicRouterRegistry) Register(router ports.LogicRouter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := router.GetName()
	if _, exists := r.routers[name]; exists {
		r.logger.Warnw("LogicRouter already registered", "router_name", name)
		return fmt.Errorf("logic router already registered: %s", name)
	}

	r.routers[name] = router
	r.logger.Infow("LogicRouter registered", "router_name", name)
	return nil
}

func (r *logicRouterRegistry) Get(name string) (ports.LogicRouter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	router, exists := r.routers[name]
	if !exists {
		return nil, fmt.Errorf("logic router not found: %s", name)
	}
	return router, nil
}

func (r *logicRouterRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.routers))
	for name := range r.routers {
		names = append(names, name)
	}
	return names
}
