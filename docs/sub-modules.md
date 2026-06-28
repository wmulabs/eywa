# 🧩 Sub-modules

Each sub-module lives in its own `go.mod` — you only include what your service needs. All sub-modules import the root `github.com/wmulabs/eywa` package and nothing else from Eywa's internals.

---

## Compatibility Matrix

Each Eywa sub-module is an independent Go module. Install only what your application needs.

| Sub-module | Import path | Go | Key dependency |
|---|---|---|---|
| Core | `github.com/wmulabs/eywa` | 1.25+ | — |
| MongoDB | `github.com/wmulabs/eywa/mongo` | 1.25+ | mongo-driver v1.17.9 |
| Redis | `github.com/wmulabs/eywa/redis` | 1.25+ | go-redis/v9 v9.19.0, redsync/v4 v4.16.0 |
| Fiber | `github.com/wmulabs/eywa/fiber` | 1.25+ | gofiber/fiber/v2 v2.52.12 |
| Weaviate | `github.com/wmulabs/eywa/providers/weaviate` | 1.25+ | weaviate-go-client/v4 v4.16.1 |
| Qdrant | `github.com/wmulabs/eywa/providers/qdrant` | 1.25+ | qdrant/go-client v1.13.0 |
| PgVector | `github.com/wmulabs/eywa/providers/pgvector` | 1.25+ | pgx/v5 v5.7.2 |
| Pinecone | `github.com/wmulabs/eywa/providers/pinecone` | 1.25+ | go-pinecone/v3 v3.1.0 |
| Anthropic | `github.com/wmulabs/eywa/providers/anthropic` | 1.25+ | anthropic-sdk-go v1.42.0 |
| OpenAI | `github.com/wmulabs/eywa/providers/openai` | 1.25+ | openai-go v1.12.0 |
| Gemini | `github.com/wmulabs/eywa/providers/gemini` | 1.25+ | google.golang.org/genai v1.54.0 |
| Bedrock | `github.com/wmulabs/eywa/providers/bedrock` | 1.25+ | aws-sdk-go-v2 v1.36.3 |
| Cloud Tasks | `github.com/wmulabs/eywa/gcp/cloudtasks` | 1.25+ | cloud.google.com/go/cloudtasks v1.13.0 |
| GCS | `github.com/wmulabs/eywa/gcp/gcs` | 1.25+ | cloud.google.com/go/storage v1.43.0 |
| MCP | `github.com/wmulabs/eywa/mcp` | 1.25+ | — |

---

## 🗄️ mongo

```bash
go get github.com/wmulabs/eywa/mongo
```

Provides MongoDB implementations for all persistence repositories.

### Connection

```go
import eywamongo "github.com/wmulabs/eywa/mongo"

mongoConn, err := eywamongo.NewMongoConnection(ctx, os.Getenv("MONGO_URL"), "mydb", "myapp")
if err != nil {
    log.Fatalf("failed to connect to MongoDB: %v", err)
}
defer mongoConn.DisconnectMongoDB(ctx)

db := mongoConn.GetDatabase()
```

### Repositories

```go
// Core — always required
spiritRepo    := eywamongo.NewSpiritRepository(db)
echoRepo      := eywamongo.NewEchoRepository(db)
chronicleRepo := eywamongo.NewChronicleRepository(db)

// Scheduling
ritualRepo    := eywamongo.NewRitualRepository(db)

// Knowledge base (RAG)
loreRepo      := eywamongo.NewLoreRepository(db)

// Long-term user memory
imprintRepo   := eywamongo.NewImprintRepository(db)

// Approval workflows
riteRepo      := eywamongo.NewRiteRepository(db)

// HTTP Tools
httpToolRepo  := eywamongo.NewHTTPToolRepository(db)

// Cost tracking
ledgerRepo    := eywamongo.NewLedgerRepository(db)

// Operator auth
operatorRepo  := eywamongo.NewOperatorRepository(db)

// Engine config (stored in DB, hot-reloaded)
weaveConfigRepo := eywamongo.NewWeaveConfigRepository(db)

// Multi-agent handoff (active-Spirit pin per session)
handoffStore  := eywamongo.NewHandoffStore(db)
```

