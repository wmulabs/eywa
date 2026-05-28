# Management Layer — Phase 1 + Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 1 (Typing Indicator — deferred pipeline steps) and Phase 2 (Auth Foundation — three-mode validator chain + fiber middleware), the prerequisite infrastructure for all subsequent management layer phases.

**Architecture:** Phase 1 adds deferred step support to `Pipeline` and wires `TypingStartStep` / `TypingStopStep` into the engine. Phase 2 adds `TokenValidator`, `AuthClaims`, and `OperatorRepository` ports, three concrete validator implementations (API key, built-in JWT, external JWKS), and the `AuthMiddleware` + `RequireRole` fiber handlers. Auth lives entirely in the fiber layer — the engine never validates tokens.

**Tech Stack:** Go 1.25.5, `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`, `github.com/gofiber/fiber/v2` (fiber sub-module only). No new deps in fiber module (jwt/bcrypt live in root).

---

## File Map

### Phase 1 — Typing Indicator

| Action | Path |
|--------|------|
| **Modify** | `internal/implementation/orchestrator/pipeline.go` |
| **Create** | `internal/implementation/orchestrator/pipeline_step_typing.go` |
| **Modify** | `internal/implementation/orchestrator/builder.go` |
| **Modify** | `internal/implementation/orchestrator/engine.go` |
| **Create** | `internal/domain/ports/typing_indicator.go` |
| **Modify** | `ports.go` |
| **Create** | `internal/implementation/orchestrator/pipeline_test.go` |
| **Create** | `internal/implementation/orchestrator/pipeline_step_typing_test.go` |

### Phase 2 — Auth Foundation

| Action | Path |
|--------|------|
| **Create** | `internal/domain/entities/operator.go` |
| **Create** | `internal/domain/ports/auth.go` |
| **Create** | `internal/infrastructure/driven/auth/apikey_validator.go` |
| **Create** | `internal/infrastructure/driven/auth/operator_auth.go` |
| **Create** | `internal/infrastructure/driven/auth/jwt_validator.go` |
| **Create** | `internal/infrastructure/driven/auth/jwks_validator.go` |
| **Modify** | `go.mod` + `go.sum` |
| **Modify** | `ports.go` |
| **Modify** | `eywa.go` |
| **Create** | `internal/infrastructure/driven/auth/apikey_validator_test.go` |
| **Create** | `internal/infrastructure/driven/auth/operator_auth_test.go` |
| **Create** | `internal/infrastructure/driven/auth/jwt_validator_test.go` |
| **Create** | `fiber/middleware/auth.go` |
| **Create** | `fiber/middleware/auth_test.go` |

---

## Task 1: Pipeline — deferred step support

**Files:**
- Modify: `internal/implementation/orchestrator/pipeline.go`
- Create: `internal/implementation/orchestrator/pipeline_test.go`

- [ ] **Step 1.1 — Write failing tests**

```go
// internal/implementation/orchestrator/pipeline_test.go
package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// testStep is a minimal ProcessingStep for unit tests.
type testStep struct {
	name   string
	err    error
	called *bool
}

func (s *testStep) Name() string           { return s.name }
func (s *testStep) Timeout() time.Duration { return 0 }
func (s *testStep) Execute(_ context.Context, _ *ProcessingState) error {
	if s.called != nil {
		*s.called = true
	}
	return s.err
}

func newTestPipeline() *Pipeline {
	logger, _ := zap.NewDevelopment()
	return NewPipeline(logger.Sugar(), trace.NewNoopTracerProvider().Tracer("test"))
}

func minimalState() *ProcessingState {
	return &ProcessingState{Event: &entities.Pulse{}}
}

func TestPipeline_DeferredStep_RunsOnSuccess(t *testing.T) {
	p := newTestPipeline()
	called := false
	p.AddDeferredStep(&testStep{name: "d", called: &called})

	if err := p.Execute(context.Background(), minimalState()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("deferred step not called on success")
	}
}

func TestPipeline_DeferredStep_RunsOnStepFailure(t *testing.T) {
	p := newTestPipeline()
	mainCalled, deferredCalled := false, false

	p.AddStep(&testStep{name: "main", err: errors.New("fail"), called: &mainCalled})
	p.AddDeferredStep(&testStep{name: "d", called: &deferredCalled})

	err := p.Execute(context.Background(), minimalState())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !deferredCalled {
		t.Error("deferred step not called on failure")
	}
}

func TestPipeline_DeferredStep_ErrorDoesNotOverwriteMainError(t *testing.T) {
	p := newTestPipeline()
	mainErr := errors.New("main")
	mainCalled, dCalled := false, false

	p.AddStep(&testStep{name: "main", err: mainErr, called: &mainCalled})
	p.AddDeferredStep(&testStep{name: "d", err: errors.New("deferred-err"), called: &dCalled})

	err := p.Execute(context.Background(), minimalState())
	if !errors.Is(err, mainErr) {
		t.Errorf("want main error %v, got %v", mainErr, err)
	}
	if !dCalled {
		t.Error("deferred step not called")
	}
}

func TestPipeline_AllDeferredSteps_RunOnFailure(t *testing.T) {
	p := newTestPipeline()
	called1, called2 := false, false
	mainCalled := false

	p.AddStep(&testStep{name: "main", err: errors.New("fail"), called: &mainCalled})
	p.AddDeferredStep(&testStep{name: "d1", called: &called1})
	p.AddDeferredStep(&testStep{name: "d2", called: &called2})

	p.Execute(context.Background(), minimalState())

	if !called1 || !called2 {
		t.Errorf("not all deferred steps ran: d1=%v d2=%v", called1, called2)
	}
}
```

