package orchestrator

import "github.com/wmulabs/eywa/internal/domain/entities"

// ImprintExtractionConfig controls automatic fact extraction from completed conversations.
type ImprintExtractionConfig struct {
	Enabled     bool
	Model       entities.SpiritModel
	MaxImprints int // per user per Spirit; default 100
}

func (c ImprintExtractionConfig) maxImprints() int {
	if c.MaxImprints <= 0 {
		return 100
	}
	return c.MaxImprints
}
