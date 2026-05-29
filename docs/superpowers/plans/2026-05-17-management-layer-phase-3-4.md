# Management Layer — Phase 3 + Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 3 (Observability API — ChronicleQueryRepository + analytics routes) and Phase 4 (Conversations API — EchoQueryRepository + session/echo routes), introducing `ManagementDeps` and `RegisterManagementRoutes` as the management fiber integration point.

**Architecture:** New query interfaces (`ChronicleQueryRepository`, `EchoQueryRepository`) are separate from the existing write-side interfaces — no breaking changes. Existing mongo structs gain the new methods and satisfy both old and new interfaces. `RegisterManagementRoutes` in the fiber sub-module conditionally registers route groups based on which deps are non-nil. Handler tests use stub repo implementations in fiber's test files — no MongoDB process required.

**Tech Stack:** Go 1.25.5, `go.mongodb.org/mongo-driver`, `github.com/gofiber/fiber/v2`, `github.com/wmulabs/eywa` (root package re-exports).

---

## File Map

### Phase 3 — Observability API

| Action | Path |
|--------|------|
| **Create** | `internal/domain/ports/chronicle_query.go` |
| **Create** | `mongo/chronicle_query.go` |
| **Modify** | `mongo/chronicle_repository.go` — change constructor return type |
| **Modify** | `ports.go` — re-export new types |
| **Create** | `fiber/management.go` |
| **Create** | `fiber/chronicle_handler.go` |
| **Create** | `fiber/chronicle_handler_test.go` |

### Phase 4 — Conversations API

| Action | Path |
|--------|------|
| **Create** | `internal/domain/ports/echo_query.go` |
| **Create** | `mongo/echo_query.go` |
| **Modify** | `mongo/echo_repository.go` — change constructor return type |
| **Modify** | `ports.go` — re-export new types |
| **Modify** | `fiber/management.go` — add echo fields + routes |
| **Create** | `fiber/echo_mgmt_handler.go` |
| **Create** | `fiber/echo_mgmt_handler_test.go` |

---

## Task 1: ChronicleQueryRepository port

**Files:**
- Create: `internal/domain/ports/chronicle_query.go`

- [ ] **Step 1.1 — Create the port**

```go
// internal/domain/ports/chronicle_query.go
package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type ChronicleListOptions struct {
	SpiritName    string
	MemoryKey     string
	HasError      bool       // true = filter to non-success statuses only
	MinIterations int        // 0 = no filter
	DateFrom      *time.Time
	DateTo        *time.Time
	Page          int
	Limit         int
}

type TokenSeries struct {
	Date             time.Time
	SpiritName       string
	PromptTokens     int
	CompletionTokens int
}

type ActionStats struct {
	ActionName   string
	CallCount    int
	ErrorCount   int
	AvgLatencyMs float64
	P95LatencyMs float64
}

type SpiritStats struct {
	SpiritName    string
	AvgIterations float64
	ErrorRate     float64
	AvgDurationMs float64
}

type ChronicleQueryRepository interface {
	List(ctx context.Context, opts ChronicleListOptions) ([]*entities.Chronicle, int64, error)
	FindByID(ctx context.Context, id string) (*entities.Chronicle, error)
	AggregateTokens(ctx context.Context, spiritName string, from, to time.Time, granularity string) ([]TokenSeries, error)
	AggregateActions(ctx context.Context, spiritName string, from, to time.Time) ([]ActionStats, error)
	AggregateSpirits(ctx context.Context, from, to time.Time) ([]SpiritStats, error)
}
```

- [ ] **Step 1.2 — Build root module to confirm no errors**

```bash
cd /path/to/eywa && go build ./...
```

Expected: no errors.

- [ ] **Step 1.3 — Commit**

```bash
git add internal/domain/ports/chronicle_query.go
git commit -m "feat(observability): ChronicleQueryRepository port + read-model types"
```

---

## Task 2: Mongo Chronicle — List + FindByID

**Files:**
- Modify: `mongo/chronicle_repository.go` (change return type only)
- Create: `mongo/chronicle_query.go`

- [ ] **Step 2.1 — Change `NewChronicleRepository` return type to `*ChronicleRepository`**

In `mongo/chronicle_repository.go`, change line 22:

```go
// Before:
func NewChronicleRepository(database *mongo.Database) eywa.ChronicleRepository {

// After:
func NewChronicleRepository(database *mongo.Database) *ChronicleRepository {
```

Callers assigning the result as `eywa.ChronicleRepository` continue to work because `*ChronicleRepository` implements the interface. Callers can now also pass the same instance as `eywa.ChronicleQueryRepository`.

- [ ] **Step 2.2 — Create `mongo/chronicle_query.go` with List + FindByID**

```go
// mongo/chronicle_query.go
package mongo

import (
	"context"
	"fmt"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Compile-time interface satisfaction check.
var _ eywa.ChronicleQueryRepository = (*ChronicleRepository)(nil)

func (r *ChronicleRepository) FindByID(ctx context.Context, id string) (*eywa.Chronicle, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid chronicle id: %w", err)
	}
	var ch eywa.Chronicle
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&ch); err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, eywa.ErrNotFound
		}
		return nil, err
	}
	return &ch, nil
}

func (r *ChronicleRepository) List(ctx context.Context, opts eywa.ChronicleListOptions) ([]*eywa.Chronicle, int64, error) {
	filter := buildChronicleListFilter(opts)

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
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

	findOpts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []*eywa.Chronicle
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func buildChronicleListFilter(opts eywa.ChronicleListOptions) bson.M {
	filter := bson.M{}
	if opts.SpiritName != "" {
		filter["agent.name"] = opts.SpiritName
	}
	if opts.MemoryKey != "" {
		filter["memory_key"] = opts.MemoryKey
	}
	if opts.HasError {
		filter["processing.status"] = bson.M{"$ne": "success"}
	}
	if opts.MinIterations > 0 {
		filter["processing.iterations_used"] = bson.M{"$gte": opts.MinIterations}
	}
	timeFilter := bson.M{}
	if opts.DateFrom != nil {
		timeFilter["$gte"] = *opts.DateFrom
	}
	if opts.DateTo != nil {
		timeFilter["$lte"] = *opts.DateTo
	}
	if len(timeFilter) > 0 {
		filter["timestamp"] = timeFilter
	}
	return filter
}
```

- [ ] **Step 2.3 — Build mongo module**

```bash
cd /path/to/eywa/mongo && go build ./...
```