- [ ] **Step 1.2 — Run tests to confirm failure**

```bash
cd /path/to/eywa && go test ./internal/implementation/orchestrator/... -run TestPipeline_Deferred -v
```

Expected: `FAIL — AddDeferredStep undefined`

- [ ] **Step 1.3 — Implement deferred steps in `pipeline.go`**

Add `deferredSteps []ProcessingStep` field to `Pipeline` struct and `AddDeferredStep` method. Replace `Execute` with the version that wraps the loop in a `defer`:

```go
// internal/implementation/orchestrator/pipeline.go
// Replace the existing Pipeline struct and Execute method:

type Pipeline struct {
	steps         []ProcessingStep
	deferredSteps []ProcessingStep
	logger        *zap.SugaredLogger
	tracer        trace.Tracer
}

func NewPipeline(logger *zap.SugaredLogger, tracer trace.Tracer) *Pipeline {
	return &Pipeline{logger: logger, tracer: tracer}
}

func (p *Pipeline) AddStep(step ProcessingStep) *Pipeline {
	p.steps = append(p.steps, step)
	return p
}

func (p *Pipeline) AddDeferredStep(step ProcessingStep) *Pipeline {
	p.deferredSteps = append(p.deferredSteps, step)
	return p
}

func (p *Pipeline) Execute(ctx context.Context, state *ProcessingState) (returnErr error) {
	ctx, span := p.tracer.Start(ctx, "Pipeline/Execute")
	defer span.End()

	defer func() {
		for _, step := range p.deferredSteps {
			if err := p.executeStep(ctx, step, state); err != nil {
				p.logger.Warnw("deferred step failed", "step", step.Name(), "error", err)
			}
		}
	}()

	state.StartTime = time.Now()
	state.ProcessingStatus = "success"

	for _, step := range p.steps {
		if err := p.executeStep(ctx, step, state); err != nil {
			p.logger.Errorw("pipeline step failed",
				"step", step.Name(),
				"error", err,
				"event_id", state.Event.ID,
				"memory_key", state.Event.MemoryKey,
			)
			state.PipelineFailedAtStep = step.Name()
			return err
		}
	}
	return nil
}
```

The `executeStep` method is unchanged — keep it exactly as it is.

- [ ] **Step 1.4 — Run tests to confirm pass**

```bash
go test ./internal/implementation/orchestrator/... -run TestPipeline_Deferred -v
```

Expected: all 4 tests PASS.

- [ ] **Step 1.5 — Commit**

```bash
git add internal/implementation/orchestrator/pipeline.go \
        internal/implementation/orchestrator/pipeline_test.go
git commit -m "feat(pipeline): add deferred step support"
```

---

## Task 2: TypingIndicator port + pipeline steps

**Files:**
- Create: `internal/domain/ports/typing_indicator.go`
- Create: `internal/implementation/orchestrator/pipeline_step_typing.go`
- Create: `internal/implementation/orchestrator/pipeline_step_typing_test.go`

- [ ] **Step 2.1 — Create the port**

```go
// internal/domain/ports/typing_indicator.go
package ports

import "context"

type TypingIndicator interface {
	StartTyping(ctx context.Context, phone string) error
	StopTyping(ctx context.Context, phone string) error
}
```

- [ ] **Step 2.2 — Write failing tests for the steps**

```go
// internal/implementation/orchestrator/pipeline_step_typing_test.go
package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.uber.org/zap"
)

type mockTypingIndicator struct {
	startCalled bool
	stopCalled  bool
	startErr    error
	stopErr     error
}

func (m *mockTypingIndicator) StartTyping(_ context.Context, _ string) error {
	m.startCalled = true
	return m.startErr
}

func (m *mockTypingIndicator) StopTyping(_ context.Context, _ string) error {
	m.stopCalled = true
	return m.stopErr
}

func testLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

func stateWithPhone(phone string) *ProcessingState {
	return &ProcessingState{Event: &entities.Pulse{ContactPhone: phone}}
}

func TestTypingStartStep_NilIndicator_NoOp(t *testing.T) {
	step := NewTypingStartStep(nil, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTypingStartStep_EmptyPhone_DoesNotCall(t *testing.T) {
	mock := &mockTypingIndicator{}
	step := NewTypingStartStep(mock, testLogger(t))
	step.Execute(context.Background(), stateWithPhone(""))
	if mock.startCalled {
		t.Error("StartTyping must not be called when ContactPhone is empty")
	}
}

func TestTypingStartStep_CallsStart(t *testing.T) {
	mock := &mockTypingIndicator{}
	step := NewTypingStartStep(mock, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mock.startCalled {
		t.Error("StartTyping was not called")
	}
}

func TestTypingStartStep_IndicatorError_DoesNotFailPipeline(t *testing.T) {
	mock := &mockTypingIndicator{startErr: errors.New("network error")}
	step := NewTypingStartStep(mock, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Error("typing indicator error must not fail the pipeline")
	}
}

func TestTypingStopStep_NilIndicator_NoOp(t *testing.T) {
	step := NewTypingStopStep(nil, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTypingStopStep_IndicatorError_DoesNotFailPipeline(t *testing.T) {
	mock := &mockTypingIndicator{stopErr: errors.New("network error")}
	step := NewTypingStopStep(mock, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Error("typing stop error must not fail the pipeline")
	}
}
```

