# Eywa — Agent Instructions

## What Is Eywa

Production-grade Go framework for conversational AI agents. Connects LLMs (OpenAI,
Anthropic, Gemini, Bedrock, VertexAI) to messaging channels (WhatsApp via 360dialog
and Twilio). 19 independent Go sub-modules — each has its own `go.mod`.

---

## Architecture: Hexagonal (Ports and Adapters)

This is the single most important rule. Every decision flows from it.

### Layer Map

```
                    ┌─────────────────────────────┐
                    │         Adapters (in)        │  channels/, fiber/
                    │  Receptor, REST, Webhooks    │
                    └──────────────┬──────────────┘
                                   │ Pulse (inbound event)
                    ┌──────────────▼──────────────┐
                    │         Ports (interfaces)   │  ports/
                    │  No implementation. Ever.    │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │        Domain / Core         │  internal/implementation/
                    │  Weave, Pipeline, Scouts     │
                    │  No infrastructure imports   │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │       Adapters (out)         │  internal/infrastructure/
                    │  Redis, Mongo, Auth, Logger  │  mongo/, redis/, providers/
                    └─────────────────────────────┘
```

### Rules — Non-Negotiable

- `internal/implementation/` MUST NOT import `internal/infrastructure/` — wire at the boundary via ports
- `ports/` contains interfaces only — no structs, no functions, no init side effects
- All outbound dependencies injected through constructor parameters, never via globals
- If domain code needs a capability, define a port interface first, then inject the adapter
- Sub-modules do NOT import each other — the root module (`eywa`) is the only shared dependency
- `internal/helpers/` holds pure utilities (no ports, no infrastructure) — importable from anywhere inside the module

### Adding a New Capability

1. Define the port interface in `ports/`
2. Implement the adapter in `internal/infrastructure/` (or a sub-module)
3. Inject via constructor in `WeaveBuilder` or relevant builder
4. Wire in `internal/implementation/` using only the port type

---

## Eywa Domain Lore

Know these terms — use them consistently in code, comments, and docs.

| Term | Role | Location |
|---|---|---|
| **Weave** | Runtime engine — orchestrates the full agent lifecycle per event | `internal/implementation/orchestrator/` |
| **Spirit** | Agent configuration — LLM, tools, memory settings, behavior | `ports/spirit.go` |
| **Pulse** | Inbound event — a message received from a channel | `ports/pulse.go` |
| **Oracle** | LLM abstraction — send prompt, receive response | `ports/oracle.go` |
| **Bond** | Distributed lock — prevents duplicate responses under concurrent load | `ports/bond.go` |
| **Voice** | Outbound adapter — sends messages back to the channel | `ports/voice.go` |
| **Receptor** | Inbound adapter — receives raw channel events and produces Pulses | `ports/receptor.go` |
| **Scout** | Context enrichment step — runs sequentially, always fail-open | `ports/scout.go` |
| **Lore** | RAG — retrieval-augmented generation, injects external knowledge | `ports/lore.go` |
| **Imprint** | Long-term memory injection — persistent user context | `ports/imprint.go` |
| **Vigil** | Human-in-the-loop takeover — pauses the agent for human response | `ports/vigil.go` |
| **Rite** | Approval workflow — gates actions behind human confirmation | `ports/rite.go` |
| **Keeper** | Scheduler — dispatches async events via Cloud Tasks | `ports/keeper.go` |
| **Conduit** | MCP (Model Context Protocol) client adapter | `mcp/` |
| **Archivist** | Conversation summarizer — compresses history to fit context window | `ports/archivist.go` |
| **Action** | Tool the LLM can call — registered on the Spirit | `ports/action.go` |

**When writing code, docs, or tests, always use these names.** Never invent synonyms
("agent" instead of "Spirit", "handler" instead of "Voice", etc.).

---

## SOLID — Applied to Eywa

### Single Responsibility

Each struct has one job. If a struct has methods doing unrelated things, split it.

```go
// Wrong: Spirit config + runtime state mixed
type Spirit struct {
    Name         string
    SystemPrompt string
    activeSession *session  // runtime — wrong place
}

// Right: config separate from runtime
type Spirit struct { Name, SystemPrompt string }
type session struct { spirit *Spirit; history []Message }
```

### Open/Closed

Extend behavior via new port implementations, not by modifying core orchestration.
Adding a new Oracle provider = new struct implementing `ports.Oracle`. Zero changes to
`internal/implementation/`.

### Liskov Substitution

Every implementation of a port must honor the full contract. If `ports.Bond` says
`AcquireLock` must be idempotent for the same key, every implementation (Redis, noOp,
test double) must uphold that.

