# 🔧 WeaveBuilder Reference

`eywa.NewWeaveBuilder(ctx)` is the single entry point for assembling a Weave. Call `.Build()` when all options are set — it validates required fields and wires everything together.

---

## Quick Reference

| Tier | Options | When to use |
|---|---|---|
| **Required** | `WithRepositories`, `WithBond`, one Oracle (`AddOracle`) | Every application |
| **Common** | `WithActionRegistry`, `WithVoiceRegistry`, `WithScoutRegistry`, `WithLogger` | Most applications |
| **Human-in-the-Loop** | `WithVigilRepository`, `WithRiteRepository` | Operator takeover + approval flows |
| **Memory & Knowledge** | `WithLoreRepository + WithLoreStore`, `WithImprintRepository`, `WithArchivist` | Long conversations, RAG |
| **Cost Control** | `WithLedgerRepository`, `WithRateLimiter` | Production hardening |
| **Guardrails** | `WithInputGuard`, `WithOutputGuard` | Prompt-injection/jailbreak block, PII redaction, output denylist |
| **Scheduling** | `WithRitualManager` + Keeper | Recurring events, delayed messages |
| **External Tools** | `WithConduit` | MCP protocol integrations |
| **Routing** | `WithPathfinderRegistry`, `WithDefaultLLMPathfinder` | Multi-Spirit routing |
| **Observability** | `WithTracer`, `WithAppInfo` | OpenTelemetry tracing |

**Minimal viable configuration** (single Spirit, no infrastructure except Redis):

```go
weave, err := eywa.NewWeaveBuilder(ctx).
    WithRepositories(repos).            // MongoDB repositories
    WithBond(bond).                     // redis.NewBondManager() or eywa.NewNoOpBond()
    AddOracle(oracle).                  // any Oracle provider
    Build()
```

Add `WithActionRegistry`, `WithVoiceRegistry`, and `WithLogger` next. Everything else is opt-in.

---

## 🔴 Required

> [!WARNING]
> These must be set or `Build()` returns an error.

### WithRepositories

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
    Build()
```

Sets all four repositories at once. Alternatively, set them individually:

```go
builder.WithSpiritRepository(spiritRepo).
        WithMemoryRepository(memoryRepo).
        WithEchoRepository(echoRepo).
        WithChronicleRepository(chronicleRepo)
```

| Repository | Interface | Purpose |
|-----------|-----------|---------|
| `SpiritRepository` | `ports.SpiritRepository` | Load/store Spirit configs (versioned) |
| `MemoryRepository` | `ports.MemoryRepository` | Read/write ephemeral Memory in Redis |
| `EchoRepository` | `ports.EchoRepository` | Persist conversation messages (Threads) |
| `ChronicleRepository` | `ports.ChronicleRepository` | Write interaction audit logs |

### WithBond

```go
builder.WithBond(eywaredis.NewBondManager(redisClient))
```

Distributed lock. Required to prevent concurrent Pulses for the same MemoryKey from corrupting Memory.

### 🔮 Oracle Provider (at least one)

```go
builder.AddAnthropic(os.Getenv("ANTHROPIC_API_KEY"))
builder.AddOpenAI(os.Getenv("OPENAI_API_KEY"))
builder.AddGemini(os.Getenv("GEMINI_API_KEY"))
builder.AddOracle(myCustomOracle) // implement ports.Oracle
```

> [!TIP]
> Multiple providers can be registered simultaneously — the OracleFactory routes based on `Spirit.ModelConfig.Provider`. Use different models for different Spirits (e.g. Sonnet for reasoning, Haiku for routing).

---

## 🟡 Optional: Registries

### WithActionRegistry

```go
actionRegistry := eywa.NewActionRegistry()
actionRegistry.Register(&TrackOrderAction{})
actionRegistry.Register(eywa.NewUpdateSubjectAction())
actionRegistry.Register(eywa.NewScheduleRitualAction())