> [!NOTE]
> **Indexes are created automatically** on first use. Each repository calls `createIndexes` during construction — no migrations needed.

> [!NOTE]
> **Spirit storage:** fully versioned. Each `Update` inserts a new document rather than overwriting, preserving the complete history of Spirit configurations.
>
> **Echo indexes:** optimized for queries by `(memory_key, subject_key, timestamp)` — supports efficient memory reconstruction and message history retrieval.
>
> **Ritual indexes:** optimized for queries by `(memory_key, status, execute_at)`.
>
> **Imprint indexes:** by `(user_key)`, `(spirit_id)`, and `(category)` — supports filtered admin listing.
>
> **Rite indexes:** by `(status, created_at)` for pending queue queries.

---

## ⚡ redis

```bash
go get github.com/wmulabs/eywa/redis
```

Provides Redis implementations for Memory, Bond, Inbox, and rate limiting.

### Connection

```go
import eywaredis "github.com/wmulabs/eywa/redis"

redisConn, err := eywaredis.NewRedisConnection(ctx, os.Getenv("REDIS_URL"), "myapp")
if err != nil {
    log.Fatalf("failed to connect to Redis: %v", err)
}
defer redisConn.DisconnectRedisDB(ctx)

client := redisConn.GetClient()
```

### Memory Repository

```go
memoryRepo := eywaredis.NewMemoryRepository(
    client,
    "myapp",    // service name prefix in Redis keys
    "prod",     // environment prefix
    3600,       // TTL in seconds (1 hour)
    nil,        // OTel tracer — pass nil to use noop, or inject your tracer
)
```

**Key pattern:** `{service}:{environment}:memory:{channel}:{user}`

Example: `myapp:prod:memory:whatsapp:+5511999999999`

> [!NOTE]
> Memory is serialized as JSON and stored with the configured TTL. Each access extends the TTL automatically.

### Bond (Distributed Lock)

```go
bond := eywaredis.NewBondManager(client)
```

Implements distributed locking using Redis SET NX with expiry. Multiple concurrent Pulses for the same MemoryKey will queue — only one processes at a time.

> [!NOTE]
> The Bond implementation uses a per-key mutex registry, so `ReleaseLock` and `ExtendLock` always target the correct lock instance.

### Message Inbox

```go
inbox := eywaredis.NewRedisMessageInbox(client, "myapp")
```

Used with `WithMessageInbox` to enable message coalescing. Messages buffered while a MemoryKey is locked are merged into the active Pulse at the start of the next processing cycle.

### Rate Limiter

```go
rateLimiter := eywaredis.NewRedisRateLimiter(
    client,
    10,           // max requests
    time.Minute,  // per window
)

builder.WithRateLimiter(rateLimiter)
```

Implements sliding-window rate limiting per MemoryKey.

> [!WARNING]
> On Redis failure, the limiter **fails open** — Pulses are allowed through to avoid blocking traffic on infrastructure issues.

### Vigil Repository

```go
vigilRepo := eywaredis.NewVigilRepository(
    client,
    "myapp",           // service name
    "prod",            // environment
)
```

Stores active Vigil seats as Redis keys with TTL. Each seat is a JSON-encoded `Vigil` struct keyed as:
`{service}:{environment}:vigil:{memoryKey}`

Supports listing all active seats via Redis `SCAN` pattern matching — no separate index required.

```go
builder.
    WithVigilRepository(vigilRepo).
    WithVigilConfig(eywa.VigilConfig{InactivityTimeout: 30 * time.Minute})
```

### Handoff Store

```go
handoffStore := eywaredis.NewHandoffStore(client, time.Hour) // ttl 0 = no expiry
builder.WithHandoffStore(handoffStore)
```

