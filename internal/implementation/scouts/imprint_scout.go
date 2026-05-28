package scouts

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

const imprintContextKey = "_imprints"

// ImprintScout loads long-term user facts from ImprintRepository and injects them
// into pulse.Knowledge[_imprints]. Injected by SpiritScoutStep for conversational
// and orchestrator Spirits when ImprintRepository is configured.
type ImprintScout struct {
	repo        ports.ImprintRepository
	maxImprints int
}

func NewImprintScout(repo ports.ImprintRepository, maxImprints int) *ImprintScout {
	if maxImprints <= 0 {
		maxImprints = 50
	}
	return &ImprintScout{repo: repo, maxImprints: maxImprints}
}

func (s *ImprintScout) Harvest(ctx context.Context, pulse *entities.Pulse, spiritName string) error {
	if pulse.ContactPhone == "" {
		return nil
	}

	imprints, err := s.repo.GetByUserKey(ctx, pulse.ContactPhone, spiritName)
	if err != nil {
		return nil // non-fatal
	}
	if len(imprints) == 0 {
		return nil
	}

	pulse.AddKnowledge(imprintContextKey, formatImprints(imprints))
	return nil
}

func formatImprints(imprints []entities.Imprint) string {
	var sb strings.Builder
	sb.WriteString("<user_memory>\n")
	for _, imp := range imprints {
		fmt.Fprintf(&sb, "- [%s] %s\n", imp.Category, imp.Fact)
	}
	sb.WriteString("</user_memory>")
	return sb.String()
}
