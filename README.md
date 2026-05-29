<p align="center">
  <img src="docs/assets/banner.png" alt="Eywa — Event-driven AI orchestration for Go" width="100%"/>
</p>

<h1 align="center">🌿 Eywa</h1>

<p align="center">
  <em>Production-grade, event-driven AI orchestration for Go</em>
</p>

<p align="center">
  🌐 <strong>English</strong> · <a href="README.pt-BR.md">Português</a>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/wmulabs/eywa"><img src="https://pkg.go.dev/badge/github.com/wmulabs/eywa.svg" alt="Go Reference"/></a>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white" alt="Go version"/>
  <img src="https://img.shields.io/badge/License-Apache--2.0-blue.svg" alt="License"/>
  <img src="https://img.shields.io/badge/Architecture-Hexagonal-6c3483" alt="Hexagonal"/>
  <img src="https://img.shields.io/badge/LLMs-8+_providers-FF6B35" alt="LLM Providers"/>
  <a href="https://github.com/wmulabs/eywa/actions/workflows/ci.yml"><img src="https://github.com/wmulabs/eywa/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://codecov.io/gh/wmulabs/eywa"><img src="https://codecov.io/gh/wmulabs/eywa/graph/badge.svg" alt="Coverage"/></a>
</p>

<p align="center">
  <a href="https://github.com/sponsors/wmulabs"><img src="https://img.shields.io/badge/Sponsor-GitHub-%23EA4AAA?logo=github" alt="GitHub Sponsors"/></a>
  <a href="https://buymeacoffee.com/wmulabs"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-wmulabs-%23FFDD00?logo=buymeacoffee&logoColor=black" alt="Buy Me a Coffee"/></a>
</p>

---

> *In Na'vi belief, Eywa is the consciousness of Pandora — a vast living network that connects every creature, carries the memory of ancestors, and orchestrates the balance of the world. When a being sends a signal into the Weave, Eywa finds the Spirit with the wisdom to answer.*
>
> *That's exactly what this library does for your AI systems.*

---

## The problem with "AI in production"

You added an LLM call to a webhook handler. It works. Then your users start sending concurrent messages and you get duplicate responses. You add Redis locking. Then conversations grow beyond the context window. You add a summarizer. Then you need the AI to take real actions. You add tool use. Then you need human oversight. You add… 

**You're not building a product anymore. You're building infrastructure.**

Eywa is that infrastructure — a battle-tested Go framework that handles the full lifecycle of an AI interaction: receiving events from any channel, enriching them with context, routing them to the right Spirit, executing tool calls, maintaining memory, observing everything, and delivering responses. All the hard parts, done once, done right.

**Stop bolting LLM calls onto webhook handlers. Start orchestrating.**

---

## 🌍 The Mythology

Every name in Eywa carries meaning from the world of Pandora. This isn't cosmetic — the names encode the architecture:

| 🔮 Term | What it is |
|--------|-----------|
| **Weave** | The living network — the runtime engine that connects everything |
| **Spirit** | An AI agent: named, with a system prompt, model config, and allowed actions |
| **Pulse** | A signal entering the Weave — a message, a webhook, a trigger |
| **Oracle** | An LLM provider — the source of wisdom (Anthropic, OpenAI, Gemini, and more) |
| **Action** | A tool the Oracle can invoke — a real-world capability the Spirit wields |
| **Scout** | Enriches a Pulse with context before the Spirit sees it |
| **Pathfinder** | Routes a Pulse to the right Spirit when multiple are available |
| **Voice** | The channel through which a Spirit's response reaches the world |
| **Memory** | Ephemeral conversation state — the Spirit's working memory per user |
| **Echo** | Persisted message history — the permanent record |
| **Chronicle** | Audit log of every interaction — for observability |
| **Bond** | Distributed lock — prevents race conditions across concurrent Pulses |
| **Keeper** | Scheduler backend (e.g. Cloud Tasks) — watches over future events |
| **Ritual** | A scheduled or recurring event — a ceremony the Keeper performs |
| **Archivist** | Summarizes long conversations so the Oracle never loses context |
| **Receptor** | Converts raw webhook payloads into Pulses |
| **Link** | Wires an event type to Scouts, a Pathfinder, and allowed Spirits |
| **Vault** | Object storage for media files (e.g. GCS) |
| **Lens** | Media processor — transcribes audio, analyzes images, extracts documents |
| **Lore** | The knowledge base — documents a Spirit can search at runtime (RAG) |
| **Imprint** | Long-term user memory — facts that persist across all conversations |
| **Ledger** | Token usage and cost tracking — with budgets and smart routing |
| **Vigil** | Human takeover — an operator acquiring a seat on a live conversation |
| **Rite** | Async approval workflow — Spirit waits for human decision before acting |
| **Conduit** | MCP client — connects to external tool servers via Model Context Protocol |

---

## ✨ What you build with Eywa

🤖 **Conversational AI agents** that handle thousands of concurrent users, maintain memory between sessions, call external APIs, and never produce duplicate responses.

🧭 **Multi-agent pipelines** where Pulses route between specialized Spirits — an orchestrator delegates to a researcher, which delegates to a writer — with configurable depth and parallel execution.

