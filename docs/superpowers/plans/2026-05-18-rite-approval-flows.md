# Rite — Approval Flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the Spirit request operator authorization (`request_rite` action), persist it in MongoDB, and resume the conversation when the operator approves or rejects via the management API.

**Architecture:** A `Rite` entity is stored in MongoDB (`rites` collection). A built-in `request_rite` action creates a pending Rite and returns `{"rite_id":"...","status":"pending"}` to the Spirit, ending its turn. When an operator approves or rejects via `POST /api/v1/rites/:id/approve|reject`, the handler calls `repo.Decide()` then fires a new `Pulse` with `rite_id`, `rite_status`, and `rite_context` in Knowledge — the Spirit resumes with full context. The `RegisterManagementRoutes` weave parameter (previously ignored) is now used to fire these Pulses.

**Tech Stack:** Go, MongoDB (`go.mongodb.org/mongo-driver`), Fiber v2, standard library.

---

## Notes for implementer

- Sub-modules (`mongo/`, `fiber/`) **cannot import `internal/`**. Use `eywa "github.com/wmulabs/eywa"` and re-exported types only.
- `buildMgmtTestApp` is defined in `fiber/chronicle_handler_test.go` — **DO NOT redefine it**.
- `mapArg` and `stringArg` helpers are defined in `internal/implementation/actions/ritual_tools.go` — reuse them in `rite_tools.go` (same package).
- `helpers.GenerateRandomID()` generates IDs in the root module's `internal/helpers` package.
- `helpers.NowUTC()` returns `time.Time` — always use instead of `time.Now()` for timestamps.
- `ports.SessionContextKey{}` is the context key for `*entities.Memory` — same pattern as `schedule_ritual`.
- `primitive.NewObjectID().Hex()` — Mongo repos generate IDs this way (see `mongo/ritual_repository.go`).
- `eywa.NotFoundError{Entity: "rite", ID: id}` — same pattern as other Mongo repos.
- The fiber `RegisterManagementRoutes` second parameter is `_ *eywa.Weave` today. Task 6 changes it to `weave *eywa.Weave` — **not a breaking change** since callers already pass it.
- When `weave == nil` (in tests via `buildMgmtTestApp(nil, deps)`), skip Pulse firing silently.
- All commands run from `/Users/willianmoraes/PROJETOS/eywa` unless step says `cd mongo` or `cd fiber`.

---

## File Map

**Root module — new:**
- `internal/domain/entities/rite.go` — `Rite` struct + `RiteStatus` type + constants
- `internal/domain/ports/rite_repository.go` — `RiteRepository` interface + `RiteListOptions`
- `internal/implementation/actions/rite_tools.go` — `RequestRiteAction` (built-in action)
- `internal/implementation/actions/rite_tools_test.go` — 4 unit tests

**Root module — modified:**
- `internal/implementation/orchestrator/builder.go` — add `riteRepo` field, `WithRiteRepository`, auto-create registry, register action in `Build()`
- `entities.go` — re-export `Rite`, `RiteStatus`, status constants
- `ports.go` — re-export `RiteRepository`, `RiteListOptions`
- `builtin.go` — re-export `NewRequestRiteAction`

**Mongo sub-module — new:**
- `mongo/rite_repository.go` — `Create`, `FindByID`, `List`, `Decide` with indexes

**Fiber sub-module — new/modified:**
- `fiber/rite_handler_test.go` — 7 tests (TDD first)
- `fiber/rite_handler.go` — `list`, `getByID`, `approve`, `reject` handlers
- `fiber/management.go` — add `RiteRepo`, use `weave` param, register rite routes

---

## Task 1: Rite entity + RiteRepository port

**Files:**
- Create: `internal/domain/entities/rite.go`
- Create: `internal/domain/ports/rite_repository.go`

- [ ] **Step 1.1 — Create `internal/domain/entities/rite.go`**

