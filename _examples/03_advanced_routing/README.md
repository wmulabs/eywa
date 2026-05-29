# 🧭 Example 03: Advanced Routing

**Complexity:** ⭐⭐⭐ Advanced  
**Time:** ~30 minutes

Context enrichment with Scouts and intelligent Spirit selection with Pathfinders. Three specialized Spirits, each handling a different domain.

---

## 📖 What You'll Learn

- Implement `Scout` to enrich Pulses with knowledge before Spirit selection
- Implement `Pathfinder` to route Pulses to the right Spirit
- Register multiple Spirits with distinct system prompts and model configs
- Wire Scouts + Pathfinder + multiple Spirits in a `Link`
- Understand the enrichment → selection → reasoning flow

---

## 🔄 The Routing Flow

```
Pulse
  ↓
[Enrichment] — Scouts run concurrently
  • user_context      → pulse.Knowledge["user_tier"] = "standard"
  • session_history   → pulse.Knowledge["message_count"] = 3
  ↓
[SpiritSelection] — Pathfinder reads pulse + available Spirits
  • keyword_pathfinder scans user message for domain keywords
  • billing keywords → billing_agent
  • sales keywords   → sales_agent
  • support keywords → support_agent
  ↓
[Reasoning] — selected Spirit processes with LLM
```

---

## 👻 The Three Spirits

| Spirit | Domain | Temperature |
|--------|--------|-------------|
| `support_agent` | Technical issues, bugs, crashes | 0.5 |
| `sales_agent` | Products, plans, pricing | 0.7 |
| `billing_agent` | Payments, invoices, charges | 0.3 |

Each Spirit has its own system prompt, model config, and allowed Actions — fully independent configuration.

---

## 🔭 Scout Interface

```go
type Scout interface {
    GetName() string
    IsApplicable(pulse *Pulse) bool   // return false to skip for irrelevant Pulses
    Harvest(ctx context.Context, pulse *Pulse) error  // write to pulse.Knowledge
}
```

> [!TIP]
> Scouts run **concurrently** within `ScoutTimeout` (default 15s). A Scout error is logged but never aborts the pipeline — always return `nil` for non-fatal failures.

> [!IMPORTANT]
> Write enriched data to `pulse.Knowledge` — this reaches the Oracle. Never write to `pulse.Metadata` (audit-only, never sent to LLM).

---

## 🧭 Pathfinder Interface

```go
type Pathfinder interface {
    GetName() string
    SelectSpirit(ctx context.Context, pulse *Pulse, availableSpirits []string) string
}
```

Return a Spirit name from `availableSpirits`. Empty string → Weave uses `DefaultAgent`.

---

## 🔗 Link Configuration

```go
weave.RegisterEventConfiguration(
    eywa.NewLink("customer_message").
        WithScouts("user_context", "session_history").  // run in this order
        WithPathfinder("keyword_pathfinder").
        WithAgents("support_agent", "sales_agent", "billing_agent").
        WithDefaultAgent("support_agent").              // fallback
        Build(),
)
```

**Spirit selection logic:**

| `AllowedAgents` count | Behavior |
|----------------------|----------|
| 0 | Use `DefaultAgent` directly |
| 1 | Use it directly (no Pathfinder) |
| 2+ | Run Pathfinder, fall back to `DefaultAgent` |

---

## ⚙️ Prerequisites

- Go 1.25+
- MongoDB and Redis running locally
- OpenAI API key

```bash
docker run -d -p 27017:27017 --name mongodb mongo:latest
docker run -d -p 6379:6379 --name redis redis:latest
```

---

## 🔑 Environment Variables

```bash
export OPENAI_API_KEY="sk-..."

# Optional — defaults shown
export MONGO_URL="mongodb://localhost:27017"
export MONGO_DATABASE="eywa_example"
export REDIS_URL="redis://localhost:6379"
export SERVICE_NAME="eywa"
export ENVIRONMENT="lcl"
```

---

## 🚀 Run

```bash
cd _examples
go run ./03_advanced_routing/main.go
```

---

## 📊 Expected Output

```
=== Eywa — Advanced Routing ===

--- Support routing ---
Spirit: support_agent
Reply:  I understand you're experiencing crashes when uploading files. Let me help you troubleshoot...

--- Sales routing ---
Spirit: sales_agent
Reply:  Great question! Our premium plan includes many powerful features...

--- Billing routing ---
Spirit: billing_agent
Reply:  I apologize for the inconvenience. Let me investigate the duplicate charge on your account...
```

---

## 🚀 Going Further

**LLM-based routing** — use `WithDefaultLLMPathfinder` instead of implementing your own:

```go
weave, _ := eywa.NewWeaveBuilder(ctx).
    WithDefaultLLMPathfinder("openai", "gpt-4o-mini", 0.1). // cheap + fast
    Build()
```

The built-in Pathfinder sends Spirit names + descriptions to the LLM and asks it to classify the intent. No keyword lists to maintain.

---

## ➡️ Next Steps

- [04 — Sync vs Async](../04_async_concept/) — production-scale event processing
- [Builder Reference](../../docs/builder.md) — full WeaveBuilder options
- [Concepts & Interfaces](../../docs/concepts.md) — detailed Scout, Pathfinder, Voice docs