📚 **RAG-powered assistants** that search a private knowledge base (Lore) at query time, retrieving the right chunks and injecting them as context before the Oracle reasons.

🧠 **Personalized experiences** where a Spirit remembers user preferences, past goals, and facts (Imprint) — not just from this session, but from every conversation the user ever had.

🙋 **Human-in-the-loop workflows** where critical actions pause for operator approval (Rite) and operators can take over live conversations directly (Vigil) — then hand back to the AI.

🔧 **MCP-native tool use** where Spirits call tools from any Model Context Protocol server (Conduit), discovering capabilities at runtime and calling them like native Actions.

⚡ **Async event processing** where webhooks return in milliseconds and processing happens reliably in the background via Cloud Tasks, with automatic retry and deduplication.

---

## 🌿 Philosophy & The Eywa Mythology

Eywa borrows its name from the neural network connecting all living beings on Pandora — an invisible, intelligent fabric through which information, memory, and intent flow across the entire world.

This framework is built on the same idea: a living orchestration layer that connects AI agents (**Spirits**) to the world through events (**Pulses**), equips them with knowledge (**Lore**, **Imprint**), tools (**Actions**), and senses (**Scouts**) — and delivers their responses through channels (**Voices**).

The mythology is not decoration. Each name encodes an architectural concept so that the codebase reads as a coherent system, not a pile of abstractions:

- **Spirits** carry intent and behavior, not infrastructure. They are named, opinionated agents.
- **Pulses** are the heartbeat of the system — every event that enters the Weave is a Pulse.
- **Oracles** provide wisdom on demand: LLM inference, isolated behind a clean port.
- **Chronicles** preserve the memory of what passed — immutably, for observability and compliance.
- **Bonds** ensure only one thread of thought per session, preventing the chaos of concurrent writes.
- **Keepers** watch over the future, delivering Rituals at the appointed time.

The names are chosen to be memorable, conceptually accurate, and distinct from every other Go framework you have used. Once you internalize the glossary, reading the source code feels like reading documentation.

---

## 🔄 The Pipeline

Every Pulse flows through the same pipeline:

```
Pulse → [Guard] → [Lock] → [Scouts] → [Pathfinder] → Spirit → Oracle → Actions → Voice
```

| Step | Description |
|------|-------------|
| 🛡️ **Guard** | Blocks or allows the Pulse based on allow/block rules |
| 🔒 **Lock** | Acquires a Bond — only one Pulse per user at a time |
| 🔭 **Scouts** | Run sequentially, enriching the Pulse with knowledge |
| 🧭 **Pathfinder** | Selects the right Spirit from the allowed set |
| 👻 **Spirit** | Provides system prompt, model config, and allowed Actions |
| 🔮 **Oracle** | Reasons over memory + Lore, calls Actions in a loop until done |
| ⚡ **Actions** | Execute tool calls (fetch data, send messages, update records) |
| 📢 **Voice** | Delivers the final response through the appropriate channel |

The pipeline also manages memory setup, message coalescing, conversation archiving, human takeover checks, and full persistence — all transparent to your application code.

---

## 📦 Installation

```bash
go get github.com/wmulabs/eywa
```

Sub-modules are opt-in — only include what you need:

```bash
# Infrastructure adapters
go get github.com/wmulabs/eywa/mongo              # MongoDB: Spirits, Echoes, Chronicles, Rituals, Lore, Rites
go get github.com/wmulabs/eywa/redis              # Redis: Memory, Bond, Vigil, rate limiter

# LLM providers
go get github.com/wmulabs/eywa/providers/anthropic # Anthropic Claude (Sonnet, Haiku, Opus)
go get github.com/wmulabs/eywa/providers/openai   # OpenAI GPT — and any OpenAI-compatible API
go get github.com/wmulabs/eywa/providers/gemini   # Google Gemini
go get github.com/wmulabs/eywa/providers/bedrock  # AWS Bedrock (Converse API — any Bedrock model)
go get github.com/wmulabs/eywa/providers/vertexai # Google Vertex AI (ADC auth, no API key)

# REST management API
go get github.com/wmulabs/eywa/fiber              # Fiber: full management REST API

# External tool servers
go get github.com/wmulabs/eywa/mcp               # MCP Conduit: connect to any MCP server

# Channels
go get github.com/wmulabs/eywa/channels/whatsapp  # WhatsApp via 360Dialog / Twilio

# GCP integrations
go get github.com/wmulabs/eywa/gcp/cloudtasks     # Cloud Tasks: async dispatch + Rituals
go get github.com/wmulabs/eywa/gcp/gcs            # GCS Vault for media storage
go get github.com/wmulabs/eywa/gcp/gemini         # Gemini: image/audio/document processing
```

---

## 🚀 Quick Start

> **No infrastructure?** Use `eywa.NewNoOpBond()` for local development and single-instance
> prototyping — no Redis required. For production multi-instance deployments, use
> `redis.NewBondManager()` and MongoDB repositories.
>
> `eywa.NewNoOpBond()` is an in-process mutex-backed Bond — safe for single Weave instance,
> not suitable for horizontal scaling.

