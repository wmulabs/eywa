# Management Layer Design — Eywa v1.1

**Date:** 2026-05-14 (revised 2026-05-16)
**Specs covered:** SPEC_00 (Auth), SPEC_03 (Config as a Service), SPEC_04 (Human Takeover), SPEC_05 (Typing Indicator), SPEC_06 (Conversations API), SPEC_07 (Observability API), + Rite (Approval Flows)

---

## Overview

This design adds a complete management layer on top of the Eywa engine. The engine core (pipeline, reasoning, actions) is unchanged. All new functionality is opt-in via builder methods and a new `RegisterManagementRoutes()` call in the fiber sub-module.

**Scope:**
- Auth: three-mode validator chain (API keys, built-in operator management, external JWT/JWKS)
- Observability API: Chronicle query + analytics (SPEC_07)
- Conversations API: Echo listing + session aggregation + operator send (SPEC_06)
- Vigil (Human Takeover): Redis-based session control with sliding TTL (SPEC_04)
- Rite (Approval Flows): MongoDB-persisted explicit authorization requests
- Typing Indicator: TypingIndicator port + deferred pipeline steps (SPEC_05)
- Config as a Service: Link CRUD in MongoDB + Redis pub/sub hot-reload (SPEC_03)

**Mythology alignment:**
All naming follows Eywa's mythology vocabulary. API responses use Eywa naming (`memory_key`, `echo`, `chronicle`, `link`, `vigil`, `rite`).

---

## New Entities

### Vigil — conversation takeover state (Redis, ephemeral)

```go
// internal/domain/entities/vigil.go
type Vigil struct {
    MemoryKey  string
    OperatorID string
    SeatSince  time.Time
    ExpiresAt  time.Time
}
```

No MongoDB collection. Redis key: `{svc}:{env}:vigil:{memory_key}`.
Value: `{operatorID}:{seatSince.Unix()}`.
TTL = configured inactivity timeout (default 30 min).
Every operator `POST echo` call resets TTL via `EXPIRE` (sliding window).
When TTL expires naturally → session auto-releases to agent mode.

### Rite — explicit authorization request (MongoDB, persistent)

```go
// internal/domain/entities/rite.go
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

type RiteStatus string

const (
    RitePending  RiteStatus = "pending"
    RiteApproved RiteStatus = "approved"
    RiteRejected RiteStatus = "rejected"
    RiteExpired  RiteStatus = "expired"
)
```

Collection: `rites`.
Indexes: `(memory_key, status)`, `(status, requested_at)`, TTL index on `expires_at`.

### Operator — management user (MongoDB, opt-in)

Only created when `WithOperatorAuth()` is used.

```go
// internal/domain/entities/operator.go
type Operator struct {
    ID           string    `bson:"_id"`
    Name         string    `bson:"name"`
    Email        string    `bson:"email"`
    Role         string    `bson:"role"`  // "admin" | "operator"
    PasswordHash string    `bson:"password_hash"`
    IsActive     bool      `bson:"is_active"`
    CreatedAt    time.Time `bson:"created_at"`
    UpdatedAt    time.Time `bson:"updated_at"`
}
```

Collection: `operators`. Index: unique on `email`.

---

## New Ports

All ports live in `internal/domain/ports/`, re-exported from root `eywa.go`.

### Auth

```go
// ports/auth.go

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

### VigilRepository

```go
// ports/vigil.go
type VigilRepository interface {
    Acquire(ctx context.Context, memoryKey, operatorID string, ttl time.Duration) error
    Release(ctx context.Context, memoryKey string) error
    Get(ctx context.Context, memoryKey string) (*entities.Vigil, error)
    Refresh(ctx context.Context, memoryKey string, ttl time.Duration) error
}
```

### RiteRepository

```go
// ports/rite.go
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

### TypingIndicator

```go
// ports/typing_indicator.go
type TypingIndicator interface {
    StartTyping(ctx context.Context, phone string) error
    StopTyping(ctx context.Context, phone string) error
}
```

