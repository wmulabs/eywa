package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// RitualRequest carries all inputs needed to create a scheduled task.
// The Event must be fully constructed before calling Schedule — Receptor
// should run at scheduling time (in the REST handler) so that memory_key and
// other derived fields are resolved upfront and stored in EventSnapshot.
type RitualRequest struct {
	EventKey   string          // routing key used by ProcessEventByKey and Cloud Tasks
	Event      *entities.Pulse // pre-built event (converter already ran, or built directly by LLM tool)
	ExecuteAt  time.Time
	CreatedBy  string // "api" | "llm" | "system"
	Recurrence *entities.RecurrenceConfig
	Metadata   map[string]any
}

// RitualManager manages the full lifecycle of scheduled tasks.
// Implementations coordinate between Keeper (provider) and ScheduledTaskRepository (persistence).
type RitualManager interface {
	Schedule(ctx context.Context, req RitualRequest) (*entities.Ritual, error)

	// Cancel cancels a pending task. sessionKey is required for ownership validation.
	Cancel(ctx context.Context, id, memoryKey string) error

	// MarkRunning transitions a task to running, signaling execution has started.
	// Returns ErrRitualTerminal when the task is already executed, failed, or cancelled
	// so the pipeline step can abort without retrying.
	MarkRunning(ctx context.Context, id string) error

	// MarkExecuted marks a running task as executed and schedules the next recurrence if configured.
	MarkExecuted(ctx context.Context, id string) error

	// MarkFailed marks a task as failed with a reason. Accepts any non-failed status
	// to handle cases where MarkRunning ran before a pipeline step rejected the event.
	MarkFailed(ctx context.Context, id string, reason string) error

	ListPendingByMemoryKey(ctx context.Context, memoryKey string, limit, offset int) ([]*entities.Ritual, error)
	CountByMemoryKey(ctx context.Context, memoryKey string) (int64, error)
}