- [ ] **Step 2.3 — Run tests to confirm failure**

```bash
go test ./internal/implementation/orchestrator/... -run TestTyping -v
```

Expected: FAIL — `NewTypingStartStep undefined`.

- [ ] **Step 2.4 — Implement the steps**

```go
// internal/implementation/orchestrator/pipeline_step_typing.go
package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type TypingStartStep struct {
	indicator ports.TypingIndicator
	logger    *zap.SugaredLogger
}

func NewTypingStartStep(indicator ports.TypingIndicator, logger *zap.SugaredLogger) *TypingStartStep {
	return &TypingStartStep{indicator: indicator, logger: logger}
}

func (s *TypingStartStep) Name() string           { return "TypingStart" }
func (s *TypingStartStep) Timeout() time.Duration { return 3 * time.Second }

func (s *TypingStartStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.indicator == nil || state.Event.ContactPhone == "" {
		return nil
	}
	if err := s.indicator.StartTyping(ctx, state.Event.ContactPhone); err != nil {
		s.logger.Warnw("typing start failed", "error", err, "phone", state.Event.ContactPhone)
	}
	return nil
}

type TypingStopStep struct {
	indicator ports.TypingIndicator
	logger    *zap.SugaredLogger
}

func NewTypingStopStep(indicator ports.TypingIndicator, logger *zap.SugaredLogger) *TypingStopStep {
	return &TypingStopStep{indicator: indicator, logger: logger}
}

func (s *TypingStopStep) Name() string           { return "TypingStop" }
func (s *TypingStopStep) Timeout() time.Duration { return 3 * time.Second }

func (s *TypingStopStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.indicator == nil || state.Event.ContactPhone == "" {
		return nil
	}
	if err := s.indicator.StopTyping(ctx, state.Event.ContactPhone); err != nil {
		s.logger.Warnw("typing stop failed", "error", err, "phone", state.Event.ContactPhone)
	}
	return nil
}
```

- [ ] **Step 2.5 — Run tests to confirm pass**

```bash
go test ./internal/implementation/orchestrator/... -run TestTyping -v
```

Expected: all 6 tests PASS.

- [ ] **Step 2.6 — Commit**

```bash
git add internal/domain/ports/typing_indicator.go \
        internal/implementation/orchestrator/pipeline_step_typing.go \
        internal/implementation/orchestrator/pipeline_step_typing_test.go
git commit -m "feat(typing): TypingIndicator port + TypingStart/Stop pipeline steps"
```

---

## Task 3: Wire Typing into builder and engine

**Files:**
- Modify: `internal/implementation/orchestrator/builder.go`
- Modify: `internal/implementation/orchestrator/engine.go`

- [ ] **Step 3.1 — Add `typingIndicator` field to `Weave` struct in `engine.go`**

In the `Weave` struct (around line 32), add after the last field before the closing brace:

```go
typingIndicator ports.TypingIndicator
```

Import `"github.com/wmulabs/eywa/internal/domain/ports"` is already present.

- [ ] **Step 3.2 — Add `typingIndicator` field and `WithTypingIndicator` to builder in `builder.go`**

In `WeaveBuilder` struct, add:

```go
typingIndicator ports.TypingIndicator
```

Add the builder method after `WithMediaProcessor`:

```go
func (b *WeaveBuilder) WithTypingIndicator(ti ports.TypingIndicator) *WeaveBuilder {
	b.typingIndicator = ti
	return b
}
```

In `Build()`, after the `mediaProcessor` wiring block (around line 580), add:

```go
if b.typingIndicator != nil {
	engine.typingIndicator = b.typingIndicator
}
```

- [ ] **Step 3.3 — Wire into `buildProcessingPipeline` in `engine.go`**

In `buildProcessingPipeline()`, immediately after the `LockAcquisitionStep` call (around line 472), add:

```go
if e.typingIndicator != nil {
	pipeline.AddStep(NewTypingStartStep(e.typingIndicator, e.logger))
}
```

At the end of `buildProcessingPipeline()`, before `return pipeline`, add:

```go
if e.typingIndicator != nil {
	pipeline.AddDeferredStep(NewTypingStopStep(e.typingIndicator, e.logger))
}
```

- [ ] **Step 3.4 — Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3.5 — Run all orchestrator tests**

```bash
go test ./internal/... -v
```

Expected: all PASS.

- [ ] **Step 3.6 — Commit**

```bash
git add internal/implementation/orchestrator/builder.go \
        internal/implementation/orchestrator/engine.go
git commit -m "feat(typing): wire TypingIndicator into WeaveBuilder and pipeline"
```

---

## Task 4: Root re-export for TypingIndicator

