# Operator CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full operator lifecycle management — create, list, get, update, deactivate — behind admin-only routes, plus a public token endpoint for login.

**Architecture:** `OperatorAuth` gains CRUD delegation methods that proxy to its private `repo`; a new `mongo.OperatorRepository` implements `eywa.OperatorRepository`; a new fiber handler owns the HTTP layer. The login route (`POST /api/v1/auth/token`) is registered directly on `app` (bypassing the `authMW` group). Operator CRUD routes (`/api/v1/operators`) live inside the `api` group and are guarded by `middleware.RequireRole(eywa.RoleAdmin)`.

**Tech Stack:** Go, MongoDB (`go.mongodb.org/mongo-driver`), Fiber v2, `golang.org/x/crypto/bcrypt`, `go-primitive.NewObjectID`

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/infrastructure/driven/auth/operator_auth.go` |
| Create | `mongo/operator_repository.go` |
| Create | `fiber/operator_handler.go` |
| Create | `fiber/operator_handler_test.go` |
| Modify | `fiber/management.go` |

---

### Task 1: OperatorAuth CRUD Delegation Methods

**Files:**
- Modify: `internal/infrastructure/driven/auth/operator_auth.go`

- [ ] **Step 1: Add delegation methods**

Append these methods after `Validate`. They proxy directly to `a.repo`.

```go
func (a *OperatorAuth) CreateOperator(ctx context.Context, op *ports.OperatorEntity) error {
	return a.repo.Create(ctx, op)
}

func (a *OperatorAuth) ListOperators(ctx context.Context, page, limit int) ([]*ports.OperatorEntity, int64, error) {
	return a.repo.List(ctx, page, limit)
}

func (a *OperatorAuth) FindOperatorByID(ctx context.Context, id string) (*ports.OperatorEntity, error) {
	return a.repo.FindByID(ctx, id)
}

func (a *OperatorAuth) UpdateOperator(ctx context.Context, op *ports.OperatorEntity) error {
	return a.repo.Update(ctx, op)
}

func (a *OperatorAuth) DeactivateOperator(ctx context.Context, id string) error {
	return a.repo.Deactivate(ctx, id)
}
```

Note: `ports.OperatorEntity` is actually `entities.Operator` — the import alias used inside `operator_auth.go` package is `github.com/wmulabs/eywa/internal/domain/entities`. Use `*entities.Operator` to match existing code in that file. The import is already present.

Corrected version:

```go
func (a *OperatorAuth) CreateOperator(ctx context.Context, op *entities.Operator) error {
	return a.repo.Create(ctx, op)
}

func (a *OperatorAuth) ListOperators(ctx context.Context, page, limit int) ([]*entities.Operator, int64, error) {
	return a.repo.List(ctx, page, limit)
}

func (a *OperatorAuth) FindOperatorByID(ctx context.Context, id string) (*entities.Operator, error) {
	return a.repo.FindByID(ctx, id)
}

func (a *OperatorAuth) UpdateOperator(ctx context.Context, op *entities.Operator) error {
	return a.repo.Update(ctx, op)
}

func (a *OperatorAuth) DeactivateOperator(ctx context.Context, id string) error {
	return a.repo.Deactivate(ctx, id)
}
```

The `entities` import is already present in that file via `github.com/wmulabs/eywa/internal/domain/ports` which imports entities. But `entities` itself is not imported directly — you'll need to add it:

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/wmulabs/eywa/internal/domain/entities"
    "github.com/wmulabs/eywa/internal/domain/ports"
    "golang.org/x/crypto/bcrypt"
)
```

- [ ] **Step 2: Verify build**

```bash
cd /path/to/eywa && go build ./internal/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/driven/auth/operator_auth.go
git commit -m "feat(operator): add CRUD delegation methods to OperatorAuth"
```

---

### Task 2: Mongo OperatorRepository

**Files:**
- Create: `mongo/operator_repository.go`

- [ ] **Step 1: Write failing test (integration-style build check)**

The mongo sub-module has no unit tests for individual repos — the compile-time interface check serves as the test. Add the file with the interface assertion first:

```go
package mongo

import (
    eywa "github.com/wmulabs/eywa"
)

var _ eywa.OperatorRepository = (*OperatorRepository)(nil)
```

- [ ] **Step 2: Verify it fails (compilation error expected)**