Expected: no errors. If the compile-time check fails, the error will be: `cannot use (*ChronicleRepository)(nil) as type eywa.ChronicleQueryRepository`.

- [ ] **Step 2.4 — Commit**

```bash
cd /path/to/eywa
git add mongo/chronicle_repository.go mongo/chronicle_query.go
git commit -m "feat(observability): mongo ChronicleRepository — List + FindByID"
```

---

## Task 3: Mongo Chronicle — analytics methods

**Files:**
- Modify: `mongo/chronicle_query.go` (add AggregateTokens, AggregateActions, AggregateSpirits)

- [ ] **Step 3.1 — Add analytics methods to `mongo/chronicle_query.go`**

Append the following to `mongo/chronicle_query.go` (after the existing functions):

```go
func (r *ChronicleRepository) AggregateTokens(ctx context.Context, spiritName string, from, to time.Time, granularity string) ([]eywa.TokenSeries, error) {
	unit := "day"
	if granularity == "week" || granularity == "month" {
		unit = granularity
	}

	matchFilter := bson.M{"timestamp": bson.M{"$gte": from, "$lte": to}}
	if spiritName != "" {
		matchFilter["agent.name"] = spiritName
	}

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "date", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$timestamp"},
					{Key: "unit", Value: unit},
				}}}},
				{Key: "spirit", Value: "$agent.name"},
			}},
			{Key: "prompt_tokens", Value: bson.D{{Key: "$sum", Value: "$token_usage.total.prompt_tokens"}}},
			{Key: "completion_tokens", Value: bson.D{{Key: "$sum", Value: "$token_usage.total.completion_tokens"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id.date", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type tokenResult struct {
		ID struct {
			Date   time.Time `bson:"date"`
			Spirit string    `bson:"spirit"`
		} `bson:"_id"`
		PromptTokens     int `bson:"prompt_tokens"`
		CompletionTokens int `bson:"completion_tokens"`
	}

	var raw []tokenResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]eywa.TokenSeries, len(raw))
	for i, r := range raw {
		result[i] = eywa.TokenSeries{
			Date:             r.ID.Date,
			SpiritName:       r.ID.Spirit,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
		}
	}
	return result, nil
}

func (r *ChronicleRepository) AggregateActions(ctx context.Context, spiritName string, from, to time.Time) ([]eywa.ActionStats, error) {
	matchFilter := bson.M{"timestamp": bson.M{"$gte": from, "$lte": to}}
	if spiritName != "" {
		matchFilter["agent.name"] = spiritName
	}

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$unwind", Value: "$processing.iterations"}},
		{{Key: "$unwind", Value: "$processing.iterations.action_calls"}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$processing.iterations.action_calls.action_name"},
			{Key: "call_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "error_count", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				"$processing.iterations.action_calls.is_error", 1, 0,
			}}}}}},
			{Key: "avg_latency", Value: bson.D{{Key: "$avg", Value: "$processing.iterations.action_calls.duration_ms"}}},
			{Key: "durations", Value: bson.D{{Key: "$push", Value: "$processing.iterations.action_calls.duration_ms"}}},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "sorted_durations", Value: bson.D{{Key: "$sortArray", Value: bson.D{
				{Key: "input", Value: "$durations"},
				{Key: "sortBy", Value: 1},
			}}}},
			{Key: "dur_count", Value: bson.D{{Key: "$size", Value: "$durations"}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "action_name", Value: "$_id"},
			{Key: "call_count", Value: 1},
			{Key: "error_count", Value: 1},
			{Key: "avg_latency_ms", Value: "$avg_latency"},
			{Key: "p95_latency_ms", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{
				"$sorted_durations",
				bson.D{{Key: "$floor", Value: bson.D{{Key: "$multiply", Value: bson.A{0.95, "$dur_count"}}}}},
			}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "call_count", Value: -1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type actionResult struct {
		ActionName   string  `bson:"action_name"`
		CallCount    int     `bson:"call_count"`
		ErrorCount   int     `bson:"error_count"`
		AvgLatencyMs float64 `bson:"avg_latency_ms"`
		P95LatencyMs float64 `bson:"p95_latency_ms"`
	}

	var raw []actionResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]eywa.ActionStats, len(raw))
	for i, r := range raw {
		result[i] = eywa.ActionStats{
			ActionName:   r.ActionName,
			CallCount:    r.CallCount,
			ErrorCount:   r.ErrorCount,
			AvgLatencyMs: r.AvgLatencyMs,
			P95LatencyMs: r.P95LatencyMs,
		}
	}
	return result, nil
}

func (r *ChronicleRepository) AggregateSpirits(ctx context.Context, from, to time.Time) ([]eywa.SpiritStats, error) {
	matchFilter := bson.M{"timestamp": bson.M{"$gte": from, "$lte": to}}

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$agent.name"},
			{Key: "total_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "error_count", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$ne", Value: bson.A{"$processing.status", "success"}}}, 1, 0,
			}}}}}},
			{Key: "avg_iterations", Value: bson.D{{Key: "$avg", Value: "$processing.iterations_used"}}},
			{Key: "avg_duration", Value: bson.D{{Key: "$avg", Value: "$processing.processing_time_ms"}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "spirit_name", Value: "$_id"},
			{Key: "avg_iterations", Value: 1},
			{Key: "error_rate", Value: bson.D{{Key: "$divide", Value: bson.A{"$error_count", "$total_count"}}}},
			{Key: "avg_duration_ms", Value: "$avg_duration"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "spirit_name", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type spiritResult struct {
		SpiritName    string  `bson:"spirit_name"`
		AvgIterations float64 `bson:"avg_iterations"`
		ErrorRate     float64 `bson:"error_rate"`
		AvgDurationMs float64 `bson:"avg_duration_ms"`
	}

	var raw []spiritResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]eywa.SpiritStats, len(raw))
	for i, r := range raw {
		result[i] = eywa.SpiritStats{
			SpiritName:    r.SpiritName,
			AvgIterations: r.AvgIterations,
			ErrorRate:     r.ErrorRate,
			AvgDurationMs: r.AvgDurationMs,
		}
	}
	return result, nil
}
```

> **Note:** `$sortArray` requires MongoDB 5.2+. `$dateTrunc` requires MongoDB 5.0+. Both are standard in any recent deployment.

You also need to add the `"time"` import to `mongo/chronicle_query.go`. The full import block at the top of the file becomes:

```go
import (
	"context"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)
```

