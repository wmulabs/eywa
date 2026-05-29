# 📖 Eywa Concepts & Extension Points

This document explains every interface in Eywa's vocabulary and how to implement each one. All interfaces are defined in `internal/domain/ports/` and re-exported from the root package facade.

---

## 👻 Spirit

A Spirit is an AI agent configuration stored in MongoDB. It is not an interface you implement — it is a data entity you create and manage.

```go
spirit := &eywa.Spirit{
    Name:         "support_spirit",          // unique identifier, used in Link.AllowedSpirits
    Description:  "Customer support agent",  // used by LLM Pathfinder to route
    SystemPrompt: "You are a helpful...",    // the agent's identity and instructions
    Specialization: "technical_support",     // optional hint for routing
    AllowedActions: []eywa.AllowedAction{    // which Actions this Spirit can invoke
        {Name: "track_order"},
        {Name: "cancel_order"},
        {Name: "update_subject"},
    },
    ModelConfig: eywa.SpiritModel{
        Provider:    "anthropic",             // must match a registered Oracle provider
        Model:       "claude-sonnet-4-6",
        Temperature: 0.5,
        MaxTokens:   2000,
        TopP:        0.9,                     // optional
    },
    EnforceVoiceDelivery:      false,
    VoiceDeliveryInstructions: "",
    BusinessErrorInstructions: "",
    IsActive:  true,
    CreatedAt: time.Now(),
}
```

> [!NOTE]
> **Versioning:** every `Update` call creates a new version rather than overwriting. You can activate any prior version via `POST /api/v1/spirits/:name/activate` with `{"version": N}`.

---

## ⚡ Action

Actions are the tools the Oracle can call during reasoning. They are the bridge between AI intent and real-world systems.

```go
type Action interface {
    GetName() string                                        // unique tool name (snake_case)
    GetDescription() string                                 // shown to the Oracle to explain when to use it
    GetParameters() map[string]interface{}                  // JSON Schema for parameters
    GetCategory() ActionCategory                            // Delivery, Retrieval, Modification, General
    IsCritical() bool                                       // true = response already delivered, skip Voice
    Validate(args map[string]interface{}) error             // called before Execute
    Execute(ctx context.Context, args map[string]interface{}) (string, error)
}
```

**Categories:**

| Category | Constant | When to use |
|----------|----------|-------------|
| Delivery | `eywa.ActionDelivery` | Sends a response to the user (WhatsApp message, email). `IsCritical: true` prevents the Voice from double-sending. |
| Retrieval | `eywa.ActionRetrieval` | Reads data (order status, user profile). `IsCritical: false`. |
| Modification | `eywa.ActionModification` | Changes state (update order, cancel subscription). `IsCritical: false`. |
| General | `eywa.ActionGeneral` | Anything else. `IsCritical: false`. |

**When should `IsCritical()` return `true`?**

| Scenario | `IsCritical()` | Reason |
|---|---|---|
| Database write that saves user state | `true` | Pipeline already mutated external state; must not be interrupted |
| Payment or external mutation | `true` | Side effect happened; inconsistent state if pipeline aborts |
| Delivery Action (WhatsApp, email) | `true` | Response already sent; prevents Voice from double-sending |
| Read-only lookup (weather, product info) | `false` | Failure is recoverable; Oracle can reason without the data |
| Notification push (non-delivery) | `false` | Failure is unfortunate but not pipeline-breaking |

**Rule of thumb:** If the failure leaves external state inconsistent with the Spirit's session, return `true`. If the Action is purely informational and the Spirit can still respond meaningfully without its output, return `false`.

**Error classification in Execute:**

```go
func (a *MyAction) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    result, err := a.service.DoThing(ctx, args["id"].(string))
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            // User made an invalid request — not retryable, show to user
            return "", eywa.NewBusinessError("item not found")
        }
        // Transient failure — retryable, hide from user
        return "", eywa.NewInfrastructureError("service unavailable", err)
    }
    return fmt.Sprintf("Done: %v", result), nil
}
```

> [!NOTE]
> `InfrastructureError`s are retried with exponential backoff when `WithActionRetry` is configured. `BusinessError`s are never retried.

---

## 🔭 Scout

Scouts enrich a Pulse's `Knowledge` before Spirit selection. They run sequentially in declaration order and are non-fatal — a Scout error is logged and the Pulse continues.

```go
type Scout interface {
    GetName() string
    IsApplicable(pulse *Pulse) bool                    // skip this Scout for irrelevant Pulses
    Harvest(ctx context.Context, pulse *Pulse) error   // populate pulse.Knowledge
}
```