```go
package entities

import "time"

type RiteStatus string

const (
	RitePending  RiteStatus = "pending"
	RiteApproved RiteStatus = "approved"
	RiteRejected RiteStatus = "rejected"
	RiteExpired  RiteStatus = "expired"
)

type Rite struct {
	ID          string                 `bson:"_id"`
	MemoryKey   string                 `bson:"memory_key"`
	SubjectKey  string                 `bson:"subject_key,omitempty"`
	EventKey    string                 `bson:"event_key"`
	Context     map[string]interface{} `bson:"context"`
	Reason      string                 `bson:"reason"`
	Status      RiteStatus             `bson:"status"`
	OperatorID  string                 `bson:"operator_id,omitempty"`
	RequestedAt time.Time              `bson:"requested_at"`
	DecidedAt   *time.Time             `bson:"decided_at,omitempty"`
	ExpiresAt   *time.Time             `bson:"expires_at,omitempty"`
}
```

- [ ] **Step 1.2 — Create `internal/domain/ports/rite_repository.go`**

```go
package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type RiteListOptions struct {
	MemoryKey string
	Status    entities.RiteStatus
	Page      int
	Limit     int
}

type RiteRepository interface {
	Create(ctx context.Context, rite *entities.Rite) error
	FindByID(ctx context.Context, id string) (*entities.Rite, error)
	List(ctx context.Context, opts RiteListOptions) ([]*entities.Rite, int64, error)
	Decide(ctx context.Context, id, operatorID string, status entities.RiteStatus) error
}
```

- [ ] **Step 1.3 — Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 1.4 — Commit**

```bash
git add internal/domain/entities/rite.go \
        internal/domain/ports/rite_repository.go
git commit -m "feat(rite): Rite entity and RiteRepository port"
```

---

## Task 2: request_rite Action (TDD)

**Files:**
- Create: `internal/implementation/actions/rite_tools_test.go`
- Create: `internal/implementation/actions/rite_tools.go`

- [ ] **Step 2.1 — Write failing tests**

Create `internal/implementation/actions/rite_tools_test.go`:

```go
package actions

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubRiteRepo struct {
	created []*entities.Rite
	err     error
}

func (s *stubRiteRepo) Create(_ context.Context, rite *entities.Rite) error {
	s.created = append(s.created, rite)
	return s.err
}
func (s *stubRiteRepo) FindByID(_ context.Context, _ string) (*entities.Rite, error) {
	return nil, nil
}
func (s *stubRiteRepo) List(_ context.Context, _ ports.RiteListOptions) ([]*entities.Rite, int64, error) {
	return nil, 0, nil
}
func (s *stubRiteRepo) Decide(_ context.Context, _, _ string, _ entities.RiteStatus) error {
	return nil
}

var _ ports.RiteRepository = (*stubRiteRepo)(nil)

func riteCtx(memoryKey, subjectKey string) context.Context {
	mem := &entities.Memory{MemoryKey: memoryKey, SubjectKey: subjectKey}
	return context.WithValue(context.Background(), ports.SessionContextKey{}, mem)
}

func TestRequestRiteAction_Execute_CreatesPendingRite(t *testing.T) {
	repo := &stubRiteRepo{}
	action := NewRequestRiteAction(repo)

	ctx := riteCtx("whatsapp:+5511999999999", "")
	result, err := action.Execute(ctx, map[string]interface{}{
		"event_key": "resume_order",
		"reason":    "Large order requires approval",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 rite created, got %d", len(repo.created))
	}
	rite := repo.created[0]
	if rite.EventKey != "resume_order" {
		t.Errorf("want event_key=resume_order, got %s", rite.EventKey)
	}
	if rite.Status != entities.RitePending {
		t.Errorf("want status=pending, got %s", rite.Status)
	}
	if rite.MemoryKey != "whatsapp:+5511999999999" {
		t.Errorf("want memory_key=whatsapp:+5511999999999, got %s", rite.MemoryKey)
	}
	if rite.ID == "" {
		t.Error("expected non-empty rite ID")
	}
	if result == "" {
		t.Error("expected non-empty result JSON")
	}
}

func TestRequestRiteAction_Validate_MissingEventKey_ReturnsBizError(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{})
	err := action.Validate(map[string]interface{}{"reason": "some reason"})
	if err == nil {
		t.Error("expected error for missing event_key")
	}
}

func TestRequestRiteAction_Validate_MissingReason_ReturnsBizError(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{})
	err := action.Validate(map[string]interface{}{"event_key": "some_event"})
	if err == nil {
		t.Error("expected error for missing reason")
	}
}

func TestRequestRiteAction_Execute_NoSession_ReturnsInfraError(t *testing.T) {
	action := NewRequestRiteAction(&stubRiteRepo{})
	_, err := action.Execute(context.Background(), map[string]interface{}{
		"event_key": "resume",
		"reason":    "needs approval",
	})
	if err == nil {
		t.Error("expected error when session missing from context")
	}
}
```

