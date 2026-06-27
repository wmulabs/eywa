# ⚖️ Eywa vs other Go agent frameworks

An honest comparison. The frameworks that matter for "best option in **Go**" are not Python stacks like
LangChain — they are Eino, Genkit Go, and LangChainGo. Each is strong at something; this page is meant
to help you pick, not to win an argument.

**One-line positioning:** Eywa is the **production runtime** for conversational agents — locking,
idempotency, human-in-the-loop, cost governance, and versioning included. The others are excellent
*developer frameworks*; Eywa is the thing you run when an agent has to survive real traffic.

---

## At a glance

| | **Eywa** | **Eino** (CloudWeGo) | **Genkit Go** (Google) | **LangChainGo** |
|---|---|---|---|---|
| Primary focus | Production conversational runtime | Graph orchestration framework | Dev framework + tooling | LLM building blocks |
| Graph/DAG orchestration | Pipeline + multi-agent (summon/handoff) | ✅ First-class, type-safe | Flows | Chains |
| Streaming | ✅ Oracle → reasoning → SSE | ✅ First-class | ✅ | Partial |
| Structured output | ✅ Cross-provider (native + validate) | ✅ | ✅ | Partial |
| Dev UI / eval tooling | Eval/replay over audit log | — | ✅ Dev UI + eval (brand strength) | — |
| Observability | ✅ Langfuse + OTel GenAI semconv | Callbacks | ✅ OTel | Partial |
| **Distributed locking** | ✅ Bond (Redlock) | — | — | — |
| **Idempotency / dedup** | ✅ IdempotencyStore | — | — | — |
| **Message coalescing** | ✅ Inbox | — | — | — |
| **Human-in-the-loop** | ✅ Vigil (takeover) + Rite (approvals) | — | — | — |
| **Cost governance** | ✅ Ledger: budgets + auto-downgrade | — | — | — |
| **Guardrails** | ✅ PII redaction + denylist + jailbreak | — | partial | — |
| Spirit (agent) versioning | ✅ Versioned, audited | — | — | — |
| Management REST API | ✅ ~21 endpoints (Fiber) | — | — | — |
| Channels | WhatsApp (360dialog, Twilio) | — | — | — |
| Providers | 4 native + OpenAI-compat (Ollama/Groq/Azure/…) | Several | Several (Google-leaning) | Broadest |
| RAG / vector stores | ✅ pgvector, pinecone, qdrant, weaviate + embedder | — | partial | ✅ Many |
| MCP client | ✅ Conduit | — | partial | partial |
| Durable execution | ✅ Checkpoint + resume | — | — | — |

> Matrix reflects each project's stated focus; verify current status against upstream before deciding.

---

## When to pick each

**Pick Eywa** when the agent is a product that must run reliably: concurrent users, no duplicate
replies, human takeover, approval gates, per-agent budgets, an audit trail, and a management API — out
of the box. This is exactly the layer the others leave to you.

**Pick Eino** when you want a pure, type-safe **graph/dataflow** engine with first-class streaming and
you are happy to build the runtime concerns (locking, sessions, HITL, channels) yourself.

**Pick Genkit Go** when **developer experience and tooling** lead — the dev UI, evaluation, and Google
ecosystem integration are its strengths. Its conversational-runtime and state concerns are shallow.

**Pick LangChainGo** when you want the **broadest provider/integration breadth** and a familiar
LangChain mental model, and you are assembling your own opinionated stack.

---

## Where Eywa is intentionally not the leader

Honesty buys credibility:

- **Graph-style orchestration** is not Eywa's primary model — it favors a pipeline + reasoning loop
  with summon/handoff. For arbitrary DAGs, Eino is more natural.
- **Dev UI / eval tooling** is less polished than Genkit's. Eywa's eval is replay over the immutable
  audit log (Chronicle), which is a different (and arguably stronger for regression) approach.
- **Provider/integration breadth** is narrower than LangChainGo's long tail.

The bet: the production-runtime concerns are the hard, valuable part and the part nobody else in Go
ships ready. Everything above is a deliberate scope choice, not an accident.
