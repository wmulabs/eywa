# 🌿 Example 01: Basic Setup

**Complexity:** ⭐ Beginner  
**Time:** ~10 minutes

Minimal setup to process a Pulse through Eywa's pipeline.

---

## 📖 What You'll Learn

- Connect to MongoDB and Redis via opt-in sub-modules
- Assemble a Weave with `WeaveBuilder`
- Register a Spirit and a Link
- Build a Pulse with the Pulse builder
- Process a Pulse and inspect the `Response`

---

## 🔄 What Happens

```
Pulse → Validation → Lock → Spirit Selection → Spirit Load
      → Session Setup → Reasoning (Oracle) → Persistence
      → Chronicle
```

The Spirit receives the Pulse, reasons with the LLM, and returns a Response. Memory is stored in Redis for future turns.

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
go run ./01_basic_setup/main.go
```

---

## 📊 Expected Output

```
=== Eywa — Basic Setup ===
Weave ready

Status:   success
Spirit:   assistant
Time:     1842ms
Reply:    Eywa is a Go framework for building event-driven, multi-agent AI systems...
```

---

## 🧠 Key Concepts

### MemoryKey

Every Pulse belongs to a `MemoryKey{Channel, User}`. This is the identity of the conversation — Redis stores memory per this key. The composite key `channel:user` scopes the Spirit's working memory.

```go
pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_001"}).
    WithUserMessage("What is Eywa?").
    WithSource("api").
    Build()
```

### Link

Links wire an event type (string key) to processing configuration — which Spirit handles it, which Scouts run, which Voice responds.

```go
weave.RegisterEventConfiguration(
    eywa.NewLink("user_message").
        WithDefaultAgent("assistant").
        Build(),
)
```

### Response

The pipeline returns a `Response` with status, Spirit used, processing time, and the LLM's message.

```go
result.Status           // "success" | "partial_success" | "failed"
result.SpiritUsed       // "assistant"
result.ProcessingTimeMs // e.g. 1842
result.Message          // the LLM's response text
```

---

## ➡️ Next Steps

- [02 — Custom Actions](../02_custom_actions/) — give your Spirit real-world capabilities
- [03 — Advanced Routing](../03_advanced_routing/) — multiple Spirits, Scouts, Pathfinders
- [04 — Sync vs Async](../04_async_concept/) — production-scale event processing