- [ ] **Step 2.2 — Run tests to verify they fail**

```bash
go test ./internal/implementation/actions/... -run TestRequestRite -v 2>&1 | head -15
```

Expected: compile error — `NewRequestRiteAction` undefined.

- [ ] **Step 2.3 — Create `internal/implementation/actions/rite_tools.go`**

```go
package actions

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
)

type RequestRiteAction struct {
	riteRepo ports.RiteRepository
}

func NewRequestRiteAction(riteRepo ports.RiteRepository) ports.Action {
	return &RequestRiteAction{riteRepo: riteRepo}
}

func (t *RequestRiteAction) GetName() string { return "request_rite" }

func (t *RequestRiteAction) GetDescription() string {
	return "Requests operator authorization for a sensitive action. " +
		"Creates a pending rite and suspends the current flow. " +
		"The operator will approve or reject, triggering the specified event_key."
}

func (t *RequestRiteAction) GetParameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"event_key": map[string]interface{}{
				"type":        "string",
				"description": "Event key to fire when the operator decides (approve or reject).",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Why authorization is being requested.",
			},
			"context": map[string]interface{}{
				"type":        "object",
				"description": "Key-value data forwarded to the operator and injected into the resume Pulse.",
			},
		},
		"required": []string{"event_key", "reason"},
	}
}

func (t *RequestRiteAction) Validate(args map[string]interface{}) error {
	if eventKey, ok := args["event_key"].(string); !ok || eventKey == "" {
		return domainerrors.NewBusinessError("missing or invalid 'event_key'")
	}
	if reason, ok := args["reason"].(string); !ok || reason == "" {
		return domainerrors.NewBusinessError("missing or invalid 'reason'")
	}
	return nil
}

func (t *RequestRiteAction) IsCritical() bool                  { return true }
func (t *RequestRiteAction) GetCategory() ports.ActionCategory { return ports.ActionGeneral }

func (t *RequestRiteAction) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	session, ok := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	if !ok || session == nil {
		return "", domainerrors.NewInfrastructureError("session not available in context")
	}

	rite := &entities.Rite{
		ID:          helpers.GenerateRandomID(),
		MemoryKey:   session.MemoryKey,
		SubjectKey:  session.SubjectKey,
		EventKey:    args["event_key"].(string),
		Reason:      args["reason"].(string),
		Context:     mapArg(args, "context"),
		Status:      entities.RitePending,
		RequestedAt: helpers.NowUTC(),
	}

	if err := t.riteRepo.Create(ctx, rite); err != nil {
		return "", fmt.Errorf("create rite: %w", err)
	}

	return fmt.Sprintf(`{"rite_id":"%s","status":"pending"}`, rite.ID), nil
}
```

- [ ] **Step 2.4 — Run tests**

```bash
go test ./internal/implementation/actions/... -run TestRequestRite -v
```

Expected: all 4 tests pass.

- [ ] **Step 2.5 — Run full internal suite**

```bash
go test ./internal/...
```

Expected: all pass.

- [ ] **Step 2.6 — Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 2.7 — Commit**

