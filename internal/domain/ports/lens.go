package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// Lens is a Pulse pre-processing step that inspects and transforms attachments before reasoning.
// Each Lens implementation handles a specific media type (audio transcription, image analysis, etc.).
type Lens interface {
	Process(ctx context.Context, event *entities.Pulse) (OracleUsage, error)
}
