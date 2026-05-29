# Eywa — Gemini Agent Instructions

## What Is Eywa

Production-grade Go framework for conversational AI agents. Connects LLMs (OpenAI,
Anthropic, Gemini, Bedrock, VertexAI) to messaging channels (WhatsApp via 360dialog
and Twilio). 19 independent Go sub-modules — each has its own `go.mod`.

---

## Architecture: Hexagonal (Ports and Adapters)

```
                    ┌─────────────────────────────┐
                    │         Adapters (in)        │  channels/, fiber/
                    └──────────────┬──────────────┘
                                   │ Pulse (inbound event)
                    ┌──────────────▼──────────────┐
                    │         Ports (interfaces)   │  ports/
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │        Domain / Core         │  internal/implementation/
                    │  NO infrastructure imports   │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │       Adapters (out)         │  internal/infrastructure/
                    │  Redis, Mongo, Auth, Logger  │  mongo/, redis/, providers/
                    └─────────────────────────────┘
```

### Non-Negotiable Rules

- `internal/implementation/` must NOT import `internal/infrastructure/`
- `ports/` contains interfaces only — no structs, no functions, no init side effects
- All outbound dependencies injected via constructor parameters, never globals
- Sub-modules do NOT import each other — root `eywa` module is the only shared dep
- New capability = define port first → implement adapter → inject via builder

---

## Eywa Domain Lore

Always use these names. Never invent synonyms.

| Term | Role |
|---|---|
| **Weave** | Runtime engine — orchestrates full agent lifecycle per event |
| **Spirit** | Agent configuration — LLM, tools, memory, behavior |
| **Pulse** | Inbound event — message received from a channel |
| **Oracle** | LLM abstraction — send prompt, receive response |
| **Bond** | Distributed lock — prevents duplicate responses under concurrent load |
| **Voice** | Outbound adapter — sends messages back to the channel |
| **Receptor** | Inbound adapter — receives raw events, produces Pulses |
| **Scout** | Context enrichment step — sequential, always fail-open |
| **Lore** | RAG — retrieval-augmented generation |
| **Imprint** | Long-term memory injection |
| **Vigil** | Human-in-the-loop takeover |
| **Rite** | Approval workflow |
| **Keeper** | Scheduler via Cloud Tasks |
| **Conduit** | MCP (Model Context Protocol) client adapter |
| **Archivist** | Conversation summarizer |
| **Action** | Tool the LLM can call |

---

## SOLID — Applied to Eywa

- **Single Responsibility**: each struct has one job — split if you need "and"
- **Open/Closed**: extend via new port implementations, not modifying orchestration core
- **Liskov**: every port implementation must honor the full contract (incl. error semantics)
- **Interface Segregation**: ports are narrow — don't pass god-objects
- **Dependency Inversion**: domain depends on `ports.*`, never on concretions

---

## Clean Code

- Names reveal intent: `acquireSessionLock`, not `lock`
- Functions do ONE thing — if you need "and", split
- No hidden side effects — `getUser` must not write to DB
- Early returns over nested if-else
- Comments explain WHY, not WHAT — never describe what the function name says
- No godoc on self-explanatory functions

---

## Testing Standards

- TDD: write failing test first
- Table-driven tests for all variation cases
- Always run with `-race`
- No real HTTP calls in unit tests — use `httptest.NewServer` or injected stub
- Skip integration tests when env not configured (`t.Skip`)
- 70% coverage floor on `fiber/`, `mongo/`, `redis/`

---

## Security Invariants — Never Weaken

- **SSRF**: call `netutil.ValidateURL` before any outbound HTTP dial
- **Body limits**: `io.LimitReader` on every HTTP client read — no unbounded `io.ReadAll`
- **API keys**: store SHA-256 hash, compare with `crypto/subtle.ConstantTimeCompare`
- **Bond cap**: `noOpBond` capped at `maxNoOpBondKeys = 10_000`
- **Rate limiting**: auth endpoints rate-limited — new unauthenticated endpoints need it too

---

## Go Rules

- Propagate caller's `ctx` — never substitute `context.Background()` in call chain
- Cleanup goroutines get bounded context: `context.WithTimeout(context.Background(), 10*time.Second)`
- Wrap errors with context: `fmt.Errorf("acquiring lock for %s: %w", id, err)`
- Sentinel errors exported for callers; internal error factories unexported
- `log.Fatal`/`os.Exit` never in library code — return errors

---

## Sub-Modules

- Each has its own `go.mod` — test changes inside that directory
- `cd redis && go test ./... -race`
- Sub-modules never import each other

---

## Commit Conventions

```
feat(fiber): add SSE streaming endpoint
fix(redis): wrap ErrBondTimeout with %w
test(gemini): skip Vertex AI tests without GCP credentials
```

No AI attribution. No co-author trailers.

---

## Never Do This

| Wrong | Right |
|---|---|
| `log.Fatal(err)` in library | `return fmt.Errorf(...: %w", err)` |
| `io.ReadAll(resp.Body)` | `io.ReadAll(io.LimitReader(resp.Body, max))` |
| Import infra from domain | Define port → inject adapter |
| `context.Background()` in call chain | Propagate caller `ctx` |
| Speculative abstraction | YAGNI |
| Comment describing what code does | Comment explaining why |
| Sub-modules importing each other | Both import root `eywa` only |
