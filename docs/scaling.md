# 📈 Scaling & Failure Scenarios

## Horizontal Scaling

Eywa is designed for stateless horizontal scaling. Multiple Weave instances can run
simultaneously pointing at the same MongoDB and Redis — no coordination configuration needed.

**How it works:**
- All conversation state lives in Redis (Memory) and MongoDB (Echoes, Spirits, Chronicle)
- The Bond (distributed lock) ensures only one instance processes a given `MemoryKey` at a time
- If instance A holds the lock for `user:123`, instance B receives a Pulse for `user:123`
  and gets `ErrMemoryBusy` — the message waits in the Inbox or is retried by the caller
- When instance A finishes, it releases the lock; the next Pulse for `user:123` can be picked up by any instance

**Cloud Run / Kubernetes:** Scale to any number of replicas. The Bond handles coordination.
No sticky sessions, no shared in-process state, no warm-up required.

**Session affinity:** Not needed and not recommended. Any instance can handle any Pulse.

---

## Failure Scenarios

### Oracle unavailable (LLM API down or rate-limited)

`ReasoningStep` returns `ErrReasoningFailed` (retriable). The pipeline aborts.
- **Cloud Tasks Keeper:** The task is retried with exponential backoff automatically.
- **Synchronous HTTP:** The Fiber handler returns 503. The client should retry.
- **Mitigation:** Configure `WeaveConfig.ReasoningTimeout` conservatively. Use Ledger
  with model routing to fall back to a cheaper Oracle when the primary is rate-limited.

### Redis unavailable

| Dependency | Failure mode | Recovery |
|---|---|---|
| Bond (lock) | `ErrLockAcquisitionFailed` (retriable) | Retry after Redis recovers |
| Memory (session) | `ErrSessionLoadFailed` (retriable) | Session rebuilt from Echoes on recovery |
| Inbox | Message not queued; Pulse rejected | Retry or dead-letter |
| PubSub (SSE) | SSE connections stop receiving events | Clients reconnect; snapshot sent on reconnect |
| Rate Limiter | Limiter fails open; traffic passes | No action needed |

**NoOpBond users:** Not affected — in-process mutex never fails.

### MongoDB unavailable

| Dependency | Failure mode | Recovery |
|---|---|---|
| Spirit repository | `ErrSpiritLoadFailed`; pipeline aborts | Spirits are cached in memory after first load |
| Echo repository | `ErrPersistenceFailed` (retriable) | Messages may be lost if persistence fails after reasoning |
| Chronicle | Logged, swallowed; pipeline continues | Chronicle fills in when MongoDB recovers |
| Imprint / Lore | Non-fatal if configured as optional | Reasoning continues without long-term memory |

**Mitigation:** Use MongoDB Atlas or a replica set with at least 3 nodes. Chronicle
and Echo writes are non-blocking — a brief outage causes gaps, not service failure.

### Lock expired before reasoning completed

**Symptom:** Duplicate responses or interleaved Chronicle entries.

**Cause:** `LockTTL` ≤ `ReasoningTimeout`. A second instance acquired the lock while the
first was still reasoning.

**Fix:** Set `LockTTL` ≥ `ReasoningTimeout + 30s`. The pipeline extends the lock every
`LockTTL/3` during reasoning. `WeaveConfig.Validate()` enforces the minimum at startup.

### Spirit configuration with deprecated model

When a provider retires a model (e.g. `claude-2` → `claude-3`):
1. The Spirit's `ModelName` field still holds the old value
2. Oracle calls return 400/404 from the provider
3. `ErrReasoningFailed` is returned for every Pulse routed to that Spirit

**Recovery:**
1. Update the Spirit via `PUT /api/v1/spirits/:name` with the new model name
2. Or use model routing rules to redirect to a working model
3. Chronicle entries for failed Pulses are preserved for audit purposes

---

## Capacity Planning

| Resource | Rule of thumb |
|---|---|
| Redis memory | ~2 KB per active session (Memory) + SSE subscriptions |
| MongoDB IOPS | 3–5 writes per Pulse (Echo, Chronicle, optionally Imprint, Lore) |
| CPU | Stateless; scale horizontally with load |
| Concurrency | 1 goroutine per active Pulse; limited by Oracle API rate limits |
| LockTTL | `ReasoningTimeout + 30s` minimum; set higher for slow Oracles |