Nil-safe in pipeline. Errors are logged and swallowed — typing never blocks delivery.
Implementations live in channel sub-modules (`channels/whatsapp/dialog360`, `channels/whatsapp/twilio`).

### LinkConfigRepository + ConfigPublisher (SPEC_03)

```go
// ports/link_config.go
type LinkConfigRepository interface {
    FindAll(ctx context.Context) ([]*entities.Link, error)
    FindByKey(ctx context.Context, key string) (*entities.Link, error)
    Save(ctx context.Context, link *entities.Link) error
    Delete(ctx context.Context, key string) error
}

type ConfigPublisher interface {
    Publish(ctx context.Context, channel, message string) error
    Subscribe(ctx context.Context, channel string, handler func(msg string)) error
}
```

`Subscribe` is blocking by design — `LinkCache.Subscribe()` runs it in a goroutine, bound to the builder's `ctx` for lifecycle management.

### ChronicleQueryRepository — separate query interface (SPEC_07)

New interface — does **not** extend `ChronicleRepository`. Avoids breaking existing implementations.

```go
// ports/chronicle_query.go

type ChronicleListOptions struct {
    SpiritName    string
    MemoryKey     string
    HasError      bool
    MinIterations int
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
    AggregateTokens(ctx context.Context, spiritName string, from, to time.Time, granularity string) ([]TokenSeries, error)
    AggregateActions(ctx context.Context, spiritName string, from, to time.Time) ([]ActionStats, error)
    AggregateSpirits(ctx context.Context, from, to time.Time) ([]SpiritStats, error)
}
```

Read-model types live in `ports/` (not `entities/`) — they are query projections, not domain state.

The existing `mongo.ChronicleRepository` struct implements both `ChronicleRepository` (write side) and `ChronicleQueryRepository` (read side). Users who don't need management features never reference `ChronicleQueryRepository`.

### EchoQueryRepository — separate query interface (SPEC_06)

New interface — does **not** extend `EchoRepository`.

```go
// ports/echo_query.go

type SessionListOptions struct {
    SpiritName string
    DateFrom   *time.Time
    DateTo     *time.Time
    Page       int
    Limit      int
}

type SessionSummary struct {
    MemoryKey      string
    LastSpiritName string
    MessageCount   int64
    LastMessageAt  time.Time
}

type EchoQueryRepository interface {
    ListSessions(ctx context.Context, opts SessionListOptions) ([]*SessionSummary, int64, error)
    FindByMemoryKeyBefore(ctx context.Context, memoryKey, beforeID string, limit int) ([]*entities.Echo, error)
}
```

`ListSessions` → `$group` aggregation over `echoes` by `memory_key`. No new collection.
`FindByMemoryKeyBefore` → cursor-based pagination for the echo list endpoint.

The existing `mongo.EchoRepository` struct gains these methods and satisfies both interfaces.

---

## Auth: Three-Mode Validator Chain

All three modes implement `TokenValidator`. Builder accepts multiple — chain tries each in order, first success wins.

### Mode 1 — API Keys (zero-infra, for getting started)

```go
// internal/infrastructure/driven/auth/apikey_validator.go
// re-exported as eywa.NewAPIKeyValidator()

func NewAPIKeyValidator(keys map[string]string) ports.TokenValidator
// keys = map[apiKey]role
// e.g. {"sk-admin-xxx": "admin", "sk-operator-yyy": "operator"}
```

No DB. Keys configured at startup via builder. Role determined by key registration.
Token passed as `Authorization: Bearer <key>`.

### Mode 2 — Built-in Operator Management (cockpit standalone)

```go
// re-exported as eywa.NewOperatorAuth()

func NewOperatorAuth(repo ports.OperatorRepository, secret []byte) *OperatorAuth
// implements TokenValidator (validates issued JWTs)
// when registered in ManagementDeps, enables additional routes:
//   POST /api/v1/auth/token               — login, issues JWT
//   GET  /api/v1/operators                — list operators (admin)
//   POST /api/v1/operators                — create operator (admin)
//   PUT  /api/v1/operators/:id            — update operator (admin)
//   POST /api/v1/operators/:id/deactivate — deactivate (admin)
```