builder.WithActionRegistry(actionRegistry)
```

Actions are the tools the Oracle can call. Register all Actions before building. Each Spirit's `AllowedActions` list controls which Actions it can invoke.

**Built-in Actions:**

| Action | Name | Description |
|--------|------|-------------|
| `NewUpdateSubjectAction()` | `update_subject` | Sets/updates the active SubjectKey in Memory, accumulates subject facts |
| `NewScheduleRitualAction()` | `schedule_ritual` | Schedules a future Pulse via the Keeper |
| `NewListRitualsAction()` | `list_rituals` | Lists pending Rituals for the current MemoryKey |
| `NewCancelRitualAction()` | `cancel_ritual` | Cancels a pending Ritual |

### WithScoutRegistry

```go
scoutRegistry := eywa.NewScoutRegistry()
scoutRegistry.Register(&UserProfileScout{})
scoutRegistry.Register(&OrderContextScout{})

builder.WithScoutRegistry(scoutRegistry)
```

Scouts run before Spirit selection and enrich the Pulse's `Knowledge` map. They run concurrently within the `ScoutTimeout`. Scout failures are logged but never abort the pipeline.

### WithPathfinderRegistry

```go
pathfinderRegistry := eywa.NewPathfinderRegistry()
pathfinderRegistry.Register(&TierPathfinder{})

builder.WithPathfinderRegistry(pathfinderRegistry)
```

Pathfinders select the Spirit when a Link has multiple `AllowedSpirits`. Only used when `Link.HasMultipleSpirits()` is true.

### WithVoiceRegistry

```go
voiceRegistry := eywa.NewVoiceRegistry()
voiceRegistry.Register(whatsapp.NewWhatsAppResponseChannel(client))
voiceRegistry.Register(eywa.NewHTTPVoice())

builder.WithVoiceRegistry(voiceRegistry)
```

Voices handle automatic response delivery after the Oracle finishes. A Link's `VoiceName` field selects which Voice is used. If a Spirit has already sent a response via a delivery Action (`IsCritical: true`), the Voice step is skipped.

---

## 🟢 Optional: Advanced Features

### WithRateLimiter

```go
builder.WithRateLimiter(eywaredis.NewRedisRateLimiter(redisClient, 10, time.Minute))
```

Per-MemoryKey rate limiting. When a Pulse exceeds the limit, the pipeline fails at `RateLimit` step and the Chronicle records `rate_limited`.

> [!NOTE]
> On Redis failure, the rate limiter **fails open** (allows the Pulse through) to avoid blocking traffic on infrastructure issues.

### WithInputGuard

```go
builder.WithInputGuard(eywa.GuardConfig{
    PromptInjectionDetection: true,
    MaxLineCount: 50,
})
```

Content-level validation applied before any processing. `PromptInjectionDetection` rejects messages matching known injection patterns.

### WithOutputGuard

```go
builder.WithOutputGuard(eywa.OutputGuardConfig{
    RedactPII:       true,
    PIIKinds:        []eywa.PIIKind{eywa.PIIEmail, eywa.PIICreditCard, eywa.PIIPhone},
    BlockedPatterns: []string{`(?i)\bssn\b`},
})
```

Sanitizes the final response before it is persisted, delivered, and audited: PII redaction plus a denylist that replaces matched responses wholesale. Disabled by default. See [Guardrails](guardrails.md) for coverage notes on streamed and notifier turns.

### WithMessageInbox + WithInboxMinWindow

```go
builder.
    WithMessageInbox(eywaredis.NewRedisMessageInbox(redisClient, "myapp")).
    WithInboxMinWindow(3 * time.Second)
```

Enables message coalescing for WhatsApp-style multi-message conversations. When a MemoryKey is locked, arriving messages are buffered in the Inbox. The active pipeline drains and merges them before reasoning.

> [!TIP]
> `WithInboxMinWindow` sets the minimum accumulation window. Pipeline steps (Enrichment, SessionSetup, etc.) count toward the window. **Recommendation: 3–5 seconds for WhatsApp.**

### WithAsyncDispatch

```go
builder.WithAsyncDispatch(cloudtasksKeeper)
```

Enables the `POST /api/v1/events/:event_key/async` endpoint. Pulses are dispatched to the Keeper immediately (for background processing) instead of processed inline. The webhook response returns in < 100ms.

### WithRitualManager

```go
ritualRepo := eywamongo.NewRitualRepository(db)
ritualService := services.NewRitualService(ritualRepo, keeper)