**Best practices:**
- Keep Scouts fast — they run within `ScoutTimeout` (default 15s) and block Spirit selection
- Use `IsApplicable` to skip Scouts for Pulses that don't need them
- Write to `pulse.Knowledge`, never to `pulse.Metadata`
- Return `nil` on recoverable errors (the Pulse can still be processed without this data)

<details>
<summary>Full Scout implementation example</summary>

```go
type OrderContextScout struct {
    orderRepo *OrderRepository
}

func (s *OrderContextScout) GetName() string { return "order_context" }

func (s *OrderContextScout) IsApplicable(pulse *eywa.Pulse) bool {
    _, hasOrderID := pulse.Payload["order_id"]
    return hasOrderID
}

func (s *OrderContextScout) Harvest(ctx context.Context, pulse *eywa.Pulse) error {
    orderID, _ := pulse.Payload["order_id"].(string)

    order, err := s.orderRepo.FindByID(ctx, orderID)
    if err != nil {
        return nil // non-fatal: Spirit proceeds without order context
    }

    pulse.Knowledge["order_id"]     = order.ID
    pulse.Knowledge["order_status"] = order.Status
    pulse.Knowledge["delivery_eta"] = order.DeliveryETA
    return nil
}
```

</details>

**Registration:**

```go
scoutRegistry := eywa.NewScoutRegistry()
scoutRegistry.Register(&OrderContextScout{orderRepo: repo})
builder.WithScoutRegistry(scoutRegistry)
```

**Linking to event types:**

```go
weave.RegisterEventConfiguration(
    eywa.NewLink("customer_message").
        WithScouts("order_context", "user_profile"). // run in this order
        WithDefaultSpirit("support_spirit").
        Build(),
)
```

---

## 🧭 Pathfinder

A Pathfinder selects the best Spirit when a Link has multiple `AllowedSpirits`. It receives the enriched Pulse (Scouts have already run) and the list of eligible Spirit names.

```go
type Pathfinder interface {
    GetName() string
    SelectSpirit(ctx context.Context, pulse *Pulse, availableSpirits []string) string
}
```

**Return value:** a Spirit name from `availableSpirits`. If empty string or not in the list, `DefaultSpirit` is used.

<details>
<summary>Custom rule-based Pathfinder example</summary>

```go
type ContentPathfinder struct{}

func (p *ContentPathfinder) GetName() string { return "content_pathfinder" }

func (p *ContentPathfinder) SelectSpirit(_ context.Context, pulse *eywa.Pulse, available []string) string {
    msg := strings.ToLower(pulse.UserMessage)

    if containsAny(msg, "invoice", "payment", "charge", "refund") {
        return findIn(available, "billing_spirit")
    }
    if containsAny(msg, "bug", "error", "crash", "help") {
        return findIn(available, "support_spirit")
    }

    // Fallback to first available
    return available[0]
}
```

</details>

> [!TIP]
> **Built-in LLM Pathfinder:** `WithDefaultLLMPathfinder("anthropic", "claude-haiku-4-5-20251001")` registers a Pathfinder that calls the LLM with all Spirit names + descriptions and asks it to classify the intent. Use this when rule-based routing is insufficient.

---

## 📡 Receptor (Inbound Converter)

Receptors convert raw webhook payloads into `[]*Pulse`. They are registered on the Weave and referenced by name in a Link's `InboundConverterName`.

```go
type Receptor interface {
    GetName() string
    Convert(ctx context.Context, eventType string, raw map[string]interface{}) ([]*Pulse, error)
}
```

**When to implement:** when you receive webhooks from a channel that Eywa doesn't have a built-in Receptor for (e.g. Telegram, Instagram, custom IoT device).

<details>
<summary>Telegram Receptor implementation example</summary>

```go
type TelegramReceptor struct{}

func (r *TelegramReceptor) GetName() string { return "telegram" }

func (r *TelegramReceptor) Convert(ctx context.Context, eventType string, raw map[string]interface{}) ([]*eywa.Pulse, error) {
    message, ok := raw["message"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("missing message in Telegram update")
    }

    userID := fmt.Sprintf("%v", message["from"].(map[string]interface{})["id"])
    text, _ := message["text"].(string)
    msgID := fmt.Sprintf("%v", message["message_id"])

    pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "telegram", User: userID}).
        WithUserMessage(text).
        WithSource("telegram").
        WithEventType(eventType).
        WithIdempotencyKey(msgID).
        WithPayload(raw).
        Build()

    return []*eywa.Pulse{pulse}, nil
}
```

</details>

**Registration:**

```go
weave.RegisterReceptor("telegram", &TelegramReceptor{})
```