Implements `eywa.HandoffStore` — pins the active Spirit per session after a peer handoff so subsequent Pulses route to it across instances. The mongo adapter (`eywamongo.NewHandoffStore(db)`) is durable; the in-memory one (`eywa.NewInMemoryHandoffStore()`) suits single-instance. See [Multi-agent](multi-agent.md).

### PubSub

```go
pubSub := eywaredis.NewPubSub(client)
```

Implements `eywa.PubSub` using Redis Pub/Sub channels. Used for real-time SSE fanout — every instance that subscribes to a channel receives every event, regardless of which instance published it.

Channels used by eywa:
- `eywa:rites` — Rite lifecycle events
- `eywa:vigil` — global Vigil seat events
- `eywa:echoes:{memoryKey}` — per-session events

```go
eywafiber.RegisterRoutes(app, weave, eywafiber.RouteDeps{
    PubSub: eywaredis.NewPubSub(client),
    // ...
})
```

---

## 🟣 providers/anthropic

```bash
go get github.com/wmulabs/eywa/providers/anthropic
```

```go
import "github.com/wmulabs/eywa/providers/anthropic"

oracle := anthropic.NewOracle(os.Getenv("ANTHROPIC_API_KEY"))
builder.AddOracle(oracle)
```

**Provider name:** `"anthropic"` — use in `Spirit.ModelConfig.Provider`.

**Supports:** text, images (Claude 3+), documents/PDFs (base64 via messages API). Does not support audio input.

**Recommended models:**

| Use case | Model |
|----------|-------|
| Primary Spirit reasoning | `claude-sonnet-4-6` |
| Routing / Archiving (cheap) | `claude-haiku-4-5-20251001` |
| Complex reasoning | `claude-opus-4-7` |

---

## 🟢 providers/openai

```bash
go get github.com/wmulabs/eywa/providers/openai
```

```go
import "github.com/wmulabs/eywa/providers/openai"

oracle := openai.NewOracle(os.Getenv("OPENAI_API_KEY"))
builder.AddOracle(oracle)
```

**Provider name:** `"openai"` — use in `Spirit.ModelConfig.Provider`.

**Supports:** text, images, audio (via Whisper-compatible input). Does not support document upload in the messages API.

**OpenAI-compatible endpoints** (same package, distinct provider names): `NewOllamaOracle`,
`NewGroqOracle`, `NewMistralOracle`, `NewTogetherOracle`, `NewOpenRouterOracle`, `NewXAIOracle`, and
`NewAzureOracle(endpoint, apiKey, apiVersion)` for **Azure OpenAI** (api-key header, `api-version`
query, deployment-based URLs handled internally; set `Spirit.ModelConfig.Model` to the deployment name
and `Provider` to `"azure"`). For retry/timeout overrides use `NewAzureOracleWithConfig(AzureConfig{…})`.
Azure has its own `AzureConfig` type — the standard `Config` (BaseURL/OrgID) stays OpenAI-only.

---

## 🔵 providers/gemini

```bash
go get github.com/wmulabs/eywa/providers/gemini
```

```go
import "github.com/wmulabs/eywa/providers/gemini"

oracle := gemini.NewOracle(os.Getenv("GEMINI_API_KEY"))
builder.AddOracle(oracle)
```

**Provider name:** `"gemini"` — use in `Spirit.ModelConfig.Provider`.

**Supports:** text, images, audio, documents (multimodal native).

---

## 🟠 providers/bedrock

```bash
go get github.com/wmulabs/eywa/providers/bedrock
```

```go
import "github.com/wmulabs/eywa/providers/bedrock"

oracle, err := bedrock.NewOracle(ctx, "us-east-1")
builder.AddOracle(oracle)
```

**Provider name:** `"bedrock"` — use in `Spirit.ModelConfig.Provider`.

Uses the AWS Bedrock **Converse API** — works with any model available in your account (Anthropic Claude, Meta Llama, Mistral, Amazon Titan, and more). Authentication is via the standard AWS credential chain (instance profile, environment variables, `~/.aws/credentials`).