```go
package main

import (
    "context"
    "fmt"
    "os"

    eywa "github.com/wmulabs/eywa"
    eywamongo "github.com/wmulabs/eywa/mongo"
    eywaredis "github.com/wmulabs/eywa/redis"
    eywaopenai "github.com/wmulabs/eywa/providers/openai"
)

func main() {
    ctx := context.Background()

    mongoConn, err := eywamongo.NewMongoConnection(ctx, os.Getenv("MONGO_URL"), "mydb", "myapp")
    if err != nil {
        log.Fatalf("failed to connect to MongoDB: %v", err)
    }
    defer mongoConn.DisconnectMongoDB(ctx)

    redisConn, err := eywaredis.NewRedisConnection(ctx, os.Getenv("REDIS_URL"), "myapp")
    if err != nil {
        log.Fatalf("failed to connect to Redis: %v", err)
    }
    defer redisConn.DisconnectRedisDB(ctx)

    db := mongoConn.GetDatabase()

    weave, err := eywa.NewWeaveBuilder(ctx).
        WithRepositories(
            eywamongo.NewSpiritRepository(db),
            eywaredis.NewMemoryRepository(redisConn.GetClient(), "myapp", "prod", 3600, nil),
            eywamongo.NewEchoRepository(db),
            eywamongo.NewChronicleRepository(db),
        ).
        WithBond(eywaredis.NewBondManager(redisConn.GetClient())).
        WithActionRegistry(eywa.NewActionRegistry()).
        WithScoutRegistry(eywa.NewScoutRegistry()).
        AddOracle(eywaopenai.NewOracle(os.Getenv("OPENAI_API_KEY"))).
        WithConfig(eywa.DefaultWeaveConfig()).
        Build()
    if err != nil {
        panic(err)
    }

    weave.RegisterEventConfiguration(
        eywa.NewLink("user_message").
            WithDefaultSpirit("assistant").
            Build(),
    )

    pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_123"}).
        WithUserMessage("What is the status of my order #4821?").
        Build()

    result, _ := weave.ProcessEventByKey(ctx, "user_message", pulse)
    fmt.Println(result.Message)
}
```

> **⚠️ Bond TTL invariant:** `LockTTL` must be at least `ReasoningTimeout + 30 seconds`.
> With the default `ReasoningTimeout` of 2 minutes, the minimum safe `LockTTL` is 2m30s.
> `WeaveConfig.Validate()` enforces this at startup — a misconfigured TTL returns an error
> from `Build()`, not a silent data integrity issue at runtime.

---

## 👻 Defining Spirits

Spirits are the agents in your system. Each has a name, a personality (system prompt), a model configuration, and a list of Actions it is allowed to call.

```go
spirit := &eywa.Spirit{
    Name:         "support_agent",
    Description:  "Customer support specialist",
    SystemPrompt: `You are a helpful support agent for Acme Corp.
You have access to order tracking and refund tools.
Always be concise and professional.`,
    AllowedActions: []eywa.AllowedAction{
        {Name: "track_order"},
        {Name: "request_refund"},
    },
    ModelConfig: eywa.SpiritModel{
        Provider:    "openai",
        Model:       "gpt-4o-mini",
        Temperature: 0.5,
        MaxTokens:   1000,
    },
    IsActive:  true,
    CreatedAt: time.Now(),
}
spiritRepo.Create(ctx, spirit)
```

> **Tip:** Spirits are versioned. Every `Update` call creates a new version. Roll back with `POST /api/v1/spirits/:name/activate` + `{"version": N}`.

---

## ⚡ Custom Actions (Tool Use)

Give Spirits real-world capabilities by implementing the `Action` interface:

```go
type TrackOrderAction struct {
    orderService *OrderService
}

func (a *TrackOrderAction) GetName() string        { return "track_order" }
func (a *TrackOrderAction) GetDescription() string { return "Track a customer order by ID." }
func (a *TrackOrderAction) IsCritical() bool       { return false }

func (a *TrackOrderAction) GetParameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "order_id": map[string]interface{}{
                "type":        "string",
                "description": "The order ID to track",
            },
        },
        "required": []string{"order_id"},
    }
}

func (a *TrackOrderAction) Validate(args map[string]interface{}) error {
    if id, _ := args["order_id"].(string); id == "" {
        return eywa.NewBusinessError("order_id is required")
    }
    return nil
}

func (a *TrackOrderAction) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    status, err := a.orderService.GetStatus(ctx, args["order_id"].(string))
    if err != nil {
        return "", eywa.NewInfrastructureError("failed to fetch order", err)
    }
    return fmt.Sprintf("Order status: %s", status), nil
}
```

```go
registry := eywa.NewActionRegistry()
registry.Register(&TrackOrderAction{orderService: svc})

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithActionRegistry(registry).
    // ...
    Build()
```

**Built-in Actions:**