```bash
cd /path/to/eywa/mongo && go build .
```

Expected: `undefined: OperatorRepository` — confirms the stub is needed.

- [ ] **Step 3: Implement OperatorRepository**

Full file `mongo/operator_repository.go`:

```go
package mongo

import (
	"context"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var _ eywa.OperatorRepository = (*OperatorRepository)(nil)

type OperatorRepository struct {
	collection *mongodriver.Collection
	logger     *zap.SugaredLogger
}

func NewOperatorRepository(database *mongodriver.Database) *OperatorRepository {
	repo := &OperatorRepository{
		collection: database.Collection("operators"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *OperatorRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongodriver.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetName("idx_email_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "is_active", Value: 1}},
			Options: options.Index().SetName("idx_is_active"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create operators indexes", "error", err)
	}
}

func (r *OperatorRepository) Create(ctx context.Context, op *eywa.Operator) error {
	if op.ID == "" {
		op.ID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	op.CreatedAt = now
	op.UpdatedAt = now
	_, err := r.collection.InsertOne(ctx, op)
	return err
}

func (r *OperatorRepository) FindByEmail(ctx context.Context, email string) (*eywa.Operator, error) {
	var op eywa.Operator
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&op)
	if err == mongodriver.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "operator", ID: email}
	}
	return &op, err
}

func (r *OperatorRepository) FindByID(ctx context.Context, id string) (*eywa.Operator, error) {
	var op eywa.Operator
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&op)
	if err == mongodriver.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "operator", ID: id}
	}
	return &op, err
}

func (r *OperatorRepository) List(ctx context.Context, page, limit int) ([]*eywa.Operator, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	cursor, err := r.collection.Find(ctx, bson.M{},
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetSkip(skip).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var ops []*eywa.Operator
	if err := cursor.All(ctx, &ops); err != nil {
		return nil, 0, err
	}
	return ops, total, nil
}

func (r *OperatorRepository) Update(ctx context.Context, op *eywa.Operator) error {
	op.UpdatedAt = time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"name":       op.Name,
			"email":      op.Email,
			"role":       op.Role,
			"is_active":  op.IsActive,
			"updated_at": op.UpdatedAt,
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": op.ID}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return &eywa.NotFoundError{Entity: "operator", ID: op.ID}
	}
	return nil
}

func (r *OperatorRepository) Deactivate(ctx context.Context, id string) error {
	update := bson.M{
		"$set": bson.M{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return &eywa.NotFoundError{Entity: "operator", ID: id}
	}
	return nil
}
```

Note: `eywa.Operator` is the root re-export of `entities.Operator` (from `entities.go`). Use it directly to keep mongo sub-module using root package only.

- [ ] **Step 4: Verify build passes**

```bash
cd /path/to/eywa/mongo && go build .
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add mongo/operator_repository.go
git commit -m "feat(operator): add mongo.OperatorRepository with email unique index"
```

---

### Task 3: Fiber Operator Handler + Routes

**Files:**
- Create: `fiber/operator_handler.go`
- Create: `fiber/operator_handler_test.go`
- Modify: `fiber/management.go`

#### Step 3a: Handler

- [ ] **Step 1: Write failing tests first**

Create `fiber/operator_handler_test.go`:

```go
package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

// stubOperatorAuth wraps a stub repo and the real OperatorAuth for login tests.
type stubOperatorRepo struct {
	operators map[string]*eywa.Operator
	err       error
}

func newStubOperatorRepo() *stubOperatorRepo {
	return &stubOperatorRepo{operators: make(map[string]*eywa.Operator)}
}

func (s *stubOperatorRepo) Create(_ context.Context, op *eywa.Operator) error {
	if s.err != nil {
		return s.err
	}
	if op.ID == "" {
		op.ID = "generated-id"
	}
	s.operators[op.ID] = op
	return nil
}

func (s *stubOperatorRepo) FindByEmail(_ context.Context, email string) (*eywa.Operator, error) {
	for _, op := range s.operators {
		if op.Email == email {
			return op, nil
		}
	}
	return nil, eywa.ErrNotFound
}

func (s *stubOperatorRepo) FindByID(_ context.Context, id string) (*eywa.Operator, error) {
	op, ok := s.operators[id]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return op, s.err
}

func (s *stubOperatorRepo) List(_ context.Context, _, _ int) ([]*eywa.Operator, int64, error) {
	ops := make([]*eywa.Operator, 0, len(s.operators))
	for _, op := range s.operators {
		ops = append(ops, op)
	}
	return ops, int64(len(ops)), s.err
}

func (s *stubOperatorRepo) Update(_ context.Context, op *eywa.Operator) error {
	if s.err != nil {
		return s.err
	}
	if _, ok := s.operators[op.ID]; !ok {
		return eywa.ErrNotFound
	}
	s.operators[op.ID] = op
	return nil
}

func (s *stubOperatorRepo) Deactivate(_ context.Context, id string) error {
	if s.err != nil {
		return s.err
	}
	op, ok := s.operators[id]
	if !ok {
		return eywa.ErrNotFound
	}
	op.IsActive = false
	return nil
}

func seedOperator(repo *stubOperatorRepo, id, email, role string) *eywa.Operator {
	hash, _ := eywa.HashPassword("secret")
	op := &eywa.Operator{
		ID:           id,
		Name:         "Test Op",
		Email:        email,
		Role:         role,
		PasswordHash: hash,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	repo.operators[id] = op
	return op
}

func operatorDeps(repo *stubOperatorRepo) ManagementDeps {
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	return ManagementDeps{
		OperatorAuth: auth,
	}
}

func TestOperatorHandler_Login_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	deps := operatorDeps(repo)
	app := buildMgmtTestApp(deps)

	body := map[string]string{"email": "admin@test.com", "password": "secret"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == nil {
		t.Error("want token in response, got nil")
	}
}

func TestOperatorHandler_Login_WrongPassword_Returns401(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	deps := operatorDeps(repo)
	app := buildMgmtTestApp(deps)

	body := map[string]string{"email": "admin@test.com", "password": "wrong"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/auth/token", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_List_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	deps := ManagementDeps{
		APIKeys:      map[string]string{"admin-key": eywa.RoleAdmin},
		OperatorAuth: auth,
	}
	app := buildMgmtTestApp(deps)

	req := httptest.NewRequest("GET", "/api/v1/operators", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["items"] == nil {
		t.Error("want items in response")
	}
}

func TestOperatorHandler_List_ForbiddenForOperatorRole_Returns403(t *testing.T) {
	repo := newStubOperatorRepo()
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	deps := ManagementDeps{
		APIKeys:      map[string]string{"op-key": eywa.RoleOperator},
		OperatorAuth: auth,
	}
	app := buildMgmtTestApp(deps)

	req := httptest.NewRequest("GET", "/api/v1/operators", nil)
	req.Header.Set("Authorization", "Bearer op-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 403 {
		t.Errorf("want 403, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Create_Returns201(t *testing.T) {
	repo := newStubOperatorRepo()
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	deps := ManagementDeps{
		APIKeys:      map[string]string{"admin-key": eywa.RoleAdmin},
		OperatorAuth: auth,
	}
	app := buildMgmtTestApp(deps)

	body := map[string]string{"name": "Alice", "email": "alice@test.com", "password": "pass123", "role": eywa.RoleOperator}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 201 {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] == nil {
		t.Error("want id in response")
	}
}

func TestOperatorHandler_Create_MissingFields_Returns400(t *testing.T) {
	repo := newStubOperatorRepo()
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	deps := ManagementDeps{
		APIKeys:      map[string]string{"admin-key": eywa.RoleAdmin},
		OperatorAuth: auth,
	}
	app := buildMgmtTestApp(deps)

	body := map[string]string{"name": "Alice"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/operators", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer admin-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_GetByID_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	deps := ManagementDeps{
		APIKeys:      map[string]string{"admin-key": eywa.RoleAdmin},
		OperatorAuth: auth,
	}
	app := buildMgmtTestApp(deps)

	req := httptest.NewRequest("GET", "/api/v1/operators/op-1", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestOperatorHandler_Deactivate_Returns200(t *testing.T) {
	repo := newStubOperatorRepo()
	seedOperator(repo, "op-1", "admin@test.com", eywa.RoleAdmin)
	auth := eywa.NewOperatorAuth(repo, []byte("test-secret"))
	deps := ManagementDeps{
		APIKeys:      map[string]string{"admin-key": eywa.RoleAdmin},
		OperatorAuth: auth,
	}
	app := buildMgmtTestApp(deps)

	req := httptest.NewRequest("DELETE", "/api/v1/operators/op-1", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if !repo.operators["op-1"].IsActive == false {
		t.Error("want operator to be deactivated")
	}
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
cd /path/to/eywa/fiber && go test ./... -run TestOperator -v 2>&1 | head -30
```

