# Eywa — Agent Instructions

## What Is Eywa

Production-grade Go framework for conversational AI agents. Connects LLMs (OpenAI,
Anthropic, Gemini, Bedrock, VertexAI) to messaging channels (WhatsApp via 360dialog
and Twilio). 19 independent Go sub-modules — each with its own `go.mod`.

---

## Architecture: Hexagonal (Ports and Adapters)

Strict hexagonal architecture enforced via Go's `internal/` visibility rules.

**Layer dependencies (top → bottom only):**

```
channels/, fiber/          ← inbound adapters
ports/                     ← interfaces only, no implementation
internal/implementation/   ← domain logic, depends only on ports
internal/infrastructure/   ← outbound adapters (Redis, Mongo, auth)
mongo/, redis/, providers/ ← sub-module adapters
```

### Hard Rules

- `internal/implementation/` MUST NOT import `internal/infrastructure/`
- `ports/` = interfaces only. Zero implementation, zero init side effects.
- Dependencies injected via constructors — no globals except `dbg.globalLogger`
- Sub-modules do NOT import each other — only the root `eywa` module
- New feature = define port interface first, then implement adapter, then inject

---

## Domain Terminology

Use these terms exactly in code, comments, tests, and docs.

| Term | Meaning |
|---|---|
| **Weave** | Runtime engine — runs the agent pipeline per inbound event |
| **Spirit** | Agent config — LLM, tools, memory, behavior settings |
| **Pulse** | Inbound event — a message received from a channel |
| **Oracle** | LLM abstraction — uniform interface across all providers |
| **Bond** | Distributed lock — prevents duplicate responses |
| **Voice** | Outbound adapter — sends messages to the channel |
| **Receptor** | Inbound adapter — converts channel events to Pulses |
| **Scout** | Enrichment step — sequential execution, fail-open contract |
| **Lore** | RAG (retrieval-augmented generation) |
| **Imprint** | Long-term memory injection into context |
| **Vigil** | Human takeover — pauses agent for human to respond |
| **Rite** | Approval workflow — gates actions on human confirmation |
| **Keeper** | Cloud Tasks scheduler |
| **Conduit** | MCP (Model Context Protocol) client |
| **Archivist** | Conversation summarizer — compresses history |
| **Action** | Tool the LLM can call |

---

## SOLID Principles

**Single Responsibility:** Each struct has one job. If describing it requires "and", split it.

**Open/Closed:** Add providers/adapters by implementing the port. Never modify orchestration core.

**Liskov:** Port implementations must honor full contract including error semantics and side effects.

**Interface Segregation:** Narrow ports. Pass only what is needed. No god-objects.

**Dependency Inversion:** Domain code depends on `ports.*` interfaces only, never on concrete types.

---

## Clean Code Standards

**Naming:**
- Reveals intent: `acquireSessionLock` not `lock`, `hasConversationHistory` not `check`
- Booleans: `is`, `has`, `can` prefix
- No unexplained abbreviations

**Functions:**
- Single responsibility — do one thing
- Early return over nesting
- No hidden side effects

**Comments:**
- Only explain WHY — never WHAT (the code shows what)
- No godoc on self-explanatory functions
- Security limitations MUST be documented

**Errors:**
- Always return — never swallow
- Wrap with context: `fmt.Errorf("context for %s: %w", id, err)`
- `log.Fatal`/`os.Exit` — only in `main()`, never in library code

---

## Testing

**Philosophy:** TDD. Write failing test first. Tests document behavior.

**Structure:** Table-driven tests for all variation cases.

**Requirements:**
- Always run with `-race` — no exceptions
- No real network in unit tests — use `httptest.NewServer` or injectable stubs
- Skip integration tests when env not available: `t.Skip("no GCP credentials...")`
- 70% coverage floor on `fiber/`, `mongo/`, `redis/`
- Test names are sentences: `TestRunScout_OnError_ContinuesPipeline`

**Running tests:**
```bash
# Single sub-module
cd redis && go test ./... -race -count=1

# Root module
go test ./... -race -count=1
```

---

## Security — Non-Negotiable

**SSRF:** All outbound HTTP must call `netutil.ValidateURL` before dialing.

**Body limits:** `io.LimitReader` on every HTTP response body. No unbounded `io.ReadAll`.

**API keys:** SHA-256 hash storage + `crypto/subtle.ConstantTimeCompare`.

**Bond cap:** In-memory maps that grow on external input must have a hard cap.

**Rate limiting:** All unauthenticated endpoints need rate limiting middleware.

---

## Go Conventions

- Propagate caller's `ctx` through the call chain
- Cleanup goroutines: `context.WithTimeout(context.Background(), 10*time.Second)`
- Error sentinels exported for `errors.Is`; error constructors unexported
- `sync.RWMutex` for read-heavy shared state; double-checked locking for lazy init
- No goroutines in library constructors

---

## Sub-Module Rules

- 19 sub-modules, each with own `go.mod`
- Test inside the sub-module directory: `cd fiber && go test ./... -race`
- After dependency changes: `go mod tidy` inside that sub-module
- Sub-modules never import each other

---

## Commits

Conventional commits with sub-module scope:

```
feat(fiber): add rate limiting to auth endpoint
fix(redis): use %w when wrapping ErrBondTimeout
test(gemini): skip Vertex AI tests when no GCP credentials
refactor(orchestrator): extract registryAccessor interface
docs: sync sub-modules.md with NewMongoConnection signature
```

No AI attribution. No co-author trailers.

---

## Absolute Don'ts

- `log.Fatal` or `os.Exit` in library code
- `io.ReadAll(resp.Body)` without `io.LimitReader`
- Outbound HTTP without SSRF check
- Importing infrastructure from domain
- `context.Background()` replacing caller context mid-chain
- Sub-modules importing each other
- Speculative abstractions (YAGNI)
- New unauthenticated endpoint without rate limiting
- Skip `-race` flag in tests
- Comments describing what the code does