| Action constructor | Tool name | Description |
|-------------------|-----------|-------------|
| `eywa.NewScheduleRitualAction()` | `schedule_ritual` | Schedule a future Pulse via the Keeper |
| `eywa.NewListRitualsAction()` | `list_rituals` | List pending Rituals for the current user |
| `eywa.NewCancelRitualAction()` | `cancel_ritual` | Cancel a pending Ritual |
| `eywa.NewUpdateSubjectAction()` | `update_subject` | Track a subject key and accumulate facts in Memory |
| `eywa.NewRememberFactAction()` | `remember_fact` | Store a persistent user fact in Imprint |
| `eywa.NewForgetFactAction()` | `forget_fact` | Remove a fact from Imprint |
| `eywa.NewSearchLoreAction()` | `search_lore` | Search the knowledge base (RAG) |
| `eywa.NewRequestRiteAction()` | `request_rite` | Request human approval before proceeding |

---

## 🔭 Context Enrichment with Scouts

Scouts run before Spirit selection and inject knowledge into the Pulse. They are the right place to load user data, feature flags, or external context.

```go
type UserProfileScout struct{ repo *UserRepository }

func (s *UserProfileScout) GetName() string { return "user_profile" }

func (s *UserProfileScout) IsApplicable(pulse *eywa.Pulse) bool {
    return pulse.ContactPhone != ""
}

func (s *UserProfileScout) Harvest(ctx context.Context, pulse *eywa.Pulse) error {
    user, err := s.repo.FindByPhone(ctx, pulse.ContactPhone)
    if err != nil {
        return nil // non-fatal — Pulse continues without this data
    }
    pulse.Knowledge["user_name"]   = user.Name
    pulse.Knowledge["user_tier"]   = user.Tier
    pulse.Knowledge["open_orders"] = user.OpenOrderCount
    return nil
}
```

```go
weave.RegisterEventConfiguration(
    eywa.NewLink("customer_message").
        WithScouts("user_profile", "order_context").
        WithDefaultSpirit("support_agent").
        Build(),
)
```

> Scouts run **sequentially** within `ScoutTimeout` (default 15s). A Scout error is logged but never aborts the pipeline.

---

## 🧭 Multi-Agent Routing with Pathfinders

Route Pulses to the right Spirit automatically based on message content:

```go
// LLM-based routing — a cheap model classifies intent
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithDefaultLLMPathfinder("openai", "gpt-4o-mini", 0.1).
    Build()

weave.RegisterEventConfiguration(
    eywa.NewLink("customer_message").
        WithSpirits("support_agent", "sales_agent", "billing_agent").
        WithDefaultSpirit("support_agent").
        Build(),
)
```

Or implement rule-based routing:

```go
type TierPathfinder struct{}

func (p *TierPathfinder) GetName() string { return "tier_router" }

func (p *TierPathfinder) SelectSpirit(_ context.Context, pulse *eywa.Pulse, available []string) string {
    if tier, _ := pulse.Knowledge["user_tier"].(string); tier == "VIP" {
        return "premium_agent"
    }
    return "standard_agent"
}
```

---

## 🤝 Multi-Agent Orchestration

Build pipelines of specialized Spirits. An Orchestrator dispatches subtasks to Sub-Spirits via the built-in `summon_spirit` tool.

```go
coordinator := &eywa.Spirit{
    Name:        "coordinator",
    Description: "Orchestrates research and writing",
    SystemPrompt: `You coordinate specialist agents.
For each request: summon the researcher, then pass results to the writer.`,
    Type: eywa.SpiritTypeOrchestrator,
    OrchestratorConfig: eywa.OrchestratorConfig{
        SubSpirits:     []string{"researcher", "writer"},
        MaxDepth:       2,
        ParallelSummon: false, // set true for independent parallel tasks
    },
    ModelConfig: eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini"},
    IsActive: true,
}
```

The orchestrator calls `summon_spirit("researcher", task)` like any other tool. Eywa routes the call, returns the result, and the orchestrator continues reasoning.

---

## 📚 RAG with Lore

Give Spirits a searchable knowledge base. Ingest documents and let the Oracle retrieve relevant chunks at query time.

```go
// Ingest documents
loreRepo := eywamongo.NewLoreRepository(db)
lore := &eywa.Lore{
    Name:        "product_docs",
    Description: "Product documentation and FAQs",
    Chunks: []eywa.LoreChunk{
        {Content: "The return policy allows 30-day returns for all items..."},
        {Content: "To track your order, visit mysite.com/track with your order ID..."},
    },
}
loreRepo.Create(ctx, lore)

// Wire to Weave
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithLoreRepository(loreRepo).
    WithLoreEmbedder(myEmbedder).  // implements LoreEmbedder port
    // ...
    Build()

// Spirit can now call search_lore
spirit.AllowedActions = []eywa.AllowedAction{{Name: "search_lore"}}
```

> **Architecture note:** `LoreEmbedder` and `LoreStore` are ports. The MongoDB adapter uses full-text search out of the box. Swap for pgvector, Qdrant, Pinecone, or Weaviate adapters for semantic vector search.

---

## 🧠 Long-Term User Memory with Imprint