builder.WithRitualManager(ritualService)
```

Enables the `/api/v1/schedule` REST endpoints and built-in Ritual Actions. Requires a Keeper (e.g. Cloud Tasks) for Ritual execution delivery.

### WithMediaStore + WithMediaProcessor

```go
vault, _ := gcs.NewGCSVault(ctx, "my-media-bucket")
lens := gcs.NewGeminiLens(
    gcs.NewGeminiImageAnalyzer(apiKey, ""),
    gcs.NewGeminiAudioTranscriber(apiKey, ""),
    gcs.NewGeminiDocumentExtractor(apiKey, ""),
)

builder.
    WithMediaStore(vault).
    WithMediaProcessor(lens)
```

When configured, the `MediaVault` pipeline step:
1. Processes Attachments (transcribes audio, analyzes images, extracts document text) using the Lens
2. Uploads raw media bytes to the Vault (GCS)
3. Adds transcriptions/descriptions to `Pulse.Knowledge` so the Oracle can reason over them

### WithVigilRepository + WithVigilConfig

```go
builder.
    WithVigilRepository(eywaredis.NewVigilRepository(client, "myapp", "prod")).
    WithVigilConfig(eywa.VigilConfig{InactivityTimeout: 30 * time.Minute})
```

Enables human takeover (Vigil). When a Vigil seat is active for a MemoryKey, `ProcessEventByKey` returns `ErrSessionHeld` instead of processing the Pulse. The management API exposes seat acquisition, message injection, and release endpoints.

### WithRiteRepository

```go
builder.WithRiteRepository(eywamongo.NewRiteRepository(db))
```

Enables the `request_rite` built-in Action. A Spirit that calls `request_rite` pauses execution and waits for an operator decision. The Rite is stored in MongoDB; the management API exposes `/rites/:id/approve` and `/rites/:id/reject`.

### WithImprintRepository + WithImprintExtraction

```go
builder.
    WithImprintRepository(eywamongo.NewImprintRepository(db)).
    WithImprintExtraction(eywa.ImprintExtractionConfig{
        Enabled:    true,
        MaxFacts:   50,
        Categories: []string{"preference", "personal", "goal"},
    })
```

Enables long-term user memory. When `Enabled` is true, facts are automatically extracted from user messages and stored as Imprints. Spirits also gain `remember_fact` and `forget_fact` Actions.

Injected as context on every subsequent conversation by the same user — the Oracle sees their preferences without the user repeating them.

### WithLoreRepository + WithLoreEmbedder + WithLoreStore

```go
builder.
    WithLoreRepository(eywamongo.NewLoreRepository(db)).    // stores Lore metadata
    WithLoreEmbedder(myEmbedder).                           // implements LoreEmbedder port
    WithLoreStore(myVectorStore)                            // optional: semantic search backend
```

Enables RAG. Spirits with `search_lore` in `AllowedActions` can search the knowledge base at query time. `LoreEmbedder` converts text to vectors; `LoreStore` provides the vector search backend (pgvector, Qdrant, Pinecone, Weaviate — see `providers/`). Without `WithLoreStore`, MongoDB full-text search is used.

### WithLedgerRepository

```go
builder.WithLedgerRepository(eywamongo.NewLedgerRepository(db))
```

Enables token usage tracking and budget enforcement. Spirits get per-month token budgets with configurable behavior on exceed (`"block"`, `"downgrade"`, `"alert"`). Usage is recorded in the Chronicle automatically.

### WithConduit

```go
import eywamcp "github.com/wmulabs/eywa/mcp"

conduit := eywamcp.NewConduit(eywamcp.ConduitConfig{
    Name:      "my_tools",
    Transport: "http",
    URL:       "http://localhost:3001",
})

