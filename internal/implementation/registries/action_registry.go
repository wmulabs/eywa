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

// All Actions must be registered before Weave initialization.
type actionRegistry struct {
	actions map[string]ports.Action
	mu      sync.RWMutex
	logger  *zap.SugaredLogger
}

func NewActionRegistry() ports.ActionRegistry {
	return &actionRegistry{
		actions: make(map[string]ports.Action),
		logger:  dbg.GetLogger(),
	}
}

func (r *actionRegistry) Register(action ports.Action) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := action.GetName()
	if _, exists := r.actions[name]; exists {
		r.logger.Warnw("Action already registered", "action_name", name)
		return fmt.Errorf("action already registered: %s", name)
	}

	r.actions[name] = action
	r.logger.Infow("Action registered", "action_name", name)
	return nil
}

// Returns on first duplicate.
func (r *actionRegistry) RegisterMultiple(actions ...ports.Action) error {
	for _, action := range actions {
		if err := r.Register(action); err != nil {
			return err
		}
	}
	return nil
}

func (r *actionRegistry) Get(name string) (ports.Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	action, exists := r.actions[name]
	if !exists {
		return nil, fmt.Errorf("action not found: %s", name)
	}

	return action, nil
}

func (r *actionRegistry) GetMultiple(names []string) ([]ports.Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actions := make([]ports.Action, 0, len(names))
	missing := make([]string, 0)

	for _, name := range names {
		action, exists := r.actions[name]
		if !exists {
			missing = append(missing, name)
			continue
		}
		actions = append(actions, action)
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("actions not found: %v", missing)
	}

	return actions, nil
}

func (r *actionRegistry) List() []ports.Action {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actions := make([]ports.Action, 0, len(r.actions))
	for _, action := range r.actions {
		actions = append(actions, action)
	}

	return actions
}

func (r *actionRegistry) Execute(ctx context.Context, actionName string, args map[string]any) (string, error) {
	ctx, span := tracer.GetTracer().Start(ctx, "ActionRegistry/Execute")
	defer span.End()

	log := r.logger

	action, err := r.Get(actionName)
	if err != nil {
		log.Errorw("Action not found in registry", "action_name", actionName, "error", err)
		return "", fmt.Errorf("action not registered: %s", actionName)
	}

	if err := action.Validate(args); err != nil {
		log.Errorw("Action argument validation failed", "action_name", actionName, "error", err)
		return "", fmt.Errorf("invalid arguments for action %s: %w", actionName, err)
	}

	log.Infow("executing Action", "action_name", actionName)
	result, err := action.Execute(ctx, args)
	if err != nil {
		log.Errorw("Action execution failed", "action_name", actionName, "error", err)
		return "", fmt.Errorf("action execution failed: %w", err)
	}

	log.Infow("Action executed successfully", "action_name", actionName)
	return result, nil
}

func (r *actionRegistry) GetActionDefinitions(ctx context.Context, actionNames []string) ([]entities.ActionDefinition, error) {
	_, span := tracer.GetTracer().Start(ctx, "ActionRegistry/GetActionDefinitions")
	defer span.End()

	log := r.logger
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]entities.ActionDefinition, 0, len(actionNames))
	missing := make([]string, 0)

	for _, name := range actionNames {
		action, exists := r.actions[name]
		if !exists {
			missing = append(missing, name)
			continue
		}

		definitions = append(definitions, entities.ActionDefinition{
			Name:        action.GetName(),
			Description: action.GetDescription(),
			Parameters:  action.GetParameters(),
		})
	}

	if len(missing) > 0 {
		log.Warnw("some Actions not found", "missing", missing)
	}

	log.Infow("fetched Action definitions", "count", len(definitions))
	return definitions, nil
}

func (r *actionRegistry) IsRegistered(actionName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.actions[actionName]
	return exists
}

func (r *actionRegistry) ListRegistered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.actions))
	for name := range r.actions {
		names = append(names, name)
	}

	return names
}