Spirits remember user preferences and facts across sessions — not just within a conversation.

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithImprintRepository(eywamongo.NewImprintRepository(db)).
    WithImprintExtraction(eywa.ImprintExtractionConfig{
        Enabled:    true,
        MaxFacts:   50,
        Categories: []string{"preference", "personal", "goal"},
    }).
    // ...
    Build()
```

When `ImprintExtractionConfig.Enabled` is true, the engine automatically extracts and stores facts from user messages. Spirits also have explicit `remember_fact` and `forget_fact` Actions.

A user who says "I prefer formal tone" in one conversation will have that preference injected as context in the next — without the user repeating it.

---

## 🙋 Human-in-the-Loop

### Vigil — Operator Takeover

An operator can acquire an exclusive seat on any live conversation. While the seat is held, the AI is blocked and the operator handles messages directly. The seat has a TTL — it auto-expires if the operator goes silent.

```go
vigilRepo := eywaredis.NewVigilRepository(client, "myapp", "prod")

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithVigilRepository(vigilRepo).
    WithVigilConfig(eywa.VigilConfig{InactivityTimeout: 30 * time.Minute}).
    // ...
    Build()
```

When a Vigil seat is active, `ProcessEventByKey` returns `ErrSessionHeld`. The management API exposes:

```
GET    /api/v1/vigil                       # list all active seats across all sessions
POST   /api/v1/vigil/:memoryKey            # operator takes the seat
POST   /api/v1/vigil/:memoryKey/echoes     # operator sends a message directly
DELETE /api/v1/vigil/:memoryKey            # release — AI resumes
GET    /api/v1/vigil/:memoryKey            # seat status
```

Subscribe to `GET /api/v1/sse/vigil` for real-time `vigil_acquired` / `vigil_released` events across all sessions.

### Rite — Approval Workflow

A Spirit can pause and request human approval before executing a critical action. The Rite is stored in MongoDB and the operator approves or rejects via the API.

```go
// Spirit calls the built-in request_rite action
spirit.AllowedActions = []eywa.AllowedAction{{Name: "request_rite"}}

// Wire the Rite repository
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithRiteRepository(eywamongo.NewRiteRepository(db)).
    // ...
    Build()
```

```
GET  /api/v1/rites              # list pending rites
POST /api/v1/rites/:id/approve  # approve — Spirit resumes execution
POST /api/v1/rites/:id/reject   # reject — Spirit receives the decision and responds
```

Subscribe to `GET /api/v1/sse/rites` for real-time `rite_created` / `rite_decided` / `rite_expired` events.

---

## 🔌 MCP Integration (Conduit)

Connect Spirits to any Model Context Protocol server. Tools are auto-discovered at startup and registered as Eywa Actions with the prefix `<conduit_name>__<tool_name>`.

```go
import eywamcp "github.com/wmulabs/eywa/mcp"

conduit := eywamcp.NewConduit(eywamcp.ConduitConfig{
    Name:      "my_tools",
    Transport: "http",
    URL:       "http://localhost:3001",
    Timeout:   15 * time.Second,
    // Headers: map[string]string{"Authorization": "Bearer " + key},
})

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithConduit(conduit). // tools auto-registered on Build()
    // ...
    Build()

// Spirit references MCP tools by prefixed name
spirit.AllowedActions = []eywa.AllowedAction{
    {Name: "my_tools__search"},
    {Name: "my_tools__create_task"},
}
```

Multiple Conduits can be attached to the same Weave — each with its own namespace.

---

## 💰 Cost Tracking with Ledger

Track token usage per Spirit with monthly budgets and automatic model routing.

```go
ledgerRepo := eywamongo.NewLedgerRepository(db)

// Set a monthly budget with downgrade on exceed
ledgerRepo.SetBudget(ctx, eywa.TokenBudget{
    SpiritID:          "assistant",
    MonthlyTokenLimit: 100_000,
    OnExceed:          "downgrade", // "block" | "downgrade" | "alert"
    DowngradeModel:    eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini"},
    AlertThreshold:    0.8,
})

// Auto-route to cheaper models based on request characteristics
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithLedgerRepository(ledgerRepo).
    WithModelRoutingRules([]eywa.ModelRoutingRule{
        {
            Name:      "long_input",
            Condition: eywa.ModelRoutingCondition{InputLengthGte: 2000},
            Model:     eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini"},
        },
    }).
    // ...
    Build()
```

---

## 🔮 LLM Providers

Eywa supports 8+ LLM providers out of the box. Mix them freely — different models for reasoning, routing, and archiving.

### Native providers

| Provider | Package | Constructor |
|----------|---------|-------------|
| 🟣 Anthropic Claude | `providers/anthropic` | `anthropic.NewOracle(apiKey)` |
| 🟢 OpenAI GPT | `providers/openai` | `openai.NewOracle(apiKey)` |
| 🔵 Google Gemini | `providers/gemini` | `gemini.NewOracle(ctx, apiKey)` |
| 🟠 AWS Bedrock | `providers/bedrock` | `bedrock.NewOracle(ctx, region)` |
| 🔴 Google Vertex AI | `providers/vertexai` | `vertexai.NewOracle(ctx, project, location)` |

### OpenAI-compatible providers (via `providers/openai`)

Any service that speaks the OpenAI API format works as a drop-in Oracle:

```go
import "github.com/wmulabs/eywa/providers/openai"