- [ ] **Step 3.2 — Build mongo module**

```bash
cd /path/to/eywa/mongo && go build ./...
```

Expected: no errors.

- [ ] **Step 3.3 — Commit**

```bash
cd /path/to/eywa
git add mongo/chronicle_query.go
git commit -m "feat(observability): mongo ChronicleRepository analytics — AggregateTokens/Actions/Spirits"
```

---

## Task 4: Root re-exports for chronicle query types

**Files:**
- Modify: `ports.go`

- [ ] **Step 4.1 — Add chronicle query types to `ports.go`**

In `ports.go`, in the first `type (...)` block (the one with `Oracle`, `Action`, etc.), add:

```go
ChronicleQueryRepository = ports.ChronicleQueryRepository
ChronicleListOptions      = ports.ChronicleListOptions
TokenSeries               = ports.TokenSeries
ActionStats               = ports.ActionStats
SpiritStats               = ports.SpiritStats
```

- [ ] **Step 4.2 — Build root module**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4.3 — Commit**

```bash
git add ports.go
git commit -m "feat(observability): re-export ChronicleQueryRepository and analytics types from root"
```

---

## Task 5: ManagementDeps + RegisterManagementRoutes skeleton

**Files:**
- Create: `fiber/management.go`

- [ ] **Step 5.1 — Create `fiber/management.go`**

```go
// fiber/management.go
package fiber

import (
	eywa "github.com/wmulabs/eywa"
	fiberlib "github.com/gofiber/fiber/v2"
	"github.com/wmulabs/eywa/fiber/middleware"
)

// ManagementDeps wires optional repositories and auth config into RegisterManagementRoutes.
// Each route group is only registered when its required dependency is non-nil.
type ManagementDeps struct {
	// Auth — at least one required for management routes to be secured.
	APIKeys        map[string]string // Mode 1: static key→role map
	OperatorAuth   *eywa.OperatorAuth // Mode 2: built-in operator JWT
	TokenValidator eywa.TokenValidator // Mode 3: external JWT / JWKS

	// Phase 3 — Observability (SPEC_07)
	ChronicleQueryRepo eywa.ChronicleQueryRepository

	// Phase 4 — Conversations (SPEC_06)
	EchoRepo      eywa.EchoRepository
	EchoQueryRepo eywa.EchoQueryRepository
}

// RegisterManagementRoutes mounts the management API onto app under /api/v1.
// All management routes require authentication. Route groups are registered
// conditionally based on which ManagementDeps fields are non-nil.
func RegisterManagementRoutes(app *fiberlib.App, _ *eywa.Weave, deps ManagementDeps) {
	validators := buildValidatorChain(deps)
	authMW := middleware.AuthMiddleware(validators...)

	api := app.Group("/api/v1", authMW)

	if deps.ChronicleQueryRepo != nil {
		ch := newChronicleHandler(deps.ChronicleQueryRepo)
		api.Get("/chronicle", ch.list)
		api.Get("/chronicle/:id", ch.findByID)

		analytics := api.Group("/analytics")
		analytics.Get("/tokens", ch.aggregateTokens)
		analytics.Get("/actions", ch.aggregateActions)
		analytics.Get("/spirits", ch.aggregateSpirits)
	}
}

func buildValidatorChain(deps ManagementDeps) []eywa.TokenValidator {
	var validators []eywa.TokenValidator
	if len(deps.APIKeys) > 0 {
		validators = append(validators, eywa.NewAPIKeyValidator(deps.APIKeys))
	}
	if deps.OperatorAuth != nil {
		validators = append(validators, deps.OperatorAuth)
	}
	if deps.TokenValidator != nil {
		validators = append(validators, deps.TokenValidator)
	}
	return validators
}
```

- [ ] **Step 5.2 — Build fiber module**

```bash
cd /path/to/eywa/fiber && go build ./...
```

Expected: compile error `newChronicleHandler undefined` — correct, the handler doesn't exist yet.

- [ ] **Step 5.3 — Create stub `fiber/chronicle_handler.go` to unblock build**

```go
// fiber/chronicle_handler.go
package fiber

import (
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
	"fmt"
	"time"
)

type chronicleHandler struct {
	repo eywa.ChronicleQueryRepository
}

func newChronicleHandler(repo eywa.ChronicleQueryRepository) *chronicleHandler {
	return &chronicleHandler{repo: repo}
}

func (h *chronicleHandler) list(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *chronicleHandler) findByID(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *chronicleHandler) aggregateTokens(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *chronicleHandler) aggregateActions(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *chronicleHandler) aggregateSpirits(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

// parseDateRange extracts date_from and date_to from query params (RFC3339).
func parseDateRange(c *fiberlib.Ctx) (from, to time.Time, err error) {
	fromStr := c.Query("date_from")
	toStr := c.Query("date_to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("date_from and date_to are required")
	}
	from, err = time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_from: %w", err)
	}
	to, err = time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_to: %w", err)
	}
	return from, to, nil
}

// imported to satisfy resthttp usage in real handlers
var _ = resthttp.ErrorResponse
```

- [ ] **Step 5.4 — Build fiber module**

```bash
cd /path/to/eywa/fiber && go build ./...
```

Expected: no errors.

- [ ] **Step 5.5 — Commit**

```bash
cd /path/to/eywa
git add fiber/management.go fiber/chronicle_handler.go
git commit -m "feat(observability): ManagementDeps + RegisterManagementRoutes + chronicle handler stub"
```

---

## Task 6: ChronicleHandler — list + detail (TDD)

**Files:**
- Create: `fiber/chronicle_handler_test.go`
- Modify: `fiber/chronicle_handler.go` (replace stubs with real implementations)

- [ ] **Step 6.1 — Write failing tests**

