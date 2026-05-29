package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// SpiritRepository persists versioned Spirits. Each Update call creates a new version;
// only one version per Spirit name can be active at a time.
type SpiritRepository interface {
	// Create stores a new Spirit at version 1.
	Create(ctx context.Context, spirit *entities.Spirit) error

	// Update writes a new version and records what changed in changeLog.
	Update(ctx context.Context, spiritName string, spirit *entities.Spirit, changeLog string) (*entities.Spirit, error)

	FindByID(ctx context.Context, id string) (*entities.Spirit, error)

	// FindActiveByName is the primary load path used by the engine after Pathfinder selection.
	FindActiveByName(ctx context.Context, spiritName string) (*entities.Spirit, error)

	// FindActiveByNames loads multiple active Spirits in one query (avoids N+1).
	// Missing Spirits are absent from the returned map — not an error.
	FindActiveByNames(ctx context.Context, spiritNames []string) (map[string]*entities.Spirit, error)

	GetVersion(ctx context.Context, spiritName string, version int) (*entities.Spirit, error)
	FindVersionHistory(ctx context.Context, name string) ([]*entities.Spirit, error)

	// Activate marks a specific version as active, deactivating all others for that name.
	Activate(ctx context.Context, spiritName string, version int) error

	Deactivate(ctx context.Context, id string) error
	RestoreVersion(ctx context.Context, id string) error
	ListActive(ctx context.Context, limit, offset int) ([]*entities.Spirit, error)
	CountActive(ctx context.Context) (int64, error)
	ListAll(ctx context.Context) ([]*entities.Spirit, error)
}

// EchoRepository is the durable log of every user-facing exchange.
// memory_key identifies who is talking; subject_key identifies what is being discussed.
type EchoRepository interface {
	// Append silently ignores non-user-facing messages.
	Append(ctx context.Context, message entities.Echo) error

	FindByMemoryKey(ctx context.Context, memoryKey string, limit, offset int) ([]*entities.Echo, error)
	FindByMemoryKeyAndSubject(ctx context.Context, memoryKey, subjectKey string, limit, offset int) ([]*entities.Echo, error)

	// FindRecentByMemoryKey returns the last N messages. Used for Memory reconstruction
	// when the Redis Memory expires and no topic is active.
	FindRecentByMemoryKey(ctx context.Context, memoryKey string, limit int) ([]*entities.Echo, error)

	// FindRecentByMemoryKeyAndSubject returns the last N messages scoped to a topic.
	// Used for topic-aware Memory reconstruction — keeps the Oracle focused on the active topic
	// while preserving per-user privacy isolation.
	FindRecentByMemoryKeyAndSubject(ctx context.Context, memoryKey, subjectKey string, limit int) ([]*entities.Echo, error)

	// FindBySubjectKey spans all sessions for a topic. Operational/admin use only —
	// must never be used to inject cross-session data into Oracle Memory.
	FindBySubjectKey(ctx context.Context, subjectKey string, limit, offset int) ([]*entities.Echo, error)

	CountByMemoryKey(ctx context.Context, memoryKey string) (int64, error)
	CountByMemoryKeyAndSubject(ctx context.Context, memoryKey, subjectKey string) (int64, error)
	CountBySubjectKey(ctx context.Context, subjectKey string) (int64, error)
}

// MemoryRepository manages Memory state in cache (Redis).
// All methods take an explicit composite key — the caller constructs it from memory_key + subject_key.
type MemoryRepository interface {
	GetMemory(ctx context.Context, key string) (*entities.Memory, error)
	SaveMemory(ctx context.Context, key string, memory *entities.Memory) error
	DeleteMemory(ctx context.Context, key string) error
}

type ChronicleRepository interface {
	LogInteraction(ctx context.Context, log *entities.Chronicle) error
	FindByMemoryKey(ctx context.Context, memoryKey string) ([]*entities.Chronicle, error)
	FindByEventKey(ctx context.Context, eventKey string) ([]*entities.Chronicle, error)
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*entities.Chronicle, error)
}
