package registries

import (
	"context"
	"fmt"
	"sync"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/tracer"
	"go.uber.org/zap"
)

// Scouts run in registration order for deterministic harvest.
type scoutRegistry struct {
	scouts map[string]ports.Scout
	order  []string // registration order preserved for deterministic harvest
	mu     sync.RWMutex
	logger *zap.SugaredLogger
}

func NewScoutRegistry() ports.ScoutRegistry {
	return &scoutRegistry{
		scouts: make(map[string]ports.Scout),
		order:  make([]string, 0),
		logger: dbg.GetLogger(),
	}
}

func (r *scoutRegistry) Register(scout ports.Scout) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := scout.GetName()
	if _, exists := r.scouts[name]; exists {
		r.logger.Warnw("Scout already registered", "scout_name", name)
		return fmt.Errorf("scout already registered: %s", name)
	}

	r.scouts[name] = scout
	r.order = append(r.order, name)
	r.logger.Infow("Scout registered", "scout_name", name)
	return nil
}

// Returns on first duplicate.
func (r *scoutRegistry) RegisterMultiple(scouts ...ports.Scout) error {
	for _, scout := range scouts {
		if err := r.Register(scout); err != nil {
			return err
		}
	}
	return nil
}

func (r *scoutRegistry) Harvest(ctx context.Context, event *entities.Pulse) error {
	ctx, span := tracer.GetTracer().Start(ctx, "ScoutRegistry/Harvest")
	defer span.End()

	log := r.logger

	r.mu.RLock()
	scouts := make([]ports.Scout, 0, len(r.order))
	for _, name := range r.order {
		scouts = append(scouts, r.scouts[name])
	}
	r.mu.RUnlock()

	originalMemoryKey := event.MemoryKey

	for _, scout := range scouts {
		if !scout.IsApplicable(event) {
			log.Debugw("Scout not applicable", "scout_name", scout.GetName(), "memory_key", event.MemoryKey)
			continue
		}

		log.Infow("running Scout", "scout_name", scout.GetName(), "memory_key", event.MemoryKey)

		if err := scout.Harvest(ctx, event); err != nil {
			log.Errorw("Scout failed", "scout_name", scout.GetName(), "error", err)
			return fmt.Errorf("scout %s failed: %w", scout.GetName(), err)
		}

		if event.MemoryKey != originalMemoryKey {
			log.Infow("Memory key updated by Scout",
				"scout_name", scout.GetName(),
				"old_memory_key", originalMemoryKey,
				"new_memory_key", event.MemoryKey,
			)

			if event.Metadata["memory_key_updated_by"] == nil {
				event.AddMetadata("memory_key_updated_by", scout.GetName())
			}
			if event.Metadata["previous_memory_key"] == nil && originalMemoryKey != "" {
				event.AddMetadata("previous_memory_key", originalMemoryKey)
			}

			originalMemoryKey = event.MemoryKey
		}
	}

	log.Infow("harvest completed", "memory_key", event.MemoryKey, "scouts_count", len(scouts))
	return nil
}

func (r *scoutRegistry) GetScout(name string) (ports.Scout, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scout, exists := r.scouts[name]
	if !exists {
		return nil, fmt.Errorf("scout not found: %s", name)
	}

	return scout, nil
}

func (r *scoutRegistry) GetMultiple(names []string) ([]ports.Scout, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scouts := make([]ports.Scout, 0, len(names))
	missing := make([]string, 0)

	for _, name := range names {
		scout, exists := r.scouts[name]
		if !exists {
			missing = append(missing, name)
			continue
		}
		scouts = append(scouts, scout)
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("scouts not found: %v", missing)
	}

	return scouts, nil
}

func (r *scoutRegistry) ListScouts() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.order))
	copy(names, r.order)
	return names
}

func (r *scoutRegistry) List() []ports.Scout {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scouts := make([]ports.Scout, 0, len(r.order))
	for _, name := range r.order {
		scouts = append(scouts, r.scouts[name])
	}

	return scouts
}