Issues HS256 JWT with configurable TTL (default 8h). Passwords hashed with bcrypt.

### Mode 3 — External JWT / JWKS (Auth0, Firebase, Google IAP, internal)

```go
// re-exported from root package

func NewJWTValidator(secret []byte) ports.TokenValidator           // HS256
func NewJWTValidatorRS256(pubKey *rsa.PublicKey) ports.TokenValidator   // RS256
func NewJWKSValidator(jwksURL, audience string) ports.TokenValidator    // OIDC/Auth0/Firebase
```

`NewJWKSValidator` fetches JWKS from the provider, caches keys, validates RS256 signatures.
Claims mapping: standard `sub` → `Subject`, custom `role` claim → `Role`.

### Validator chain — in ManagementDeps

Auth validators live only in `ManagementDeps` (fiber layer). Engine builder has no auth methods.

```go
deps := eywafiber.ManagementDeps{
    APIKeys:        map[string]string{"sk-admin-xxx": "admin"},
    OperatorAuth:   eywa.NewOperatorAuth(mongo.NewOperatorRepository(db), secret),
    TokenValidator: eywa.NewJWKSValidator(jwksURL, audience),
    // ... other deps
}
eywafiber.RegisterManagementRoutes(app, weave, deps)
```

---

## Pipeline Changes

### Deferred Steps

`Pipeline` gains support for steps that always execute after the main step sequence, regardless of success or failure:

```go
// internal/implementation/orchestrator/pipeline.go

type Pipeline struct {
    steps         []ProcessingStep
    deferredSteps []ProcessingStep
    logger        *zap.SugaredLogger
    tracer        trace.Tracer
}

func (p *Pipeline) AddDeferredStep(step ProcessingStep) *Pipeline

func (p *Pipeline) Execute(ctx context.Context, state *ProcessingState) (returnErr error) {
    defer func() {
        for _, step := range p.deferredSteps {
            if err := p.executeStep(ctx, step, state); err != nil {
                p.logger.Warnw("deferred step failed", "step", step.Name(), "error", err)
                // never overwrites returnErr
            }
        }
    }()
    // existing step loop unchanged
}
```

> **Note:** Lock release is NOT a pipeline step. `processEventWithConfig` releases the lock directly after `pipeline.Execute()` returns (lines 405–409 of engine.go), covering both success and failure cases. The deferred step mechanism is specifically for `TypingStopStep`.

### New pipeline step: VigilCheckStep

A dedicated step inserted immediately after `LockAcquisitionStep`. Responsible for a single concern: checking whether an operator holds the session.

```go
// internal/implementation/orchestrator/pipeline_step_vigil.go

type VigilCheckStep struct {
    vigilRepo ports.VigilRepository
    logger    *zap.SugaredLogger
}

func (s *VigilCheckStep) Name() string           { return "VigilCheck" }
func (s *VigilCheckStep) Timeout() time.Duration { return 3 * time.Second }

func (s *VigilCheckStep) Execute(ctx context.Context, state *ProcessingState) error {
    if s.vigilRepo == nil {
        return nil
    }
    vigil, err := s.vigilRepo.Get(ctx, state.Event.MemoryKey)
    if err != nil {
        s.logger.Warnw("vigil check failed, allowing processing", "error", err)
        return nil // fail-open: infra error must not block the user
    }
    if vigil != nil {
        state.ProcessingStatus = "session_held"
        return ErrSessionHeld
    }
    return nil
}
```

`ErrSessionHeld` is non-retriable. `processEventWithConfig` calls `logInteraction` on pipeline failure — `statusFromError` maps `SESSION_HELD` → `"session_held"` in the Chronicle.

New error added to `orchestrator/errors.go`:

```go
var ErrSessionHeld = &OrchestrationError{
    Code:      "SESSION_HELD",
    Message:   "session held by operator vigil",
    Retriable: false,
}

func IsSessionHeld(err error) bool {
    var oe *OrchestrationError
    return errors.As(err, &oe) && oe.Code == "SESSION_HELD"
}
```