**Model name format:** the full Bedrock model ID, e.g. `anthropic.claude-3-5-sonnet-20241022-v2:0`.

**Supports:** text, images (model-dependent). No explicit audio or document upload.

---

## 🔴 providers/vertexai

```bash
go get github.com/wmulabs/eywa/providers/vertexai
```

```go
import "github.com/wmulabs/eywa/providers/vertexai"

oracle, err := vertexai.NewOracle(ctx, "my-project", "us-central1")
builder.AddOracle(oracle)
```

**Provider name:** `"vertexai"` — use in `Spirit.ModelConfig.Provider`.

Uses Google Application Default Credentials — no API key required. Suitable for Cloud Run and GKE deployments with a service account. Supports any Gemini model available in Vertex AI.

**Model name format:** e.g. `gemini-2.0-flash-001`.

**Supports:** text, images, audio, documents (same multimodal capabilities as Gemini).

---

## 🔍 Vector Store Providers (Lore)

These providers implement the `eywa.LoreStore` port for semantic vector search. Use them with `WithLoreStore` on the builder when full-text MongoDB search is insufficient.

### providers/pgvector

```bash
go get github.com/wmulabs/eywa/providers/pgvector
```

```go
import "github.com/wmulabs/eywa/providers/pgvector"

store, err := pgvector.NewLoreStore(pgvector.Config{
    DSN:       os.Getenv("POSTGRES_DSN"),
    Dimension: 1536,   // must match your embedding model output
})
builder.WithLoreStore(store)
```

Uses the `pgvector` PostgreSQL extension. Table and indexes are created automatically. Best choice when your stack already uses PostgreSQL.

### providers/qdrant

```bash
go get github.com/wmulabs/eywa/providers/qdrant
```

```go
import "github.com/wmulabs/eywa/providers/qdrant"

store, err := qdrant.NewLoreStore(qdrant.Config{
    Host:       "localhost",
    Port:       6334,
    Collection: "eywa_lore",
    Dimension:  1536,
})
builder.WithLoreStore(store)
```

Uses Qdrant's gRPC client. Collection is created automatically if it doesn't exist.

### providers/pinecone

```bash
go get github.com/wmulabs/eywa/providers/pinecone
```

```go
import "github.com/wmulabs/eywa/providers/pinecone"

store, err := pinecone.NewLoreStore(pinecone.Config{
    APIKey:    os.Getenv("PINECONE_API_KEY"),
    IndexName: "eywa-lore",
    Namespace: "prod",
})
builder.WithLoreStore(store)
```

Serverless Pinecone — no infrastructure to manage. Best for cloud-native setups.

### providers/weaviate

```bash
go get github.com/wmulabs/eywa/providers/weaviate
```

```go
import "github.com/wmulabs/eywa/providers/weaviate"

store, err := weaviate.NewLoreStore(weaviate.Config{
    Host:      "localhost:8080",
    ClassName: "EywaLore",
    APIKey:    "",          // optional — for Weaviate Cloud
})
builder.WithLoreStore(store)
```

Uses Weaviate's GraphQL-based vector search. Class schema is upserted automatically.

> [!NOTE]
> All four vector store adapters share the same `eywa.LoreStore` interface — swap between them by changing the constructor, zero other code changes needed.

---

## 🌐 fiber

```bash
go get github.com/wmulabs/eywa/fiber
```

Mounts the full REST API on a Fiber application.

### RegisterRoutes

A single registrar takes a `RouteDeps` struct. Open routes (health + events) mount with an empty
struct; each management group mounts when its repo is set, always behind auth. See
[authentication.md](authentication.md) for the full model.

```go
import (
    "github.com/gofiber/fiber/v2"
    eywafiber "github.com/wmulabs/eywa/fiber"
)

app := fiber.New()

eywafiber.RegisterRoutes(app, weave, eywafiber.RouteDeps{
    APIKeys:    map[string]string{adminKey: "admin"}, // required to expose management routes
    SpiritRepo: spiritRepo,                            // authenticated Spirit CRUD
    EchoRepo:   echoRepo,
})

app.Listen(":8080")
```