### Interface Segregation

Ports are narrow. A Scout that only needs to read user context should receive a reader
interface, not the full `Spirit` struct. Do not pass god-objects; carve out what you need.

### Dependency Inversion

Domain depends on abstractions (`ports.*`), never on concretions (`*redis.Bond`,
`*mongo.Repository`). Injection happens at wiring time in builders/constructors.

---

## Clean Code Rules

### Naming

- Names reveal intent: `acquireSessionLock`, not `lock`, not `doLocking`
- Boolean functions/vars: `is`, `has`, `can` prefix — `isAvailable`, `hasConversationHistory`
- Avoid abbreviations unless universal in the domain (`ctx`, `err`, `msg` are fine; `proc` for `processor` is not)
- Unexported names for package-internal concerns; exported names for ports and public API

### Functions

- Do ONE thing. If you need "and" to describe what it does, split it.
- Max ~40 lines before questioning the design — not a hard rule, a smell detector
- No hidden side effects: a function named `getUser` must not write to a DB
- Early returns over nested if-else pyramids

### Comments

- Only explain the WHY when the code cannot — non-obvious decisions, trade-offs, known limitations
- Never describe what the function name already says
- No godoc on self-explanatory functions — `// NewOracle creates a new oracle` is noise
- Security-relevant limitations MUST be commented (e.g., TOCTOU on DNS rebinding)

### Error Handling

- Always return errors — never swallow them silently
- Wrap with context: `fmt.Errorf("acquiring session lock for %s: %w", sessionID, err)`
- Sentinel errors for public API: `var ErrSessionHeld = errors.New("session already held")`
- Internal factory functions for rich errors with context: `newErrSessionHeld(sessionID)`
- `log.Fatal` / `os.Exit` NEVER in library code — only in `main()` or examples

---

## Testing Standards

### Philosophy

- TDD: write the failing test first, then the minimal implementation to make it pass
- Tests document behavior — a test name is a sentence: `TestRunScout_OnError_ContinuesPipeline`
- Tests must be fast and deterministic — no sleeps, no real network, no real DB

### Structure

Use table-driven tests for all cases with variations:

```go
func TestIsPrivateIP(t *testing.T) {
    cases := []struct {
        name    string
        ip      string
        private bool
    }{
        {"loopback", "127.0.0.1", true},
        {"public", "8.8.8.8", false},
        {"RFC1918", "192.168.1.1", true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := IsPrivateIP(net.ParseIP(tc.ip))
            if got != tc.private {
                t.Errorf("IsPrivateIP(%s) = %v, want %v", tc.ip, got, tc.private)
            }
        })
    }
}
```

### Dependencies in Tests

- Never make real HTTP calls in unit tests — inject a stub or use `httptest.NewServer`
- Infrastructure-dependent tests (Vertex AI, real Redis) must `t.Skip` when env not configured
- Use injectable function fields on structs for hard-to-mock dependencies (see `httpToolHandler.testerFunc`)

### Race Detector

Always run with `-race`. No exceptions. A test that passes without `-race` but fails with it is broken.

### Coverage

- 70% minimum on `fiber/`, `mongo/`, `redis/`
- Critical security paths (SSRF validation, API key comparison) need explicit tests
- Coverage number is a floor, not a goal — meaningful behavior coverage over line coverage

### Integration Test Convention

```go
func requireGCPCredentials(t *testing.T) {
    t.Helper()
    if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" { return }
    home, _ := os.UserHomeDir()
    if _, err := os.Stat(filepath.Join(home, ".config/gcloud/application_default_credentials.json")); err == nil { return }
    t.Skip("no GCP credentials — set GOOGLE_APPLICATION_CREDENTIALS or run `gcloud auth application-default login`")
}
```

---

## Security Invariants — Never Break These

These were hardened during the pre-release audit. Any change that weakens them is a blocker.

### SSRF Protection

All outbound HTTP calls (HTTPTool, MCP Conduit) must call `netutil.ValidateURL` before dialing:

```go
// internal/helpers/netutil/netutil.go
if err := netutil.ValidateURL(rawURL); err != nil {
    return fmt.Errorf("SSRF guard: %w", err)
}
```

Blocks: `file://`, `ftp://`, private IPs, loopback, link-local, IMDS (169.254.169.254), IPv6 ULA.

### Response Body Limits

Every HTTP client read uses `io.LimitReader`. No `io.ReadAll` on unbounded bodies:

```go
limited := io.LimitReader(resp.Body, maxResponseBytes)
data, err := io.ReadAll(limited)
```

### API Key Storage

Store SHA-256 hash, compare with constant-time function:

```go
hash := sha256.Sum256([]byte(rawKey))
subtle.ConstantTimeCompare(stored[:], hash[:])
```

### Bond Cap

`noOpBond` map capped at `maxNoOpBondKeys = 10_000`. Any in-memory map that grows
without bound on external input needs a cap.

### Rate Limiting

Auth endpoints have rate limiting. New unauthenticated endpoints need rate limiting.

---

## Go-Specific Rules

### Concurrency

- Protect shared mutable state — `sync.RWMutex` for read-heavy, `sync.Mutex` otherwise
- Double-checked locking pattern for lazy initialization (see `dbg.GetLogger`)
- Goroutines that call external hooks get a bounded context: `context.WithTimeout(context.Background(), 10*time.Second)`
- Never start goroutines inside library constructors — callers can't cancel them

### Context

- First parameter, always: `func (o *GeminiOracle) Query(ctx context.Context, req *Request)`
- Propagate the caller's context down — do NOT substitute `context.Background()` except for cleanup goroutines
- Document when you intentionally break context propagation and why

### Error Wrapping

```go
// Wrap with %w for sentinel matching via errors.Is
return fmt.Errorf("acquiring lock for session %s: %w", id, ErrBondTimeout)

// Multiple wrapping levels are fine — %w chains work
```

### Exported vs Unexported

- Public API = exported. Everything else unexported.
- Sentinel errors: exported (`ErrSessionHeld`) so callers can `errors.Is`
- Internal error constructors: unexported (`newErrSessionHeld`)
- Optional interface checks: unexported (`registryAccessor`)

### No Global State (Except Logger)

`dbg.globalLogger` is the only permitted global. Document why at every new global:
what the side effect is, who owns initialization, thread-safety.

---

## Sub-Module Rules

- Each sub-module has its own `go.mod` and is independently versioned
- Sub-modules import `github.com/wmulabs/eywa` (root) for types and ports — nothing else
- Sub-modules NEVER import each other
- Test changes to a sub-module in that sub-module: `cd redis && go test ./... -race`
- Updating a dependency in a sub-module requires `go mod tidy` in that sub-module's directory

---

## Running the Full Stack Locally

```bash
# Test all sub-modules
for mod in . fiber mongo redis mcp \
           channels/whatsapp/dialog360 channels/whatsapp/twilio \
           providers/openai providers/anthropic providers/gemini \
           providers/bedrock providers/vertexai; do
  echo "=== $mod ==="
  (cd "$mod" && go test ./... -race -count=1)
done

# Lint all sub-modules
for mod in . fiber mongo redis mcp \
           channels/whatsapp/dialog360 channels/whatsapp/twilio \
           providers/openai providers/anthropic providers/gemini \
           providers/bedrock providers/vertexai; do
  (cd "$mod" && golangci-lint run ./...)
done

# Vuln check
for mod in . fiber mongo redis mcp; do
  (cd "$mod" && govulncheck ./...)
done
```

---

## Commit Conventions

Conventional commits. Scope = sub-module when relevant.

```
feat(fiber): add SSE streaming for long-running Spirit responses
fix(redis): wrap ErrBondTimeout with %w for errors.Is compatibility
test(gemini): skip Vertex AI tests when no GCP credentials available
docs: update sub-modules.md with NewRedisConnection signature change
refactor(orchestrator): extract registryAccessor interface
chore: add golangci-lint config
```

**No AI attribution. No co-author trailers. No "Generated by" in commit messages.**

---

## What NOT to Do

| Wrong | Right |
|---|---|
| `log.Fatal(err)` in library code | `return fmt.Errorf("...": %w", err)` |
| `io.ReadAll(resp.Body)` | `io.ReadAll(io.LimitReader(resp.Body, max))` |
| Import infrastructure from domain | Define a port, inject the adapter |
| `context.Background()` in call chain | Propagate caller's `ctx` |
| Speculative abstraction "for future use" | YAGNI — build what is needed now |
| Comment describing what code does | Comment explaining why it does it |
| Godoc on self-explanatory function | No godoc, or one sentence max |
| Global mutable state | Constructor injection |
| Test that makes real HTTP calls | `httptest.NewServer` or injected stub |
| Sub-modules importing each other | Both import root `eywa` only |
| New unauthenticated endpoint without rate limit | Add middleware before shipping |
| Skip `-race` in tests | Always `-race` |
| Ignore linter output | Fix or explicitly justify suppression |