`statusFromError` gains the case:
```go
case "SESSION_HELD":
    return "session_held"
```

### TypingStartStep + TypingStopStep

`TypingStartStep` — normal step:

```go
func (s *TypingStartStep) Execute(ctx context.Context, state *ProcessingState) error {
    if s.indicator == nil || state.Event.ContactPhone == "" {
        return nil
    }
    if err := s.indicator.StartTyping(ctx, state.Event.ContactPhone); err != nil {
        s.logger.Warnw("typing start failed", "error", err)
    }
    return nil // never fails the pipeline
}
func (s *TypingStartStep) Timeout() time.Duration { return 3 * time.Second }
```

`TypingStopStep` — **deferred step**, registered via `AddDeferredStep()`. Guarantees `StopTyping` even on pipeline failure.

Both registered automatically when `WithTypingIndicator()` is configured.

### Updated pipeline order (reflects actual `buildProcessingPipeline`)

```
1.  ValidationStep
2.  RateLimitStep          (if rateLimiter configured)
3.  LockAcquisitionStep
4.  VigilCheckStep         ← new (no-op if vigilRepo nil)
5.  TypingStartStep        ← new (no-op if indicator nil)
6.  RitualMarkStep         (if scheduledTaskManager configured)
7.  ScoutStep
8.  FilterStep
9.  SpiritSelectionStep
10. SpiritLoadStep
11. SpiritScoutStep
12. OrchestratorRoutingStep
13. MediaVaultStep          (if mediaStore configured)
14. MediaProcessingStep     (if mediaProcessor configured)
15. SessionSetupStep
16. ArchivistStep           (if archivist configured)
17. MessageCoalescingStep
18. CostEnforcementStep     (if ledgerRepo configured)
19. ModelRoutingStep        (if modelRoutingRules configured)
20. ConditionEvaluationStep
21. ReasoningStep
22. NotificationStep
23. PersistenceStep
24. ResponseDeliveryStep
25. AuditLogStep
26. ImprintExtractionStep   (if imprintRepository configured)
27. LedgerUpdateStep        (if ledgerRepo configured)
28. MarkExecutedStep        (if scheduledTaskManager configured)
--- deferred ---
D1. TypingStopStep          ← new deferred (no-op if indicator nil)
```

---

## Management Layer (fiber sub-module)

### Auth Middleware

```go
// fiber/middleware/auth.go

func AuthMiddleware(validators ...ports.TokenValidator) fiber.Handler
func RequireRole(roles ...string) fiber.Handler
func ClaimsFromCtx(c *fiber.Ctx) *ports.AuthClaims
```

`AuthMiddleware` tries each validator in order. First success wins.

### ManagementDeps

```go
// fiber/management.go

type ManagementDeps struct {
    // Auth — at least one required for management routes
    APIKeys        map[string]string          // Mode 1
    OperatorAuth   *eywa.OperatorAuth         // Mode 2 (also enables /auth/token + /operators)
    TokenValidator ports.TokenValidator       // Mode 3

    // Repositories
    ChronicleQueryRepo ports.ChronicleQueryRepository
    EchoRepo           ports.EchoRepository
    EchoQueryRepo      ports.EchoQueryRepository
    VigilRepo          ports.VigilRepository
    RiteRepo           ports.RiteRepository
    VoiceRegistry      ports.VoiceRegistry    // for operator send-message via Voice

    // Optional — SPEC_03
    LinkConfigRepo  ports.LinkConfigRepository
    ConfigPublisher ports.ConfigPublisher

    // Vigil config
    VigilConfig VigilConfig
}

type VigilConfig struct {
    InactivityTimeout time.Duration // default: 30min
}

func RegisterManagementRoutes(app *fiber.App, weave eywa.Weave, deps ManagementDeps)
```

`VigilConfig` is also used in the engine builder (`WithVigilConfig`). It is defined once in the root `eywa` package and re-used in `ManagementDeps`.

### Routes