```go
// fiber/chronicle_handler_test.go
package fiber

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
	fiberlib "github.com/gofiber/fiber/v2"
)

// stubChronicleQueryRepo is a test double for ChronicleQueryRepository.
type stubChronicleQueryRepo struct {
	chronicles []*eywa.Chronicle
	total      int64
	tokens     []eywa.TokenSeries
	actions    []eywa.ActionStats
	spirits    []eywa.SpiritStats
	err        error
}

func (s *stubChronicleQueryRepo) List(_ context.Context, _ eywa.ChronicleListOptions) ([]*eywa.Chronicle, int64, error) {
	return s.chronicles, s.total, s.err
}
func (s *stubChronicleQueryRepo) FindByID(_ context.Context, id string) (*eywa.Chronicle, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, c := range s.chronicles {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, eywa.ErrNotFound
}
func (s *stubChronicleQueryRepo) AggregateTokens(_ context.Context, _ string, _, _ time.Time, _ string) ([]eywa.TokenSeries, error) {
	return s.tokens, s.err
}
func (s *stubChronicleQueryRepo) AggregateActions(_ context.Context, _ string, _, _ time.Time) ([]eywa.ActionStats, error) {
	return s.actions, s.err
}
func (s *stubChronicleQueryRepo) AggregateSpirits(_ context.Context, _, _ time.Time) ([]eywa.SpiritStats, error) {
	return s.spirits, s.err
}

func buildMgmtTestApp(deps ManagementDeps) *fiberlib.App {
	app := fiberlib.New(fiberlib.Config{DisableStartupMessage: true})
	RegisterManagementRoutes(app, nil, deps)
	return app
}

func chronicleDeps(repo *stubChronicleQueryRepo) ManagementDeps {
	return ManagementDeps{
		APIKeys:            map[string]string{"test-key": "admin"},
		ChronicleQueryRepo: repo,
	}
}

func TestChronicleHandler_List_Returns200WithItems(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		chronicles: []*eywa.Chronicle{{ID: "abc", MemoryKey: "k1"}},
		total:      1,
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []*eywa.Chronicle `json:"items"`
		Total int64             `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 item, got %d", len(body.Items))
	}
	if body.Total != 1 {
		t.Errorf("want total 1, got %d", body.Total)
	}
}

func TestChronicleHandler_List_ReturnsEmptySlice(t *testing.T) {
	stub := &stubChronicleQueryRepo{chronicles: nil, total: 0}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []interface{} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items must be empty slice, not null")
	}
}

func TestChronicleHandler_List_NoAuth_Returns401(t *testing.T) {
	stub := &stubChronicleQueryRepo{}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_FindByID_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		chronicles: []*eywa.Chronicle{{ID: "abc123", MemoryKey: "k1"}},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle/abc123", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) == "" {
		t.Error("expected non-empty body")
	}
}

func TestChronicleHandler_FindByID_NotFound_Returns404(t *testing.T) {
	stub := &stubChronicleQueryRepo{chronicles: nil}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/chronicle/notexist", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 6.2 — Run tests to confirm failure**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestChronicleHandler_List|TestChronicleHandler_FindByID" -v
```

Expected: tests FAIL with 501 status (stub returns NotImplemented).

- [ ] **Step 6.3 — Implement `list` and `findByID` in `fiber/chronicle_handler.go`**

Replace the stub `list` and `findByID` methods:

```go
func (h *chronicleHandler) list(c *fiberlib.Ctx) error {
	opts := eywa.ChronicleListOptions{
		SpiritName:    c.Query("spirit_name"),
		MemoryKey:     c.Query("memory_key"),
		HasError:      c.QueryBool("has_error"),
		MinIterations: c.QueryInt("min_iterations"),
		Page:          c.QueryInt("page", 1),
		Limit:         c.QueryInt("limit", resthttp.DefaultPageLimit),
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_from: use RFC3339"})
		}
		opts.DateFrom = &t
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_to: use RFC3339"})
		}
		opts.DateTo = &t
	}

	items, total, err := h.repo.List(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if items == nil {
		items = []*eywa.Chronicle{}
	}
	return c.JSON(fiberlib.Map{"items": items, "total": total, "page": opts.Page, "limit": opts.Limit})
}

func (h *chronicleHandler) findByID(c *fiberlib.Ctx) error {
	item, err := h.repo.FindByID(c.Context(), c.Params("id"))
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(item)
}
```

Also remove the `var _ = resthttp.ErrorResponse` line from the stub — `resthttp` is now used directly.

The full `fiber/chronicle_handler.go` after this step:

```go
// fiber/chronicle_handler.go
package fiber

import (
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
)

type chronicleHandler struct {
	repo eywa.ChronicleQueryRepository
}

func newChronicleHandler(repo eywa.ChronicleQueryRepository) *chronicleHandler {
	return &chronicleHandler{repo: repo}
}

func (h *chronicleHandler) list(c *fiberlib.Ctx) error {
	opts := eywa.ChronicleListOptions{
		SpiritName:    c.Query("spirit_name"),
		MemoryKey:     c.Query("memory_key"),
		HasError:      c.QueryBool("has_error"),
		MinIterations: c.QueryInt("min_iterations"),
		Page:          c.QueryInt("page", 1),
		Limit:         c.QueryInt("limit", resthttp.DefaultPageLimit),
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_from: use RFC3339"})
		}
		opts.DateFrom = &t
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_to: use RFC3339"})
		}
		opts.DateTo = &t
	}

	items, total, err := h.repo.List(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if items == nil {
		items = []*eywa.Chronicle{}
	}
	return c.JSON(fiberlib.Map{"items": items, "total": total, "page": opts.Page, "limit": opts.Limit})
}

func (h *chronicleHandler) findByID(c *fiberlib.Ctx) error {
	item, err := h.repo.FindByID(c.Context(), c.Params("id"))
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(item)
}

func (h *chronicleHandler) aggregateTokens(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *chronicleHandler) aggregateActions(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *chronicleHandler) aggregateSpirits(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

// parseDateRange extracts date_from and date_to from query params (RFC3339).
func parseDateRange(c *fiberlib.Ctx) (from, to time.Time, err error) {
	fromStr := c.Query("date_from")
	toStr := c.Query("date_to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("date_from and date_to are required")
	}
	from, err = time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_from: %w", err)
	}
	to, err = time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_to: %w", err)
	}
	return from, to, nil
}
```

- [ ] **Step 6.4 — Run tests to confirm pass**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestChronicleHandler_List|TestChronicleHandler_FindByID" -v
```

Expected: all 5 tests PASS.

- [ ] **Step 6.5 — Commit**

```bash
cd /path/to/eywa
git add fiber/chronicle_handler.go fiber/chronicle_handler_test.go
git commit -m "feat(observability): chronicle list + detail handlers with tests"
```

---

## Task 7: ChronicleHandler — analytics (TDD)

**Files:**
- Modify: `fiber/chronicle_handler_test.go` (add analytics tests)
- Modify: `fiber/chronicle_handler.go` (implement analytics methods)

- [ ] **Step 7.1 — Add analytics tests to `fiber/chronicle_handler_test.go`**

Append to `fiber/chronicle_handler_test.go`:

```go
func TestChronicleHandler_AggregateTokens_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		tokens: []eywa.TokenSeries{
			{SpiritName: "support", PromptTokens: 100, CompletionTokens: 50},
		},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []eywa.TokenSeries `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 {
		t.Errorf("want 1 series, got %d", len(body.Data))
	}
}

func TestChronicleHandler_AggregateTokens_MissingDates_Returns400(t *testing.T) {
	app := buildMgmtTestApp(chronicleDeps(&stubChronicleQueryRepo{}))

	req := httptest.NewRequest("GET", "/api/v1/analytics/tokens", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestChronicleHandler_AggregateActions_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		actions: []eywa.ActionStats{
			{ActionName: "search_lore", CallCount: 10, ErrorCount: 1, AvgLatencyMs: 120.5},
		},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/actions?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []eywa.ActionStats `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 {
		t.Errorf("want 1 action stat, got %d", len(body.Data))
	}
}

func TestChronicleHandler_AggregateSpirits_Returns200(t *testing.T) {
	stub := &stubChronicleQueryRepo{
		spirits: []eywa.SpiritStats{
			{SpiritName: "support", AvgIterations: 2.3, ErrorRate: 0.05, AvgDurationMs: 850},
		},
	}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []eywa.SpiritStats `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Data) != 1 {
		t.Errorf("want 1 spirit stat, got %d", len(body.Data))
	}
}

