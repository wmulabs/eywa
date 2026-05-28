# Example 07: Human Takeover (Vigil)

Demonstrates **Vigil** — Eywa's human-in-the-loop takeover system.

## What this shows

- Operator acquiring an exclusive "seat" on a conversation via `VigilRepository.Acquire`
- AI engine returning `ErrSessionHeld` when a seat is active
- Operator releasing the seat to hand control back to the AI
- Seat TTL: Redis-backed expiry so a forgotten seat doesn't lock the conversation forever

## Key concepts

### Vigil flow

```
AI handles messages normally
  → Operator calls Acquire(memoryKey)  → seat created with TTL
  → AI is blocked (ErrSessionHeld) while seat is held
  → Operator sends direct messages via the management API
  → Operator calls Release(memoryKey) → AI resumes
```

### VigilRepository

```go
type VigilRepository interface {
    Acquire(ctx context.Context, memoryKey MemoryKey, operatorID string, ttl time.Duration) error
    Get(ctx context.Context, memoryKey MemoryKey) (*VigilSeat, error)
    Release(ctx context.Context, memoryKey MemoryKey) error
}
```

### VigilConfig

```go
eywa.VigilConfig{InactivityTimeout: 30 * time.Minute}
```

Sets the default TTL for seats. If the operator goes silent, the seat auto-expires and the AI resumes.

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```

## Integration with management API

In production, operators acquire/release seats via REST endpoints registered by `eywa/fiber`:

```
POST   /api/v1/vigil/:memoryKey/acquire
DELETE /api/v1/vigil/:memoryKey
POST   /api/v1/vigil/:memoryKey/echoes   (direct message while holding seat)
```
