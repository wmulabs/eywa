package fiber

import (
	"context"
	"errors"
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

// loreIngestor / loreSearcher are the slices of *eywa.Weave the handler needs — seams so tests can
// stub ingestion and search without building a full engine.
type loreIngestor interface {
	IngestLore(ctx context.Context, ingestion eywa.LoreIngestion) error
}

type loreSearcher interface {
	SearchLore(ctx context.Context, loreID, queryText string, opts eywa.LoreSearchOptions) ([]eywa.LoreChunk, error)
}

// chunkLister is an optional capability of a LoreStore (the mongo store implements it). Stores that
// cannot enumerate chunks (pure vector databases) simply don't — the endpoint answers 501.
type chunkLister interface {
	ListChunks(ctx context.Context, loreID string, limit, offset int) ([]eywa.LoreChunk, int64, error)
}

type loreHandler struct {
	repo     eywa.LoreRepository
	store    eywa.LoreStore // optional; enables vector cleanup on delete + chunk listing
	ingestor loreIngestor   // optional; enables document ingestion
	searcher loreSearcher   // optional; enables the query tester
}

func newLoreHandler(repo eywa.LoreRepository, store eywa.LoreStore, ingestor loreIngestor, searcher loreSearcher) *loreHandler {
	return &loreHandler{repo: repo, store: store, ingestor: ingestor, searcher: searcher}
}

func (h *loreHandler) list(c *fiberlib.Ctx) error {
	lores, err := h.repo.List(c.Context())
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if lores == nil {
		lores = []eywa.Lore{}
	}
	return c.JSON(fiberlib.Map{"items": lores})
}

func (h *loreHandler) create(c *fiberlib.Ctx) error {
	var lore eywa.Lore
	if err := c.BodyParser(&lore); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if lore.Name == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "name is required"})
	}
	if _, err := h.repo.GetByName(c.Context(), lore.Name); err == nil {
		return c.Status(fiberlib.StatusConflict).JSON(fiberlib.Map{"error": "lore with this name already exists"})
	}
	if lore.ID == "" {
		lore.ID = uuid.New().String()
	}
	// The repo stamps its own copy (value receiver) — stamp here too so the response carries them.
	now := time.Now().UTC()
	lore.CreatedAt = now
	lore.UpdatedAt = now

	if err := h.repo.Create(c.Context(), lore); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.Status(fiberlib.StatusCreated).JSON(lore)
}

func (h *loreHandler) getByID(c *fiberlib.Ctx) error {
	lore, err := h.repo.GetByID(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "lore not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(lore)
}

func (h *loreHandler) update(c *fiberlib.Ctx) error {
	var lore eywa.Lore
	if err := c.BodyParser(&lore); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	lore.ID = c.Params("id")
	lore.UpdatedAt = time.Now().UTC()

	if err := h.repo.Update(c.Context(), lore); err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "lore not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(lore)
}

func (h *loreHandler) delete(c *fiberlib.Ctx) error {
	id := c.Params("id")
	if err := h.repo.Delete(c.Context(), id); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	// Vector cleanup is best-effort: the Lore config is gone; orphaned vectors are unreachable
	// (search is always scoped by lore_id) and merely occupy space until re-ingest.
	if h.store != nil {
		if err := h.store.Delete(c.Context(), id); err != nil {
			newLogger().Warnw("lore vectors not deleted", "lore_id", id, "error", err)
		}
	}
	return c.SendStatus(fiberlib.StatusNoContent)
}

func (h *loreHandler) ingestDocument(c *fiberlib.Ctx) error {
	var body struct {
		Text       string         `json:"text"`
		DocumentID string         `json:"document_id"`
		Metadata   map[string]any `json:"metadata"`
		NoChunk    bool           `json:"no_chunk"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Text == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "text is required"})
	}

	loreID := c.Params("id")
	err := h.ingestor.IngestLore(c.Context(), eywa.LoreIngestion{
		LoreID:     loreID,
		Text:       body.Text,
		DocumentID: body.DocumentID,
		Metadata:   body.Metadata,
		NoChunk:    body.NoChunk,
	})
	if err != nil {
		return c.Status(fiberlib.StatusBadGateway).JSON(fiberlib.Map{"error": err.Error()})
	}
	return c.Status(fiberlib.StatusCreated).JSON(fiberlib.Map{"lore_id": loreID, "document_id": body.DocumentID})
}

func (h *loreHandler) listChunks(c *fiberlib.Ctx) error {
	lister, ok := h.store.(chunkLister)
	if !ok {
		return c.Status(fiberlib.StatusNotImplemented).JSON(fiberlib.Map{"error": "the configured lore store cannot list chunks"})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	chunks, total, err := lister.ListChunks(c.Context(), c.Params("id"), limit, offset)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if chunks == nil {
		chunks = []eywa.LoreChunk{}
	}
	return c.JSON(fiberlib.Map{"items": chunks, "total": total, "limit": limit, "offset": offset})
}

func (h *loreHandler) query(c *fiberlib.Ctx) error {
	var body struct {
		Query    string  `json:"query"`
		TopK     int     `json:"top_k"`
		MinScore float64 `json:"min_score"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Query == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "query is required"})
	}
	if body.TopK <= 0 {
		body.TopK = 5
	}

	chunks, err := h.searcher.SearchLore(c.Context(), c.Params("id"), body.Query, eywa.LoreSearchOptions{
		TopK:     body.TopK,
		MinScore: body.MinScore,
	})
	if err != nil {
		return c.Status(fiberlib.StatusBadGateway).JSON(fiberlib.Map{"error": err.Error()})
	}
	if chunks == nil {
		chunks = []eywa.LoreChunk{}
	}
	return c.JSON(fiberlib.Map{"items": chunks})
}