**With OIDC protection on the internal endpoint** (required when using Cloud Tasks):

```go
import (
    eywafiber "github.com/wmulabs/eywa/fiber"
    "github.com/wmulabs/eywa/gcp/cloudtasks"
)

eywafiber.RegisterRoutes(app, weave, eywafiber.RouteDeps{
    InternalMiddleware: []fiber.Handler{
        cloudtasks.NewCloudTasksOIDCMiddleware(os.Getenv("SERVICE_URL")),
    },
})
```

### Route Reference

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness — always 200 if the process is alive |
| `GET` | `/ready` | Readiness — checks repositories and LLM factory |
| `POST` | `/api/v1/events/:event_key` | Synchronous: process Pulse, wait for response |
| `POST` | `/api/v1/events/:event_key/async` | Async: dispatch to Keeper, return immediately |
| `POST` | `/api/v1/events/:event_key/schedule` | Schedule a future Ritual |
| `GET` | `/api/v1/schedule?memory_key=...` | List pending Rituals for a MemoryKey |
| `DELETE` | `/api/v1/schedule/:id?memory_key=...` | Cancel a Ritual |
| `POST` | `/api/v1/spirits` | Create a Spirit |
| `GET` | `/api/v1/spirits` | List active Spirits (paginated) |
| `GET` | `/api/v1/spirits/:name` | Get active Spirit by name |
| `PUT` | `/api/v1/spirits/:name` | Update Spirit (creates new version) |
| `DELETE` | `/api/v1/spirits/:name` | Deactivate Spirit |
| `POST` | `/api/v1/spirits/:name/activate` | Activate specific Spirit version |
| `POST` | `/api/v1/spirits/:name/deactivate` | Deactivate Spirit |
| `GET` | `/api/v1/messages?memory_key=...` | Query Echo message history |
| `POST/GET` | `/api/v1/app-tokens` · `DELETE /api/v1/app-tokens/:id` | Mint / list / revoke event app-tokens |
| `POST` | `/internal/execute-event` | Keeper callback — processes scheduled Pulses |

Management routes also include chronicle/analytics, echoes, event-configurations, engine config, HTTP
tools, Vigil, Rites, imprints, operators, and SSE — see [rest-api.md](rest-api.md) for the full list.

> [!NOTE]
> Health and event routes are open by default; **all management routes (Spirit CRUD, messages, schedule,
> app-tokens, etc.) require auth** and mount only when their repo is set in `RouteDeps`. Async routes
> (`.../async`, `/api/v1/schedule`) need `weave.GetAsyncDispatcher()` / `GetRitualManager()`. Event
> ingestion can be gated with `EventAuth` / `EventVerifiers` — see [authentication.md](authentication.md).

---

## ☁️ gcp/cloudtasks

```bash
go get github.com/wmulabs/eywa/gcp/cloudtasks
```

Cloud Tasks Keeper for async dispatch and Ritual scheduling.

### Setup

```go
import "github.com/wmulabs/eywa/gcp/cloudtasks"

keeper, err := cloudtasks.NewCloudTasksKeeper(ctx, cloudtasks.CloudTasksConfig{
    Project:             "my-gcp-project",
    Location:            "us-central1",
    Queue:               "eywa-events",
    TargetBaseURL:       "https://my-service.run.app",
    TargetAudience:      "https://my-service.run.app",     // OIDC audience for Cloud Run auth
    ServiceAccountEmail: "eywa@my-project.iam.gserviceaccount.com", // optional
})
```

The Keeper creates Cloud Tasks that POST to `{TargetBaseURL}/internal/execute-event` with OIDC authentication. The `NewCloudTasksOIDCMiddleware` middleware on the Fiber app verifies the token.

### OIDC Middleware