**Files:**
- Modify: `ports.go`

- [ ] **Step 4.1 — Add `TypingIndicator` to the type alias block in `ports.go`**

In `ports.go`, in the first `type (...)` block (the one with `Oracle`, `Action`, etc.), add:

```go
TypingIndicator = ports.TypingIndicator
```

- [ ] **Step 4.2 — Build root module**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4.3 — Commit**

```bash
git add ports.go
git commit -m "feat(typing): re-export TypingIndicator from root package"
```

---

## Task 5: Operator entity + Auth ports

**Files:**
- Create: `internal/domain/entities/operator.go`
- Create: `internal/domain/ports/auth.go`

- [ ] **Step 5.1 — Create Operator entity**

```go
// internal/domain/entities/operator.go
package entities

import "time"

type Operator struct {
	ID           string    `bson:"_id"           json:"id"`
	Name         string    `bson:"name"          json:"name"`
	Email        string    `bson:"email"         json:"email"`
	Role         string    `bson:"role"          json:"role"`
	PasswordHash string    `bson:"password_hash" json:"-"`
	IsActive     bool      `bson:"is_active"     json:"is_active"`
	CreatedAt    time.Time `bson:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"    json:"updated_at"`
}
```

- [ ] **Step 5.2 — Create Auth ports**

```go
// internal/domain/ports/auth.go
package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type TokenValidator interface {
	Validate(ctx context.Context, token string) (*AuthClaims, error)
}

type AuthClaims struct {
	Subject string
	Role    string
}

type OperatorRepository interface {
	Create(ctx context.Context, op *entities.Operator) error
	FindByEmail(ctx context.Context, email string) (*entities.Operator, error)
	FindByID(ctx context.Context, id string) (*entities.Operator, error)
	List(ctx context.Context, page, limit int) ([]*entities.Operator, int64, error)
	Update(ctx context.Context, op *entities.Operator) error
	Deactivate(ctx context.Context, id string) error
}

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
)
```

- [ ] **Step 5.3 — Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5.4 — Commit**

```bash
git add internal/domain/entities/operator.go \
        internal/domain/ports/auth.go
git commit -m "feat(auth): Operator entity and auth ports"
```

---

## Task 6: APIKeyValidator

**Files:**
- Create: `internal/infrastructure/driven/auth/apikey_validator.go`
- Create: `internal/infrastructure/driven/auth/apikey_validator_test.go`

- [ ] **Step 6.1 — Write failing tests**

```go
// internal/infrastructure/driven/auth/apikey_validator_test.go
package auth

import (
	"context"
	"testing"
)

func TestAPIKeyValidator_ValidKey_ReturnsCorrectRole(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{
		"sk-admin-123": "admin",
		"sk-op-456":   "operator",
	})

	claims, err := v.Validate(context.Background(), "sk-admin-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != "admin" {
		t.Errorf("want role admin, got %s", claims.Role)
	}
	if claims.Subject != "sk-admin-123" {
		t.Errorf("want subject sk-admin-123, got %s", claims.Subject)
	}
}

func TestAPIKeyValidator_ValidOperatorKey(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{"sk-op-456": "operator"})

	claims, err := v.Validate(context.Background(), "sk-op-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Role != "operator" {
		t.Errorf("want role operator, got %s", claims.Role)
	}
}

func TestAPIKeyValidator_InvalidKey_ReturnsError(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{"sk-admin-123": "admin"})

	if _, err := v.Validate(context.Background(), "not-a-valid-key"); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestAPIKeyValidator_EmptyKey_ReturnsError(t *testing.T) {
	v := NewAPIKeyValidator(map[string]string{"sk-admin-123": "admin"})

	if _, err := v.Validate(context.Background(), ""); err == nil {
		t.Error("expected error for empty key")
	}
}
```

- [ ] **Step 6.2 — Run tests to confirm failure**

```bash
go test ./internal/infrastructure/driven/auth/... -run TestAPIKey -v
```

Expected: FAIL — `NewAPIKeyValidator undefined`.

- [ ] **Step 6.3 — Implement**

```go
// internal/infrastructure/driven/auth/apikey_validator.go
package auth

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

type APIKeyValidator struct {
	keys map[string]string // token → role
}

func NewAPIKeyValidator(keys map[string]string) ports.TokenValidator {
	return &APIKeyValidator{keys: keys}
}

func (v *APIKeyValidator) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	role, ok := v.keys[token]
	if !ok {
		return nil, fmt.Errorf("invalid api key")
	}
	return &ports.AuthClaims{Subject: token, Role: role}, nil
}
```

- [ ] **Step 6.4 — Run tests to confirm pass**

```bash
go test ./internal/infrastructure/driven/auth/... -run TestAPIKey -v
```

Expected: all 4 PASS.

- [ ] **Step 6.5 — Commit**

```bash
git add internal/infrastructure/driven/auth/apikey_validator.go \
        internal/infrastructure/driven/auth/apikey_validator_test.go
git commit -m "feat(auth): APIKeyValidator — mode 1 zero-infra token validation"
```

---

## Task 7: Add JWT + bcrypt dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 7.1 — Add dependencies**

```bash
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto
```

- [ ] **Step 7.2 — Build to confirm**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 7.3 — Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang-jwt and x/crypto dependencies"
```

