package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ActionRetryConfig controls how the executor retries transient (infrastructure) Action failures.
// Business errors are never retried — only errors classified as InfrastructureError.
type ActionRetryConfig struct {
	// MaxAttempts is the total number of attempts (1 = no retry, 3 = 2 retries after first failure).
	// Defaults to 1 (no retry) when zero.
	MaxAttempts int

	// BaseDelay is the wait time before the first retry. Doubles on each subsequent attempt.
	BaseDelay time.Duration
}

var defaultActionRetryConfig = ActionRetryConfig{MaxAttempts: 1}

type ActionResult struct {
	ActionName      string
	Result          string
	Error           error
	IsCritical      bool
	IsVoiceDelivery bool  // true when a VoiceDelivery category Action succeeded
	DurationMs      int64 // wall-clock time for the execution (including retries)
}

type ActionExecutor interface {
	ExecuteAll(ctx context.Context, actionCalls []ports.OracleToolCall) []ActionResult
	GetAction(name string) (ports.Action, error)
}

// registryAccessor is an optional interface for ActionExecutor implementations
// that expose their ActionRegistry. Used to decouple GetActionRegistry and RegisterAction
// from the concrete DefaultActionExecutor type, allowing custom wrappers and decorators.
type registryAccessor interface {
	GetRegistry() ports.ActionRegistry
}

type DefaultActionExecutor struct {
	actionRegistry ports.ActionRegistry
	parallel       bool
	retryConfig    ActionRetryConfig
	logger         *zap.SugaredLogger
	tracer         trace.Tracer
}

func NewActionExecutor(
	actionRegistry ports.ActionRegistry,
	parallel bool,
	logger *zap.SugaredLogger,
	tracer trace.Tracer,
) *DefaultActionExecutor {
	return &DefaultActionExecutor{
		actionRegistry: actionRegistry,
		parallel:       parallel,
		retryConfig:    defaultActionRetryConfig,
		logger:         logger,
		tracer:         tracer,
	}
}

const minRetryDelay = 50 * time.Millisecond

// WithRetry enforces a 50ms minimum delay when retries are enabled to avoid hammering a struggling dependency.
func (t *DefaultActionExecutor) WithRetry(cfg ActionRetryConfig) *DefaultActionExecutor {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	if cfg.BaseDelay < 0 {
		cfg.BaseDelay = 0
	}
	if cfg.MaxAttempts > 1 && cfg.BaseDelay == 0 {
		cfg.BaseDelay = minRetryDelay
	}
	t.retryConfig = cfg
	return t
}

func (t *DefaultActionExecutor) ExecuteAll(ctx context.Context, actionCalls []ports.OracleToolCall) []ActionResult {
	ctx, span := t.tracer.Start(ctx, "ActionExecutor/ExecuteAll")
	defer span.End()

	results := make([]ActionResult, len(actionCalls))

	if t.parallel && len(actionCalls) > 1 {
		var wg sync.WaitGroup
		for i, call := range actionCalls {
			wg.Add(1)
			go func(index int, c ports.OracleToolCall) {
				defer wg.Done()
				results[index] = t.executeOne(ctx, c)
			}(i, call)
		}
		wg.Wait()
	} else {
		for i, call := range actionCalls {
			results[i] = t.executeOne(ctx, call)
		}
	}

	return results
}

// executeOne retries only InfrastructureErrors — business errors are user-facing constraints, not transient failures.
func (t *DefaultActionExecutor) executeOne(ctx context.Context, call ports.OracleToolCall) ActionResult {
	ctx, span := t.tracer.Start(ctx, "ActionExecutor/ExecuteOne")
	defer span.End()

	start := time.Now()
	result := ActionResult{
		ActionName: call.Name,
	}
	defer func() { result.DurationMs = time.Since(start).Milliseconds() }()

	action, err := t.actionRegistry.Get(call.Name)
	if err != nil {
		result.Error = fmt.Errorf("action not found: %w", err)
		t.logger.Errorw("Action not found", "action", call.Name, "error", err)
		return result
	}

	result.IsCritical = action.IsCritical()
	if call.IsCriticalOverride != nil {
		result.IsCritical = *call.IsCriticalOverride
	}
	result.IsVoiceDelivery = action.GetCategory() == ports.ActionDelivery

	if err := action.Validate(call.Arguments); err != nil {
		result.Error = fmt.Errorf("invalid arguments: %w", err)
		t.logger.Errorw("Action validation failed", "action", call.Name, "error", err)
		return result
	}

	maxAttempts := t.retryConfig.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	delay := t.retryConfig.BaseDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		executionResult, execErr := action.Execute(ctx, call.Arguments)
		if execErr == nil {
			result.Result = executionResult
			return result
		}

		wrappedErr := fmt.Errorf("execution failed: %w", execErr)

		if !errors.IsInfrastructureError(execErr) {
			result.Error = wrappedErr
			t.logger.Warnw("Action failed with non-retriable error",
				"action", call.Name,
				"error", execErr,
				"attempt", attempt,
			)
			return result
		}

		if attempt < maxAttempts {
			t.logger.Warnw("Action failed with transient error, retrying",
				"action", call.Name,
				"error", execErr,
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"retry_delay", delay,
			)
			// Use time.NewTimer instead of time.After so the timer is stopped
			// when the context cancels, preventing a short-lived goroutine leak
			// per retry iteration.
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				result.Error = fmt.Errorf("execution cancelled during retry: %w", ctx.Err())
				return result
			case <-timer.C:
			}
			timer.Stop()
			delay *= 2
		} else {
			result.Error = wrappedErr
			t.logger.Errorw("Action failed after all retries",
				"action", call.Name,
				"error", execErr,
				"attempts", maxAttempts,
			)
		}
	}

	return result
}

func (t *DefaultActionExecutor) GetAction(name string) (ports.Action, error) {
	action, err := t.actionRegistry.Get(name)
	if err != nil {
		return nil, fmt.Errorf("get action %q: %w", name, err)
	}
	return action, nil
}

// GetRegistry satisfies the registryAccessor interface, exposing the ActionRegistry
// for management APIs and dynamic registration without coupling to concrete type.
func (t *DefaultActionExecutor) GetRegistry() ports.ActionRegistry {
	return t.actionRegistry
}
