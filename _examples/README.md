# 🧪 Eywa Examples

Runnable examples demonstrating Eywa's core concepts, from minimal setup to advanced multi-agent routing.

---

## 📋 Examples

| # | Example | Complexity | What it shows |
|---|---------|-----------|---------------|
| 01 | [Basic Setup](./01_basic_setup/) | ⭐ Beginner | Minimal Weave — connect MongoDB + Redis, register a Spirit, process a Pulse |
| 02 | [Custom Actions](./02_custom_actions/) | ⭐⭐ Intermediate | Implement the `Action` interface; tools the Oracle invokes; multi-action turns |
| 03 | [Advanced Routing](./03_advanced_routing/) | ⭐⭐⭐ Advanced | Scouts (context enrichment) + Pathfinder (Spirit selection) across specialist Spirits |
| 04 | [Sync vs Async](./04_async_concept/) | ⭐⭐ Intermediate | `ProcessMultipleEventsByKey` and the async Cloud Tasks (Keeper) pattern |
| 05 | [Multi-Provider](./05_multi_provider/) | ⭐⭐ Intermediate | One Weave running Spirits on different LLM providers simultaneously |
| 06 | [RAG with Lore](./06_rag_with_lore/) | ⭐⭐⭐ Advanced | RAG: ingest documents, `LoreEmbedder` port, `search_lore` action |
| 07 | [Human Takeover](./07_human_takeover/) | ⭐⭐⭐ Advanced | Vigil: operator acquires/releases a seat; `ErrSessionHeld` |
| 08 | [Approval Workflow](./08_approval_workflow/) | ⭐⭐⭐ Advanced | Rites: a Spirit requests approval for a high-stakes action; operator decides |
| 09 | [Long-Term Memory](./09_long_term_memory/) | ⭐⭐ Intermediate | Imprint: `remember_fact`, auto-extraction, cross-session facts |
| 10 | [Cost Tracking](./10_cost_tracking/) | ⭐⭐ Intermediate | Ledger: `TokenBudget`, `ModelRoutingRule`, usage stats |
| 11 | [MCP Client](./11_mcp_client/) | ⭐⭐⭐ Advanced | Conduit: connect to an MCP server, auto-discover tools as Actions |
| 12 | [Management API](./12_management_api/) | ⭐⭐⭐ Advanced | The `eywa/fiber` REST management layer with operator auth |
| 13 | [Multi-Agent](./13_multi_agent/) | ⭐⭐⭐ Advanced | Orchestrator Spirit delegating via `summon_spirit` / `OrchestratorConfig` |
| 14 | [Lore Matching](./14_lore_matching/) | ⭐⭐⭐ Advanced | Lore as a queryable store: `IngestObject`, `SearchLore`, `LoreFilter`, `GroupByDocument` |

Each example has its own `README.md` with a deeper walkthrough. All run with just MongoDB, Redis, and an
LLM API key.

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
go run ./05_multi_provider/main.go
go run ./06_rag_with_lore/main.go
go run ./07_human_takeover/main.go
go run ./08_approval_workflow/main.go
go run ./09_long_term_memory/main.go
go run ./10_cost_tracking/main.go
go run ./11_mcp_client/main.go
go run ./12_management_api/main.go
go run ./13_multi_agent/main.go
go run ./14_lore_matching/main.go
```

---

## 📚 Learning Path

```
Foundations
  01_basic_setup      → Weave, Spirit, Pulse, Link, MemoryKey
  02_custom_actions   → Action interface, ActionRegistry, error classification
  03_advanced_routing → Scout, Pathfinder, multi-Spirit Links
  04_async_concept    → ProcessMultipleEventsByKey, SYNC vs ASYNC, Keeper

Providers & knowledge
  05_multi_provider   → multiple Oracles, per-Spirit models
  06_rag_with_lore    → RAG, LoreEmbedder, search_lore action
  14_lore_matching    → IngestObject, SearchLore, filters, GroupByDocument

Human-in-the-loop & memory
  07_human_takeover   → Vigil seat acquire/release
  08_approval_workflow→ Rites approval gating
  09_long_term_memory → Imprint persistent facts

Production
  10_cost_tracking    → Ledger budgets, model routing
  11_mcp_client       → Conduit / MCP tool discovery
  12_management_api    → Fiber REST management layer
  13_multi_agent      → orchestrator + summon_spirit
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
| Multiple Oracle providers | 05 |
| RAG via `search_lore` action | 06 |
| `LoreEmbedder` port | 06 |
| Vigil (human takeover) | 07 |
| Rite (approval workflow) | 08 |
| Imprint (long-term facts) | 09 |
| Ledger (`TokenBudget`, `ModelRoutingRule`) | 10 |
| Conduit (MCP client) | 11 |
| Fiber management API | 12 |
| `summon_spirit` / `OrchestratorConfig` | 13 |
| `IngestObject` / `SearchLore` (matching) | 14 |
| `LoreFilter` / `GroupByDocument` | 14 |

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
- [Authentication & Security](../docs/authentication.md) — management vs. event auth, app tokens, webhook signatures
- [REST API](../docs/rest-api.md) — endpoint reference
- [Sub-modules](../docs/sub-modules.md) — MongoDB, Redis, GCP, Fiber, WhatsApp setup