---

## Task 8: OperatorAuth — built-in JWT issuer and validator

**Files:**
- Create: `internal/infrastructure/driven/auth/operator_auth.go`
- Create: `internal/infrastructure/driven/auth/operator_auth_test.go`

- [ ] **Step 8.1 — Write failing tests**

```go
// internal/infrastructure/driven/auth/operator_auth_test.go
package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type mockOperatorRepo struct {
	byEmail map[string]*entities.Operator
	byID    map[string]*entities.Operator
}

func (r *mockOperatorRepo) FindByEmail(_ context.Context, email string) (*entities.Operator, error) {
	op, ok := r.byEmail[email]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return op, nil
}
func (r *mockOperatorRepo) FindByID(_ context.Context, id string) (*entities.Operator, error) {
	op, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return op, nil
}
func (r *mockOperatorRepo) Create(_ context.Context, _ *entities.Operator) error              { return nil }
func (r *mockOperatorRepo) List(_ context.Context, _, _ int) ([]*entities.Operator, int64, error) {
	return nil, 0, nil
}
func (r *mockOperatorRepo) Update(_ context.Context, _ *entities.Operator) error   { return nil }
func (r *mockOperatorRepo) Deactivate(_ context.Context, _ string) error           { return nil }

func testOperatorRepo(t *testing.T, password string) *mockOperatorRepo {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	op := &entities.Operator{
		ID:           "op-1",
		Email:        "admin@eywa.io",
		Role:         ports.RoleAdmin,
		PasswordHash: hash,
		IsActive:     true,
	}
	return &mockOperatorRepo{
		byEmail: map[string]*entities.Operator{op.Email: op},
		byID:    map[string]*entities.Operator{op.ID: op},
	}
}

func TestOperatorAuth_Login_Success(t *testing.T) {
	repo := testOperatorRepo(t, "correct-pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	token, expiresAt, err := auth.Login(context.Background(), "admin@eywa.io", "correct-pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if expiresAt.Before(time.Now()) {
		t.Error("token already expired on creation")
	}
}

func TestOperatorAuth_Login_WrongPassword(t *testing.T) {
	repo := testOperatorRepo(t, "correct-pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, _, err := auth.Login(context.Background(), "admin@eywa.io", "wrong-pass"); err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestOperatorAuth_Login_UnknownEmail(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, _, err := auth.Login(context.Background(), "unknown@eywa.io", "pass"); err == nil {
		t.Error("expected error for unknown email")
	}
}

func TestOperatorAuth_Validate_RoundTrip(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	token, _, err := auth.Login(context.Background(), "admin@eywa.io", "pass")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	claims, err := auth.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Role != ports.RoleAdmin {
		t.Errorf("want role %s, got %s", ports.RoleAdmin, claims.Role)
	}
	if claims.Subject != "op-1" {
		t.Errorf("want subject op-1, got %s", claims.Subject)
	}
}

func TestOperatorAuth_Validate_WrongSecret(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth1 := NewOperatorAuth(repo, []byte("secret-1"))
	auth2 := NewOperatorAuth(repo, []byte("secret-2"))

	token, _, _ := auth1.Login(context.Background(), "admin@eywa.io", "pass")

	if _, err := auth2.Validate(context.Background(), token); err == nil {
		t.Error("expected validation failure with different secret")
	}
}

func TestOperatorAuth_Validate_InvalidToken(t *testing.T) {
	repo := testOperatorRepo(t, "pass")
	auth := NewOperatorAuth(repo, []byte("secret"))

	if _, err := auth.Validate(context.Background(), "not.a.jwt"); err == nil {
		t.Error("expected error for invalid token format")
	}
}

func TestHashPassword_Roundtrip(t *testing.T) {
	hash, err := HashPassword("my-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "my-password" {
		t.Error("hash must differ from plaintext")
	}
	// Verify the hash works with OperatorAuth's internal bcrypt check.
	// We test this implicitly through TestOperatorAuth_Login_Success.
}
```

- [ ] **Step 8.2 — Run tests to confirm failure**

```bash
go test ./internal/infrastructure/driven/auth/... -run TestOperatorAuth -v
```

Expected: FAIL — `NewOperatorAuth undefined`.

- [ ] **Step 8.3 — Implement**

```go
// internal/infrastructure/driven/auth/operator_auth.go
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"golang.org/x/crypto/bcrypt"
)

type OperatorAuth struct {
	repo     ports.OperatorRepository
	secret   []byte
	tokenTTL time.Duration
}

func NewOperatorAuth(repo ports.OperatorRepository, secret []byte) *OperatorAuth {
	return &OperatorAuth{repo: repo, secret: secret, tokenTTL: 8 * time.Hour}
}

func (a *OperatorAuth) WithTokenTTL(ttl time.Duration) *OperatorAuth {
	a.tokenTTL = ttl
	return a
}

type operatorClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Login authenticates credentials and returns a signed HS256 JWT.
func (a *OperatorAuth) Login(ctx context.Context, email, password string) (token string, expiresAt time.Time, err error) {
	op, err := a.repo.FindByEmail(ctx, email)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}
	if !op.IsActive {
		return "", time.Time{}, fmt.Errorf("account is inactive")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(op.PasswordHash), []byte(password)); err != nil {
		return "", time.Time{}, fmt.Errorf("invalid credentials")
	}

	expiresAt = time.Now().Add(a.tokenTTL)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, operatorClaims{
		Role: op.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   op.ID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := t.SignedString(a.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

// Validate implements ports.TokenValidator — verifies JWTs issued by Login.
func (a *OperatorAuth) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	t, err := jwt.ParseWithClaims(token, &operatorClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := t.Claims.(*operatorClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	return &ports.AuthClaims{Subject: claims.Subject, Role: claims.Role}, nil
}

// HashPassword bcrypt-hashes a plain-text password for storage.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}
```