builder.WithConduit(conduit)
```

Connects to a Model Context Protocol server at build time. Tools are auto-discovered and registered as Actions with the prefix `{conduit_name}__{tool_name}`. Multiple Conduits can be attached.

### WithLogicRouterRegistry

```go
registry := eywa.NewLogicRouterRegistry()
registry.Register(myRouter) // implements ports.LogicRouter

builder.WithLogicRouterRegistry(registry)
```

Registers custom logic routers. Logic routers are referenced by name in Spirit configurations for multi-agent orchestration chains.

---

## 🧭 Optional: Routing Intelligence

### WithDefaultLLMPathfinder

```go
builder.WithDefaultLLMPathfinder("anthropic", "claude-haiku-4-5-20251001", 0.1)
```

Registers a built-in LLM Pathfinder that uses a cheap model to classify Pulse intent and select the best Spirit from the Link's `AllowedSpirits`. Automatically activated when a Link has multiple spirits and no explicit Pathfinder is specified.

> [!TIP]
> Use a **fast, cheap model** for routing — `claude-haiku-4-5-20251001` or `gpt-4o-mini`. Routing doesn't need deep reasoning. Low temperature (0.1–0.2) for consistent decisions.

- **provider**: Oracle provider name (must be registered via `AddOracle` / `AddAnthropic` etc.)
- **model**: Use a fast, cheap model — routing doesn't need deep reasoning
- **temperature**: Default `0.2` — low for consistent routing decisions

### WithPathfinderRegistry (custom Pathfinder)

Implement `ports.Pathfinder` and register it:

```go
type PriorityPathfinder struct{}

func (p *PriorityPathfinder) GetName() string { return "priority_pathfinder" }

func (p *PriorityPathfinder) SelectSpirit(ctx context.Context, pulse *eywa.Pulse, available []string) string {
    if tier, _ := pulse.Knowledge["user_tier"].(string); tier == "VIP" {
        for _, name := range available {
            if name == "premium_spirit" {
                return name
            }
        }
    }
    return available[0]
}
```

---

## 🗂️ Optional: Memory & Archiving

### WithMemoryReconstruction

```go
builder.WithMemoryReconstruction(true, 20)
```

Controls automatic Memory reconstruction from Echo when Redis TTL expires.

- **enabled**: Default `true`
- **limit**: Number of recent messages to load (default `10`)

### WithDefaultLLMArchivist

```go
builder.
    WithDefaultLLMArchivist("anthropic", "claude-haiku-4-5-20251001", 20).
    WithArchivistConfig(0.1, 512)
```

Enables automatic conversation summarization. When `Threads` depth reaches `threshold`, the Archivist summarizes the oldest half and stores it in `Memory.Summary`. Subsequent Oracle calls receive the summary instead of raw history.

> [!TIP]
> Use a **cheap model** for summarization — the Archivist doesn't need creativity, just compression. Low temperature (0.1) and capped tokens (512) keep costs minimal.

- **provider / model**: Use a fast, cheap model
- **threshold**: Message count trigger (minimum 2)
- `WithArchivistConfig(temperature, maxTokens)` — optional fine-tuning

### WithArchivistKeepRecent

```go
builder.
    WithDefaultLLMArchivist("anthropic", "claude-haiku-4-5-20251001", 20).
    WithArchivistKeepRecent(8)
```

After summarization, keep the `N` most recent messages verbatim. Default: `threshold / 2`. The rest is summarized.

### WithArchivist (custom implementation)

```go
archivist := eywa.NewArchivist("anthropic", "claude-haiku-4-5-20251001", oracleFactory)
builder.WithArchivist(archivist, 20)
```

Use when you need a custom `Archivist` implementation or already have an `OracleFactory` built.

---

## 📊 Optional: Observability & Config

### WithLogger

```go
logger, _ := zap.NewProduction()
builder.WithLogger(logger.Sugar().Named("myapp"))
```

Default: Eywa creates a production zap logger with ISO8601 timestamps. The logger is shared across all pipeline steps and Actions via `eywa.GetLogger()`.

### WithTracer

```go
builder.WithTracer(otel.GetTracerProvider().Tracer("myapp"))
```

Default: no-op tracer. Wire in your OpenTelemetry tracer provider before calling `Build()` (or after, via `otel.SetTracerProvider`).

### WithAppInfo

```go
builder.WithAppInfo("myapp", "v1.2.3")
```

Sets service name and version for `/health` and `/ready` endpoints.

### WithConfig

```go
config := eywa.DefaultWeaveConfig()
config.ReasoningTimeout = 90 * time.Second
config.MaxReasoningIterations = 15
config.ParallelActionExecution = true

