package actions

import (
	"context"
	"fmt"
	"strings"

	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// SearchLoreAction lets the Oracle query a Lore knowledge base by name.
// Available when the Spirit has LoreIDs configured and a LoreStore + LoreEmbedder are wired.
type SearchLoreAction struct {
	loreRepo  ports.LoreRepository
	loreStore ports.LoreStore
	embedder  ports.LoreEmbedder
}

func NewSearchLoreAction(
	loreRepo ports.LoreRepository,
	loreStore ports.LoreStore,
	embedder ports.LoreEmbedder,
) *SearchLoreAction {
	return &SearchLoreAction{loreRepo: loreRepo, loreStore: loreStore, embedder: embedder}
}

func (a *SearchLoreAction) GetName() string { return "search_lore" }

func (a *SearchLoreAction) GetDescription() string {
	return "Searches a named knowledge base (Lore) for content relevant to a query. " +
		"Use this when you need to look up specific information beyond what is already in context."
}

func (a *SearchLoreAction) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"lore_name": map[string]any{
				"type":        "string",
				"description": "Name of the knowledge base to search.",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "The search query.",
			},
			"top_k": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 5).",
			},
		},
		"required": []string{"lore_name", "query"},
	}
}

func (a *SearchLoreAction) GetCategory() ports.ActionCategory { return ports.ActionRetrieval }
func (a *SearchLoreAction) IsCritical() bool                  { return false }

func (a *SearchLoreAction) Validate(args map[string]any) error {
	if name, _ := args["lore_name"].(string); name == "" {
		return domainerrors.NewBusinessError("lore_name is required")
	}
	if query, _ := args["query"].(string); query == "" {
		return domainerrors.NewBusinessError("query is required")
	}
	return nil
}

func (a *SearchLoreAction) Execute(ctx context.Context, args map[string]any) (string, error) {
	loreName, _ := args["lore_name"].(string)
	query, _ := args["query"].(string)
	topK := 5
	if k, ok := args["top_k"].(float64); ok && k > 0 {
		topK = int(k)
	}

	lore, err := a.loreRepo.GetByName(ctx, loreName)
	if err != nil {
		return "", domainerrors.NewBusinessError(fmt.Sprintf("knowledge base %q not found", loreName))
	}

	embeddings, err := a.embedder.Embed(ctx, []string{query})
	if err != nil || len(embeddings) == 0 {
		return "", domainerrors.NewInfrastructureError("failed to embed search query", err)
	}

	chunks, err := a.loreStore.Search(ctx, lore.ID, embeddings[0], topK, 0.0)
	if err != nil {
		return "", domainerrors.NewInfrastructureError("lore search failed", err)
	}

	if len(chunks) == 0 {
		return fmt.Sprintf("No relevant content found in knowledge base %q for query: %s", loreName, query), nil
	}

	var sb strings.Builder
	for i, chunk := range chunks {
		fmt.Fprintf(&sb, "[%d] %s\n", i+1, chunk.Content)
	}
	return strings.TrimSpace(sb.String()), nil
}