- [ ] **Step 8.4 — Run tests to confirm pass**

```bash
go test ./internal/infrastructure/driven/auth/... -run "TestOperatorAuth|TestHash" -v
```

Expected: all 7 PASS.

- [ ] **Step 8.5 — Commit**

```bash
git add internal/infrastructure/driven/auth/operator_auth.go \
        internal/infrastructure/driven/auth/operator_auth_test.go
git commit -m "feat(auth): OperatorAuth — built-in JWT issuer and validator with bcrypt"
```

---

## Task 9: External JWT validators — HS256, RS256, JWKS

**Files:**
- Create: `internal/infrastructure/driven/auth/jwt_validator.go`
- Create: `internal/infrastructure/driven/auth/jwks_validator.go`
- Create: `internal/infrastructure/driven/auth/jwt_validator_test.go`

- [ ] **Step 9.1 — Write failing tests**

```go
// internal/infrastructure/driven/auth/jwt_validator_test.go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signHS256(t *testing.T, secret []byte, subject, role string, ttl time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	signed, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("sign hs256: %v", err)
	}
	return signed
}

func signRS256(t *testing.T, key *rsa.PrivateKey, subject, role string, ttl time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign rs256: %v", err)
	}
	return signed
}

func TestJWTValidator_HS256_ValidToken(t *testing.T) {
	secret := []byte("test-secret")
	v := NewJWTValidator(secret)
	token := signHS256(t, secret, "user-1", "admin", time.Hour)

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("want subject user-1, got %s", claims.Subject)
	}
	if claims.Role != "admin" {
		t.Errorf("want role admin, got %s", claims.Role)
	}
}

func TestJWTValidator_HS256_WrongSecret(t *testing.T) {
	v := NewJWTValidator([]byte("secret-A"))
	token := signHS256(t, []byte("secret-B"), "user-1", "admin", time.Hour)

	if _, err := v.Validate(context.Background(), token); err == nil {
		t.Error("expected error with wrong secret")
	}
}

func TestJWTValidator_HS256_ExpiredToken(t *testing.T) {
	secret := []byte("secret")
	v := NewJWTValidator(secret)
	token := signHS256(t, secret, "user-1", "admin", -time.Hour)

	if _, err := v.Validate(context.Background(), token); err == nil {
		t.Error("expected error for expired token")
	}
}

func TestJWTValidatorRS256_ValidToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	v := NewJWTValidatorRS256(&key.PublicKey)
	token := signRS256(t, key, "user-2", "operator", time.Hour)

	claims, err := v.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-2" {
		t.Errorf("want subject user-2, got %s", claims.Subject)
	}
	if claims.Role != "operator" {
		t.Errorf("want role operator, got %s", claims.Role)
	}
}

func TestJWTValidatorRS256_WrongKey(t *testing.T) {
	keyA, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyB, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := NewJWTValidatorRS256(&keyB.PublicKey)
	token := signRS256(t, keyA, "user-2", "operator", time.Hour)

	if _, err := v.Validate(context.Background(), token); err == nil {
		t.Error("expected error with wrong public key")
	}
}
```

- [ ] **Step 9.2 — Run tests to confirm failure**

```bash
go test ./internal/infrastructure/driven/auth/... -run TestJWT -v
```

Expected: FAIL — `NewJWTValidator undefined`.

- [ ] **Step 9.3 — Implement `jwt_validator.go`**

```go
// internal/infrastructure/driven/auth/jwt_validator.go
package auth

import (
	"context"
	"crypto/rsa"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// jwtClaims is shared by JWTValidator and JWKSValidator.
type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type JWTValidator struct {
	keyFunc jwt.Keyfunc
}

// NewJWTValidator validates HS256 tokens signed with secret.
func NewJWTValidator(secret []byte) ports.TokenValidator {
	return &JWTValidator{
		keyFunc: func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secret, nil
		},
	}
}

// NewJWTValidatorRS256 validates RS256 tokens signed with the matching private key.
func NewJWTValidatorRS256(pubKey *rsa.PublicKey) ports.TokenValidator {
	return &JWTValidator{
		keyFunc: func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pubKey, nil
		},
	}
}

func (v *JWTValidator) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	t, err := jwt.ParseWithClaims(token, &jwtClaims{}, v.keyFunc)
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := t.Claims.(*jwtClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("missing subject: %w", err)
	}
	return &ports.AuthClaims{Subject: sub, Role: claims.Role}, nil
}
```

- [ ] **Step 9.4 — Implement `jwks_validator.go`**