func TestChronicleHandler_Analytics_ReturnsEmptySlice(t *testing.T) {
	stub := &stubChronicleQueryRepo{}
	app := buildMgmtTestApp(chronicleDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/analytics/spirits?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []interface{} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Data == nil {
		t.Error("data must be empty slice, not null")
	}
}
```

- [ ] **Step 7.2 — Run tests to confirm failure**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestChronicleHandler_Aggregate" -v
```

Expected: all FAIL with 501 status.

- [ ] **Step 7.3 — Implement analytics methods in `fiber/chronicle_handler.go`**

Replace the three stub analytics methods with:

```go
func (h *chronicleHandler) aggregateTokens(c *fiberlib.Ctx) error {
	from, to, err := parseDateRange(c)
	if err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	granularity := c.Query("granularity", "day")
	result, err := h.repo.AggregateTokens(c.Context(), c.Query("spirit_name"), from, to, granularity)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if result == nil {
		result = []eywa.TokenSeries{}
	}
	return c.JSON(fiberlib.Map{"data": result})
}

func (h *chronicleHandler) aggregateActions(c *fiberlib.Ctx) error {
	from, to, err := parseDateRange(c)
	if err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	result, err := h.repo.AggregateActions(c.Context(), c.Query("spirit_name"), from, to)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if result == nil {
		result = []eywa.ActionStats{}
	}
	return c.JSON(fiberlib.Map{"data": result})
}

func (h *chronicleHandler) aggregateSpirits(c *fiberlib.Ctx) error {
	from, to, err := parseDateRange(c)
	if err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	result, err := h.repo.AggregateSpirits(c.Context(), from, to)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if result == nil {
		result = []eywa.SpiritStats{}
	}
	return c.JSON(fiberlib.Map{"data": result})
}
```

- [ ] **Step 7.4 — Run all chronicle tests**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestChronicleHandler" -v
```

Expected: all 10 tests PASS.

- [ ] **Step 7.5 — Commit**

```bash
cd /path/to/eywa
git add fiber/chronicle_handler.go fiber/chronicle_handler_test.go
git commit -m "feat(observability): chronicle analytics handlers with tests"
```

---

## Task 8: EchoQueryRepository port

**Files:**
- Create: `internal/domain/ports/echo_query.go`

- [ ] **Step 8.1 — Create the port**

```go
// internal/domain/ports/echo_query.go
package ports

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type SessionListOptions struct {
	SpiritName string     // ignored in current mongo impl (Echo lacks spirit_name field)
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	Limit      int
}

type SessionSummary struct {
	MemoryKey      string
	LastSpiritName string // empty when derived from Echo alone; enriched by Chronicle if available
	MessageCount   int64
	LastMessageAt  time.Time
}

type EchoQueryRepository interface {
	ListSessions(ctx context.Context, opts SessionListOptions) ([]*SessionSummary, int64, error)
	FindByMemoryKeyBefore(ctx context.Context, memoryKey, beforeID string, limit int) ([]*entities.Echo, error)
}
```

- [ ] **Step 8.2 — Build root module**

```bash
cd /path/to/eywa && go build ./...
```

Expected: no errors.

- [ ] **Step 8.3 — Commit**

```bash
git add internal/domain/ports/echo_query.go
git commit -m "feat(conversations): EchoQueryRepository port + SessionSummary type"
```

---

## Task 9: Mongo Echo query implementation

**Files:**
- Modify: `mongo/echo_repository.go` (change constructor return type)
- Create: `mongo/echo_query.go`

- [ ] **Step 9.1 — Change `NewEchoRepository` return type**

In `mongo/echo_repository.go`, change line 23:

```go
// Before:
func NewEchoRepository(database *mongo.Database) eywa.EchoRepository {

// After:
func NewEchoRepository(database *mongo.Database) *EchoRepository {
```

- [ ] **Step 9.2 — Create `mongo/echo_query.go`**

```go
// mongo/echo_query.go
package mongo

import (
	"context"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Compile-time interface satisfaction check.
var _ eywa.EchoQueryRepository = (*EchoRepository)(nil)

// ListSessions aggregates the messages collection to produce one SessionSummary per memory_key.
// SpiritName filter in SessionListOptions is silently ignored — Echo documents do not carry
// a spirit_name field. Enrich LastSpiritName from ChronicleQueryRepository at the handler layer
// if needed.
func (r *EchoRepository) ListSessions(ctx context.Context, opts eywa.SessionListOptions) ([]*eywa.SessionSummary, int64, error) {
	matchFilter := bson.M{"is_user_facing": true}
	timeFilter := bson.M{}
	if opts.DateFrom != nil {
		timeFilter["$gte"] = *opts.DateFrom
	}
	if opts.DateTo != nil {
		timeFilter["$lte"] = *opts.DateTo
	}
	if len(timeFilter) > 0 {
		matchFilter["timestamp"] = timeFilter
	}

	// Count distinct sessions first.
	countPipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$memory_key"}}}},
		{{Key: "$count", Value: "total"}},
	}
	countCursor, err := r.collection.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer countCursor.Close(ctx)
	var countResult []struct {
		Total int64 `bson:"total"`
	}
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}
	var total int64
	if len(countResult) > 0 {
		total = countResult[0].Total
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

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$memory_key"},
			{Key: "message_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "last_message_at", Value: bson.D{{Key: "$max", Value: "$timestamp"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "last_message_at", Value: -1}}}},
		{{Key: "$skip", Value: skip}},
		{{Key: "$limit", Value: int64(limit)}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	type sessionResult struct {
		MemoryKey     string    `bson:"_id"`
		MessageCount  int64     `bson:"message_count"`
		LastMessageAt time.Time `bson:"last_message_at"`
	}

	var raw []sessionResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, 0, err
	}

	result := make([]*eywa.SessionSummary, len(raw))
	for i, r := range raw {
		result[i] = &eywa.SessionSummary{
			MemoryKey:     r.MemoryKey,
			MessageCount:  r.MessageCount,
			LastMessageAt: r.LastMessageAt,
		}
	}
	return result, total, nil
}

// FindByMemoryKeyBefore returns user-facing messages for memoryKey with IDs less than beforeID,
// sorted newest-first. Pass empty beforeID to get the most recent messages.
func (r *EchoRepository) FindByMemoryKeyBefore(ctx context.Context, memoryKey, beforeID string, limit int) ([]*eywa.Echo, error) {
	if memoryKey == "" {
		return nil, nil
	}
	filter := bson.M{"memory_key": memoryKey, "is_user_facing": true}
	if beforeID != "" {
		oid, err := primitive.ObjectIDFromHex(beforeID)
		if err == nil {
			filter["_id"] = bson.M{"$lt": oid}
		}
	}
	if limit < 1 {
		limit = 20
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []*eywa.Echo
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}
```

> **Note:** `mongo/echo_query.go` imports `"time"` via `sessionResult.LastMessageAt`. Add it to the import block.

The full import block for `mongo/echo_query.go`:

```go
import (
	"context"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)
```

- [ ] **Step 9.3 — Build mongo module**

```bash
cd /path/to/eywa/mongo && go build ./...
```

Expected: no errors. If the compile-time check fails: `cannot use (*EchoRepository)(nil) as type eywa.EchoQueryRepository`.

- [ ] **Step 9.4 — Commit**

```bash
cd /path/to/eywa
git add mongo/echo_repository.go mongo/echo_query.go
git commit -m "feat(conversations): mongo EchoRepository — ListSessions + FindByMemoryKeyBefore"
```

---

## Task 10: Root re-exports + ManagementDeps EchoQuery fields

**Files:**
- Modify: `ports.go`
- Modify: `fiber/management.go`

- [ ] **Step 10.1 — Add echo query types to `ports.go`**

In `ports.go`, in the first `type (...)` block, add:

```go
EchoQueryRepository = ports.EchoQueryRepository
SessionListOptions  = ports.SessionListOptions
SessionSummary      = ports.SessionSummary
```

- [ ] **Step 10.2 — Add echo fields to `ManagementDeps` in `fiber/management.go` and register echo routes**

In `fiber/management.go`, the `ManagementDeps` struct already has `EchoRepo` and `EchoQueryRepo` fields from Step 5.1. `RegisterManagementRoutes` does not yet use them. Add the echo route block after the chronicle block:

```go
if deps.EchoQueryRepo != nil {
    eh := newEchoMgmtHandler(deps.EchoQueryRepo, deps.EchoRepo)
    echoes := api.Group("/echoes")
    echoes.Get("/sessions", eh.listSessions)
    echoes.Get("/sessions/:memoryKey", eh.sessionDetail)
    echoes.Get("", eh.listEchoes)
}
```

- [ ] **Step 10.3 — Create stub `fiber/echo_mgmt_handler.go` to unblock build**

```go
// fiber/echo_mgmt_handler.go
package fiber

import (
	eywa "github.com/wmulabs/eywa"
	fiberlib "github.com/gofiber/fiber/v2"
)

type echoMgmtHandler struct {
	queryRepo eywa.EchoQueryRepository
	echoRepo  eywa.EchoRepository
}

func newEchoMgmtHandler(queryRepo eywa.EchoQueryRepository, echoRepo eywa.EchoRepository) *echoMgmtHandler {
	return &echoMgmtHandler{queryRepo: queryRepo, echoRepo: echoRepo}
}

func (h *echoMgmtHandler) listSessions(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *echoMgmtHandler) sessionDetail(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}

func (h *echoMgmtHandler) listEchoes(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}
```

- [ ] **Step 10.4 — Build all modules**

```bash
cd /path/to/eywa && go build ./... && cd fiber && go build ./...
```

Expected: no errors.

- [ ] **Step 10.5 — Commit**

```bash
cd /path/to/eywa
git add ports.go fiber/management.go fiber/echo_mgmt_handler.go
git commit -m "feat(conversations): re-export EchoQueryRepository + echo management handler stub"
```

---

## Task 11: EchoMgmtHandler — sessions + detail (TDD)

**Files:**
- Create: `fiber/echo_mgmt_handler_test.go`
- Modify: `fiber/echo_mgmt_handler.go`

- [ ] **Step 11.1 — Write failing tests**

```go
// fiber/echo_mgmt_handler_test.go
package fiber

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
)

type stubEchoQueryRepo struct {
	sessions []*eywa.SessionSummary
	total    int64
	echoes   []*eywa.Echo
	err      error
}

func (s *stubEchoQueryRepo) ListSessions(_ context.Context, _ eywa.SessionListOptions) ([]*eywa.SessionSummary, int64, error) {
	return s.sessions, s.total, s.err
}
func (s *stubEchoQueryRepo) FindByMemoryKeyBefore(_ context.Context, _, _ string, _ int) ([]*eywa.Echo, error) {
	return s.echoes, s.err
}

func echoDeps(queryRepo *stubEchoQueryRepo) ManagementDeps {
	return ManagementDeps{
		APIKeys:       map[string]string{"test-key": "admin"},
		EchoQueryRepo: queryRepo,
	}
}

func TestEchoMgmtHandler_ListSessions_Returns200(t *testing.T) {
	now := time.Now()
	stub := &stubEchoQueryRepo{
		sessions: []*eywa.SessionSummary{
			{MemoryKey: "whatsapp:+5511999999999", MessageCount: 10, LastMessageAt: now},
		},
		total: 1,
	}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []*eywa.SessionSummary `json:"items"`
		Total int64                  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 session, got %d", len(body.Items))
	}
}

func TestEchoMgmtHandler_ListSessions_ReturnsEmptySlice(t *testing.T) {
	stub := &stubEchoQueryRepo{sessions: nil, total: 0}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []interface{} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items must be empty slice, not null")
	}
}

func TestEchoMgmtHandler_SessionDetail_Returns200(t *testing.T) {
	now := time.Now()
	stub := &stubEchoQueryRepo{
		sessions: []*eywa.SessionSummary{
			{MemoryKey: "whatsapp:+5511", MessageCount: 5, LastMessageAt: now},
		},
	}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions/whatsapp:+5511", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		MemoryKey    string `json:"memory_key"`
		MessageCount int64  `json:"message_count"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.MemoryKey != "whatsapp:+5511" {
		t.Errorf("want memory_key whatsapp:+5511, got %q", body.MemoryKey)
	}
}

func TestEchoMgmtHandler_SessionDetail_NotFound_Returns404(t *testing.T) {
	stub := &stubEchoQueryRepo{sessions: nil}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes/sessions/whatsapp:+unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 11.2 — Run tests to confirm failure**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestEchoMgmtHandler_List|TestEchoMgmtHandler_Session" -v
```

Expected: FAIL with 501 status.

- [ ] **Step 11.3 — Implement `listSessions` and `sessionDetail` in `fiber/echo_mgmt_handler.go`**

Replace the stub methods with:

```go
func (h *echoMgmtHandler) listSessions(c *fiberlib.Ctx) error {
	opts := eywa.SessionListOptions{
		SpiritName: c.Query("spirit_name"),
		Page:       c.QueryInt("page", 1),
		Limit:      c.QueryInt("limit", resthttp.DefaultPageLimit),
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_from: use RFC3339"})
		}
		opts.DateFrom = &t
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_to: use RFC3339"})
		}
		opts.DateTo = &t
	}

	sessions, total, err := h.queryRepo.ListSessions(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if sessions == nil {
		sessions = []*eywa.SessionSummary{}
	}
	return c.JSON(fiberlib.Map{"items": sessions, "total": total, "page": opts.Page, "limit": opts.Limit})
}

func (h *echoMgmtHandler) sessionDetail(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	sessions, _, err := h.queryRepo.ListSessions(c.Context(), eywa.SessionListOptions{
		Page:  1,
		Limit: 1,
	})
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	for _, s := range sessions {
		if s.MemoryKey == memoryKey {
			return c.JSON(fiberlib.Map{
				"memory_key":       s.MemoryKey,
				"last_spirit_name": s.LastSpiritName,
				"message_count":    s.MessageCount,
				"last_message_at":  s.LastMessageAt,
				"vigil":            nil,
			})
		}
	}
	return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "session not found"})
}
```

> **Note on sessionDetail:** In the stub test, `ListSessions` returns the full list and we search by memoryKey. In production, the mongo impl would filter by memoryKey directly. For correctness, add a `MemoryKey` field to `SessionListOptions` and use it in `ListSessions` to filter:

Add `MemoryKey string` to `SessionListOptions` in `internal/domain/ports/echo_query.go`:

```go
type SessionListOptions struct {
	SpiritName string
	MemoryKey  string     // filter to a specific session (for detail endpoint)
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	Limit      int
}
```

Then update `sessionDetail` to use it:

```go
func (h *echoMgmtHandler) sessionDetail(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	sessions, _, err := h.queryRepo.ListSessions(c.Context(), eywa.SessionListOptions{
		MemoryKey: memoryKey,
		Page:      1,
		Limit:     1,
	})
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if len(sessions) == 0 {
		return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "session not found"})
	}
	s := sessions[0]
	return c.JSON(fiberlib.Map{
		"memory_key":       s.MemoryKey,
		"last_spirit_name": s.LastSpiritName,
		"message_count":    s.MessageCount,
		"last_message_at":  s.LastMessageAt,
		"vigil":            nil,
	})
}
```

Also update `mongo/echo_query.go`'s `ListSessions` to apply the MemoryKey filter when set:

```go
// In buildSessionMatch (or inline in ListSessions), after the time filter block:
if opts.MemoryKey != "" {
    matchFilter["memory_key"] = opts.MemoryKey
}
```

Update `stubEchoQueryRepo.ListSessions` in test to match by MemoryKey:

```go
func (s *stubEchoQueryRepo) ListSessions(_ context.Context, opts eywa.SessionListOptions) ([]*eywa.SessionSummary, int64, error) {
	if opts.MemoryKey != "" {
		for _, sess := range s.sessions {
			if sess.MemoryKey == opts.MemoryKey {
				return []*eywa.SessionSummary{sess}, 1, s.err
			}
		}
		return []*eywa.SessionSummary{}, 0, s.err
	}
	return s.sessions, s.total, s.err
}
```

Also update the full `fiber/echo_mgmt_handler.go` with required imports:

```go
// fiber/echo_mgmt_handler.go
package fiber

import (
	"time"

	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
)

type echoMgmtHandler struct {
	queryRepo eywa.EchoQueryRepository
	echoRepo  eywa.EchoRepository
}

func newEchoMgmtHandler(queryRepo eywa.EchoQueryRepository, echoRepo eywa.EchoRepository) *echoMgmtHandler {
	return &echoMgmtHandler{queryRepo: queryRepo, echoRepo: echoRepo}
}

func (h *echoMgmtHandler) listSessions(c *fiberlib.Ctx) error {
	opts := eywa.SessionListOptions{
		SpiritName: c.Query("spirit_name"),
		Page:       c.QueryInt("page", 1),
		Limit:      c.QueryInt("limit", resthttp.DefaultPageLimit),
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_from: use RFC3339"})
		}
		opts.DateFrom = &t
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid date_to: use RFC3339"})
		}
		opts.DateTo = &t
	}

	sessions, total, err := h.queryRepo.ListSessions(c.Context(), opts)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if sessions == nil {
		sessions = []*eywa.SessionSummary{}
	}
	return c.JSON(fiberlib.Map{"items": sessions, "total": total, "page": opts.Page, "limit": opts.Limit})
}

func (h *echoMgmtHandler) sessionDetail(c *fiberlib.Ctx) error {
	memoryKey := c.Params("memoryKey")
	sessions, _, err := h.queryRepo.ListSessions(c.Context(), eywa.SessionListOptions{
		MemoryKey: memoryKey,
		Page:      1,
		Limit:     1,
	})
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if len(sessions) == 0 {
		return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "session not found"})
	}
	s := sessions[0]
	return c.JSON(fiberlib.Map{
		"memory_key":       s.MemoryKey,
		"last_spirit_name": s.LastSpiritName,
		"message_count":    s.MessageCount,
		"last_message_at":  s.LastMessageAt,
		"vigil":            nil,
	})
}

func (h *echoMgmtHandler) listEchoes(c *fiberlib.Ctx) error {
	return c.SendStatus(fiberlib.StatusNotImplemented)
}
```

- [ ] **Step 11.4 — Run tests**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestEchoMgmtHandler_List|TestEchoMgmtHandler_Session" -v
```

Expected: all 4 tests PASS.

- [ ] **Step 11.5 — Commit**

```bash
cd /path/to/eywa
git add internal/domain/ports/echo_query.go \
        mongo/echo_query.go \
        fiber/echo_mgmt_handler.go \
        fiber/echo_mgmt_handler_test.go
git commit -m "feat(conversations): echo sessions list + detail handlers with tests"
```

---

## Task 12: EchoMgmtHandler — echo list (TDD)

**Files:**
- Modify: `fiber/echo_mgmt_handler_test.go` (add echo list tests)
- Modify: `fiber/echo_mgmt_handler.go` (implement listEchoes)

- [ ] **Step 12.1 — Add echo list tests to `fiber/echo_mgmt_handler_test.go`**

Append:

```go
func TestEchoMgmtHandler_ListEchoes_MissingMemoryKey_Returns400(t *testing.T) {
	stub := &stubEchoQueryRepo{}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400 when memory_key missing, got %d", resp.StatusCode)
	}
}

func TestEchoMgmtHandler_ListEchoes_Returns200WithMessages(t *testing.T) {
	stub := &stubEchoQueryRepo{
		echoes: []*eywa.Echo{
			{ID: "msg1", MemoryKey: "whatsapp:+5511", Role: "user", Content: "hello"},
		},
	}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes?memory_key=whatsapp:+5511", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []*eywa.Echo `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 echo, got %d", len(body.Items))
	}
}

