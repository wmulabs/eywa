# Eywa — GitHub Copilot Instructions

Eywa is a production-grade Go framework for conversational AI agents using strict
hexagonal architecture. 19 independent Go sub-modules, each with its own `go.mod`.

## Architecture Rules

- **Hexagonal (ports and adapters)** — `internal/implementation/` (domain) must never
  import `internal/infrastructure/` (adapters). All dependencies injected via ports.
- `ports/` contains interfaces only — no structs, no implementation.
- Sub-modules (`fiber/`, `redis/`, `mongo/`, `providers/*`, `channels/*`) never import
  each other. All share only the root `eywa` module.
- New capability: define port interface → implement adapter → inject via builder.

## Domain Terms — Use Exactly

`Weave` (runtime engine), `Spirit` (agent config), `Pulse` (inbound event),
`Oracle` (LLM abstraction), `Bond` (distributed lock), `Voice` (outbound adapter),
`Receptor` (inbound adapter), `Scout` (enrichment step, sequential, fail-open),
`Lore` (RAG), `Imprint` (long-term memory), `Vigil` (human takeover),
`Rite` (approval workflow), `Keeper` (scheduler), `Conduit` (MCP client),
`Archivist` (summarizer), `Action` (LLM-callable tool).

## Code Standards

- SOLID always: single responsibility, open/closed via ports, narrow interfaces
- Functions do ONE thing. Early return over nesting.
- Names reveal intent. Booleans: `is`/`has`/`can` prefix.
- Comments explain WHY only — never WHAT. No godoc on self-explanatory functions.
- Errors: always return, wrap with `%w`, never `log.Fatal` in library code.
- No hidden side effects. No speculative abstractions (YAGNI).

## Testing

- TDD: failing test first, then minimal implementation.
- Table-driven tests always. Always run with `-race`.
- No real network in unit tests — `httptest.NewServer` or injectable stubs.
- `t.Skip` when external credentials not available (GCP, AWS, etc.).
- Test inside sub-module: `cd redis && go test ./... -race`

## Security — Never Weaken

- SSRF: `netutil.ValidateURL` before any outbound HTTP dial.
- Body limits: `io.LimitReader` on every HTTP response — no bare `io.ReadAll`.
- API keys: SHA-256 hash + `crypto/subtle.ConstantTimeCompare`.
- Rate limit all unauthenticated endpoints.
- In-memory maps growing on external input must have a hard cap.

## Go Conventions

- Propagate caller `ctx` — never substitute `context.Background()` mid-chain.
- Cleanup goroutines get bounded context: `context.WithTimeout(context.Background(), 10*time.Second)`.
- Sentinel errors exported; error constructor functions unexported.

## Commits

```
feat(fiber): description
fix(redis): description  
test(gemini): description
```

No AI attribution. No co-author trailers.