**Linking:**

```go
eywa.NewLink("telegram_message").
    WithInboundConverter("telegram").
    WithDefaultSpirit("my_spirit").
    Build()
```

> [!NOTE]
> The Fiber handler automatically calls the registered Receptor when a webhook arrives at `POST /api/v1/events/telegram_message`.

---

## 📢 Voice

A Voice delivers the Spirit's response back to the user through a specific channel. It runs after the Oracle loop completes, unless a delivery Action (`IsCritical: true`) already sent a response.

```go
type Voice interface {
    GetName() string
    ShouldAutoRespond() bool                                                  // true = auto-send when no delivery Action ran
    SendResponse(ctx context.Context, event *Pulse, response string) error
    GetChannelMetadata(event *Pulse) map[string]interface{}
}
```

<details>
<summary>Slack Voice implementation example</summary>

```go
type SlackVoice struct {
    client  *slack.Client
    channel string
}

func (v *SlackVoice) GetName() string         { return "slack" }
func (v *SlackVoice) ShouldAutoRespond() bool { return true }

func (v *SlackVoice) SendResponse(ctx context.Context, event *eywa.Pulse, response string) error {
    _, _, err := v.client.PostMessageContext(ctx, v.channel, slack.MsgOptionText(response, false))
    return err
}

func (v *SlackVoice) GetChannelMetadata(event *eywa.Pulse) map[string]interface{} {
    return map[string]interface{}{"channel": v.channel, "source": event.Source}
}
```

</details>

**Registration:**

```go
voiceRegistry := eywa.NewVoiceRegistry()
voiceRegistry.Register(&SlackVoice{client: slackClient, channel: "#support"})
builder.WithVoiceRegistry(voiceRegistry)
```

**Linking:**

```go
eywa.NewLink("slack_message").
    WithVoice("slack").
    WithDefaultSpirit("support_spirit").
    Build()
```

---

## 🔮 Oracle (LLM Provider)

The Oracle is the LLM interface. Eywa ships with Anthropic, OpenAI, and Gemini providers. Implement this interface to add any other model.

```go
type Oracle interface {
    GetName() string
    GetAvailableModels() []string
    GenerateResponse(ctx context.Context, req *OracleRequest) (*OracleResponse, error)
    IsAvailable() bool
    GetConfig() map[string]interface{}

    // model is the specific model string from Spirit.ModelConfig.Model.
    SupportsImages(model string) bool
    SupportsAudio(model string) bool
    SupportsDocuments(model string) bool
}
```

The `OracleRequest` carries the full conversation (system prompt + Threads as messages + available tools). The provider translates to its native API format and returns a normalized `OracleResponse`.

**Registration:**

```go
builder.AddOracle(myCustomOracle)
```

The Oracle is selected per-Spirit via `Spirit.ModelConfig.Provider` matching `Oracle.GetName()`.

---

## 📜 Archivist

The Archivist compresses long conversation histories into summaries, preserving context without exceeding token limits.

```go
type Archivist interface {
    Summarize(ctx context.Context, messages []Thread) (string, OracleUsage, error)
}
```

**Built-in:** `eywa.NewArchivist(provider, model, factory)` — calls the Oracle with a summarization prompt.

**Custom:** implement to use a different summarization strategy (e.g. extractive rather than abstractive, or a local model).

---

## 🔗 Link (Event Configuration)

Links wire event types to their processing configuration. Every `ProcessEventByKey` call looks up the Link registered for that event key.

<details>
<summary>Full Link configuration example</summary>

```go
link := eywa.NewLink("customer_message").
    WithInboundConverter("whatsapp_360dialog").   // Receptor name
    WithScouts("user_profile", "order_context").   // run in order
    WithPathfinder("content_pathfinder").          // explicit Pathfinder
    WithSpirits(                                    // eligible Spirits
        "support_spirit",
        "sales_spirit",
        "billing_spirit",
    ).
    WithDefaultSpirit("support_spirit").            // fallback
    WithVoice("whatsapp").                         // response channel
    WithGuards(
        eywa.Guard{
            Field:     "metadata.channel",
            AllowList: []string{"whatsapp"},
        },
        eywa.Guard{
            Field:     "source",
            BlockList: []string{"test_user"},
        },
    ).
    WithIngestionTimeout(5 * time.Second).
    WithProcessingTimeout(90 * time.Second).
    Build()

weave.RegisterEventConfiguration(link)
```

</details>

> [!NOTE]
> **Guard evaluation:** BlockList is checked first. AllowList is checked second (if non-empty). All Guards are evaluated in AND — a Pulse must pass all rules. A missing field value skips that Guard.