func TestEchoMgmtHandler_ListEchoes_ReturnsEmptySlice(t *testing.T) {
	stub := &stubEchoQueryRepo{echoes: nil}
	app := buildMgmtTestApp(echoDeps(stub))

	req := httptest.NewRequest("GET", "/api/v1/echoes?memory_key=whatsapp:+5511", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []interface{} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items must be empty slice, not null")
	}
}
```

- [ ] **Step 12.2 — Run tests to confirm failure**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestEchoMgmtHandler_ListEchoes" -v
```

Expected: FAIL — 501 for the requests-with-memory_key tests.

- [ ] **Step 12.3 — Implement `listEchoes` in `fiber/echo_mgmt_handler.go`**

Replace the stub `listEchoes`:

```go
func (h *echoMgmtHandler) listEchoes(c *fiberlib.Ctx) error {
	memoryKey := c.Query("memory_key")
	if memoryKey == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "memory_key is required"})
	}
	beforeID := c.Query("before_id")
	limit := c.QueryInt("limit", resthttp.DefaultPageLimit)

	echoes, err := h.queryRepo.FindByMemoryKeyBefore(c.Context(), memoryKey, beforeID, limit)
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	if echoes == nil {
		echoes = []*eywa.Echo{}
	}
	return c.JSON(fiberlib.Map{"items": echoes, "limit": limit})
}
```