```bash
git add internal/implementation/actions/rite_tools.go \
        internal/implementation/actions/rite_tools_test.go
git commit -m "feat(rite): request_rite action — creates pending rite from Spirit context"
```

---

## Task 3: Builder wiring

**Files:**
- Modify: `internal/implementation/orchestrator/builder.go`

- [ ] **Step 3.1 — Add `riteRepo` field to `WeaveBuilder` struct**

Read `internal/implementation/orchestrator/builder.go`. Find:
```go
	vigilRepo       ports.VigilRepository
	vigilConfig     VigilConfig
```

Add after `vigilConfig`:
```go
	riteRepo        ports.RiteRepository
```

- [ ] **Step 3.2 — Add `WithRiteRepository` method**

Find the `WithVigilConfig` method (around line 347). Add after it:

```go
func (b *WeaveBuilder) WithRiteRepository(repo ports.RiteRepository) *WeaveBuilder {
	b.riteRepo = repo
	return b
}
```

- [ ] **Step 3.3 — Auto-create action registry when riteRepo is set**

Find this block (around line 609):
```go
	// Auto-create action registry if HTTP tools are configured but no registry was provided.
	if b.httpToolRepo != nil && b.actionRegistry == nil {
		b.actionRegistry = registries.NewActionRegistry()
	}
```

Change it to:
```go
	// Auto-create action registry if HTTP tools or Rite are configured but no registry was provided.
	if (b.httpToolRepo != nil || b.riteRepo != nil) && b.actionRegistry == nil {
		b.actionRegistry = registries.NewActionRegistry()
	}
```

- [ ] **Step 3.4 — Register request_rite action in `Build()`**

Find this block (around line 648):
```go
	if b.vigilRepo != nil {
		engine.vigilRepo = b.vigilRepo
	}
```

Add immediately after:
```go
	if b.riteRepo != nil {
		if err := engine.RegisterAction(actions.NewRequestRiteAction(b.riteRepo)); err != nil {
			b.logger.Warnw("failed to register request_rite action", "error", err)
		}
	}
```

- [ ] **Step 3.5 — Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3.6 — Run full internal tests**

```bash
go test ./internal/...
```

Expected: all pass.

- [ ] **Step 3.7 — Commit**

```bash
git add internal/implementation/orchestrator/builder.go
git commit -m "feat(rite): WithRiteRepository — auto-registers request_rite action in Build()"
```

---

## Task 4: Re-exports

**Files:**
- Modify: `entities.go`
- Modify: `ports.go`
- Modify: `builtin.go`

- [ ] **Step 4.1 — Add Rite types to `entities.go`**

Read `entities.go`. Find the `type (...)` block. Add inside it (alongside `Vigil`, etc.):

```go
	Rite       = entities.Rite
	RiteStatus = entities.RiteStatus
```

Also add a `var` block after the existing `var` blocks (or add to existing var block if present):

```go
var (
	RitePending  = entities.RitePending
	RiteApproved = entities.RiteApproved
	RiteRejected = entities.RiteRejected
	RiteExpired  = entities.RiteExpired
)
```

- [ ] **Step 4.2 — Add RiteRepository to `ports.go`**

Read `ports.go`. Find the `type (...)` block. Add inside it (alongside `VigilRepository`, etc.):

```go
	RiteRepository = ports.RiteRepository
	RiteListOptions = ports.RiteListOptions
```

- [ ] **Step 4.3 — Add NewRequestRiteAction to `builtin.go`**

Read `builtin.go`. Find the actions `var (...)` block. Add:

```go
	NewRequestRiteAction = actions.NewRequestRiteAction
```

- [ ] **Step 4.4 — Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4.5 — Commit**

```bash
git add entities.go ports.go builtin.go
git commit -m "feat(rite): re-export Rite, RiteStatus, RiteRepository, NewRequestRiteAction from root package"
```

---

## Task 5: Mongo RiteRepository

**Files:**
- Create: `mongo/rite_repository.go`

- [ ] **Step 5.1 — Create `mongo/rite_repository.go`**

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

