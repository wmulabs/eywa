# Example 10: Cost Tracking (Ledger)

Demonstrates **Ledger** — Eywa's built-in token usage and cost tracking.

## What this shows

- `LedgerRepository` storing monthly token usage per spirit
- `TokenBudget` setting a monthly token limit with configurable enforcement
- `ModelRoutingRule` automatically routing to cheaper models based on request characteristics
- Reading usage and budget via `ledgerRepo.GetMonthUsage` / `GetBudget`

## Key concepts

### TokenBudget

```go
eywa.TokenBudget{
    SpiritID:          "assistant",
    MonthlyTokenLimit: 100_000,
    OnExceed:          "downgrade",  // "block" | "downgrade" | "alert"
    DowngradeModel: eywa.SpiritModel{
        Provider: "openai", Model: "gpt-4o-mini",
    },
    AlertThreshold: 0.8,  // fire hook at 80% usage
}
```

### ModelRoutingRule

Rules evaluated in order; first match wins. Useful for automatic cost optimization:

```go
[]eywa.ModelRoutingRule{
    {
        Name:      "long_input_downgrade",
        Condition: eywa.ModelRoutingCondition{InputLengthGte: 2000},
        Model:     eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini"},
    },
    {
        Name:      "vision_upgrade",
        Condition: eywa.ModelRoutingCondition{HasAttachments: true},
        Model:     eywa.SpiritModel{Provider: "openai", Model: "gpt-4o"},
    },
}
```

### LedgerRepository

```go
type LedgerRepository interface {
    Track(ctx context.Context, entry LedgerEntry) error
    GetMonthUsage(ctx context.Context, spiritID, month string) (*LedgerEntry, error)
    GetBudget(ctx context.Context, spiritID string) (*TokenBudget, error)
    SetBudget(ctx context.Context, budget TokenBudget) error
}
```

Month format: `"2026-01"` (YYYY-MM).

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```