```go
// internal/infrastructure/driven/auth/jwks_validator.go
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type JWKSValidator struct {
	jwksURL  string
	audience string
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	cacheTTL  time.Duration
}

// NewJWKSValidator validates RS256 JWTs using keys from a JWKS endpoint (Auth0, Firebase, Google IAP).
// Claims mapping: standard sub → Subject, custom role claim → Role.
func NewJWKSValidator(jwksURL, audience string) ports.TokenValidator {
	return &JWKSValidator{
		jwksURL:  jwksURL,
		audience: audience,
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: time.Hour,
	}
}

func (v *JWKSValidator) fetchKeys() error {
	resp, err := http.Get(v.jwksURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

func (v *JWKSValidator) publicKey(kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	stale := time.Since(v.fetchedAt) > v.cacheTTL || len(v.keys) == 0
	key := v.keys[kid]
	v.mu.RUnlock()

	if stale || key == nil {
		if err := v.fetchKeys(); err != nil {
			return nil, err
		}
		v.mu.RLock()
		key = v.keys[kid]
		v.mu.RUnlock()
	}
	if key == nil {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}
	return key, nil
}

func (v *JWKSValidator) Validate(_ context.Context, token string) (*ports.AuthClaims, error) {
	t, err := jwt.ParseWithClaims(token, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.publicKey(kid)
	})
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := t.Claims.(*jwtClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("missing subject: %w", err)
	}
	return &ports.AuthClaims{Subject: sub, Role: claims.Role}, nil
}

func rsaPublicKeyFromJWK(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
```

- [ ] **Step 9.5 — Run all auth tests**

```bash
go test ./internal/infrastructure/driven/auth/... -v
```

Expected: all tests PASS.

- [ ] **Step 9.6 — Commit**

```bash
git add internal/infrastructure/driven/auth/jwt_validator.go \
        internal/infrastructure/driven/auth/jwks_validator.go \
        internal/infrastructure/driven/auth/jwt_validator_test.go
git commit -m "feat(auth): JWTValidator (HS256/RS256) and JWKSValidator (OIDC)"
```

---

## Task 10: Root re-exports for auth

**Files:**
- Modify: `ports.go`
- Modify: `eywa.go`

- [ ] **Step 10.1 — Add auth ports to `ports.go`**

In the `type (...)` block in `ports.go`, add:

```go
TokenValidator     = ports.TokenValidator
AuthClaims         = ports.AuthClaims
OperatorRepository = ports.OperatorRepository
```

Also add constants in `ports.go`:

```go
const (
	RoleAdmin    = ports.RoleAdmin
	RoleOperator = ports.RoleOperator
)
```

- [ ] **Step 10.2 — Add auth constructors and Operator type to `eywa.go`**

Add the import for the auth package:

```go
import (
    // existing imports ...
    "github.com/wmulabs/eywa/internal/infrastructure/driven/auth"
)
```

Add to the `type (...)` block in `eywa.go`:

```go
Operator     = entities.Operator
OperatorAuth = auth.OperatorAuth
```

Add to the `var (...)` block in `eywa.go`:

```go
NewAPIKeyValidator   = auth.NewAPIKeyValidator
NewOperatorAuth      = auth.NewOperatorAuth
NewJWTValidator      = auth.NewJWTValidator
NewJWTValidatorRS256 = auth.NewJWTValidatorRS256
NewJWKSValidator     = auth.NewJWKSValidator
HashPassword         = auth.HashPassword
```

- [ ] **Step 10.3 — Build root module**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 10.4 — Commit**

```bash
git add ports.go eywa.go
git commit -m "feat(auth): re-export auth types and constructors from root package"
```

---

## Task 11: Fiber auth middleware

**Files:**
- Create: `fiber/middleware/auth.go`
- Create: `fiber/middleware/auth_test.go`

> The fiber sub-module uses `eywa.TokenValidator` and `eywa.AuthClaims` (public re-exports from root). The `internal/` packages of root are not importable from fiber.

- [ ] **Step 11.1 — Write failing tests**