builder.WithConfig(config)
```

Applies a full `WeaveConfig`. Shorthand setters are also available:

| Setter | Default | Description |
|--------|---------|-------------|
| `WithLockTTL(d)` | `30s` | Bond TTL per Pulse |
| `WithLockAcquireTimeout(d)` | `10s` | Max wait to acquire Bond |
| `WithScoutTimeout(d)` | `15s` | Concurrent Scout execution budget |
| `WithReasoningTimeout(d)` | `60s` | Oracle loop total time budget |
| `WithSpiritSelectionTimeout(d)` | `10s` | Pathfinder execution budget |
| `WithMaxIterations(n)` | `10` | Max Oracle loop iterations |
| `WithMaxMemoryMessages(n)` | `50` | Max Threads before trimming |
| `WithParallelActionExecution(bool)` | `false` | Run Actions concurrently per iteration |
| `WithActionRetry(attempts, delay)` | `1, 0` | Retry InfrastructureErrors from Actions |
| `WithMaxIterationsMessage(msg)` | built-in | Message when max iterations reached |
| `WithValidationLimits(keyLen, msgLen, ctxLen)` | `200, 4096, 32768` | Input size caps |

---

## ⚙️ WeaveConfig Defaults

```go
func DefaultWeaveConfig() *WeaveConfig {
    return &WeaveConfig{
        LockTTL:                    30 * time.Second,
        LockAcquireTimeout:         10 * time.Second,
        ScoutTimeout:               15 * time.Second,
        ReasoningTimeout:           60 * time.Second,
        SpiritSelectionTimeout:     10 * time.Second,
        MaxReasoningIterations:     10,
        MaxMemoryMessages:          50,
        MaxSessionKeyLength:        200,
        MaxUserMessageLength:       4096,
        MaxEventContextSize:        32768,
        EnableMemoryReconstruction: true,
        MemoryReconstructionLimit:  10,
        ParallelActionExecution:    false,
        ActionRetryMaxAttempts:     1,
    }
}
```

---

## 🧪 Full Builder Example

```go
weave, err := eywa.NewWeaveBuilder(ctx).
    // Required
    WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
    WithBond(bond).
    AddAnthropic(os.Getenv("ANTHROPIC_API_KEY")).
    AddOpenAI(os.Getenv("OPENAI_API_KEY")).

    // Registries
    WithActionRegistry(actionRegistry).
    WithScoutRegistry(scoutRegistry).
    WithVoiceRegistry(voiceRegistry).

    // Routing
    WithDefaultLLMPathfinder("anthropic", "claude-haiku-4-5-20251001").

    // Archiving
    WithDefaultLLMArchivist("anthropic", "claude-haiku-4-5-20251001", 20).
    WithArchivistConfig(0.1, 512).

    // Coalescing
    WithMessageInbox(inbox).
    WithInboxMinWindow(3 * time.Second).

    // Async / Scheduling
    WithAsyncDispatch(keeper).
    WithRitualManager(ritualService).

    // Media
    WithMediaStore(vault).
    WithMediaProcessor(lens).

    // Guards
    WithRateLimiter(rateLimiter).
    WithInputGuard(eywa.GuardConfig{PromptInjectionDetection: true}).

    // Config
    WithConfig(eywa.DefaultWeaveConfig()).
    WithReasoningTimeout(90 * time.Second).
    WithMaxIterations(15).
    WithParallelActionExecution(true).
    WithActionRetry(3, 500*time.Millisecond).

    // Observability
    WithAppInfo("myapp", "v1.0.0").
    WithLogger(logger.Sugar()).
    WithTracer(tracer).

    Build()
```