```
── AUTH (when OperatorAuth configured) ── public
POST /api/v1/auth/token                   login: { "email", "password" } → { "token", "expires_at" }

── OPERATORS (when OperatorAuth configured) ── admin
GET  /api/v1/operators
POST /api/v1/operators
PUT  /api/v1/operators/:id
POST /api/v1/operators/:id/deactivate

── OBSERVABILITY (Chronicle) ── operator+
GET  /api/v1/chronicle                    ?spirit_name, memory_key, has_error, min_iterations, date_from, date_to, page, limit
GET  /api/v1/chronicle/:id
GET  /api/v1/analytics/tokens             ?spirit_name, date_from, date_to, granularity (day|week|month)
GET  /api/v1/analytics/actions            ?spirit_name, date_from, date_to
GET  /api/v1/analytics/spirits            ?date_from, date_to

── CONVERSATIONS (Echo) ── operator+
GET  /api/v1/echoes/sessions              ?spirit_name, date_from, date_to, page, limit
GET  /api/v1/echoes/sessions/:memoryKey   detail: last_spirit, message_count, vigil status
GET  /api/v1/echoes                       ?memory_key (required), before_id (cursor), limit

── VIGIL (Human Takeover) ── operator+
POST   /api/v1/vigil/:memoryKey           take-seat:     { "operator_id": "..." }
DELETE /api/v1/vigil/:memoryKey           release-seat
GET    /api/v1/vigil/:memoryKey           status:        { operator_id, seat_since, expires_at, held_for_seconds }
POST   /api/v1/vigil/:memoryKey/echoes    send-message:  { "content": "...", "deliver": true|false }
                                          ↳ saves Echo role="operator", Refresh() TTL, delivers via Voice if deliver=true

── RITE (Approval Flows) ── operator+
GET  /api/v1/rites                        ?memory_key, status, page, limit
GET  /api/v1/rites/:id
POST /api/v1/rites/:id/approve            { "operator_id": "...", "note": "..." }
POST /api/v1/rites/:id/reject             { "operator_id": "...", "note": "..." }
                                          ↳ Decide() → fires new Pulse with Knowledge["rite_id"]+["rite_status"]

── CONFIG (Links) ── admin only
GET    /api/v1/links
POST   /api/v1/links
GET    /api/v1/links/:key
PUT    /api/v1/links/:key
DELETE /api/v1/links/:key
POST   /api/v1/links/reload               force hot-reload via Redis pub/sub
```

### Role matrix

| Route group | Minimum role |
|---|---|
| Chronicle + Analytics | `operator` |
| Echoes + Sessions | `operator` |
| Vigil (take/release/send) | `operator` |
| Rite (list/approve/reject) | `operator` |
| Operators CRUD | `admin` |
| Links CRUD + reload | `admin` |

### Rite → new Pulse on decision

When operator approves or rejects, handler parses `rite.MemoryKey` (format: `"{channel}:{user}"`)
back into channel + user components to construct the new Pulse:

```go
// rite.MemoryKey = "whatsapp:+5511999999999"
parts := strings.SplitN(rite.MemoryKey, ":", 2)
memKey := eywa.MemoryKey{Channel: parts[0], User: parts[1]}

pulse := eywa.NewPulse(memKey).
    WithEventType(rite.EventKey).
    AddKnowledge("rite_id", rite.ID).
    AddKnowledge("rite_status", string(newStatus)).
    AddKnowledge("rite_context", rite.Context).
    AddMetadata("triggered_by", "rite_decision").
    Build()

weave.ProcessEventByKey(ctx, rite.EventKey, pulse)
```

Spirit sees `rite_status` in Knowledge and acts accordingly.

---

## New Repositories

### mongo sub-module

```go
mongo.NewRiteRepository(db)        // collection: "rites"
mongo.NewLinkConfigRepository(db)  // collection: "links"
mongo.NewOperatorRepository(db)    // collection: "operators"

// Existing mongo.ChronicleRepository gains: List(), AggregateTokens(), AggregateActions(), AggregateSpirits()
// Satisfies both ChronicleRepository (write) and ChronicleQueryRepository (read).

// Existing mongo.EchoRepository gains: ListSessions(), FindByMemoryKeyBefore()
// Satisfies both EchoRepository (write/read) and EchoQueryRepository (management queries).
```