var _ eywa.RiteRepository = (*RiteRepository)(nil)

type RiteRepository struct {
	collection *mongodriver.Collection
	logger     *zap.SugaredLogger
}

func NewRiteRepository(database *mongodriver.Database) *RiteRepository {
	repo := &RiteRepository{
		collection: database.Collection("rites"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *RiteRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongodriver.IndexModel{
		{
			Keys: bson.D{
				{Key: "memory_key", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("idx_memory_key_status"),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "requested_at", Value: -1},
			},
			Options: options.Index().SetName("idx_status_requested_at"),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetName("idx_expires_at_ttl").SetExpireAfterSeconds(0),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create rites indexes", "error", err)
	}
}

func (r *RiteRepository) Create(ctx context.Context, rite *eywa.Rite) error {
	if rite.ID == "" {
		rite.ID = primitive.NewObjectID().Hex()
	}
	_, err := r.collection.InsertOne(ctx, rite)
	return err
}

func (r *RiteRepository) FindByID(ctx context.Context, id string) (*eywa.Rite, error) {
	var rite eywa.Rite
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&rite)
	if err == mongodriver.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "rite", ID: id}
	}
	return &rite, err
}

func (r *RiteRepository) List(ctx context.Context, opts eywa.RiteListOptions) ([]*eywa.Rite, int64, error) {
	filter := bson.M{}
	if opts.MemoryKey != "" {
		filter["memory_key"] = opts.MemoryKey
	}
	if opts.Status != "" {
		filter["status"] = opts.Status
	}

	page := opts.Page
	if page < 1 {
		page = 1
	}
	limit := opts.Limit
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	cursor, err := r.collection.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "requested_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var rites []*eywa.Rite
	if err := cursor.All(ctx, &rites); err != nil {
		return nil, 0, err
	}
	return rites, total, nil
}

func (r *RiteRepository) Decide(ctx context.Context, id, operatorID string, status eywa.RiteStatus) error {
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"operator_id": operatorID,
			"decided_at":  now,
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return &eywa.NotFoundError{Entity: "rite", ID: id}
	}
	return nil
}
```

- [ ] **Step 5.2 — Build mongo module**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/mongo && go build ./...
```

Expected: no errors.

- [ ] **Step 5.3 — Commit**

```bash
cd /Users/willianmoraes/PROJETOS/eywa
git add mongo/rite_repository.go
git commit -m "feat(rite): MongoDB RiteRepository — Create/FindByID/List/Decide with TTL index"
```

---

## Task 6: Fiber rite handlers (TDD)

**Files:**
- Create: `fiber/rite_handler_test.go`
- Create: `fiber/rite_handler.go`
- Modify: `fiber/management.go`

### 6a — Tests first

- [ ] **Step 6.1 — Create `fiber/rite_handler_test.go`**

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

type stubRiteRepoForFiber struct {
	rites     map[string]*eywa.Rite
	decideErr error
	err       error
	lastDecide struct {
		id         string
		operatorID string
		status     eywa.RiteStatus
	}
}

func newStubRiteRepo() *stubRiteRepoForFiber {
	return &stubRiteRepoForFiber{rites: make(map[string]*eywa.Rite)}
}

func (s *stubRiteRepoForFiber) Create(_ context.Context, rite *eywa.Rite) error {
	s.rites[rite.ID] = rite
	return s.err
}
func (s *stubRiteRepoForFiber) FindByID(_ context.Context, id string) (*eywa.Rite, error) {
	r, ok := s.rites[id]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return r, s.err
}
func (s *stubRiteRepoForFiber) List(_ context.Context, _ eywa.RiteListOptions) ([]*eywa.Rite, int64, error) {
	rites := make([]*eywa.Rite, 0, len(s.rites))
	for _, r := range s.rites {
		rites = append(rites, r)
	}
	return rites, int64(len(rites)), s.err
}
func (s *stubRiteRepoForFiber) Decide(_ context.Context, id, operatorID string, status eywa.RiteStatus) error {
	if s.decideErr != nil {
		return s.decideErr
	}
	s.lastDecide.id = id
	s.lastDecide.operatorID = operatorID
	s.lastDecide.status = status
	if r, ok := s.rites[id]; ok {
		r.Status = status
		r.OperatorID = operatorID
	}
	return nil
}

