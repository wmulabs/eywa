package fiber

import (
	"time"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type linkHandler struct {
	cache *eywa.ConfigCache
}

func newLinkHandler(cache *eywa.ConfigCache) *linkHandler {
	return &linkHandler{cache: cache}
}

type guardDTO struct {
	Field     string   `json:"field"`
	BlockList []string `json:"block_list"`
	AllowList []string `json:"allow_list"`
}

type linkDTO struct {
	EventType            string                 `json:"event_type"`
	InboundConverterName string                 `json:"inbound_converter_name"`
	RequireScouts        []string               `json:"require_scouts"`
	PathfinderName       string                 `json:"pathfinder_name"`
	AllowedSpirits       []string               `json:"allowed_spirits"`
	DefaultSpirit        string                 `json:"default_spirit"`
	VoiceName            string                 `json:"voice_name"`
	ChannelName          string                 `json:"channel_name"`
	IngestionTimeoutMs   int64                  `json:"ingestion_timeout_ms"`
	ProcessingTimeoutMs  int64                  `json:"processing_timeout_ms"`
	Guards               []guardDTO             `json:"guards"`
	Metadata             map[string]any `json:"metadata"`
}

func linkToDTO(l *eywa.Link) linkDTO {
	guards := make([]guardDTO, len(l.Guards))
	for i, g := range l.Guards {
		guards[i] = guardDTO{Field: g.Field, BlockList: g.BlockList, AllowList: g.AllowList}
	}
	return linkDTO{
		EventType:            l.EventType,
		InboundConverterName: l.InboundConverterName,
		RequireScouts:        l.RequireScouts,
		PathfinderName:       l.PathfinderName,
		AllowedSpirits:       l.AllowedSpirits,
		DefaultSpirit:        l.DefaultSpirit,
		VoiceName:            l.VoiceName,
		ChannelName:          l.ChannelName,
		IngestionTimeoutMs:   l.IngestionTimeout.Milliseconds(),
		ProcessingTimeoutMs:  l.ProcessingTimeout.Milliseconds(),
		Guards:               guards,
		Metadata:             l.Metadata,
	}
}

func dtoToLink(dto linkDTO) *eywa.Link {
	guards := make([]eywa.Guard, len(dto.Guards))
	for i, g := range dto.Guards {
		guards[i] = eywa.Guard{Field: g.Field, BlockList: g.BlockList, AllowList: g.AllowList}
	}
	link := eywa.NewLink(dto.EventType).
		WithInboundConverter(dto.InboundConverterName).
		WithScouts(dto.RequireScouts...).
		WithPathfinder(dto.PathfinderName).
		WithSpirits(dto.AllowedSpirits...).
		WithDefaultSpirit(dto.DefaultSpirit).
		WithVoice(dto.VoiceName).
		WithMetadata(dto.Metadata).
		WithGuards(guards...).
		Build()
	link.ChannelName = dto.ChannelName
	link.IngestionTimeout = time.Duration(dto.IngestionTimeoutMs) * time.Millisecond
	link.ProcessingTimeout = time.Duration(dto.ProcessingTimeoutMs) * time.Millisecond
	return link
}

func (h *linkHandler) list(c *fiberlib.Ctx) error {
	links := h.cache.GetAllLinks()
	dtos := make([]linkDTO, len(links))
	for i, l := range links {
		dtos[i] = linkToDTO(l)
	}
	return c.JSON(fiberlib.Map{"items": dtos})
}

func (h *linkHandler) getByKey(c *fiberlib.Ctx) error {
	key := c.Params("eventType")
	link, ok := h.cache.GetLink(key)
	if !ok {
		return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "event configuration not found"})
	}
	return c.JSON(linkToDTO(link))
}

func (h *linkHandler) save(c *fiberlib.Ctx) error {
	key := c.Params("eventType")
	var dto linkDTO
	if err := c.BodyParser(&dto); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if dto.EventType == "" {
		dto.EventType = key
	}
	if dto.EventType != key {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "event_type in body must match URL parameter"})
	}
	link := dtoToLink(dto)
	if err := h.cache.SaveLink(c.Context(), link); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(linkToDTO(link))
}

func (h *linkHandler) delete(c *fiberlib.Ctx) error {
	key := c.Params("eventType")
	if err := h.cache.DeleteLink(c.Context(), key); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.SendStatus(fiberlib.StatusNoContent)
}
