# Example 13: Multi-Agent Orchestration

Demonstrates Eywa's **multi-spirit orchestration** — an orchestrator spirit delegating tasks to specialist spirits.

## What this shows

- `SpiritTypeOrchestrator` with the built-in `summon_spirit` tool
- `OrchestratorConfig` controlling which spirits can be summoned, delegation depth, and parallelism
- Sequential orchestration: researcher → writer pipeline
- `MaxDepth` preventing infinite delegation chains

## Key concepts

### Orchestrator spirit

```go
coordinator := &eywa.Spirit{
    Type: eywa.SpiritTypeOrchestrator,
    OrchestratorConfig: eywa.OrchestratorConfig{
        SubSpirits:     []string{"researcher", "writer"},
        MaxDepth:       2,
        ParallelSummon: false,  // true = concurrent dispatch
    },
}
```

Setting `Type: SpiritTypeOrchestrator` enables the `summon_spirit` built-in tool. The orchestrator calls it like any other tool — the engine routes the call to the named sub-spirit.

### OrchestratorConfig fields

| Field | Description |
|---|---|
| `SubSpirits` | Spirits this orchestrator can summon (whitelist) |
| `MaxDepth` | Max delegation chain depth (prevents infinite loops) |
| `ParallelSummon` | `true` = dispatch all summons concurrently |

### Research pipeline (this example)

```
coordinator
  1. summon_spirit("researcher", question)  → structured research summary
  2. summon_spirit("writer", research)      → polished prose response
  → return writer output to user
```

### Parallel summon

Set `ParallelSummon: true` for independent tasks (e.g., translate the same text into multiple languages simultaneously).

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```

## Architecture note

Sub-spirits are regular spirits — they can have their own tools, model configs, and memory. The orchestrator pattern composes existing spirits rather than requiring special types.
