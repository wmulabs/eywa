# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.47.1](https://github.com/wmulabs/eywa/compare/v1.47.0...v1.47.1) (2026-07-10)


### Bug Fixes

* **api:** snake_case json tags, lore response timestamps, truthful REST docs ([#202](https://github.com/wmulabs/eywa/issues/202)) ([1552e6e](https://github.com/wmulabs/eywa/commit/1552e6e399fd9d7802364065f3b9ab66862460cf))

## [1.47.0](https://github.com/wmulabs/eywa/compare/v1.46.0...v1.47.0) (2026-07-09)


### Features

* ledger/budgets and lore management REST API ([#197](https://github.com/wmulabs/eywa/issues/197)) ([03ae9e8](https://github.com/wmulabs/eywa/commit/03ae9e8519073fb44b6fe000aa298ef1511143f7))

## [1.46.0](https://github.com/wmulabs/eywa/compare/v1.45.1...v1.46.0) (2026-07-08)


### Features

* **fiber:** opt-in CORS + cockpit-ready management example ([#195](https://github.com/wmulabs/eywa/issues/195)) ([e67a260](https://github.com/wmulabs/eywa/commit/e67a26080256889c5b3f757ed9a167fbe13ac844))

## [1.45.1](https://github.com/wmulabs/eywa/compare/v1.45.0...v1.45.1) (2026-07-06)


### Bug Fixes

* **spirit:** persist full Spirit config on create/update ([#187](https://github.com/wmulabs/eywa/issues/187)) ([bd54cab](https://github.com/wmulabs/eywa/commit/bd54caba42f94c5bad622cfee757d385f6a6ca76))

## [1.45.0](https://github.com/wmulabs/eywa/compare/v1.44.0...v1.45.0) (2026-06-28)


### Features

* **slack:** Slack Events API channel ([#184](https://github.com/wmulabs/eywa/issues/184)) ([cd992f2](https://github.com/wmulabs/eywa/commit/cd992f2be566c996c58f6ccf456411924017ea48))

## [1.44.0](https://github.com/wmulabs/eywa/compare/v1.43.0...v1.44.0) (2026-06-28)


### Features

* **telegram:** Telegram Bot API channel ([#182](https://github.com/wmulabs/eywa/issues/182)) ([429d5fe](https://github.com/wmulabs/eywa/commit/429d5fecfb887e0db0e6112fcaca450c33c159d0))

## [1.43.0](https://github.com/wmulabs/eywa/compare/v1.42.0...v1.43.0) (2026-06-27)


### Features

* peer-to-peer Spirit handoff (transfer of control) ([#176](https://github.com/wmulabs/eywa/issues/176)) ([3831cde](https://github.com/wmulabs/eywa/commit/3831cdebfa092ebeaae05c881c08aadc25a4dcd9))

## [1.42.0](https://github.com/wmulabs/eywa/compare/v1.41.0...v1.42.0) (2026-06-27)


### Features

* proactive model downgrade when budget hits AlertThreshold ([#174](https://github.com/wmulabs/eywa/issues/174)) ([2ad65e4](https://github.com/wmulabs/eywa/commit/2ad65e445b50a55164e0963ab0674e5f30efaa17))

## [1.41.0](https://github.com/wmulabs/eywa/compare/v1.40.0...v1.41.0) (2026-06-27)


### Features

* output guardrails (PII redaction + denylist) and jailbreak hardening ([#171](https://github.com/wmulabs/eywa/issues/171)) ([42fe424](https://github.com/wmulabs/eywa/commit/42fe424925c07a5ff22f950ca8d7724eee5d3b5c))

## [1.40.0](https://github.com/wmulabs/eywa/compare/v1.39.0...v1.40.0) (2026-06-27)


### Features

* **openai:** Azure OpenAI provider via NewAzureOracle ([#169](https://github.com/wmulabs/eywa/issues/169)) ([bfcd273](https://github.com/wmulabs/eywa/commit/bfcd2738aca2552e6525b6905e6840771e45fe5b))

## [1.39.0](https://github.com/wmulabs/eywa/compare/v1.38.0...v1.39.0) (2026-06-27)


### Features

* signature-based authentication for event ingestion (HMAC + provider webhooks) ([#161](https://github.com/wmulabs/eywa/issues/161)) ([71d1ff3](https://github.com/wmulabs/eywa/commit/71d1ff3d7ea064789ca45c3e177f159491cf4573))

## [1.38.0](https://github.com/wmulabs/eywa/compare/v1.37.0...v1.38.0) (2026-06-27)


### ⚠ BREAKING CHANGES

* **fiber:** single secure route registrar; auth all sensitive endpoints ([#158](https://github.com/wmulabs/eywa/issues/158))

### Features

* **fiber:** single secure route registrar; auth all sensitive endpoints ([#158](https://github.com/wmulabs/eywa/issues/158)) ([ce4e07f](https://github.com/wmulabs/eywa/commit/ce4e07f1efcaa9c9fc12f08677f2b2db0a5ed0ce))

## [2.0.0](https://github.com/wmulabs/eywa/compare/v1.37.0...v2.0.0) (2026-06-22)


### ⚠ BREAKING CHANGES

* **fiber:** single secure route registrar; auth all sensitive endpoints ([#158](https://github.com/wmulabs/eywa/issues/158))

### Features

* **fiber:** single secure route registrar; auth all sensitive endpoints ([#158](https://github.com/wmulabs/eywa/issues/158)) ([ce4e07f](https://github.com/wmulabs/eywa/commit/ce4e07f1efcaa9c9fc12f08677f2b2db0a5ed0ce))

## [1.37.0](https://github.com/wmulabs/eywa/compare/v1.36.0...v1.37.0) (2026-06-19)


### Features

* **orchestrator:** memoize tool results across durable resume ([#154](https://github.com/wmulabs/eywa/issues/154)) ([be11e6e](https://github.com/wmulabs/eywa/commit/be11e6ebfc6c7d417e4070abf6fea03039372456))

## [1.36.0](https://github.com/wmulabs/eywa/compare/v1.35.0...v1.36.0) (2026-06-19)


### Features

* **mongo,redis:** CheckpointStore adapters for durable execution ([#152](https://github.com/wmulabs/eywa/issues/152)) ([95d7c69](https://github.com/wmulabs/eywa/commit/95d7c6916a100266fb18bf8333b11fceae932d1c))

## [1.35.0](https://github.com/wmulabs/eywa/compare/v1.34.0...v1.35.0) (2026-06-19)


### Features

* **orchestrator:** checkpoint and resume reasoning turns (durable execution) ([#150](https://github.com/wmulabs/eywa/issues/150)) ([456f445](https://github.com/wmulabs/eywa/commit/456f4458db3200a48bec4c06fd1ff98d8ffec2b7))

## [1.34.0](https://github.com/wmulabs/eywa/compare/v1.33.0...v1.34.0) (2026-06-19)


### Features

* **pinecone:** metadata-filtered search (FilterableLoreStore) ([#146](https://github.com/wmulabs/eywa/issues/146)) ([9618043](https://github.com/wmulabs/eywa/commit/9618043589dec79ba555ecf42ae91da00df1dcc4))

## [1.33.0](https://github.com/wmulabs/eywa/compare/v1.32.0...v1.33.0) (2026-06-18)


### Features

* **qdrant:** metadata-filtered search (FilterableLoreStore) ([#145](https://github.com/wmulabs/eywa/issues/145)) ([5274811](https://github.com/wmulabs/eywa/commit/5274811bb22e5fca01f4def6b2abf66a87bcedd1))

## [1.32.0](https://github.com/wmulabs/eywa/compare/v1.31.0...v1.32.0) (2026-06-18)


### Features

* **lore:** distinct-object search (GroupByDocument) ([#142](https://github.com/wmulabs/eywa/issues/142)) ([8844537](https://github.com/wmulabs/eywa/commit/884453706b943446163273653b5b88cd66679c2e))

## [1.31.0](https://github.com/wmulabs/eywa/compare/v1.30.0...v1.31.0) (2026-06-18)


### Features

* **pgvector:** metadata-filtered search (FilterableLoreStore) ([#139](https://github.com/wmulabs/eywa/issues/139)) ([21f2149](https://github.com/wmulabs/eywa/commit/21f21494ff866babe38602d45d78ea7c24ede1e7))

## [1.30.0](https://github.com/wmulabs/eywa/compare/v1.29.0...v1.30.0) (2026-06-18)


### Features

* **lore:** filterable search foundation + direct query API ([#134](https://github.com/wmulabs/eywa/issues/134)) ([d76b99b](https://github.com/wmulabs/eywa/commit/d76b99bbfbeaf0a9cd09451ca29347f5aa508790))

## [1.29.0](https://github.com/wmulabs/eywa/compare/v1.28.0...v1.29.0) (2026-06-18)


### Features

* **rag:** recursive, rune-safe text chunker ([#125](https://github.com/wmulabs/eywa/issues/125)) ([9dfa151](https://github.com/wmulabs/eywa/commit/9dfa1510ad306b7b0f3a3068f6e221886eea2102))

## [1.28.0](https://github.com/wmulabs/eywa/compare/v1.27.0...v1.28.0) (2026-06-18)


### Features

* **openai:** native LoreEmbedder (embeddings API) ([#123](https://github.com/wmulabs/eywa/issues/123)) ([f50fad9](https://github.com/wmulabs/eywa/commit/f50fad91910dc2a232e3b6044615ca306e36a09b))

## [1.27.0](https://github.com/wmulabs/eywa/compare/v1.26.0...v1.27.0) (2026-06-18)


### Features

* **trial:** regression scorer for Chronicle replay ([#121](https://github.com/wmulabs/eywa/issues/121)) ([cf8bbce](https://github.com/wmulabs/eywa/commit/cf8bbcebead4422d329489d9fb9bf76dc20b9fe4))

## [1.26.0](https://github.com/wmulabs/eywa/compare/v1.25.0...v1.26.0) (2026-06-18)


### Features

* **trial:** replay recorded interactions from the Chronicle ([#119](https://github.com/wmulabs/eywa/issues/119)) ([f4a0f4c](https://github.com/wmulabs/eywa/commit/f4a0f4ceb4d0441f4f59251b436990176cfb3333))

## [1.25.0](https://github.com/wmulabs/eywa/compare/v1.24.0...v1.25.0) (2026-06-18)


### Features

* **langfuse:** OTLP exporter sub-module ([#117](https://github.com/wmulabs/eywa/issues/117)) ([1ee5738](https://github.com/wmulabs/eywa/commit/1ee5738d0699be5e762812426b2dd95f3ea5ef9f))

## [1.24.0](https://github.com/wmulabs/eywa/compare/v1.23.0...v1.24.0) (2026-06-18)


### Features

* GenAI semantic-convention spans (observability) ([#114](https://github.com/wmulabs/eywa/issues/114)) ([12a519e](https://github.com/wmulabs/eywa/commit/12a519e3957f8314208698aece53bfeec56a2780))

## [1.23.0](https://github.com/wmulabs/eywa/compare/v1.22.0...v1.23.0) (2026-06-17)


### Features

* **ollama:** native structured output (format schema) ([#112](https://github.com/wmulabs/eywa/issues/112)) ([e66b246](https://github.com/wmulabs/eywa/commit/e66b246785c487e592e70bf099877d0e2f391147))

## [1.22.0](https://github.com/wmulabs/eywa/compare/v1.21.0...v1.22.0) (2026-06-17)


### Features

* **ollama:** native token streaming (GenerateStream) ([#110](https://github.com/wmulabs/eywa/issues/110)) ([cad6abf](https://github.com/wmulabs/eywa/commit/cad6abf4f886c708386e9d2cbd885bf467d46871))

## [1.21.0](https://github.com/wmulabs/eywa/compare/v1.20.0...v1.21.0) (2026-06-17)


### Features

* **ollama:** native Ollama provider ([#108](https://github.com/wmulabs/eywa/issues/108)) ([824afe1](https://github.com/wmulabs/eywa/commit/824afe1ae2be88b3df542afda71d39db97f14649))

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
