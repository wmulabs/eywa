# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.20.0](https://github.com/wmulabs/eywa/compare/v1.19.0...v1.20.0) (2026-06-17)


### Features

* per-Spirit structured output (Spirit.ResponseFormat) ([#106](https://github.com/wmulabs/eywa/issues/106)) ([4d5ce65](https://github.com/wmulabs/eywa/commit/4d5ce659384acd017f70ee91536e19a776eea240))

## [1.19.0](https://github.com/wmulabs/eywa/compare/v1.18.0...v1.19.0) (2026-06-17)


### Features

* **orchestrator:** structured output in the reasoning loop ([#104](https://github.com/wmulabs/eywa/issues/104)) ([97fc28a](https://github.com/wmulabs/eywa/commit/97fc28a6cf8e0f140ef0d2eb0c0220fdd5793c09))

## [1.18.0](https://github.com/wmulabs/eywa/compare/v1.17.0...v1.18.0) (2026-06-17)


### Features

* **anthropic:** native structured output (tool-forcing) ([#102](https://github.com/wmulabs/eywa/issues/102)) ([512738d](https://github.com/wmulabs/eywa/commit/512738d3dac42471fbdb7ba8ca7a86a8bf9d2b9f))

## [1.17.0](https://github.com/wmulabs/eywa/compare/v1.16.0...v1.17.0) (2026-06-17)


### Features

* **gemini:** native structured output (responseSchema) ([#100](https://github.com/wmulabs/eywa/issues/100)) ([6df8278](https://github.com/wmulabs/eywa/commit/6df827861183817d9543095373d4b01e242137cb))

## [1.16.0](https://github.com/wmulabs/eywa/compare/v1.15.0...v1.16.0) (2026-06-16)


### Features

* **openai:** native structured output (response_format json_schema) ([#98](https://github.com/wmulabs/eywa/issues/98)) ([a167429](https://github.com/wmulabs/eywa/commit/a1674294cd1ad70f507331d8725d4774bc5d1a43))

## [1.15.0](https://github.com/wmulabs/eywa/compare/v1.14.0...v1.15.0) (2026-06-16)


### Features

* structured-output port surface + schema validation ([#96](https://github.com/wmulabs/eywa/issues/96)) ([2d1a529](https://github.com/wmulabs/eywa/commit/2d1a52943bc6e6ee17525441d354635d563694b1))

## [1.14.0](https://github.com/wmulabs/eywa/compare/v1.13.0...v1.14.0) (2026-06-16)


### Features

* stream agent turns end-to-end over SSE ([#94](https://github.com/wmulabs/eywa/issues/94)) ([093d8e2](https://github.com/wmulabs/eywa/commit/093d8e20f9681c9b45eda93f10784648ea5e1304))

## [1.13.0](https://github.com/wmulabs/eywa/compare/v1.12.0...v1.13.0) (2026-06-16)


### Features

* **gemini:** native token streaming (GenerateStream) ([#92](https://github.com/wmulabs/eywa/issues/92)) ([ed66fdc](https://github.com/wmulabs/eywa/commit/ed66fdc5bcefa0f7c139b0839c1f502457af82ce))

## [1.12.0](https://github.com/wmulabs/eywa/compare/v1.11.0...v1.12.0) (2026-06-16)


### Features

* **openai:** native token streaming (GenerateStream) ([#90](https://github.com/wmulabs/eywa/issues/90)) ([6de8f27](https://github.com/wmulabs/eywa/commit/6de8f27c6c9884fbc4ab7ab1a6051d0e0489ff47))

## [1.11.0](https://github.com/wmulabs/eywa/compare/v1.10.0...v1.11.0) (2026-06-16)


### Features

* **anthropic:** native token streaming (GenerateStream) ([#88](https://github.com/wmulabs/eywa/issues/88)) ([1c7ce2b](https://github.com/wmulabs/eywa/commit/1c7ce2be9850bf016f16fef41bb69ed38d93d021))

## [1.10.0](https://github.com/wmulabs/eywa/compare/v1.9.0...v1.10.0) (2026-06-16)


### Features

* **orchestrator:** streaming reasoning spine (ExecuteStream) ([#86](https://github.com/wmulabs/eywa/issues/86)) ([51b5861](https://github.com/wmulabs/eywa/commit/51b5861d59a925c506a74264ba0b58523fb617a8))

## [1.9.0](https://github.com/wmulabs/eywa/compare/v1.8.0...v1.9.0) (2026-06-16)


### Features

* **orchestrator:** arg-aware action ban allows corrected-argument retries ([#84](https://github.com/wmulabs/eywa/issues/84)) ([85ff168](https://github.com/wmulabs/eywa/commit/85ff1685a056f308b82b996832e1fe06d2ef58ff))

## [1.8.0](https://github.com/wmulabs/eywa/compare/v1.7.0...v1.8.0) (2026-06-16)


### Features

* **orchestrator:** confidence-gated human takeover for low-confidence turns ([#81](https://github.com/wmulabs/eywa/issues/81)) ([c1b69cd](https://github.com/wmulabs/eywa/commit/c1b69cd5ffe25d9aa555b8e0d4ab00431226fb05))

## [1.7.0](https://github.com/wmulabs/eywa/compare/v1.6.0...v1.7.0) (2026-06-15)


### Features

* **orchestrator:** per-Spirit draft model for cheaper intermediate steps ([#79](https://github.com/wmulabs/eywa/issues/79)) ([8daacf5](https://github.com/wmulabs/eywa/commit/8daacf525251b6e1a3d313aaf76f04d48a136628))

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
