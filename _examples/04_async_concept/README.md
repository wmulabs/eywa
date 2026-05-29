# ⚡ Example 04: Sync vs Async Processing

**Complexity:** ⭐⭐ Intermediate  
**Time:** ~15 minutes

Understand the two processing modes in Eywa and when to use each.

---

## 📖 What You'll Learn

- SYNC processing: caller waits for results
- ASYNC processing: caller gets 200 OK fast, Weave processes in background via Keeper
- `ProcessMultipleEventsByKey` for handling multiple concurrent Pulses
- When to choose each mode

---

## 🔄 SYNC vs ASYNC

### SYNC (this example)

```
Client → ProcessEventByKey(ctx, key, pulse)
                │
                ▼ (waits)
         Full pipeline runs
                │
                ▼
         Response returned to caller
```

Good for: development, testing, low-volume use cases, direct integrations.

### ASYNC (production)

```
Client → POST /api/v1/events/:event_key/async
                │
                ▼ (< 100ms)
         Keeper.Schedule(now)   ← Cloud Tasks task created
                │
         Client receives 200 OK immediately
                │
                ▼ (seconds later, via Cloud Tasks)
         POST /internal/execute-event
                │
                ▼
         Full pipeline runs (retry on failure)
```

Good for: production, high-volume webhooks, WhatsApp, any channel where you must respond in < 1s.

---

## ⚡ ProcessMultipleEventsByKey

This example uses `ProcessMultipleEventsByKey` — processes multiple Pulses concurrently using goroutines. Each Pulse has a different `MemoryKey` (different user), so they run in parallel without locking conflicts.

```go
results, err := weave.ProcessMultipleEventsByKey(ctx, "demo_message", pulses)
```

> [!NOTE]
> Pulses with the **same** `MemoryKey` are serialized by the Bond (distributed lock) — only one processes at a time. Pulses with **different** `MemoryKey`s run fully concurrently.

---

## 🏗️ Setting Up ASYNC in Production

```go
import "github.com/wmulabs/eywa/gcp/cloudtasks"

keeper, _ := cloudtasks.NewCloudTasksKeeper(ctx, cloudtasks.CloudTasksConfig{
    Project:        "my-gcp-project",
    Location:       "us-central1",
    Queue:          "eywa-events",
    TargetBaseURL:  "https://my-service.run.app",
    TargetAudience: "https://my-service.run.app",
})

weave, _ := eywa.NewWeaveBuilder(ctx).
    // ... repositories, bond, oracle ...
    WithAsyncDispatch(keeper).  // enables POST /api/v1/events/:key/async
    Build()
```

The Fiber handler automatically dispatches to Cloud Tasks when `WithAsyncDispatch` is wired. The `/internal/execute-event` endpoint processes the Keeper callbacks.

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
go run ./04_async_concept/main.go
```

---

## 📊 Expected Output

```
=== Eywa — Sync vs Async Processing ===

Processing 3 Pulses concurrently (SYNC mode)...
Caller is waiting for all results...

Completed in 3.2s — 3/3 succeeded

[1] What is 5 + 7?
    → 5 + 7 equals 12.

[2] What is the capital of Brazil?
    → The capital of Brazil is Brasília.

[3] Who wrote Hamlet?
    → Hamlet was written by William Shakespeare.

─────────────────────────────────────────
SYNC  — client waits, immediate results
        good for: dev, testing, low-volume

ASYNC — POST /api/v1/events/:key/async
        returns 200 OK in < 100ms
        Keeper (Cloud Tasks) fires processing later
        good for: production, high-volume webhooks
        built-in: retry, deduplication, OIDC auth
```

---

## 📊 Comparison Table

| | SYNC | ASYNC |
|--|------|-------|
| Webhook response time | Full processing time | < 100ms |
| Retry on failure | Manual | Automatic (Cloud Tasks) |
| Deduplication | `IdempotencyKey` | `IdempotencyKey` + Cloud Tasks |
| Scale | Limited by timeout | Scales horizontally |
| Good for | Dev / low-volume | Production / high-volume |
| Extra infra | None | Cloud Tasks queue |

---

## ➡️ Next Steps

- [Builder Reference](../../docs/builder.md) — `WithAsyncDispatch`, `WithRitualManager`
- [Sub-modules](../../docs/sub-modules.md) — `gcp/cloudtasks` setup
- [Architecture](../../docs/architecture.md) — async processing flow diagram
