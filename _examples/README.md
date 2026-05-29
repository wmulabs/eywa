# 🧪 Eywa Examples

Runnable examples demonstrating Eywa's core concepts, from minimal setup to advanced multi-agent routing.

---

## 📋 Examples

### [01 — Basic Setup](./01_basic_setup/)
**Complexity:** ⭐ Beginner · **Time:** ~10 minutes

Minimal Weave setup: connect to MongoDB + Redis, register a Spirit, process a Pulse.

```bash
export OPENAI_API_KEY="sk-..."
go run ./01_basic_setup/main.go
```

---

### [02 — Custom Actions](./02_custom_actions/)
**Complexity:** ⭐⭐ Intermediate · **Time:** ~20 minutes

Implement the `Action` interface and register tools that the Oracle can invoke during reasoning. Demonstrates weather retrieval, math calculation, and multi-action turns.

```bash
export OPENAI_API_KEY="sk-..."
go run ./02_custom_actions/main.go
```

---

### [03 — Advanced Routing](./03_advanced_routing/)
**Complexity:** ⭐⭐⭐ Advanced · **Time:** ~30 minutes

Context enrichment with Scouts + intelligent Spirit selection with a Pathfinder. Three specialized Spirits (support, sales, billing) with a keyword-based router.

```bash
export OPENAI_API_KEY="sk-..."
go run ./03_advanced_routing/main.go
```

---

### [04 — Sync vs Async](./04_async_concept/)
**Complexity:** ⭐⭐ Intermediate · **Time:** ~15 minutes

SYNC processing with `ProcessMultipleEventsByKey` plus explanation of the ASYNC pattern (Cloud Tasks Keeper) for production webhooks.

```bash
export OPENAI_API_KEY="sk-..."
go run ./04_async_concept/main.go
```

---

## 🚀 Quick Setup

### Infrastructure

```bash
docker run -d -p 27017:27017 --name mongodb mongo:latest
docker run -d -p 6379:6379 --name redis redis:latest
```

### Environment Variables

```bash
# Required
export OPENAI_API_KEY="sk-..."

# Optional — defaults shown
export MONGO_URL="mongodb://localhost:27017"
export MONGO_DATABASE="eywa_example"
export REDIS_URL="redis://localhost:6379"
export SERVICE_NAME="eywa"
export ENVIRONMENT="lcl"
```

### Run from the examples directory

All examples share a single `go.mod` in `_examples/`. Run from there:

```bash
cd _examples

go run ./01_basic_setup/main.go
go run ./02_custom_actions/main.go
go run ./03_advanced_routing/main.go
go run ./04_async_concept/main.go
```

---

## 📚 Learning Path

```
01_basic_setup
    ↓ learn: Weave, Spirit, Pulse, Link, MemoryKey
02_custom_actions
    ↓ learn: Action interface, ActionRegistry, error classification
03_advanced_routing
    ↓ learn: Scout, Pathfinder, multi-Spirit Links
04_async_concept
    ↓ learn: ProcessMultipleEventsByKey, SYNC vs ASYNC, Keeper
```

---

## 🔑 Key Concepts Across Examples

| Concept | Introduced in |
|---------|--------------|
| `WeaveBuilder` | 01 |
| `Spirit` + `Link` | 01 |
| `Pulse` builder | 01 |
| `MemoryKey{Channel, User}` | 01 |
| `Action` interface | 02 |
| `BusinessError` / `InfrastructureError` | 02 |
| `Scout` interface | 03 |
| `Pathfinder` interface | 03 |
| `ProcessMultipleEventsByKey` | 04 |
| ASYNC via Keeper | 04 |

---

## 🐛 Troubleshooting

**MongoDB connection fails:**
```bash
docker ps | grep mongodb
docker start mongodb
```

**Redis connection fails:**
```bash
docker ps | grep redis
docker start redis
```

**Spirit already exists (after first run):**
Safe to ignore — `Create` logs a note but processing continues normally. To reset:
```bash
mongosh --eval 'use eywa_example; db.dropDatabase()'
```

**OpenAI errors:**
- Verify key: `echo $OPENAI_API_KEY`
- Check credits: https://platform.openai.com/usage
- `gpt-4o-mini` is cheapest and supports function calling

---

## 📖 Documentation

- [Architecture](../docs/architecture.md) — pipeline, hexagonal structure, core entities
- [Builder Reference](../docs/builder.md) — every WeaveBuilder option
- [Concepts & Interfaces](../docs/concepts.md) — implement Action, Scout, Pathfinder, Voice, Receptor
- [Sub-modules](../docs/sub-modules.md) — MongoDB, Redis, GCP, Fiber, WhatsApp setup
