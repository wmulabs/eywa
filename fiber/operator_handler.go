package fiber

import (
	"errors"

	fiberlib "github.com/gofiber/fiber/v2"
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
)

type operatorHandler struct {
	auth *eywa.OperatorAuth
}

func newOperatorHandler(auth *eywa.OperatorAuth) *operatorHandler {
	return &operatorHandler{auth: auth}
}

func (h *operatorHandler) login(c *fiberlib.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Email == "" || body.Password == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "email and password are required"})
	}

	token, expiresAt, err := h.auth.Login(c.Context(), body.Email, body.Password)
	if err != nil {
		return c.Status(fiberlib.StatusUnauthorized).JSON(fiberlib.Map{"error": err.Error()})
	}
	return c.JSON(fiberlib.Map{"token": token, "expires_at": expiresAt})
}

func (h *operatorHandler) list(c *fiberlib.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	ops, total, err := h.auth.ListOperators(c.Context(), page, limit)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(fiberlib.Map{"items": ops, "total": total})
}

func (h *operatorHandler) create(c *fiberlib.Ctx) error {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.Name == "" || body.Email == "" || body.Password == "" || body.Role == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "name, email, password, and role are required"})
	}

	hash, err := eywa.HashPassword(body.Password)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	op := &eywa.Operator{
		Name:         body.Name,
		Email:        body.Email,
		PasswordHash: hash,
		Role:         body.Role,
		IsActive:     true,
	}
	if err := h.auth.CreateOperator(c.Context(), op); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.Status(fiberlib.StatusCreated).JSON(op)
}

func (h *operatorHandler) getByID(c *fiberlib.Ctx) error {
	id := c.Params("id")
	op, err := h.auth.FindOperatorByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "operator not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(op)
}

func (h *operatorHandler) update(c *fiberlib.Ctx) error {
	id := c.Params("id")
	existing, err := h.auth.FindOperatorByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "operator not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}

	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}

	if body.Name != "" {
		existing.Name = body.Name
	}
	if body.Email != "" {
		existing.Email = body.Email
	}
	if body.Role != "" {
		existing.Role = body.Role
	}

	if err := h.auth.UpdateOperator(c.Context(), existing); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(existing)
}

func (h *operatorHandler) deactivate(c *fiberlib.Ctx) error {
	id := c.Params("id")
	if err := h.auth.DeactivateOperator(c.Context(), id); err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "operator not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(fiberlib.Map{"id": id, "status": "deactivated"})
}
