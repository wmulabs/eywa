package fiber

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	"go.opentelemetry.io/otel/trace/noop"
)

type SpiritManagementHandler struct {
	spiritRepo eywa.SpiritRepository
}

func NewSpiritManagementHandler(spiritRepo eywa.SpiritRepository) *SpiritManagementHandler {
	return &SpiritManagementHandler{spiritRepo: spiritRepo}
}

func (h *SpiritManagementHandler) CreateSpirit(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Create")
	defer span.End()

	log := newLogger()

	var req struct {
		Name                      string               `json:"name"`
		Description               string               `json:"description"`
		Specialization            string               `json:"specialization"`
		SystemPrompt              string               `json:"system_prompt"`
		EnforceVoiceDelivery      bool                 `json:"enforce_voice_delivery,omitempty"`
		VoiceDeliveryInstructions string               `json:"voice_delivery_instructions,omitempty"`
		BusinessErrorInstructions string               `json:"business_error_instructions,omitempty"`
		AllowedActions            []eywa.AllowedAction `json:"allowed_actions"`
		ModelConfig               eywa.SpiritModel     `json:"model_config"`
		Metadata                  map[string]any       `json:"metadata"`
	}

	if err := c.BodyParser(&req); err != nil {
		log.Errorw("failed to parse request", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid request format"})
	}

	if errs := resthttp.ValidateSpiritCreate(req.Name, req.SystemPrompt, req.ModelConfig); errs.HasErrors() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Validation failed", "details": errs.Errors})
	}

	spirit := &eywa.Spirit{
		ID:                        uuid.New().String(),
		Name:                      req.Name,
		Description:               req.Description,
		Specialization:            req.Specialization,
		SystemPrompt:              req.SystemPrompt,
		EnforceVoiceDelivery:      req.EnforceVoiceDelivery,
		VoiceDeliveryInstructions: req.VoiceDeliveryInstructions,
		BusinessErrorInstructions: req.BusinessErrorInstructions,
		AllowedActions:            req.AllowedActions,
		ModelConfig:               req.ModelConfig,
		Metadata:                  req.Metadata,
		CreatedBy:                 c.Get("X-User-ID", "system"),
	}

	if err := h.spiritRepo.Create(ctx, spirit); err != nil {
		log.Errorw("failed to create spirit", "error", err)
		return resthttp.ErrorResponse(c, err)
	}

	log.Infow("spirit created", "name", req.Name, "id", spirit.ID)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "spirit": spirit})
}

func (h *SpiritManagementHandler) GetSpirit(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Get")
	defer span.End()

	log := newLogger()
	name := c.Params("name")

	spirit, err := h.spiritRepo.FindActiveByName(ctx, name)
	if err != nil {
		log.Warnw("failed to get spirit", "name", name, "error", err)
		return resthttp.ErrorResponse(c, err)
	}

	return c.JSON(fiber.Map{"success": true, "spirit": spirit})
}

func (h *SpiritManagementHandler) UpdateSpirit(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Update")
	defer span.End()

	log := newLogger()
	name := c.Params("name")

	var req struct {
		Description               string               `json:"description"`
		Specialization            string               `json:"specialization"`
		SystemPrompt              string               `json:"system_prompt"`
		EnforceVoiceDelivery      bool                 `json:"enforce_voice_delivery,omitempty"`
		VoiceDeliveryInstructions string               `json:"voice_delivery_instructions,omitempty"`
		BusinessErrorInstructions string               `json:"business_error_instructions,omitempty"`
		AllowedActions            []eywa.AllowedAction `json:"allowed_actions"`
		ModelConfig               eywa.SpiritModel     `json:"model_config"`
		Metadata                  map[string]any       `json:"metadata"`
	}

	if err := c.BodyParser(&req); err != nil {
		log.Errorw("failed to parse request", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid request format"})
	}

	if errs := resthttp.ValidateSpiritUpdate(req.SystemPrompt, req.ModelConfig); errs.HasErrors() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Validation failed", "details": errs.Errors})
	}

	spirit := &eywa.Spirit{
		Name:                      name,
		Description:               req.Description,
		Specialization:            req.Specialization,
		SystemPrompt:              req.SystemPrompt,
		EnforceVoiceDelivery:      req.EnforceVoiceDelivery,
		VoiceDeliveryInstructions: req.VoiceDeliveryInstructions,
		BusinessErrorInstructions: req.BusinessErrorInstructions,
		AllowedActions:            req.AllowedActions,
		ModelConfig:               req.ModelConfig,
		Metadata:                  req.Metadata,
		CreatedBy:                 c.Get("X-User-ID", "system"),
	}

	changeLog := c.Get("X-Change-Log", "Spirit updated via API")

	updated, err := h.spiritRepo.Update(ctx, name, spirit, changeLog)
	if err != nil {
		log.Errorw("failed to update spirit", "error", err, "name", name)
		return resthttp.ErrorResponse(c, err)
	}

	log.Infow("spirit updated", "name", name, "version", updated.Version)
	return c.JSON(fiber.Map{"success": true, "spirit": updated})
}

