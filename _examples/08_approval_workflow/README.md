# Example 08: Approval Workflow (Rites)

Demonstrates **Rites** — Eywa's async human-approval system for high-stakes AI actions.

## What this shows

- Spirit using `RequestRiteAction` to pause and request human approval before proceeding
- Operator listing pending rites via `RiteRepository.List`
- Operator approving or rejecting a rite via `RiteRepository.Decide`
- Approved rites unblock the AI flow; rejected rites return an error to the spirit

## Key concepts

### Rite lifecycle

```
Spirit calls RequestRiteAction("send_invoice", payload)
  → Rite created with status "pending"
  → Spirit response: "Waiting for approval"
  → Operator reviews rite in management UI or API
  → Operator calls Decide(id, operatorID, RiteApproved | RiteRejected)
  → AI flow continues or aborts
```

### RiteRepository

```go
type RiteRepository interface {
    Create(ctx context.Context, rite *Rite) error
    Get(ctx context.Context, id string) (*Rite, error)
    List(ctx context.Context, opts RiteListOptions) ([]*Rite, int64, error)
    Decide(ctx context.Context, id, operatorID string, status RiteStatus) error
}
```

### RequestRiteAction

Built-in action that creates a rite and blocks until the workflow is resolved (async — the spirit signals "pending" and the decision happens out-of-band).

## Running

```bash
export OPENAI_API_KEY=sk-...
export MONGO_URL=mongodb://localhost:27017
export REDIS_URL=redis://localhost:6379

go run .
```

## Integration with management API

REST endpoints from `eywa/fiber`:

```
GET  /api/v1/rites              (list with ?status=pending filter)
GET  /api/v1/rites/:id
POST /api/v1/rites/:id/decide   { "status": "approved", "comment": "..." }
```
