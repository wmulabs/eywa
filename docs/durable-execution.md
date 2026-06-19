# ⏸️ Durable Execution — checkpoint & resume

An agent turn is long and expensive: multiple reasoning iterations, several high-latency LLM calls, N
tool calls. If anything interrupts it mid-flight — a deploy, a crash, a Cloud Run request timeout,
scale-to-zero — the whole turn is normally lost, and a retry restarts from iteration 0: re-paying every
token and **re-running non-idempotent tools** (sending a message twice, charging twice).

Durable execution lets a turn **survive the interruption and resume where it stopped** — "Temporal-lite
for agents in Go". It is opt-in; with it off, the synchronous path is unchanged with zero overhead.

## How it works

1. The reasoning loop derives a stable `turnID` from the Pulse idempotency key (falling back to the
   event ID) — stable across retries.
2. After each completed iteration it **saves a snapshot** of the turn state (working context, iterations
   done, plan, banned signatures, token accounting, memoized tool results).
3. On (re)entry it **loads** any checkpoint for that `turnID` and resumes from the next iteration.
4. On completion it **deletes** the checkpoint; on an error it **keeps** it so a retry resumes. Orphans
   expire via the adapter's TTL.

No new infrastructure is required for the common case: Cloud Run kills the turn → the Keeper/Cloud Tasks
delivery retries → the engine re-enters with the same idempotency key → the checkpoint is found → resume.
The [Bond](concepts.md) distributed lock still guarantees a single executor during resume.

## Enabling it

Wire a `CheckpointStore`. Off (the default, `nil`) means no checkpointing.

```go
import eywaredis "github.com/wmulabs/eywa/redis"
// or eywamongo "github.com/wmulabs/eywa/mongo"

weave, _ := eywa.NewWeaveBuilder(ctx).
    WithCheckpointStore(eywaredis.NewCheckpointStore(redisClient)).
    // ...
    Build()
```

### Adapters

| Adapter | Constructor | Storage | Orphan cleanup |
|---|---|---|---|
| Redis | `redis.NewCheckpointStore(client)` | one string key per turn (`eywa:checkpoint:<turnID>`) | key TTL (default 24h) |
| Mongo | `mongo.NewCheckpointStore(db)` | one document per turn (`_id = turnID`) | TTL index on `expires_at` |

Both also expose `NewCheckpointStoreWithTTL(..., ttl)`. The port is bytes-based, so you can implement
`eywa.CheckpointStore` against any backend:

```go
type CheckpointStore interface {
    Save(ctx context.Context, turnID string, data []byte) error
    Load(ctx context.Context, turnID string) (data []byte, found bool, err error)
    Delete(ctx context.Context, turnID string) error
}
```

## Tool memoization

Resuming would re-run the in-flight iteration's tools, duplicating side effects. On a durable turn each
**successful** Action result is memoized by its `(name, arguments)` signature in the snapshot; before
executing a call, an identical signature **reuses** the stored result instead of running it again. So a
message already sent is not sent twice on resume, and an identical call repeated within a turn is not
executed twice. Errored calls are not memoized — they can still be retried with corrected arguments.

## Guarantees & limits

- **Fail-open**: a load/encode/save/delete error logs a warning and never breaks the turn.
- **Opt-in**: no store → no behavior change, zero overhead.
- **Idempotency boundary**: a tool that completed in the *in-flight* iteration just before the crash was
  not yet checkpointed, so it can still repeat. Critical tools should carry their own idempotency key.
- **LLM non-determinism**: completed iterations are restored from the snapshot, not re-queried.