### redis sub-module

```go
redis.NewVigilRepository(client, service, env)
// key: {service}:{env}:vigil:{memory_key}
// Acquire → SET NX EX
// Refresh → EXPIRE (sliding TTL reset)
// Release → DEL
// Get     → GET + parse
```

### Config as a Service — LinkCache

```go
// internal/implementation/orchestrator/link_cache.go
type LinkCache struct {
    mu        sync.RWMutex
    links     map[string]*entities.Link
    repo      ports.LinkConfigRepository
    publisher ports.ConfigPublisher
    logger    *zap.SugaredLogger
}

func (c *LinkCache) GetLink(key string) (*entities.Link, bool)
func (c *LinkCache) LoadAll(ctx context.Context) error   // called in Build()
func (c *LinkCache) Subscribe(ctx context.Context)       // goroutine started in Build(), bound to builder ctx
func (c *LinkCache) Invalidate(ctx context.Context, key string) error
```

`Weave.RegisterEventConfiguration()` stays for backward compat (writes in-memory).
When `LinkCache` is configured, it takes precedence over static registrations on lookup.

---

## Builder Changes

All new methods are optional. Auth validators are NOT on the builder — the engine pipeline never validates tokens. Auth lives entirely in `ManagementDeps` (fiber layer).

```go
// Human Takeover — engine needs VigilRepository to construct VigilCheckStep
WithVigilRepository(repo ports.VigilRepository) *WeaveBuilder
WithVigilConfig(cfg VigilConfig) *WeaveBuilder  // VigilConfig defined in root eywa package

// Rite
WithRiteRepository(repo ports.RiteRepository) *WeaveBuilder

// Typing
WithTypingIndicator(ti ports.TypingIndicator) *WeaveBuilder

// Config as a Service
WithLinkConfigRepository(repo ports.LinkConfigRepository) *WeaveBuilder
WithConfigPublisher(pub ports.ConfigPublisher) *WeaveBuilder
```

`Build()` validation: if `LinkConfigRepository` present but `ConfigPublisher` absent → log warning, CRUD works but hot-reload disabled.

---

## Built-in Action: request_rite

New built-in action registered optionally:

```go
actionRegistry.Register(eywa.NewRequestRiteAction(riteRepo))
// tool name: "request_rite"
// requires: RiteRepository
```

**Parameters:**

```go
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
```

`event_key` is explicit in the schema: the Spirit (LLM) declares which event should fire when the operator decides. This gives each Spirit full control over its approval resume flow.

`MemoryKey` and `SubjectKey` are read from `ctx.Value(ports.SessionContextKey{})` (same pattern as `schedule_ritual`).

Action creates Rite with `status=pending`. Returns to Spirit: `{"rite_id":"...","status":"pending"}`.
Spirit ends its turn — pipeline completes normally. Resumes when operator calls approve/reject.

---

## Implementation Order

| Phase | Spec | Delivers | Depends on |
|---|---|---|---|
| 1 | SPEC_05 | Typing Indicator: port + deferred pipeline + steps | — |
| 2 | SPEC_00 | Auth: ports + 3 validators + fiber middleware | — |
| 3 | SPEC_07 | Observability API: ChronicleQueryRepository + mongo impl + analytics routes | Phase 2 |
| 4 | SPEC_06 | Conversations API: EchoQueryRepository + mongo impl + session routes | Phase 2 |
| 5 | SPEC_04 | Vigil: Redis repo + VigilCheckStep + take/release/send routes | Phase 2, 4 |
| 6 | Rite | Rite: entity + Mongo repo + request_rite Action + approve/reject routes + Pulse trigger | Phase 2, 5 |
| 7 | SPEC_03 | Config as a Service: LinkCache + Mongo repo + Redis pub/sub + Link CRUD routes | Phase 2 |
| 8 | — | Integration: RegisterManagementRoutes unified, root re-exports, integration tests | All |

Each phase is independently shippable. Management routes per phase are registered incrementally in `RegisterManagementRoutes` as deps become available.