func (h *SpiritManagementHandler) DeleteSpirit(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Delete")
	defer span.End()

	log := newLogger()
	name := c.Params("name")

	spirit, err := h.spiritRepo.FindActiveByName(ctx, name)
	if err != nil {
		log.Warnw("failed to find spirit for deletion", "error", err, "name", name)
		return resthttp.ErrorResponse(c, err)
	}

	if err := h.spiritRepo.Deactivate(ctx, spirit.ID); err != nil {
		log.Errorw("failed to deactivate spirit", "error", err, "name", name)
		return resthttp.ErrorResponse(c, err)
	}

	log.Infow("spirit deactivated", "name", name)
	return c.JSON(fiber.Map{"success": true, "message": "Spirit deactivated successfully"})
}

func (h *SpiritManagementHandler) ListSpirits(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/List")
	defer span.End()

	log := newLogger()
	limit, offset := resthttp.ParsePagination(c)

	total, err := h.spiritRepo.CountActive(ctx)
	if err != nil {
		log.Errorw("failed to count spirits", "error", err)
		return resthttp.ErrorResponse(c, err)
	}

	spirits, err := h.spiritRepo.ListActive(ctx, limit, offset)
	if err != nil {
		log.Errorw("failed to list spirits", "error", err)
		return resthttp.ErrorResponse(c, err)
	}

	if spirits == nil {
		spirits = []*eywa.Spirit{}
	}

	return c.JSON(fiber.Map{"items": spirits, "total": total, "limit": limit, "offset": offset})
}

func (h *SpiritManagementHandler) ActivateSpirit(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Activate")
	defer span.End()

	log := newLogger()
	name := c.Params("name")

	var req struct {
		Version int `json:"version"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid request format"})
	}

	if errs := resthttp.ValidateVersion(req.Version); errs.HasErrors() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Validation failed", "details": errs.Errors})
	}

	if err := h.spiritRepo.Activate(ctx, name, req.Version); err != nil {
		log.Errorw("failed to activate spirit", "error", err, "name", name)
		return resthttp.ErrorResponse(c, err)
	}

	log.Infow("spirit activated", "name", name, "version", req.Version)
	return c.JSON(fiber.Map{"success": true, "message": "Spirit activated successfully"})
}

func (h *SpiritManagementHandler) DeactivateSpirit(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Deactivate")
	defer span.End()

	log := newLogger()
	name := c.Params("name")

	spirit, err := h.spiritRepo.FindActiveByName(ctx, name)
	if err != nil {
		log.Warnw("failed to find spirit for deactivation", "error", err, "name", name)
		return resthttp.ErrorResponse(c, err)
	}

	if err := h.spiritRepo.Deactivate(ctx, spirit.ID); err != nil {
		log.Errorw("failed to deactivate spirit", "error", err, "name", name)
		return resthttp.ErrorResponse(c, err)
	}

	log.Infow("spirit deactivated", "name", name)
	return c.JSON(fiber.Map{"success": true, "message": "Spirit deactivated successfully"})
}

// GetSpiritVersions returns the full version history for a Spirit, newest first.
func (h *SpiritManagementHandler) GetSpiritVersions(c *fiber.Ctx) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(c.Context(), "Handler/Spirit/Versions")
	defer span.End()

	name := c.Params("name")
	versions, err := h.spiritRepo.FindVersionHistory(ctx, name)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if versions == nil {
		versions = []*eywa.Spirit{}
	}
	return c.JSON(fiber.Map{"name": name, "versions": versions, "total": len(versions)})
}