func riteDeps(rr *stubRiteRepoForFiber) ManagementDeps {
	return ManagementDeps{
		APIKeys:  map[string]string{"test-key": "admin"},
		RiteRepo: rr,
	}
}

func seedRite(repo *stubRiteRepoForFiber, id, memoryKey, eventKey string, status eywa.RiteStatus) *eywa.Rite {
	r := &eywa.Rite{
		ID:          id,
		MemoryKey:   memoryKey,
		EventKey:    eventKey,
		Reason:      "needs approval",
		Status:      status,
		RequestedAt: time.Now().UTC(),
	}
	repo.rites[id] = r
	return r
}

func TestRiteHandler_List_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	items, _ := result["items"].([]interface{})
	if len(items) != 1 {
		t.Errorf("want 1 item, got %d", len(items))
	}
}

func TestRiteHandler_GetByID_Returns200(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites/rite-1", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] != "rite-1" {
		t.Errorf("want id=rite-1, got %v", result["id"])
	}
}

func TestRiteHandler_GetByID_NotFound_Returns404(t *testing.T) {
	repo := newStubRiteRepo()
	app := buildMgmtTestApp(riteDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/rites/unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Approve_Returns200_CallsDecide(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]interface{}{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.lastDecide.id != "rite-1" {
		t.Errorf("want Decide called with rite-1, got %s", repo.lastDecide.id)
	}
	if repo.lastDecide.status != eywa.RiteApproved {
		t.Errorf("want status=approved, got %s", repo.lastDecide.status)
	}
}

func TestRiteHandler_Approve_MissingOperatorID_Returns400(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]interface{}{}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestRiteHandler_Reject_Returns200_CallsDecide(t *testing.T) {
	repo := newStubRiteRepo()
	seedRite(repo, "rite-1", "whatsapp:+55199", "resume_order", eywa.RitePending)
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]interface{}{"operator_id": "op-1", "note": "not authorized"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/rite-1/reject", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.lastDecide.status != eywa.RiteRejected {
		t.Errorf("want status=rejected, got %s", repo.lastDecide.status)
	}
}

func TestRiteHandler_Approve_NotFound_Returns404(t *testing.T) {
	repo := newStubRiteRepo()
	app := buildMgmtTestApp(riteDeps(repo))

	body := map[string]interface{}{"operator_id": "op-1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/rites/unknown/approve", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 6.2 — Run tests to verify they fail**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/fiber && go test ./... -run TestRiteHandler -v 2>&1 | head -20
```

Expected: compile errors — `ManagementDeps.RiteRepo`, `riteHandler` undefined.

### 6b — Implementation

- [ ] **Step 6.3 — Add `RiteRepo` to `ManagementDeps` in `fiber/management.go`**

Read `fiber/management.go`. After `VigilConfig eywa.VigilConfig`, add:

```go
	// Phase 8 — Approval Flows / Rite
	RiteRepo eywa.RiteRepository
```

Change the function signature from:
```go
func RegisterManagementRoutes(app *fiberlib.App, _ *eywa.Weave, deps ManagementDeps) {
```
to:
```go
func RegisterManagementRoutes(app *fiberlib.App, weave *eywa.Weave, deps ManagementDeps) {
```

After the vigil block, add:

```go
	if deps.RiteRepo != nil {
		rh := newRiteHandler(deps.RiteRepo, weave)
		rites := api.Group("/rites")
		rites.Get("", rh.list)
		rites.Get("/:id", rh.getByID)
		rites.Post("/:id/approve", rh.approve)
		rites.Post("/:id/reject", rh.reject)
	}
```

- [ ] **Step 6.4 — Create `fiber/rite_handler.go`**

```go
package fiber

import (
	"errors"
	"strings"

	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
)

type riteHandler struct {
	riteRepo eywa.RiteRepository
	weave    *eywa.Weave // nil in tests — skip Pulse firing
}

func newRiteHandler(riteRepo eywa.RiteRepository, weave *eywa.Weave) *riteHandler {
	return &riteHandler{riteRepo: riteRepo, weave: weave}
}

func (h *riteHandler) list(c *fiberlib.Ctx) error {
	opts := eywa.RiteListOptions{
		MemoryKey: c.Query("memory_key"),
		Status:    eywa.RiteStatus(c.Query("status")),
		Page:      c.QueryInt("page", 1),
		Limit:     c.QueryInt("limit", 20),
	}
	rites, total, err := h.riteRepo.List(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(fiberlib.Map{"items": rites, "total": total})
}

func (h *riteHandler) getByID(c *fiberlib.Ctx) error {
	id := c.Params("id")
	rite, err := h.riteRepo.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "rite not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(rite)
}

func (h *riteHandler) approve(c *fiberlib.Ctx) error {
	return h.decide(c, eywa.RiteApproved)
}

func (h *riteHandler) reject(c *fiberlib.Ctx) error {
	return h.decide(c, eywa.RiteRejected)
}

func (h *riteHandler) decide(c *fiberlib.Ctx, newStatus eywa.RiteStatus) error {
	id := c.Params("id")
	var body struct {
		OperatorID string `json:"operator_id"`
		Note       string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if body.OperatorID == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "operator_id is required"})
	}

	rite, err := h.riteRepo.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, eywa.ErrNotFound) {
			return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "rite not found"})
		}
		return resthttp.ErrorResponse(c, err)
	}

	if err := h.riteRepo.Decide(c.Context(), id, body.OperatorID, newStatus); err != nil {
		return resthttp.ErrorResponse(c, err)
	}

	// Fire resume Pulse — skipped when weave is nil (tests, or ops without engine wired).
	if h.weave != nil {
		parts := strings.SplitN(rite.MemoryKey, ":", 2)
		if len(parts) == 2 {
			memKey := eywa.MemoryKey{Channel: parts[0], User: parts[1]}
			pb := eywa.NewPulse(memKey).
				WithEventType(rite.EventKey).
				AddKnowledge("rite_id", rite.ID).
				AddKnowledge("rite_status", string(newStatus)).
				AddKnowledge("rite_context", rite.Context).
				AddMetadata("triggered_by", "rite_decision")
			if body.Note != "" {
				pb = pb.AddKnowledge("rite_note", body.Note)
			}
			_, _ = h.weave.ProcessEventByKey(c.Context(), rite.EventKey, pb.Build())
		}
	}

	return c.JSON(fiberlib.Map{"id": id, "status": string(newStatus)})
}
```

- [ ] **Step 6.5 — Run target tests**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/fiber && go test ./... -run TestRiteHandler -v
```

Expected: all 7 tests pass.

- [ ] **Step 6.6 — Run full fiber suite**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/fiber && go test -count=1 ./...
```

Expected: all pass.

- [ ] **Step 6.7 — Commit**

```bash
cd /Users/willianmoraes/PROJETOS/eywa
git add fiber/rite_handler.go fiber/rite_handler_test.go fiber/management.go
git commit -m "feat(rite): list/get/approve/reject handlers with Pulse trigger on decision"
```

---

## Task 7: Full verification

- [ ] **Step 7.1 — Run all root module tests**

```bash
cd /Users/willianmoraes/PROJETOS/eywa && go test ./...
```

Expected: all pass.

- [ ] **Step 7.2 — Run all fiber module tests**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/fiber && go test -count=1 ./...
```

Expected: all pass.

- [ ] **Step 7.3 — Build redis module**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/redis && go build ./...
```

Expected: no errors.

- [ ] **Step 7.4 — Build mongo module**

```bash
cd /Users/willianmoraes/PROJETOS/eywa/mongo && go build ./...
```

Expected: no errors.