```go
oidcMiddleware := cloudtasks.NewCloudTasksOIDCMiddleware(os.Getenv("SERVICE_URL"))
eywafiber.RegisterRoutes(app, weave, eywafiber.RouteDeps{
    InternalMiddleware: []fiber.Handler{oidcMiddleware},
})
```

> [!TIP]
> Empty audience → middleware is a no-op (useful in local development).

---

## 🗳️ gcp/gcs

```bash
go get github.com/wmulabs/eywa/gcp/gcs
```

GCS-backed Vault for media storage.

```go
import "github.com/wmulabs/eywa/gcp/gcs"

vault, err := gcs.NewGCSVault(ctx, "my-media-bucket")
defer vault.Close()

builder.WithMediaStore(vault)
```

When a Pulse carries Attachments (images, audio, video, documents), the `MediaVault` pipeline step uploads them to GCS after processing. Returns public `storage.googleapis.com` URLs stored in the Echo record.

---

## 🔵 gcp/gemini

```bash
go get github.com/wmulabs/eywa/gcp/gemini
```

Gemini-powered media processing (image analysis, audio transcription, document extraction).

```go
import "github.com/wmulabs/eywa/gcp/gemini"

eye, err := gemini.NewGeminiImageAnalyzer(os.Getenv("GEMINI_API_KEY"), "") // "" = default model
ear, err := gemini.NewGeminiAudioTranscriber(os.Getenv("GEMINI_API_KEY"), "")
doc, err := gemini.NewGeminiDocumentExtractor(os.Getenv("GEMINI_API_KEY"), "")
```

Use these with `WithMediaProcessor` on the builder:

```go
// Implement ports.Lens to combine them:
type GeminiLens struct {
    eye *gemini.GeminiImageAnalyzer
    ear *gemini.GeminiAudioTranscriber
    doc *gemini.GeminiDocumentExtractor
}

func (l *GeminiLens) Analyze(ctx context.Context, data []byte, mime string) (string, eywa.OracleUsage, error) {
    return l.eye.Analyze(ctx, data, mime)
}
func (l *GeminiLens) Transcribe(ctx context.Context, data []byte, mime string) (string, eywa.OracleUsage, error) {
    return l.ear.Transcribe(ctx, data, mime)
}
func (l *GeminiLens) Extract(ctx context.Context, data []byte, mime string) (string, eywa.OracleUsage, error) {
    return l.doc.Extract(ctx, data, mime)
}

builder.WithMediaProcessor(&GeminiLens{eye: eye, ear: ear, doc: doc})
```

> [!NOTE]
> The `MediaVault` pipeline step calls the Lens for each Attachment type and injects the result into `Pulse.Knowledge` (e.g. `"transcription"`, `"image_description"`, `"document_text"`), making it available to the Oracle automatically.

---

## 📱 channels/whatsapp

```bash
go get github.com/wmulabs/eywa/channels/whatsapp
```

WhatsApp integration: Actions for sending messages and templates, Voice for auto-responses.

### Actions

```go
import "github.com/wmulabs/eywa/channels/whatsapp"

actionRegistry.Register(whatsapp.NewSendWhatsAppMessageTool(client))
actionRegistry.Register(whatsapp.NewSendWhatsAppTemplateTool(client))
```

**send_whatsapp_message** — sends a text message, optionally with buttons (up to 3) or an image URL. `IsCritical: true` — prevents the Voice from sending a duplicate response.

**send_whatsapp_template** — sends a pre-approved Meta template. Parameters are ordered arrays per component (header, body, buttons). Supports text, image, and video parameters.

### Voice

```go
voiceRegistry.Register(whatsapp.NewWhatsAppResponseChannel(client))
```

Registers the `"whatsapp"` Voice. Auto-responds with the Spirit's reply when no `send_whatsapp_message` Action was called during processing.

---

### channels/whatsapp/dialog360

```bash
go get github.com/wmulabs/eywa/channels/whatsapp/dialog360
```