// Local models
ollama   := openai.NewOllamaOracle("http://localhost:11434")

// Cloud providers
groq     := openai.NewGroqOracle(os.Getenv("GROQ_API_KEY"))
mistral  := openai.NewMistralOracle(os.Getenv("MISTRAL_API_KEY"))
together := openai.NewTogetherOracle(os.Getenv("TOGETHER_API_KEY"))
router   := openai.NewOpenRouterOracle(os.Getenv("OPENROUTER_API_KEY"))
xai      := openai.NewXAIOracle(os.Getenv("XAI_API_KEY"))

// Mix them all
weave, _ := eywa.NewWeaveBuilder(ctx).
    AddOracle(groq).
    AddOracle(mistral).
    AddOracle(ollama).
    Build()
```

Each Spirit's `ModelConfig.Provider` selects which Oracle handles it:

```go
spirit.ModelConfig = eywa.SpiritModel{
    Provider: "groq",
    Model:    "llama-3.3-70b-versatile",
}
```

---

## 🗓️ Conversation Memory & Archiving

Memory is automatic. For long conversations, wire the Archivist to prevent context overflow:

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithDefaultLLMArchivist("anthropic", "claude-haiku-4-5-20251001", 20).
    WithArchivistConfig(0.1, 512).
    Build()
```

When the conversation reaches 20 messages, the Archivist summarizes the oldest half and stores it in Memory. The Oracle receives the summary — the thread of conversation never breaks.

---

## 🌐 REST Management API (Fiber)

Mount a full management API in two lines:

```go
import eywafiber "github.com/wmulabs/eywa/fiber"

app := fiber.New()
eywafiber.RegisterManagementRoutes(app, weave, eywafiber.ManagementDeps{
    APIKeys:            map[string]string{"my-api-key": "admin"},
    OperatorAuth:       eywa.NewOperatorAuth(operatorRepo, []byte(jwtSecret)),
    EchoRepo:           echoRepo,
    EchoQueryRepo:      echoRepo,
    ChronicleQueryRepo: chronicleRepo,
    WeaveConfigRepo:    eywamongo.NewWeaveConfigRepository(db),
    ConfigCache:        eywa.NewConfigCache(linkRepo, nil, nil),
    HTTPToolRepo:       eywamongo.NewHTTPToolRepository(db),
    VigilRepo:          vigilRepo,
    VigilConfig:        eywa.VigilConfig{InactivityTimeout: 30 * time.Minute},
    RiteRepo:           eywamongo.NewRiteRepository(db),
    ImprintRepo:        eywamongo.NewImprintRepository(db),
    PubSub:             eywaredis.NewPubSub(redisClient), // enables SSE + real-time event fanout
})
app.Listen(":8080")
```

Routes registered (all under `/api/v1`, auth required except `/auth/token`):

| Resource | Routes |
|----------|--------|
| 🔍 Discovery | `GET /discovery` — all registered actions, scouts, classifiers, channels, routers |
| 👻 Spirits | `GET/POST /spirits` · `GET/PUT/DELETE /spirits/:name` · `GET /spirits/:name/versions` |
| 📜 Chronicle | `GET /chronicle` · `GET /chronicle/:id` |
| 📊 Analytics | `GET /analytics/tokens` · `/analytics/actions` · `/analytics/spirits` |
| 💬 Conversations | `GET /echoes/sessions` · `GET /echoes/sessions/:key` · `POST /echoes/sessions/:key/messages` |
| 🧠 Imprints | `GET /imprints` (filter by user/spirit/category) · `DELETE /imprints/:id` |
| ⚙️ Config | `GET/PUT /event-configurations/:eventType` · `GET/PUT /admin/engine-config` |
| 🔧 HTTP Tools | `GET/POST /http-tools` · `GET/PUT/DELETE /http-tools/:id` · `POST /http-tools/:id/test` |
| 🙋 Vigil | `GET /vigil` (all active) · `POST/DELETE/GET /vigil/:memoryKey` · `POST /vigil/:memoryKey/echoes` |
| ✅ Rites | `GET /rites` · `GET /rites/:id` · `POST /rites/:id/approve` · `POST /rites/:id/reject` |
| 👤 Operators | `GET/POST /operators` · `GET/PUT/DELETE /operators/:id` |
| 📡 SSE | `GET /sse/rites` · `GET /sse/vigil` · `GET /sse/echoes/:memoryKey` |
| 🔑 Auth | `POST /auth/token` (public) |

> **Security note:** The fiber package also exports `RegisterRoutes`, a lighter alternative that mounts only the event-ingestion and Spirit CRUD endpoints — without authentication middleware. Do not expose `RegisterRoutes` to the public internet without adding your own auth layer or placing the server behind a network boundary. Spirit system prompts are a critical attack surface: an unauthenticated write to `/api/v1/spirits` is prompt injection at the infrastructure level. For production deployments, prefer `RegisterManagementRoutes`, which enforces authentication on all management endpoints.

---

## 📡 Real-Time SSE

