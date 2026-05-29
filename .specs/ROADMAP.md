# Eywa — Specs Roadmap

## Status Legend

| Symbol | Meaning |
|---|---|
| ✅ | Done |
| 🔄 | In Progress |
| ⬜ | Not started |

---

## v1 — SHIPPED ✅

All v1 library features are implemented. Specs deleted after implementation.

| Spec | Feature |
|---|---|
| ✅ SPEC_12 | Media Pipeline Refactoring |
| ✅ SPEC_14 | Agent Types & Channel Model |
| ✅ SPEC_01 | Multi-Spirit Orchestration (logic / llm_redirect / llm_delegate + parallel) |
| ✅ SPEC_08 | Lore — RAG / Knowledge Base |
| ✅ SPEC_11 | Imprint — Long-Term User Memory |
| ✅ SPEC_15 | Conduit — MCP Client (HTTP JSON-RPC 2.0) |
| ✅ SPEC_16 | Ledger — Cost Intelligence + Model Routing |
| ✅ SPEC_17 | Trial — Evals (5 scorers, YAML/JSON loader) |

---

## Management Layer — Cockpit ✅

All cockpit-layer features are implemented.

| Order | Spec | Status | Notes |
|---|---|---|---|
| 1 | **SPEC_00 — Auth** | ✅ Done | `TokenValidator`, `AuthMiddleware`, `RequireRole`, `JWTValidator` (HS256/RS256), `JWKSValidator` (OIDC), `OperatorAuth`, `APIKeyValidator`. Wired via `ManagementDeps`. |
| 2 | **SPEC_07 — Observability API** | ✅ Done | `/chronicle` (list + detail) + `/analytics/tokens`, `/actions`, `/spirits`. |
| 3 | **SPEC_06 — Conversations API** | ✅ Done | Sessions list/detail, echoes cursor pagination, `POST /echoes/sessions/:memoryKey/messages` (operator send). |
| 4 | **SPEC_03 — Config as a Service** | ✅ Done | EventConfiguration in MongoDB + Redis pub/sub hot-reload. `WeaveConfig` GET/PUT/reload. |
| 5 | **SPEC_02 — HTTP Tools** | ✅ Done | |
| 6 | **SPEC_04 — Human Takeover** | ✅ Done | Vigil (Redis seat), Rite (MongoDB approval flows), `request_rite` action, operator direct message, Pulse resume. Operator CRUD + `POST /auth/token`. |
| 7 | **SPEC_05 — Typing Indicator** | ✅ Done | `TypingIndicator` port, pipeline steps, builder wiring. |
| 8 | **SPEC_10 — Per-Agent Tool Config** | ✅ Done | `AllowedAction{Name, IsCritical *bool, DescriptionOverride}` — overrides applied at cycle start. |

---

## v2 — Launch Readiness

These specs gate public launch. Ordered by priority.

### P0 — Must ship before launch

| Order | Spec | Status | Notes |
|---|---|---|---|
| 1 | **SPEC_09 — LLM Providers** | ✅ Done | OpenAI-compat bridge (`providers/openai` + compat layer); AWS Bedrock (`providers/bedrock`); Vertex AI native (`providers/vertexai`). [Details](SPEC_09_MORE_LLM_PROVIDERS.md) |
| 2 | **SPEC_19 — Docs & Examples** | ✅ Done | README overhaul (mythology storytelling, full feature coverage), examples 05–13, godoc sweep. [Details](SPEC_19_DOCS_AND_EXAMPLES.md) |

### P1 — High value, ship soon after

| Order | Spec | Status | Notes |
|---|---|---|---|
| 3 | **SPEC_18 — Lore Vector Adapters** | ✅ Done | pgvector (`providers/pgvector`), Qdrant (`providers/qdrant`), Pinecone (`providers/pinecone`), Weaviate (`providers/weaviate`) — each is a standalone sub-module. [Details](SPEC_18_LORE_VECTOR_ADAPTERS.md) |
| 4 | **SPEC_13 — Management UI** | ⬜ Not started | Standalone cockpit web app (`eywa-cockpit` repo). After P0 ships. [Details](SPEC_13_MANAGEMENT_UI.md) |

---

## Dependency Graph

```
SPEC_09 (LLM Providers) — independent
SPEC_18 (Vector Adapters) — independent
SPEC_19 (Docs) — depends on SPEC_09 (examples need providers)
SPEC_13 (UI) — after all P0
```

---

## Launch Checklist

- [x] SPEC_09 — OpenAI-compat (Ollama, Groq, Mistral, Together, xAI, OpenRouter)
- [x] SPEC_09 — AWS Bedrock
- [x] SPEC_09 — Vertex AI native
- [x] SPEC_18 — pgvector adapter
- [x] SPEC_18 — Qdrant adapter
- [x] SPEC_18 — Pinecone adapter
- [x] SPEC_18 — Weaviate adapter
- [x] SPEC_19 — README overhaul
- [x] SPEC_19 — Examples 05–13
- [x] SPEC_19 — Godoc sweep
- [ ] SPEC_13 — Management UI (eywa-cockpit)