**Spirit selection logic:**

| Scenario | Behavior |
|----------|----------|
| `AllowedSpirits` is empty | Use `DefaultSpirit` directly |
| `AllowedSpirits` has 1 entry | Use it directly (no Pathfinder) |
| `AllowedSpirits` has 2+ entries | Run Pathfinder, fall back to `DefaultSpirit` |

---

## ⚡ Pulse Builder

Create Pulses with the builder — it validates `MemoryKey` at construction time:

```go
pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "whatsapp", User: "+5511999999999"}).
    WithUserMessage("What is my order status?").
    WithSource("whatsapp_360dialog").
    WithContactPhone("+5511999999999").
    WithIdempotencyKey("wamid.abc123").
    WithEventType("customer_message").
    WithSubjectKey(eywa.SubjectKey{Entity: "order", ID: "123"}).
    AddKnowledge("user_tier", "VIP").
    AddMetadata("provider", "360dialog").
    AddAttachment(&eywa.Artifact{
        Type:     eywa.ArtifactTypeImage,
        Data:     imageBytes,
        MimeType: "image/jpeg",
    }).
    Build()
```

**Knowledge vs Metadata vs Payload:**

| Field | Purpose | Reaches Oracle? |
|-------|---------|----------------|
| `Knowledge` | Context the Oracle needs to reason (user tier, order status, etc.) | ✅ Yes — injected into system prompt |
| `Metadata` | Audit/debug data (wamid, phone_number_id, timestamps) | ❌ No |
| `Payload` | Raw webhook data | ❌ No |

---

## 🗓️ Ritual (Scheduled Events)

Rituals are scheduled Pulses. Create them via the API or via the `schedule_ritual` Action from within a Spirit conversation.

```go
// Via API
POST /api/v1/events/customer_message/schedule
{
  "execute_at": "2026-06-01T10:00:00Z",
  "payload": {
    "user_id": "123",
    "trigger": "follow_up"
  },
  "recurrence": {
    "cron": "0 9 * * 1",   // every Monday at 9am
    "timezone": "America/Sao_Paulo",
    "max_runs": 12,
    "ends_at": "2026-12-31T00:00:00Z"
  }
}
```

> [!NOTE]
> When the Keeper fires the Ritual, the Weave processes it through the same pipeline. The Pulse carries `ritual_id` in Metadata, which the `RitualMark` step uses to mark it as executed. The `RitualService` automatically schedules the next occurrence after a successful run, until `MaxRuns` is reached or `EndsAt` passes.

---

## 🔒 Bond (Distributed Lock)

Bond prevents concurrent processing of Pulses for the same session. Only one pipeline
execution runs per `MemoryKey` at a time — if a second Pulse arrives while the first
is still processing, the Bond returns `acquired=false` and the pipeline returns `ErrMemoryBusy`.

```go
type Bond interface {
    AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
    ReleaseLock(ctx context.Context, key string) error
    ExtendLock(ctx context.Context, key string, ttl time.Duration) error
}
```

| Implementation | Use case |
|---|---|
| `eywa.NewNoOpBond()` | Single-instance, local dev, tests — no Redis required |
| `redis.NewBondManager(client)` | Multi-instance production |

**TTL invariant:** Set `LockTTL` ≥ `ReasoningTimeout + 30s` in `WeaveConfig`. The pipeline extends the lock
automatically during reasoning. If you implement a custom Bond, `ExtendLock` must reset the TTL.

---

## 🗓️ Keeper (Scheduler)

Keeper schedules events for future delivery. It is a thin provider boundary — all scheduling
business logic lives in `RitualManager`. Implement it to connect Eywa to any job queue
(Cloud Tasks, SQS, BullMQ, cron daemon).

```go
type Keeper interface {
    Schedule(ctx context.Context, eventKey string, event *Pulse, executeAt time.Time) (taskID string, err error)
    Cancel(ctx context.Context, taskID string) error
}
```

`executeAt`: pass `time.Now()` for immediate async dispatch; a future time for scheduled execution.
`Cancel` must be idempotent — return `nil` if the task no longer exists.

---

## ⏱️ Limiter (Rate Limiter)

Limiter controls how many Pulses a given key (typically `MemoryKey` or contact ID) can
send per time window. Failures must **fail open** — if the underlying store is unreachable,
`Allow` should return `true, nil` to avoid blocking traffic.

```go
type Limiter interface {
    Allow(ctx context.Context, key string) (bool, error)
}
```

Register with `WithRateLimiter(limiter)`. The pipeline checks `Allow` early — if it returns
`false`, the Pulse is rejected with a 429 response.