```go
import "github.com/wmulabs/eywa/channels/whatsapp/dialog360"

client := dialog360.NewDialog360Client(dialog360.Dialog360Config{
    APIKey:  os.Getenv("DIALOG360_API_KEY"),
    BaseURL: "",  // defaults to https://waba-v2.360dialog.io
    Timeout: 30 * time.Second,
})

// Receptor: converts 360Dialog webhooks to Pulses
weave.RegisterReceptor("whatsapp_360dialog", dialog360.NewWhatsApp360DialogInbound(client))
```

The 360Dialog Receptor handles:
- Text messages
- Images, audio, video, documents (downloads bytes automatically via `DownloadMedia`)
- Location messages (converted to text)
- Reactions
- Interactive button/list replies
- Groups messages by sender for multi-sender webhooks

> [!NOTE]
> **Media download:** 360Dialog uses its own CDN host for downloads (replaces the Facebook CDN URL automatically).

---

### channels/whatsapp/twilio

```bash
go get github.com/wmulabs/eywa/channels/whatsapp/twilio
```

```go
import "github.com/wmulabs/eywa/channels/whatsapp/twilio"

client := twilio.NewTwilioClient(twilio.TwilioConfig{
    AccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
    AuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
    FromNumber: os.Getenv("TWILIO_FROM_NUMBER"),

    // Map readable template names to Twilio Content SIDs
    // Allows Spirits to use "follow_up_reminder" instead of "HXb1234..."
    TemplateSIDs: map[string]string{
        "follow_up_reminder":    "HXb1234567890abcdef",
        "delivery_confirmation": "HXc9876543210fedcba",
    },

    PollInterval:    2 * time.Second,
    DeliveryTimeout: 15 * time.Second,
})

weave.RegisterReceptor("whatsapp_twilio", twilio.NewWhatsAppTwilioInbound(client))
```

> [!NOTE]
> **SendMessage** blocks until a terminal delivery status (delivered, read, sent, failed, undelivered) is reached or timeout expires. This gives you reliable delivery confirmation at the cost of holding the connection open.

> [!NOTE]
> **Template variables** are automatically converted from Eywa's component format (header/body/button arrays) to Twilio's flat numbered format (`{"1": "value", "2": "value"}`). Empty values are replaced with `"N/D"` to prevent Twilio error 21656.

> [!TIP]
> **Error classification:** Twilio error codes are mapped to `BusinessError` (invalid number, unsubscribed) or `InfrastructureError` (service unavailable, rate limit) automatically.

---

## ✈️ channels/telegram

```bash
go get github.com/wmulabs/eywa/channels/telegram
```

Telegram Bot API channel (webhook-based). Implements `eywa.Receptor` (webhook `Update` → Pulse), `eywa.Voice` (response → chat), and a `RequestVerifier` for the webhook secret token.

```go
import "github.com/wmulabs/eywa/channels/telegram"

client := telegram.NewClient(os.Getenv("TELEGRAM_BOT_TOKEN"))

weave.RegisterReceptor("telegram", telegram.NewInbound(client)) // client enables media downloads
voiceRegistry.Register(telegram.NewVoice(client))

// Event auth: verify the secret token set via Telegram's setWebhook.
verifier := telegram.NewSecretTokenVerifier(os.Getenv("TELEGRAM_WEBHOOK_SECRET"))
```

- **Inbound:** maps `message.chat.id` to the MemoryKey (`telegram:<chatID>`), `message.text` (or caption) to the user message, `update_id` to the idempotency key. Handles text and media (photo, voice, audio, video, document) — media bytes are downloaded via `getFile` and attached for the media pipeline. Bot-authored messages are ignored.
- **Outbound:** `sendMessage` via the Bot API (fixed host — no SSRF surface; response body is bounded).
- **Auth:** constant-time compare of the `X-Telegram-Bot-Api-Secret-Token` header.

> [!NOTE]
> Slack is on the roadmap with the same shape (Receptor + Voice + signed-request verifier).
