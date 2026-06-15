package scouts

import (
	"context"
	"fmt"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

const loreContextKey = entities.LoreContextKnowledgeKey

// LoreScout retrieves relevant Lore chunks for the Pulse's UserMessage via vector search.
// It is injected automatically by SpiritScoutStep when Spirit.LoreIDs is non-empty.
// Results are written to pulse.Knowledge[_lore_context] and prepended to the system prompt.
type LoreScout struct {
	loreRepo  ports.LoreRepository
	loreStore ports.LoreStore
	embedder  ports.LoreEmbedder
	topK      int
	minScore  float64
}

func NewLoreScout(
	loreRepo ports.LoreRepository,
	loreStore ports.LoreStore,
	embedder ports.LoreEmbedder,
	topK int,
	minScore float64,
) *LoreScout {
	if topK <= 0 {
		topK = 5
	}
	if minScore <= 0 {
		minScore = 0.7
	}
	return &LoreScout{
		loreRepo:  loreRepo,
		loreStore: loreStore,
		embedder:  embedder,
		topK:      topK,
		minScore:  minScore,
	}
}

func (s *LoreScout) Harvest(ctx context.Context, pulse *entities.Pulse, loreIDs []string) error {
	embeddings, err := s.embedder.Embed(ctx, []string{pulse.UserMessage})
	if err != nil || len(embeddings) == 0 {
		return nil // non-fatal: skip RAG if embedding fails
	}
	queryVec := embeddings[0]

	lores, err := s.loreRepo.GetByIDs(ctx, loreIDs)
	if err != nil {
		return nil
	}

	var allChunks []entities.LoreChunk
	for _, lore := range lores {
		chunks, err := s.loreStore.Search(ctx, lore.ID, queryVec, s.topK, s.minScore)
		if err != nil {
			continue
		}
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	pulse.AddKnowledge(loreContextKey, formatChunks(lores, allChunks))
	return nil
}

func formatChunks(lores []entities.Lore, chunks []entities.LoreChunk) string {
	loreNames := make(map[string]string, len(lores))
	for _, l := range lores {
		loreNames[l.ID] = l.Name
	}

	var sb strings.Builder
	for _, chunk := range chunks {
		name := loreNames[chunk.LoreID]
		if name == "" {
			name = chunk.LoreID
		}
		// id is emitted so the model can cite sources as [chunk:<id>] when grounding is enabled.
		fmt.Fprintf(&sb, "<lore name=%q id=%q>\n%s\n</lore>\n", name, chunk.ID, chunk.Content)
	}
	return strings.TrimSpace(sb.String())
}
