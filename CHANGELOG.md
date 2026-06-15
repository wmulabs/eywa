# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.6.0](https://github.com/wmulabs/eywa/compare/v1.5.0...v1.6.0) (2026-06-15)


### Features

* **orchestrator:** add a turn-scoped plan/scratchpad for multi-step tasks ([#77](https://github.com/wmulabs/eywa/issues/77)) ([f99fccd](https://github.com/wmulabs/eywa/commit/f99fccd8772993ba5c8517b6cbfd66d5e2496158))

## [1.5.0](https://github.com/wmulabs/eywa/compare/v1.4.0...v1.5.0) (2026-06-15)


### Features

* **orchestrator:** enforce source citations for RAG answers (grounding) ([#70](https://github.com/wmulabs/eywa/issues/70)) ([ed9c7e7](https://github.com/wmulabs/eywa/commit/ed9c7e7f09b28e5e2ee2ea536272871ad90c1660))

## [1.4.0](https://github.com/wmulabs/eywa/compare/v1.3.0...v1.4.0) (2026-06-15)


### Features

* **orchestrator:** add gated self-critique (reflection) before delivery ([#69](https://github.com/wmulabs/eywa/issues/69)) ([eb0dcf5](https://github.com/wmulabs/eywa/commit/eb0dcf52fa56a8bca5be0c40283eb6e650ea5ff1))

## [1.3.0](https://github.com/wmulabs/eywa/compare/v1.2.0...v1.3.0) (2026-06-15)


### Features

* **orchestrator:** compress the reasoning working context in long loops ([#68](https://github.com/wmulabs/eywa/issues/68)) ([f6c760a](https://github.com/wmulabs/eywa/commit/f6c760a06a7d3861a8067f330cc2e491846cc2d2))

## [1.2.0](https://github.com/wmulabs/eywa/compare/v1.1.0...v1.2.0) (2026-06-15)


### Features

* **orchestrator:** detect reasoning stalls and force a final synthesis ([#67](https://github.com/wmulabs/eywa/issues/67)) ([1cfbb5a](https://github.com/wmulabs/eywa/commit/1cfbb5a7d69047c4ffbdb4ae301abbd83a332937))

## [1.1.0](https://github.com/wmulabs/eywa/compare/v1.0.1...v1.1.0) (2026-06-15)


### Features

* **orchestrator:** bound Action result size in the reasoning context ([#66](https://github.com/wmulabs/eywa/issues/66)) ([c5545c5](https://github.com/wmulabs/eywa/commit/c5545c531674b7267f93f85f285da55d31593ae5))

## [1.0.1](https://github.com/wmulabs/eywa/compare/v1.0.0...v1.0.1) (2026-06-14)


### Bug Fixes

* v1 review bug fixes and prod cleanup ([#63](https://github.com/wmulabs/eywa/issues/63)) ([0982f92](https://github.com/wmulabs/eywa/commit/0982f92dd05738ac668ad8a4dc50846355819eef))

## [Unreleased]

## [1.0.0] - 2026-05-28

### Added
- `internal/helpers/netutil` — shared SSRF validation package used by HTTPToolExecutor and MCP Conduit
- `HTTPTool.MaxResponseBytes` — configurable response body size limit for HTTP tool execution
- Rate limiting middleware (10 req/min per IP) on `POST /api/v1/auth/token`
- OTel span events for deferred pipeline step failures
- `noOpBond` key cap (10,000 concurrent keys) with descriptive error when exceeded
- `registryAccessor` optional interface for `ActionExecutor` — allows custom executors to expose their registry
- MCP Conduit: SSRF protection via URL validation; 4 MiB response body limit
- Redis `MemoryRepository`: injectable OTel tracer (pass `nil` for noop)
- `ErrBondLockNotFound` and `ErrBondExtendFailed` sentinel errors in redis package
- `.golangci.yml` with explicit linter allowlist
- CI: 70% coverage threshold for `fiber`, `mongo`, `redis` sub-modules
- CI: `golangci-lint` runs across all 16 sub-modules
- CI: `govulncheck` runs across all sub-modules
- CI: API stability snapshot for `fiber`, `mongo`, `redis`, `mcp` sub-modules
- Unit tests for `netutil.ValidateURL` and `netutil.IsPrivateIP` covering all private IP ranges and bypass vectors

### Changed
- `NewMongoConnection` now returns `(*MongoConnection, error)` instead of calling `log.Fatalw` (breaking change from pre-release)
- `NewRedisConnection` now returns `(*RedisRepository, error)` instead of calling `log.Fatalw` (breaking change from pre-release)
- `NewMemoryRepository` now accepts a `trace.Tracer` parameter as the 5th argument (pass `nil` for noop behavior)
- `runScout` is now fail-open: Scout errors are logged but do not abort the pipeline (matches documented behavior)
- `APIKeyValidator` stores SHA-256 hashes of API keys instead of raw plaintext
- `dbg.GetLogger()` uses RWMutex with double-checked locking instead of `sync.Once` (enables safe `SetLogger` after init)
- `WeaveBuilder.Build()` godoc now documents the global logger side effect
- `GetRedisClient()` is deprecated — use `GetClient()` instead
- WhatsApp channel clients (dialog360, Twilio): response body reads bounded by `io.LimitReader`
- CostAlertHook goroutines use a fresh 10-second bounded context instead of the (possibly cancelled) caller context
- Gemini Oracle: caller context forwarded to `genai.NewClient` for Vertex AI initialization
- Scouts documented as running sequentially (not concurrently)

### Fixed
- Data race on `inboundConverters` map — now protected by `sync.RWMutex`
- Race condition in `dbg.SetLogger` — replaced `sync.Once` with mutex-based double-checked locking
- SSRF vulnerability in `HTTPToolExecutor` — URL validation blocks private/loopback IP ranges
- Unbounded response body reads in `HTTPToolExecutor` — now limited to 1 MiB (configurable)
- Unbounded response body reads in `mcp/conduit.go` — now limited to 4 MiB
- Fragile PubSub message prefix check — replaced manual index access with `strings.HasPrefix`
- `ErrSessionHeld` — internal callsites now use `newErrSessionHeld()` factory to avoid shared mutable pointer mutation
- `GetActionRegistry()` type-asserting against `*DefaultActionExecutor` — replaced with optional `registryAccessor` interface
- Stale API examples in `docs/sub-modules.md`, `README.md`, `README.pt-BR.md`
- Scout behavior mismatch between docs ("never aborts") and code (was aborting) — code now matches docs

### Security
- **SSRF protection** added to `HTTPToolExecutor` (URL scheme + private IP validation)
- **SSRF protection** added to MCP Conduit (`validateMCPURL`)
- **API key hashing**: `APIKeyValidator` now stores SHA-256 hashes; comparison uses `crypto/subtle.ConstantTimeCompare`
- **Rate limiting** on auth token endpoint: 10 requests/minute per IP
- **Response body limits** enforced on all HTTP client reads (HTTPToolExecutor, MCP Conduit, WhatsApp clients, JWKS validator, HTTP error parser)
- **RegisterRoutes** security warning added to godoc and README — Spirit CRUD endpoints are unauthenticated

[Unreleased]: https://github.com/wmulabs/eywa/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/wmulabs/eywa/releases/tag/v1.0.0
