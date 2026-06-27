# 🤝 Multi-agent

Eywa offers two complementary ways to compose Spirits. They are building blocks — pick whichever fits
the flow, or combine them.

| Pattern | Control flow | Who answers the user | Use when |
|---|---|---|---|
| **Orchestration** (summon) | Call-and-return within one turn | The orchestrator, after synthesizing sub-results | A manager coordinates specialists and composes a single answer |
| **Handoff** | Transfer of control, persisted across turns | The target Spirit, directly | Triage routes to a specialist that then owns the conversation |

---

## Orchestration (summon)

An `orchestrator` Spirit delegates sub-tasks to `SubSpirits` and composes their results. Delegation is
either deterministic (a `LogicRouter`) or LLM-driven (the auto-injected `summon_spirit` action). Each
summon runs the sub-Spirit's pipeline with a fresh, isolated context and returns a result string to the
orchestrator, which stays in control. Bounded by `OrchestratorConfig.MaxDepth`; `ParallelSummon` runs
independent summons concurrently.

```go
spirit.Type = eywa.SpiritTypeOrchestrator
spirit.OrchestratorConfig = eywa.OrchestratorConfig{
    SubSpirits:     []string{"researcher", "writer"},
    MaxDepth:       3,
    ParallelSummon: true,
}
```

## Handoff

A handoff transfers control to a peer Spirit. Unlike summon, it **pins** the target as the session's
active Spirit (via a `HandoffStore`), so the target answers the current turn **and** every subsequent
turn until it hands off again. Decentralized: each leading Spirit decides, via the auto-injected
`handoff_spirit` action, limited to its own `AllowedTargets`.

```go
builder.WithHandoffStore(eywa.NewInMemoryHandoffStore()) // or redis/mongo adapter for multi-instance

triage.HandoffConfig = eywa.HandoffConfig{
    AllowedTargets:  []string{"billing", "sales"},
    ContextTransfer: eywa.HandoffContextSession, // "session" | "summary" | "none"
    MaxHandoffs:     3,                           // per-turn relay-loop guard
}
```

Wire a `HandoffStore` or handoff is disabled (behaviour unchanged). Adapters:

- `eywa.NewInMemoryHandoffStore()` — single-instance / tests.
- `eywaredis.NewHandoffStore(client, ttl)` — multi-instance; `ttl` bounds pin lifetime (0 = no expiry).
- `eywamongo.NewHandoffStore(db)` — multi-instance, durable.

### Context transfer

`ContextTransfer` controls what the receiver inherits — the **session conversation**, never the sending
Spirit's own knowledge (the receiver always uses its own Lore/Imprint):

- `session` — the full conversation history.
- `summary` — an Archivist-compressed summary (requires `WithArchivist`; falls back to full session).
- `none` — the receiver starts clean.

### Loop protection

`MaxHandoffs` bounds transfers **within a single turn** (A→B→C…), so a transfer-and-continue chain can't
relay forever — on the cap the handoff is rejected and the current Spirit answers. Handoffs across
separate user turns are user-driven and not counted. To return to a previous Spirit, list it as an
allowed target so the specialist can hand back.

### Coverage caveat

A handoff sets the pin **before** running the target, so the pin survives even if the target's turn then
fails — the conversation still routes to the target next turn. The target's answer is delivered and
persisted to the shared session, so the conversation stays continuous regardless of transfer mode.
