package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Archivist compresses a slice of threads into a concise text summary.
// Used by ArchivistStep to reduce Threads when it approaches the configured threshold.
// The summary is scoped to (memory_key, subject_key) — never mixes messages from different subjects.
type Archivist interface {
	Summarize(ctx context.Context, memoryKey, subjectKey string, messages []entities.Thread) (string, error)
}