- [ ] **Step 12.4 — Run all echo management tests**

```bash
cd /path/to/eywa/fiber && go test ./... -run "TestEchoMgmtHandler" -v
```

Expected: all 7 tests PASS.

- [ ] **Step 12.5 — Commit**

```bash
cd /path/to/eywa
git add fiber/echo_mgmt_handler.go fiber/echo_mgmt_handler_test.go
git commit -m "feat(conversations): echo list endpoint with cursor-based pagination"
```

---

## Task 13: Full build + test verification

- [ ] **Step 13.1 — Run all root module tests**

```bash
cd /path/to/eywa && go test ./... -v
```

Expected: all PASS.

- [ ] **Step 13.2 — Run all fiber module tests**

```bash
cd /path/to/eywa/fiber && go test ./... -v
```

Expected: all PASS. Total: 10 chronicle tests + 7 echo tests + 8 auth middleware tests = 25.

- [ ] **Step 13.3 — Build mongo module**

```bash
cd /path/to/eywa/mongo && go build ./...
```

Expected: no errors.

- [ ] **Step 13.4 — Build root + fiber**

```bash
cd /path/to/eywa && go build ./... && cd fiber && go build ./...
```

Expected: no errors.

- [ ] **Step 13.5 — Commit if any uncommitted changes**

```bash
git status
```

---

## Spec Coverage Check

| Spec section | Covered by |
|---|---|
| ChronicleQueryRepository — List, FindByID | Tasks 1, 2, 4, 6 |
| ChronicleQueryRepository — AggregateTokens | Tasks 1, 3, 4, 7 |
| ChronicleQueryRepository — AggregateActions | Tasks 1, 3, 4, 7 |
| ChronicleQueryRepository — AggregateSpirits | Tasks 1, 3, 4, 7 |
| Mongo implementation — Chronicle query side | Tasks 2, 3 |
| GET /chronicle, GET /chronicle/:id | Task 6 |
| GET /analytics/tokens, /actions, /spirits | Task 7 |
| ManagementDeps + RegisterManagementRoutes | Task 5 |
| Validator chain from ManagementDeps | Task 5 |
| EchoQueryRepository — ListSessions, FindByMemoryKeyBefore | Tasks 8, 9, 10 |
| SessionListOptions.MemoryKey filter | Task 11 |
| Mongo implementation — Echo query side | Task 9 |
| GET /echoes/sessions, /echoes/sessions/:memoryKey | Task 11 |
| GET /echoes (cursor pagination) | Task 12 |
| Re-exports in root ports.go | Tasks 4, 10 |

**Not in this plan (later phases):**
- VigilRepository + VigilCheckStep + ErrSessionHeld (Phase 5)
- RiteRepository + request_rite Action (Phase 6)
- LinkConfigRepository + LinkCache + Config pub/sub (Phase 7)
- OperatorAuth routes /auth/token + /operators CRUD (Phase 8)
- RegisterManagementRoutes Phase 8 unified integration (Phase 8)