```go
// fiber/middleware/auth_test.go
package middleware

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"github.com/gofiber/fiber/v2"
)

type stubValidator struct {
	claims *eywa.AuthClaims
	err    error
}

func (v *stubValidator) Validate(_ context.Context, _ string) (*eywa.AuthClaims, error) {
	return v.claims, v.err
}

func buildTestApp(validators ...eywa.TokenValidator) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(validators...))
	app.Get("/", func(c *fiber.Ctx) error {
		claims := ClaimsFromCtx(c)
		if claims == nil {
			return c.SendString("no-claims")
		}
		return c.SendString(claims.Role)
	})
	return app
}

func TestAuthMiddleware_MissingHeader_Returns401(t *testing.T) {
	app := buildTestApp(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}})
	req := httptest.NewRequest("GET", "/", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_NonBearerScheme_Returns401(t *testing.T) {
	app := buildTestApp(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401 for non-Bearer scheme, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_ValidToken_SetsClaimsAndCallsNext(t *testing.T) {
	app := buildTestApp(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "admin" {
		t.Errorf("want body 'admin', got %q", string(body))
	}
}

func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	app := buildTestApp(&stubValidator{err: fmt.Errorf("invalid")})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_FirstValidatorWins(t *testing.T) {
	failing := &stubValidator{err: fmt.Errorf("nope")}
	succeeding := &stubValidator{claims: &eywa.AuthClaims{Role: "operator"}}
	app := buildTestApp(failing, succeeding)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200 (second validator succeeds), got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "operator" {
		t.Errorf("want body 'operator', got %q", string(body))
	}
}

func TestRequireRole_AllowedRole_PassesThrough(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(&stubValidator{claims: &eywa.AuthClaims{Role: "admin"}}))
	app.Get("/admin", RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestRequireRole_InsufficientRole_Returns403(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(&stubValidator{claims: &eywa.AuthClaims{Role: "operator"}}))
	app.Get("/admin", RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("want 403, got %d", resp.StatusCode)
	}
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(AuthMiddleware(&stubValidator{claims: &eywa.AuthClaims{Role: "operator"}}))
	app.Get("/ops", RequireRole("admin", "operator"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	req := httptest.NewRequest("GET", "/ops", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, _ := app.Test(req)
	if resp.StatusCode != 200 {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 11.2 — Run tests to confirm failure**

```bash
cd fiber && go test ./middleware/... -run "TestAuth|TestRequire" -v
```

Expected: FAIL — `middleware package not found`.

- [ ] **Step 11.3 — Implement**

```go
// fiber/middleware/auth.go
package middleware

import (
	"strings"

	eywa "github.com/wmulabs/eywa"
	"github.com/gofiber/fiber/v2"
)

type authContextKey struct{}

// AuthMiddleware tries each validator in order. First success sets claims and calls Next.
// All validators must fail for the request to be rejected with 401.
func AuthMiddleware(validators ...eywa.TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		bearer := c.Get("Authorization")
		if bearer == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header",
			})
		}

		token, found := strings.CutPrefix(bearer, "Bearer ")
		if !found {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "authorization header must use Bearer scheme",
			})
		}

		for _, v := range validators {
			claims, err := v.Validate(c.Context(), token)
			if err == nil {
				c.Locals(authContextKey{}, claims)
				return c.Next()
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid or expired token",
		})
	}
}

// RequireRole rejects requests where the authenticated role is not in the allowed list.
// Must be placed after AuthMiddleware in the handler chain.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		claims := ClaimsFromCtx(c)
		if claims == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing auth claims",
			})
		}
		if _, ok := allowed[claims.Role]; !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "insufficient permissions",
			})
		}
		return c.Next()
	}
}

// ClaimsFromCtx retrieves the authenticated claims set by AuthMiddleware.
// Returns nil if the request was not authenticated.
func ClaimsFromCtx(c *fiber.Ctx) *eywa.AuthClaims {
	claims, _ := c.Locals(authContextKey{}).(*eywa.AuthClaims)
	return claims
}
```

- [ ] **Step 11.4 — Run tests to confirm pass**

```bash
cd fiber && go test ./middleware/... -v
```

Expected: all 8 PASS.

- [ ] **Step 11.5 — Build fiber module**

```bash
cd fiber && go build ./...
```

Expected: no errors.

- [ ] **Step 11.6 — Commit**

```bash
git add fiber/middleware/auth.go fiber/middleware/auth_test.go
git commit -m "feat(auth): fiber AuthMiddleware and RequireRole handlers"
```

---

## Task 12: Full build + test verification

- [ ] **Step 12.1 — Run all root module tests**

```bash
go test ./... -v
```

Expected: all PASS.

- [ ] **Step 12.2 — Run all fiber module tests**

```bash
cd fiber && go test ./... -v
```

Expected: all PASS.

- [ ] **Step 12.3 — Build everything**

```bash
go build ./... && cd fiber && go build ./...
```

Expected: no errors.

- [ ] **Step 12.4 — Final commit if any uncommitted changes**

```bash
git status
# only commit if something was missed above
```

---

## Spec Coverage Check

| Spec section | Covered by |
|---|---|
| Deferred steps in Pipeline | Task 1 |
| TypingIndicator port | Task 2 |
| TypingStartStep + TypingStopStep | Task 2 |
| Wire Typing into builder/engine | Task 3 |
| TypingIndicator root re-export | Task 4 |
| Operator entity | Task 5 |
| TokenValidator + AuthClaims + OperatorRepository ports | Task 5 |
| RoleAdmin + RoleOperator constants | Task 5 + 10 |
| APIKeyValidator (Mode 1) | Task 6 |
| JWT dependencies | Task 7 |
| OperatorAuth — Login + Validate + HashPassword | Task 8 |
| JWTValidator HS256 + RS256 (Mode 3) | Task 9 |
| JWKSValidator OIDC (Mode 3) | Task 9 |
| Root re-exports: all auth constructors | Task 10 |
| AuthMiddleware — validator chain | Task 11 |
| RequireRole — role enforcement | Task 11 |
| ClaimsFromCtx — claims extraction | Task 11 |

**Not in this plan (Phase 3+):**
- ManagementDeps struct + RegisterManagementRoutes (Phase 3+)
- VigilCheckStep + ErrSessionHeld (Phase 5)
- All repository implementations (Phase 3-7)
- All management routes (Phase 3-8)