Expected: compile error — `operatorDeps` / handler routes not found.

- [ ] **Step 3: Implement operator_handler.go**

Create `fiber/operator_handler.go`:

```go
package fiber

import (
	"errors"

	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
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
```

- [ ] **Step 4: Update management.go — add login route + operator routes**

In `RegisterManagementRoutes`, add the following after the `authMW` and `api` group setup and before the first `if deps.ChronicleQueryRepo != nil` block:

```go
// Public auth route — registered on app directly to bypass authMW.
if deps.OperatorAuth != nil {
    oh := newOperatorHandler(deps.OperatorAuth)
    app.Post("/api/v1/auth/token", oh.login)

    ops := api.Group("/operators", middleware.RequireRole(eywa.RoleAdmin))
    ops.Get("", oh.list)
    ops.Post("", oh.create)
    ops.Get("/:id", oh.getByID)
    ops.Put("/:id", oh.update)
    ops.Delete("/:id", oh.deactivate)
}
```

The final `RegisterManagementRoutes` function after the edit should look like:

```go
func RegisterManagementRoutes(app *fiberlib.App, weave *eywa.Weave, deps ManagementDeps) {
	validators := buildValidatorChain(deps)
	if len(validators) == 0 {
		panic("RegisterManagementRoutes: at least one auth validator must be configured")
	}
	authMW := middleware.AuthMiddleware(validators...)

	api := app.Group("/api/v1", authMW)

	if deps.OperatorAuth != nil {
		oh := newOperatorHandler(deps.OperatorAuth)
		app.Post("/api/v1/auth/token", oh.login)

		ops := api.Group("/operators", middleware.RequireRole(eywa.RoleAdmin))
		ops.Get("", oh.list)
		ops.Post("", oh.create)
		ops.Get("/:id", oh.getByID)
		ops.Put("/:id", oh.update)
		ops.Delete("/:id", oh.deactivate)
	}

	if deps.ChronicleQueryRepo != nil {
		// ... existing code unchanged ...
	}
	// ... rest unchanged ...
}
```

- [ ] **Step 5: Run tests — expect pass**

```bash
cd /path/to/eywa/fiber && go test ./... -run TestOperator -v
```

Expected: all 8 tests pass.

- [ ] **Step 6: Run full fiber test suite**

```bash
cd /path/to/eywa/fiber && go test ./... -v 2>&1 | tail -20
```

Expected: all tests pass, no regressions.

- [ ] **Step 7: Commit**

```bash
git add fiber/operator_handler.go fiber/operator_handler_test.go fiber/management.go
git commit -m "feat(operator): fiber handler with login + admin CRUD routes"
```

---

### Task 4: Full Verification

**Files:** none (read-only)

- [ ] **Step 1: Build root package**

```bash
cd /path/to/eywa && go build ./...
```

Expected: no errors.

- [ ] **Step 2: Build mongo sub-module**

```bash
cd /path/to/eywa/mongo && go build .
```

Expected: no errors.

- [ ] **Step 3: Build fiber sub-module**

```bash
cd /path/to/eywa/fiber && go build .
```

Expected: no errors.

- [ ] **Step 4: Run all tests**

```bash
cd /path/to/eywa && go test ./...
cd /path/to/eywa/fiber && go test ./...
cd /path/to/eywa/mongo && go test ./... 2>/dev/null || true
```

Expected: root + fiber tests pass; mongo skipped (requires live DB).

- [ ] **Step 5: Commit if any final fixes were needed**

```bash
git add -A
git commit -m "fix(operator): address review feedback"
```

---

## Route Summary

| Method | Path | Auth | Role |
|--------|------|------|------|
| POST | `/api/v1/auth/token` | public | — |
| GET | `/api/v1/operators` | JWT | admin |
| POST | `/api/v1/operators` | JWT | admin |
| GET | `/api/v1/operators/:id` | JWT | admin |
| PUT | `/api/v1/operators/:id` | JWT | admin |
| DELETE | `/api/v1/operators/:id` | JWT | admin |