When `PubSub` is set in `ManagementDeps`, the management API gains three Server-Sent Events endpoints. The cockpit and any custom dashboard can subscribe to lifecycle events without polling.

```typescript
// Browser — subscribe to Rite lifecycle events
const es = new EventSource('/api/v1/sse/rites', { withCredentials: true })
es.onmessage = (e) => {
    const { event, rite } = JSON.parse(e.data)
    if (event === 'rite_created') showApprovalToast(rite)
}
```

| Endpoint | Events |
|----------|--------|
| `GET /api/v1/sse/rites` | `rite_created` · `rite_decided` · `rite_expired` |
| `GET /api/v1/sse/vigil` | `vigil_acquired` · `vigil_released` |
| `GET /api/v1/sse/echoes/:memoryKey` | `message_added` · `vigil_acquired` · `vigil_released` · `rite_created` |

Backed by Redis PubSub — all events fan out across every running instance. Connection keeps alive with a 30-second heartbeat ping. Nginx buffering is disabled automatically.

---

## 🌿 Roadmap — eywa-cockpit

> *Deep within the Weave, something stirs. The roots of a new tree are taking hold — one that lets you see every Spirit, every Pulse, every Rite and Vigil seat through a single luminous interface.*

**eywa-cockpit** is a full management UI for the Eywa engine. The management API it connects to is already shipped and production-ready (see [Management API](#-management-api--server-sent-events)).

| Feature | Status |
|---------|--------|
| **Hometree** — token usage charts, Spirit health, pending Rites | 🚧 In progress |
| **Spirit Grove** — create and version Spirits with a configuration editor | 📋 Planned |
| **Echo Chamber** — live conversation inspector with Vigil takeover | 📋 Planned |
| **Vigil Watch** — active operator seats, real-time via SSE | 📋 Planned |
| **Rite Chamber** — approval queue with one-click approve/reject | 📋 Planned |
| **Chronicle** — full audit log with cost breakdown | 📋 Planned |
| **Pulse Flows** — visual event routing configuration | 📋 Planned |
| **Conduit Gateway** — HTTP tool builder with live test runner | 📋 Planned |

Follow progress or contribute at [github.com/wmulabs/eywa-cockpit](https://github.com/wmulabs/eywa-cockpit) (coming soon).

---

## 🧪 Examples

All 13 examples are runnable with just MongoDB, Redis, and an LLM API key:

| Example | Concepts |
|---------|---------|
| [`01_basic_setup`](_examples/01_basic_setup/) | Minimal Weave — Spirit, Pulse, `ProcessEventByKey` |
| [`02_custom_actions`](_examples/02_custom_actions/) | Implementing and registering custom Actions (tool use) |
| [`03_advanced_routing`](_examples/03_advanced_routing/) | Scouts + Pathfinders + multi-Spirit routing |
| [`04_async_concept`](_examples/04_async_concept/) | Sync vs async processing comparison |
| [`05_multi_provider`](_examples/05_multi_provider/) | Multiple Oracle providers — Spirits on different models |
| [`06_rag_with_lore`](_examples/06_rag_with_lore/) | RAG: ingest documents, `search_lore` action, `LoreEmbedder` port |
| [`07_human_takeover`](_examples/07_human_takeover/) | Vigil: operator acquires/releases seat, `ErrSessionHeld` |
| [`08_approval_workflow`](_examples/08_approval_workflow/) | Rites: Spirit requests approval, operator decides |
| [`09_long_term_memory`](_examples/09_long_term_memory/) | Imprint: `remember_fact`, auto-extraction, cross-session facts |
| [`10_cost_tracking`](_examples/10_cost_tracking/) | Ledger: `TokenBudget`, `ModelRoutingRule`, usage stats |
| [`11_mcp_client`](_examples/11_mcp_client/) | Conduit: connect to MCP server, auto-discover tools |
| [`12_management_api`](_examples/12_management_api/) | Full Fiber management API with operator auth |
| [`13_multi_agent`](_examples/13_multi_agent/) | Orchestrator Spirit: `summon_spirit`, `OrchestratorConfig` |

---

## 🛠️ Troubleshooting

**`SPIRIT_NOT_FOUND` — Spirit was not returned by the Pathfinder**
A Pulse's event type was matched by a Link, but the Pathfinder selected a Spirit name
that was not registered. Check that `weave.RegisterSpirit()` was called for the Spirit
name returned by your Pathfinder, and that the Spirit's `Activate()` succeeded.

**`ErrSessionHeld` / `ErrMemoryBusy` — Concurrent message for same session**
A second Pulse arrived for a `MemoryKey` while the first was still being processed.
The Bond (distributed lock) is working as intended: the second Pulse is held in the
`Inbox` (if configured) or rejected. The caller should retry after a short delay.
If this happens too frequently, tune `LockTTL` and `ReasoningTimeout`.

**Oracle unavailable (503 / rate limit)**
The `ErrReasoningFailed` error is retriable — the pipeline returns it as a retriable
`OrchestrationError`. If using Cloud Tasks as Keeper, the task will be retried with
exponential backoff automatically. For synchronous calls, check `IsRetriable(err)` and
implement your own retry logic.

**Lock expired before reasoning completed**
Symptom: duplicate responses or interleaved Chronicle entries for the same session.
Cause: `LockTTL` ≤ `ReasoningTimeout`. Fix: set `LockTTL` to at least `ReasoningTimeout + 30s`.
`WeaveConfig.Validate()` enforces this — run it explicitly if you construct config manually.

**Redis connection error on startup**
If using `NoOpBond` for local development, remove the Redis dependency entirely.
If using `redis.NewBondManager()`, ensure the Redis URL is correct and the instance is reachable.
Check `bond.Ping()` in your health check.

---

## 🔒 Security Defaults

Eywa ships with conservative defaults for most settings, but one requires explicit action in production:

**Prompt injection detection is enabled by default.** `InputGuard.PromptInjectionDetection` defaults
to `true` in `DefaultWeaveConfig()`. If you find false positives for your use case, you can disable it:

```go
weave, err := eywa.NewWeaveBuilder(ctx).
    // ... other options ...
    WithInputGuard(eywa.GuardConfig{
        MaxLineCount:             200,  // reject messages with more than N lines (0 = disabled)
        PromptInjectionDetection: false, // disable only if you trust all input sources
    }).
    Build()
```

See [`WithInputGuard`](docs/builder.md#withinputguard) for tuning options.

**What to never log:** Avoid logging raw `Pulse.UserMessage` in production — it may contain PII or
credentials. Eywa's structured logging (Zap) logs metadata but not message content by default.

**Oracle API keys:** Pass via environment variables, never hard-coded. Eywa reads them through the
provider constructors; the values are never stored or logged.

---

## 🏗️ Architecture

Eywa is built on **hexagonal architecture** (ports & adapters). The domain has zero infrastructure dependencies — you swap MongoDB for Postgres, Redis for Valkey, OpenAI for Bedrock, without touching the engine.

```
┌────────────────────────────────────────────────────────────────┐
│                         Your Application                        │
├────────────────────────────────────────────────────────────────┤
│  Fiber routes │ WhatsApp receptor │ Cloud Tasks callback        │
├────────────────────────────────────────────────────────────────┤
│                    Weave (engine core)                          │
│   Pipeline · Memory · Archivist · Pathfinder · Actions          │
├──────────┬──────────┬──────────┬──────────┬────────────────────┤
│  MongoDB │  Redis   │  OpenAI  │ Bedrock  │  Any MCP server    │
│  adapter │  adapter │  Oracle  │  Oracle  │  (via Conduit)     │
└──────────┴──────────┴──────────┴──────────┴────────────────────┘
```

Every external system is behind a port (interface). The `_examples/` directory shows how to compose these adapters for real workloads.

**Guides:**
- [Scaling & Failure Scenarios](docs/scaling.md)
- [Operations Guide](docs/operations.md)

---

## 🤝 Contributing

Every adapter you write extends the Weave. Since everything in Eywa is wired through interfaces, contributing means implementing one port and publishing it as a standalone sub-module — no engine internals required.

**What the community can build:**

| Type | Interface | Examples |
|------|-----------|---------|
| 🔮 LLM Oracle | `eywa.Oracle` | Cohere, DeepSeek, Fireworks AI, Azure OpenAI, llama.cpp |
| 🔍 Vector Store | `eywa.LoreStore` | Chroma, Milvus, OpenSearch, Redis Vector Sets |
| 📣 Channel | `eywa.Voice` + `eywa.Receptor` | Telegram, Slack, Discord, SMS, Email, WeChat |
| 🗄️ Repository | `eywa.*Repository` | PostgreSQL, DynamoDB, Firestore, Valkey |
| ☁️ Cloud | `eywa.Vault` + `eywa.Lens` | S3, Azure Blob, AWS Transcribe |

See [CONTRIBUTING.md](CONTRIBUTING.md) for structure, requirements, and how to publish.

### Running tests

```bash
# All tests
make test

# Tests + coverage summary
make coverage

# Interactive HTML coverage report
make coverage-html
```

**Every PR must include tests.** See [CONTRIBUTING.md → Testing](CONTRIBUTING.md#testing) for the conventions used across this codebase.

---

## ☕ Support

If Eywa saved you time — or just made you think differently about AI infrastructure — consider buying a coffee. It helps keep the Weave growing.

<p align="center">
  <a href="https://github.com/sponsors/wmulabs">
    <img src="https://img.shields.io/badge/Sponsor_on_GitHub-%23EA4AAA?style=for-the-badge&logo=github" alt="GitHub Sponsors"/>
  </a>
  &nbsp;
  <a href="https://buymeacoffee.com/wmulabs">
    <img src="https://img.shields.io/badge/Buy_Me_a_Coffee-%23FFDD00?style=for-the-badge&logo=buymeacoffee&logoColor=black" alt="Buy Me a Coffee"/>
  </a>
</p>

---

## 📜 License

Apache 2.0 — see [LICENSE](LICENSE).

---

<p align="center">
  <sub>🌿 Inspired by the neural network of Pandora. Built for production AI systems in Go.</sub>
</p>
