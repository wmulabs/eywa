# Example 09: Long-Term Memory (Imprint)

Demonstrates **Imprint** — Eywa's per-user persistent fact storage.

## What this shows

- Spirit using `RememberFact` and `ForgetFact` built-in actions
- Automatic fact extraction via `ImprintExtractionConfig` (the engine extracts facts from messages without explicit tool calls)
- Facts persisted per user across sessions
- Reading stored facts via `ImprintRepository.GetByUserKey`

## Key concepts

### Imprint vs. Echo (conversation memory)

| Echo (short-term) | Imprint (long-term) |
|---|---|
| Sliding window of messages | Persistent facts across sessions |
| Cleared on TTL / window size | Never expires (unless ForgetFact called) |
| Scoped to a channel+user session | Scoped to user across all channels |

### ImprintExtractionConfig

```go
eywa.ImprintExtractionConfig{
    Enabled:    true,
    MaxFacts:   50,
    Categories: []string{"preference", "personal", "goal"},
}
```

When enabled, the engine automatically extracts and stores facts from user messages. The spirit does not need to call `RememberFact` explicitly.

### ImprintRepository

```go
type ImprintRepository interface {
    Store(ctx context.Context, userKey string, fact Fact) error
    GetByUserKey(ctx context.Context, userKey string) ([]*Fact, error)
    Delete(ctx context.Context, userKey, factID string) error
}
```

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```
