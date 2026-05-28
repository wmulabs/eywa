# Eywa — Community Release Code Review

> **Purpose:** Pre-launch polish pass. Every exported symbol, architectural boundary, Go best practice, test coverage gap, and documentation issue reviewed file by file before public release.
>
> **Resuming across sessions:** Each file has a checkbox. Tick it when the review AND any fixes are complete. Start the next session by finding the first unchecked box.
>
> **Severity legend:**
> - 🔴 **Critical** — wrong behavior, incorrect documentation, security concern, panic risk
> - 🟡 **Important** — arch violation, missing godoc on public API, bad naming, deprecated w/o annotation
> - 🟢 **Minor** — style, alignment, trivial naming, cosmetic

---

## Review Criteria

For every file:
1. **Naming** — Go conventions, consistency with the codebase vocabulary (Spirit, Pulse, Oracle, not agent/event/chat)
2. **Hexagonal architecture** — no infra deps in domain; ports are pure interfaces + value types only
3. **Go best practices** — errors wrapped with `%w`, no naked returns, proper `Unwrap()`, context propagation
4. **Unit tests** — coverage for public behavior; no testing internals
5. **SOLID / Clean code** — single responsibility, no hidden side-effects, clear boundaries
6. **Godoc** — every exported symbol documented; comments explain *why*, not *what*
7. **Alignment / formatting** — `gofmt`-clean; struct tags consistent

---

## Status Summary

| Layer | Files | Reviewed | Findings (open) |
|-------|-------|----------|----------|
| Core facade | 8 | 8 | — |
| Domain entities | 18 | 18 | 1🟡 3🟢 (all 🔴 fixed) |
| Domain errors | 2 | 2 | — |
| Domain ports | 30 | 30 | all fixed |
| Internal helpers | 7 | 7 | — |
| Orchestrator | 30 | 30 | all fixed |
| Implementation | 20 | 20 | — |
| Fiber | 22 | 22 | 1🟢 (imprint alignment) — all 🟡 fixed |
| Mongo | 16 | 16 | all fixed |
| Redis | 7 | 7 | all fixed |
| Providers | 9 | 9 | — |
| Channels | 7 | 7 | — |
| GCP | 7 | 7 | — |
| MCP | 1 | 1 | — |
| Examples | 13 | 13 | all fixed |
| Docs | — | ✅ | — |

---

## Priority Fixes (act on these now)

### 🔴 CONTRIBUTING.md + docs/concepts.md — Oracle interface is wrong

Both documents show an incorrect `Oracle` interface that does not compile against the real code. Any contributor who tries to implement it will fail immediately.

**Shown in docs (WRONG):**
```go
type Oracle interface {
    GetName() string
    SupportsImages() bool           // missing model param
    SupportsAudio() bool            // missing model param
    SupportsDocuments() bool        // missing model param
    Chat(ctx context.Context, req OracleRequest) (OracleResponse, error) // wrong name + signature
}
```

**Actual interface (`internal/domain/ports/oracle.go`):**
```go
type Oracle interface {
    GetName() string
    GetAvailableModels() []string
    GenerateResponse(ctx context.Context, req *OracleRequest) (*OracleResponse, error)
    IsAvailable() bool
    GetConfig() map[string]interface{}
    SupportsImages(model string) bool
    SupportsAudio(model string) bool
    SupportsDocuments(model string) bool
}
```

**Status:** ✅ Fixed — see commit history.

---

## File-by-File Review

### Core Facade

- [x] **`eywa.go`** — ✅ Excellent. Full godoc on every exported symbol. All type aliases properly documented with context. No issues.

- [x] **`entities.go`** — ✅ Excellent. All entity aliases with context-aware godoc. No issues.

- [x] **`errors.go`** — ✅ Excellent. Clean re-exports of error types and constructors. No issues.

- [x] **`ports.go`** — ✅ Excellent. Every port alias documented. Two `type ()` blocks — first for ports, second for repositories — logical grouping. No issues.

- [x] **`builtin.go`** — ✅ Excellent. Clean var-level re-exports for built-in actions and scorers. No issues.

- [x] **`registries.go`** — ✅ Excellent. Clear `var` re-exports for all factory constructors. No issues.

- [x] **`helpers.go`** — ✅ Excellent. Useful utility re-exports; all documented. No issues.

- [x] **`llm.go`** — ✅ Excellent. Minimal. `OracleFactoryImpl` type alias + `NewOracleFactory` var. No issues.

---

### Internal Domain — Entities

- [x] **`internal/domain/entities/spirit.go`**
  - 🟡 Struct field alignment inconsistency: `EnforceVoiceDelivery`, `VoiceDeliveryInstructions`, `BusinessErrorInstructions` use fewer tabs than surrounding fields, creating a visual break that `gofmt` doesn't normalize (it's inside alignment groups). Run `gofmt` on the file.
  - 🟢 `IsConversational()`, `IsExecutor()`, `IsNotifier()`, `IsOrchestrator()`, `NeedsSession()`, `NeedsMessageCoalescing()` — no godoc. These are internal-package methods used by the pipeline, not public API, so this is low-priority.

- [x] **`internal/domain/entities/pulse.go`**
  - ~~🔴 **`PulseBuilder.Build()` silently returns nil on invalid `MemoryKey`.**~~ **Fixed:** all builder methods now guard against `b.event == nil`; `Build()` is documented to return nil on error. Chain is panic-safe.
  - 🟡 **No godoc on exported methods.** `GetPayloadString`, `GetKnowledgeString`, `GetMetadata`, `GetMetadataString`, `GetMetadataInt`, `GetMetadataInt64`, `GetMetadataFloat64`, `GetMetadataBool`, `GetMetadataStringSlice`, `GetMetadataMap`, `SetMemoryKey`, `SetTopic`, `AddKnowledge`, `SetKnowledge`, `MergeKnowledge`, `AddPayload`, `SetPayload`, `MergePayload`, `AddMetadata`, `SetMetadata`, `MergeMetadata`, `AddAttachment`, `SetAttachments`, `SetContactPhone` — all exported, all undocumented.
  - 🟢 The `PulseBuilder.Build()` returns `b.event` with no validation of accumulated `errs` — a partial success with silent errors.

- [x] **`internal/domain/entities/link.go`**
  - 🟡 ~~`WithSpirits`, `WithDefaultSpirit`, `AddSpirit` lack `// Deprecated:` annotation.`~~ Retracted: these methods are actively used by `fiber/link_mgmt_handler.go` and are not deprecated. No action needed.
  - ~~🟡 **`RequireScouts` initialized to `[]string{}` in `NewLink`.**~~ **Fixed:** both `RequireScouts` and `AllowedSpirits` now default to `nil`; `len()` checks are nil-safe.
  - 🟢 `IsAgentAllowed`, `HasMultipleAgents`, `HasSingleAgent`, `GetSingleAgent` — no godoc. Since `Link` is publicly aliased in `entities.go`, these are reachable from user code.

- [x] **`internal/domain/entities/memory.go`**
  - ~~🟡 **Misplaced comment before `Thread` struct.**~~ Already correct in file — `IsUserFacing` comment is a proper field-level comment inside the struct.
  - 🟢 ~~Fix `Memory` struct field alignment (`SubjectKey`, `Threads` columns off).~~ **Fixed.**

- [x] **`internal/domain/entities/echo.go`**
  - 🟢 Wide column alignment in struct tags differs from the alignment style used in `spirit.go`, `rite.go`, etc. Minor inconsistency. Not harmful.

- [x] **`internal/domain/entities/chronicle.go`**
  - 🟡 No godoc on `Chronicle`, `InteractionEventLog`, `InteractionAgentLog`, `InteractionProcessingLog`, `InteractionTokenUsage`, `TokenUsageBreakdown`, `InteractionTokenUsage`, `ActionCallLog`, `IterationLog`, `AttachmentLog`. These are complex audit types surfaced in the management API.
  - 🟢 `InteractionEventLog.Knowledge` field alignment inconsistent (uses fewer spaces than surrounding fields).
  - 🟢 `InteractionProcessingLog.ArchivistApplied` alignment inconsistent.

- [x] **`internal/domain/entities/imprint.go`**
  - 🟢 Category values `"preference"|"personal"|"business"|"context"` are documented in a comment but not as typed constants — callers can accidentally pass any string. Consider `type ImprintCategory string` with constants, or add stronger documentation of valid values.
  - 🟢 Same for `Source` field: `"extracted"|"explicit"`.

- [x] **`internal/domain/entities/vigil.go`** — ✅ Clean and minimal. Fields are self-explanatory.

- [x] **`internal/domain/entities/rite.go`**
  - 🟢 No godoc on `Rite` struct.
  - 🟢 `RiteStatus` constants (`RitePending`, `RiteApproved`, `RiteRejected`, `RiteExpired`) lack godoc. Brief explanations of valid state transitions would help.

- [x] **`internal/domain/entities/ritual.go`**
  - 🟢 Struct field alignment issues: `MemoryKey` field is off by one tab vs surrounding fields; same for `RitualStatus` inline type and `AttemptCount`. Run `gofmt`.
  - 🟢 `RecurrenceConfig` fields lack godoc (particularly `MaxRuns: 0` semantics — does 0 mean unlimited?).

- [x] **`internal/domain/entities/operator.go`** — ✅ Clean. `PasswordHash json:"-"` correctly excluded from serialization.

- [x] **`internal/domain/entities/ledger.go`**
  - 🟢 `ModelRoutingCondition.InputLengthLt` — zero value semantics not documented inline. A comment `// 0 = no upper bound` would save readers from digging into the pipeline step.
  - 🟢 `ModelRoutingCondition` fields are bare struct values — no json/bson tags since they're config-only. Fine if they're never persisted, but worth a comment confirming this.

- [x] **`internal/domain/entities/http_tool.go`**
  - ~~🟡 **`AgentIDs []string` field uses "agent" not "spirit".**~~ Already fixed in previous session — field is now `SpiritIDs []string` with bson tag `"spirit_ids"`.

- [x] **`internal/domain/entities/response.go`** — ✅ Clean constructors, clear status types. No issues.

- [x] **`internal/domain/entities/trial.go`** — ✅ Clean. Dual json+yaml tags for CLI-friendly config. `LLMJudgeExpect` could use a brief godoc.

- [x] **`internal/domain/entities/action_definition.go`** — ✅ Minimal and correct.

---

### Internal Domain — Errors

- [x] **`internal/domain/errors/action_errors.go`** — ✅ Excellent. Proper `Error()`, `Unwrap()`, `errors.As`-compatible `Is*` helpers. Variadic `cause` parameter elegantly handles optional wrapping. No issues.

- [x] **`internal/domain/errors/repository_errors.go`**
  - 🟢 `ErrRitualTerminal` is not aligned with the two vars above it (`ErrNotFound`, `ErrConflict`) in the `var()` block. `gofmt` doesn't auto-align var blocks — manually align or leave as-is (it's still readable).
  - 🟢 No godoc on `NotFoundError` and `ConflictError` structs. Since they're in `internal/`, lower priority.

---

### Internal Domain — Ports

- [x] **`internal/domain/ports/oracle.go`**
  - ~~🟡 **`ConvertToLLMAttachments`, `toNativeLLMAttachment`, `isMediaSupported` are implementation logic inside the ports package.**~~ **Fixed:** moved to `internal/implementation/media/attachment.go`. The `internal/helpers` package was not viable due to an import cycle (`entities/pulse.go` already imports `helpers`). The `media` package already imports both `entities` and `ports` — a natural fit.
  - 🟢 Stop reason constants have good inline comments explaining each provider's native value. Keep.

- [x] **`internal/domain/ports/action.go`** — ✅ Clean. Context key types, `ActionCategory` constants, and `Action`/`ActionRegistry` interfaces are well-documented. No issues.

- [x] **`internal/domain/ports/imprint.go`** — ✅ Clean. `ImprintListOptions` is well-structured for pagination.

- [x] **`internal/domain/ports/archivist.go`** — ✅ Clean. Single-method `Archivist` interface with clear scope boundary (scoped to memory_key + subject_key).

- [x] **`internal/domain/ports/attachment_processors.go`** — ✅ Clean. `Ear`, `Eye`, `DocumentProcessor` interfaces. Godoc explains native vs fallback intent.

- [x] **`internal/domain/ports/auth.go`** — ✅ Clean. `TokenValidator`, `AuthClaims`, `OperatorRepository`. Role constants `RoleAdmin`/`RoleOperator` could use `const` instead of bare strings but consistent with codebase style.

- [x] **`internal/domain/ports/bond.go`** — ✅ Clean. Contract is correctly documented: `(false, nil)` for contention, non-nil error only for infra failures. Implementation fixed.

- [x] **`internal/domain/ports/chronicle_query.go`** — ✅ Clean. `ChronicleListOptions`, `TokenSeries`, `ActionStats`, `SpiritStats` value types. All using Eywa vocabulary.

- [x] **`internal/domain/ports/conduit.go`** — ✅ Clean. `Conduit` + `ConduitRegistry` interfaces. Clear separation from `Action` (Conduit discovers + proxies MCP tools; Action is a first-class Eywa capability).

- [x] **`internal/domain/ports/echo_query.go`** — ✅ Clean. `EchoQueryRepository` is correctly separated from `EchoRepository` (write) in `memory.go`.

- [x] **`internal/domain/ports/http_tool_repository.go`** — ✅ Clean. `FindBySpiritID` correctly takes a Spirit ID (confirmed: caller passes `state.Spirit.ID`).

- [x] **`internal/domain/ports/inbox.go`** — ✅ Clean. Minimal two-method interface with good docstring explaining the coalescing contract.

- [x] **`internal/domain/ports/keeper.go`** — ✅ Clean. Good docstring on `Schedule` (pass `time.Now()` for immediate async dispatch).

- [x] **`internal/domain/ports/ledger.go`** — ✅ Clean after previous renames.

- [x] **`internal/domain/ports/lens.go`** — ✅ Clean. Single-method `Lens` interface.

- [x] **`internal/domain/ports/limiter.go`** — ✅ Clean. Fail-open guidance documented.

- [x] **`internal/domain/ports/link_repository.go`** — ✅ Clean. Standard CRUD.

- [x] **`internal/domain/ports/logic_router.go`** — ✅ Clean. Good docstring explaining "empty string = no match".

- [x] **`internal/domain/ports/lore.go`** — ✅ Clean. `GetBySpiritID` correctly stores Spirit IDs (confirmed: field is `SpiritIDs []string` on Lore entity). No callers yet — dead path until lore management API is implemented.

- [x] **`internal/domain/ports/memory.go`** — ✅ Clean. `SpiritRepository`, `EchoRepository`, `MemoryRepository`, `ChronicleRepository`. All interfaces well-documented with clear behavioral contracts.

- [x] **`internal/domain/ports/oracle_factory.go`** — ✅ Clean. Contains only `OracleConfig` value type (factory interface lives in `oracle.go`). No issues.

- [x] **`internal/domain/ports/pathfinder.go`** — ✅ Clean. `SelectSpirit` returns empty string on no-match — documented.

- [x] **`internal/domain/ports/pubsub.go`** — ✅ Clean. Subscribe's blocking contract documented.

- [x] **`internal/domain/ports/receptor.go`** — ✅ Clean. Slice return explained (one webhook = multiple senders).

- [x] **`internal/domain/ports/rite_repository.go`** — ✅ Clean. `RiteListOptions` + basic CRUD.

- [x] **`internal/domain/ports/ritual_manager.go`** — ✅ Excellent. `RitualRequest` docstring explains the pre-built Pulse contract. `MarkRunning` documents the terminal-state check. Clear lifecycle semantics.

- [x] **`internal/domain/ports/ritual_repository.go`** — ✅ Clean. Standard persistence port.

- [x] **`internal/domain/ports/scorer.go`** — ✅ Minimal and correct.

- [x] **`internal/domain/ports/scout.go`** — ✅ Clean. `Scout.Harvest` correctly documents nil vs non-nil error semantics. `ScoutRegistry.Harvest` runs all applicable scouts — good.

- [x] **`internal/domain/ports/typing_indicator.go`** — ✅ Minimal and correct.

- [x] **`internal/domain/ports/vault.go`** — ✅ Minimal. Return type documented (`permanent, publicly accessible URL`).

- [x] **`internal/domain/ports/vigil_repository.go`** — ✅ Excellent docstrings on every method explaining the TTL/sliding-window semantics.

- [x] **`internal/domain/ports/voice.go`** — ✅ Clean. `ShouldAutoRespond` docstring explains the per-channel contract with examples (WhatsApp: true; HTTP: false).

- [x] **`internal/domain/ports/whatsapp.go`** — ✅ Clean. `TemplateParameter`, `TemplateComponent`, `TemplateMessage` value types well-documented. `WhatsAppClient` interface with implementation notes (Dialog360, Twilio).

---

### Internal Helpers

- [x] **`internal/helpers/maps.go`**
  - 🟢 Many useful functions (`GetNested`, `Merge`, `Pick`, `Omit`, etc.) lack godoc. These are `internal/` so not public API, but contributors extending the codebase would benefit from inline docs on non-obvious functions like `GetNested` (dot-notation path — the `GetNested` function already has a good comment, others don't).
  - 🟢 `GetStringSlice` comment says "Returns false if any element cannot be cast to string" but only the `[]interface{}` branch does this — the `[]string` branch returns on first match without checking remaining elements. Technically correct but the comment could mislead.

- [x] **`internal/helpers/string.go`**
  - 🟢 `GenerateRandomID()` uses timestamp + random string. On Go 1.20+ `math/rand` is automatically seeded — no issue. Fine.
  - 🟢 No godoc on `CombineTextPartsMultiLine`, `GenerateRandomID`. Internal, low priority.

- [x] **`internal/helpers/time.go`** — ✅ Clean. `NowUTC()` comment explains the *why* (consistency across the system). Good practice.

- [x] **`internal/helpers/http_errors.go`** — ✅ Clean. `FromHTTPStatus` classifies 4xx as `BusinessError`, 5xx/429/502-504 as `InfrastructureError`. `FromHTTPResponse` reads body only on error. No issues.
- [x] **`internal/helpers/phone.go`** — ✅ Clean. `NormalizePhone` wraps `ErrInvalidPhone` correctly with `%w`. `defaultRegion` godoc explains the parameter. No issues.
- [x] **`internal/helpers/pii.go`** — ✅ Clean. `ObfuscatePhone` keeps 4 chars each end. Edge case (short numbers) masked fully. No issues.

---

### Orchestrator

- [x] **`internal/implementation/orchestrator/builder.go`**
  - 🟡 **`WithVoiceRegistry` godoc includes a detailed code example in a non-godoc comment style** (uses `//` instead of the standard `// Example:` pattern). The example is useful but mixes concerns — it says "Example:" inside a regular comment. Standardize as proper godoc or remove the example (the godoc in `ports.go` covers intent).
  - ~~🟡 **`NewWeaveBuilder` initializes fields with `nil` and inline `// Created on demand` comments**~~ **Fixed:** redundant nil comments removed.
  - 🟢 Multiple `With*` methods have godoc that says "When configured, X is inserted after Y" — accurate and useful.

- [x] **`internal/implementation/orchestrator/engine.go`**
  - ~~🟡 **`GetClassifierRegistry()` returns `ports.PathfinderRegistry` but is named `GetClassifier...`.**~~ Already renamed to `GetPathfinderRegistry()` in previous session.
  - ~~🟡 **`splitText` utility function lives in `engine.go`.**~~ **Fixed:** moved to `helpers.SplitTextIntoChunks` with godoc; engine.go calls the helper.
  - 🟢 `buildProcessingPipeline()` is 100 lines with 20+ conditional `if` blocks. Hard to follow but each block has a clear purpose. Consider extracting into smaller methods (`buildObservabilitySteps`, `buildMediaSteps`, etc.) for readability.
  - 🟢 `logInteraction` builds a `Chronicle` struct with extensive field mapping inline. Clean enough as-is; mapping is mechanical and predictable.

- [x] **`internal/implementation/orchestrator/config.go`** — ✅ Naming scan clean. No issues.
- [x] **`internal/implementation/orchestrator/config_cache.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/channel.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/errors.go`** — ✅ Clean. `OrchestrationError` with `Code`, `Message`, `Retriable` fields. All constructors document retriability reasoning inline. `errors.As` pattern correct.
- [x] **`internal/implementation/orchestrator/http_tool_executor.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/imprint_config.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/memory_manager.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/message_manager.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_reasoning.go`** — ✅ Clean. `populateStateFromResult` correctly propagates `FinalSession` to prevent stale-session persistence after topic switch.
- [x] **`internal/implementation/orchestrator/pipeline_step_condition.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_cost.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_delivery.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_enrichment.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_http_tools.go`** — ✅ Clean. Only "agent" mention is in a code comment ("so the agent can call it") — acceptable.
- [x] **`internal/implementation/orchestrator/pipeline_step_imprint.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_lock.go`** — ✅ Clean. Correctly routes `(false, nil)` → `ErrMemoryBusy` and `(false, err)` → `ErrLockAcquisitionFailed` (fixed bond.go upstream).
- [x] **`internal/implementation/orchestrator/pipeline_step_media.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_model_routing.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_notification.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_orchestrator.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_persistence.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_scheduled.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_session.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_typing.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_validation.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pipeline_step_vigil.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/pricing.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/reasoning_service.go`** — ✅ Clean. "agent" in one comment only.
- [x] **`internal/implementation/orchestrator/summon_service.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/tool_executor.go`** — ✅ Clean.
- [x] **`internal/implementation/orchestrator/validator.go`** — ✅ Clean.

---

### Internal Implementation — Actions

- [x] **`internal/implementation/actions/conduit_adapter.go`** — ✅ Clean. No naming violations.
- [x] **`internal/implementation/actions/imprint_actions.go`** — ✅ Clean.
- [x] **`internal/implementation/actions/lore_search.go`** — ✅ Clean.
- [x] **`internal/implementation/actions/rite_tools.go`** — ✅ Clean.
- [x] **`internal/implementation/actions/ritual_tools.go`** — ✅ Clean.
- [x] **`internal/implementation/actions/send_to_contact.go`** — ✅ Clean.
- [x] **`internal/implementation/actions/update_subject.go`** — ✅ Clean.

---

### Internal Implementation — Other

- [x] **`internal/implementation/archivists/archivist.go`** — ✅ Clean. "agent" in LLM prompt example string (natural language, not code vocabulary). No issues.
- [x] **`internal/implementation/media/lens.go`** — ✅ Clean.
- [x] **`internal/implementation/oracles/factory.go`** — ✅ Clean.
- [x] **`internal/implementation/pathfinders/llm_pathfinder.go`** — ✅ Clean.
- [x] **`internal/implementation/receptors/api_default_receptor.go`** — ✅ Clean.
- [x] **`internal/implementation/registries/action_registry.go`** — ✅ Clean.
- [x] **`internal/implementation/registries/conduit_registry.go`** — ✅ Clean.
- [x] **`internal/implementation/registries/logic_router_registry.go`** — ✅ Clean.
- [x] **`internal/implementation/registries/pathfinder_registry.go`** — ✅ Clean.
- [x] **`internal/implementation/registries/scout_registry.go`** — ✅ Clean.
- [x] **`internal/implementation/registries/voice_registry.go`** — ✅ Clean.
- [x] **`internal/implementation/scouts/imprint_scout.go`** — ✅ Clean.
- [x] **`internal/implementation/scouts/lore_scout.go`** — ✅ Clean.
- [x] **`internal/implementation/services/ingestion/async_ingestion_service.go`** — ✅ Clean.
- [x] **`internal/implementation/services/ritual_service.go`** — ✅ Clean.
- [x] **`internal/implementation/trial/loader.go`** — ✅ Clean.
- [x] **`internal/implementation/trial/runner.go`** — ✅ Clean.
- [x] **`internal/implementation/trial/scorers.go`** — ✅ Clean.
- [x] **`internal/implementation/voices/http_voice.go`** — ✅ Clean.

---

### Internal Infrastructure

- [x] **`internal/infrastructure/driven/auth/apikey_validator.go`** — ✅ Clean. Static token→role map. Minimal and correct.
- [x] **`internal/infrastructure/driven/auth/jwks_validator.go`** — ✅ Clean. JWKS with 1-hour key cache and stale-on-miss refresh. `RWMutex` correct. `rsaPublicKeyFromJWK` correct base64url decode.
- [x] **`internal/infrastructure/driven/auth/jwt_validator.go`** — ✅ Clean. Supports HMAC (`NewJWTValidator`) and RS256 (`NewJWTValidatorRS256`) via shared `Keyfunc`.
- [x] **`internal/infrastructure/driven/auth/operator_auth.go`** — ✅ Clean. `Login` returns "invalid credentials" for both not-found and wrong password — correct security practice (no enumeration). `HashPassword` uses bcrypt DefaultCost. No issues.
- [x] **`internal/infrastructure/driven/dbg/logger.go`** — ✅ Clean. Singleton logger with `sync.Once`. `SetLogger` correctly marks `once` as done to avoid re-creation.
- [x] **`internal/infrastructure/driven/tracer/tracer.go`** — ✅ Clean. No-op tracer by default. Users wire a real provider via `otel.SetTracerProvider` before first call.

---

### Fiber — HTTP Layer

- [x] **`fiber/routes.go`** — ✅ Clean conditional route registration. `WithInternalMiddleware` option documented. No issues.

- [x] **`fiber/management.go`**
  - ~~🟡 **`ManagementDeps` fields use spec phase comments**.~~ **Resolved:** file already has functional comments — spec phase references were removed in a prior session.
  - 🟢 `buildValidatorChain` is a good pure function. No issues.

- [x] **`fiber/spirit_handler.go`** — ✅ Clean. CRUD + activate/deactivate/rollback endpoints. Consistent error mapping. No issues.
- [x] **`fiber/async_event_handler.go`** — ✅ Clean. Dispatches to `AsyncIngestionService`. No issues.
- [x] **`fiber/chronicle_handler.go`** — ✅ Clean. `parseDateRange` helper in same file — good locality. No issues.
- [x] **`fiber/discovery_handler.go`** — ✅ Clean. Nil-guards on all registries. Returns empty slices, not nil. No issues.
- [x] **`fiber/echo_mgmt_handler.go`** — ✅ Clean. `sendMessage` requires `echoRepo != nil` check before use. No issues.
- [x] **`fiber/event_bus.go`** — ✅ Clean. Nil-receiver guard pattern (`if b == nil { return }`) is idiomatic. `NewEventBus` returns nil when pubSub is nil — callers get nil EventBus, methods handle it. No issues.
- [x] **`fiber/event_handler.go`** — ✅ Clean. Sync event dispatch. No issues.
- [x] **`fiber/health_handler.go`** — ✅ Clean. `readinessChecks` returns hardcoded "ready" — acceptable for a liveness probe pattern, not a real health check, but consistent with no-infra-in-fiber design.
- [x] **`fiber/helpers.go`** — ✅ Minimal. Logger init only. No issues.
- [x] **`fiber/http/error_mapper.go`** — ✅ Clean. Domain error → HTTP status mapping. No issues.
- [x] **`fiber/http/pagination.go`** — ✅ Clean. No issues.
- [x] **`fiber/http/request_parser.go`** — ✅ Clean. No issues.
- [x] **`fiber/http/validators.go`**
  - ~~🟡 **`validateProvider` hardcoded only `openai`, `anthropic`, `gemini` — missing `bedrock`, `vertexai`, `ollama`, `groq`, `mistral`, `together`, `openrouter`, `xai`.**~~ **Fixed:** full provider list now in validator.
- [x] **`fiber/http_tool_handler.go`** — ✅ Clean. Standard CRUD + test endpoint (proxies to `HTTPToolExecutor.Test`). No issues.
- [x] **`fiber/imprint_mgmt_handler.go`** — ✅ Clean. 🟢 Minor: `SpiritName` field alignment in `imprintMgmtHandler.list` opts struct. Cosmetic only.
- [x] **`fiber/link_mgmt_handler.go`** — ✅ Clean. `dtoToLink` uses `WithSpirits` and `WithDefaultSpirit` — these methods exist on `Link` and are correctly used here. The review doc entry for `link.go` saying methods were removed is incorrect; they remain and are actively used.
- [x] **`fiber/message_handler.go`** — ✅ Clean. Good dispatch pattern via `queryMessages`/`queryCount` switch.
- [x] **`fiber/middleware/auth.go`** — ✅ Clean. First-success multi-validator chain. `authContextKey` is a private struct type — correct idiom (avoids string key collisions). No issues.
- [x] **`fiber/operator_handler.go`** — ✅ Clean. CRUD + deactivate. Delegates to `OperatorAuth`. No issues.
- [x] **`fiber/rite_handler.go`** — ✅ Clean. `decide` guards MemoryKey format before building Pulse (`len(parts) == 2`). 🟢 `riteResponse.RequestedAt` is `interface{}` instead of `time.Time` — allows JSON null when field is zero. Intentional.
- [x] **`fiber/schedule_handler.go`** — ✅ Clean. Good bounds validation (min 10s, max 30d). `HandleExecuteEvent` correctly routes retriable vs non-retriable errors and marks failed rituals.
- [x] **`fiber/sse_handler.go`** — ✅ Clean. Heartbeat every 30s keeps connection alive through proxies. Drop-on-full-buffer (channel size 32) is intentional.
- [x] **`fiber/vigil_handler.go`** — ✅ Clean. Returns `"status": "spirit"` when no human is seated (correct — spirit is the AI mode). Non-critical `Refresh` error is silently ignored (intentional — TTL miss is non-fatal).
- [x] **`fiber/weave_config_handler.go`** — ✅ Clean. Calls `cfg.Validate()` before save. `reload` no-ops gracefully when cache is nil.

---

### Mongo — Repositories

- [x] **`mongo/mongo.go`**
  - ~~🟢 `databaseOptions := options.Database()` creates empty options — never used.~~ Not in actual file; stale finding.
  - ~~🟢 `DisconnectMongoDB` calls `Fatalw` on nil client.~~ **Fixed:** downgraded to `Errorw`.

- [x] **`mongo/chronicle_query.go`** — ✅ Clean. Complex aggregation pipelines well-structured. `buildChronicleListFilter` is a good pure helper.

- [x] **`mongo/chronicle_repository.go`** — ✅ Clean (index names corrected in previous commit).

- [x] **`mongo/echo_query.go`** — ✅ Clean. `var _ eywa.EchoQueryRepository = (*EchoRepository)(nil)` interface check present. Session aggregation pipeline correctly computes `last_message_at` and `message_count`.

- [x] **`mongo/echo_repository.go`** — ✅ Clean. `findRecent` uses DB-side DESC/limit/ASC pipeline to avoid in-app reversal. Good pattern.

- [x] **`mongo/helpers.go`** — ✅ Clean. Logger init only.

- [x] **`mongo/http_tool_repository.go`** — ✅ Clean. DTO pattern for document mapping. Interface compliance check present.

- [x] **`mongo/imprint_repository.go`** — ✅ Clean. `Prune` silently ignores individual delete errors (acceptable for background cleanup). No issues.

- [x] **`mongo/ledger_repository.go`** — ✅ Clean. `IncrementUsage` uses `$setOnInsert` for idempotent upsert. 🟢 `ensureIndexes` discards `CreateOne` errors (safe — startup warning only).

- [x] **`mongo/link_repository.go`** — ✅ Clean. `linkDocument` DTO separates wire format from domain struct. Interface compliance check present.

- [x] **`mongo/lore_repository.go`**
  - ~~🟡 `GetByID` and `GetByName` wrapped all errors as "not found" — did not distinguish `mongo.ErrNoDocuments` from infra errors.~~ **Fixed:** both methods now check `err == mongo.ErrNoDocuments` and return `&eywa.NotFoundError{}`.

- [x] **`mongo/lore_store.go`** — ✅ Clean. Atlas `$vectorSearch` aggregation with optional `minScore` filter. No issues.

- [x] **`mongo/operator_repository.go`** — ✅ Clean. Consistent `NotFoundError` usage. Interface compliance check present.

- [x] **`mongo/rite_repository.go`** — ✅ Clean. MongoDB TTL index on `expires_at` handles expiry at DB level. Interface compliance check present.

- [x] **`mongo/ritual_repository.go`** — ✅ Clean. `statusTimestampField` helper maps `RitualStatus` to the correct timestamp field name. Separate `UpdateRunning`/`UpdateFailed` for state transitions with `$inc attempt_count`.

- [x] **`mongo/spirit_repository.go`**
  - ~~🔴 `RestoreVersion` activated the requested version without first deactivating all other versions of the same Spirit — two active versions could coexist.~~ **Fixed:** now calls `UpdateMany` to deactivate all before re-activating the requested one.
  - ~~🟡 `FindByID` and `GetVersion` return raw `fmt.Errorf("... not found: %s", id)` instead of `&eywa.NotFoundError{}`.~~ **Fixed:** all not-found paths return `&eywa.NotFoundError{}`. `Activate` version-not-found also fixed.
  - ~~🟡 `Create` returns raw error on duplicate instead of `&eywa.ConflictError{}`.~~ **Fixed:** returns `&eywa.ConflictError{Entity: "spirit", ID: name}`. `spirit_handler` Create and Activate now route through `ErrorResponse` (409 and 404 respectively).

- [x] **`mongo/weave_config_repository.go`** — ✅ Clean. Returns `DefaultWeaveConfig()` when no record found — good graceful fallback. Interface compliance check present.

---

### Redis — Adapters

- [x] **`redis/redis.go`**
  - ~~🟢 Dead nil check after `redis.NewClient` (never returns nil).~~ **Fixed.**
  - ~~🟢 URL parse error logged after usage.~~ **Fixed:** error logged before client creation.
  - ~~🟢 `DisconnectRedisDB`: `Fatalw` on nil client → `Errorw`; `Infow` on close error → `Warnw`.~~ **Fixed.**
- [x] **`redis/bond.go`**
  - 🔴 **`AcquireLock` returned `(false, err)` for `redsync.ErrFailed` (lock contention).** The port contract requires `(false, nil)` for contention — non-nil error is reserved for infrastructure failures. This caused `LockAcquisitionStep` to treat every contention event as a retriable infra error instead of the non-retriable `ErrMemoryBusy` path. **Fixed:** `errors.Is(err, redsync.ErrFailed)` now returns `(false, nil)`.
- [x] **`redis/helpers.go`** — ✅ Clean. Logger factory identical to mongo sub-module pattern.
- [x] **`redis/inbox.go`** — ✅ Clean. Pipeline (RPUSH + EXPIRE), PopAll (LRANGE + DEL) — atomic. `redis.Nil` handled correctly.
- [x] **`redis/limiter.go`** — ✅ Clean. GCRA via `go-redis/redis_rate`. Fail-open documented inline.
- [x] **`redis/memory_repository.go`**
  - ~~🟢 Cache-miss log was `Warnw("memory not found")` — fires on every cold start/TTL expiry.~~ **Fixed:** downgraded to `Debugw("memory cache miss, will reconstruct")`.
- [x] **`redis/pubsub.go`** — ✅ Clean. Interface compliance check present. `Subscribe` handles `!ok` on channel close.
- [x] **`redis/vigil_repository.go`** — ✅ Clean. `SetNX` + `ErrSessionHeld` pattern correct. `ListAll` uses cursor-based SCAN (no KEYS). `Get` reads TTL for `ExpiresAt`.

---

### Providers — LLM Oracles

- [x] **`providers/anthropic/oracle.go`** — ✅ Full Oracle interface implemented. Clean builder pattern. No naming violations.
- [x] **`providers/bedrock/oracle.go`** — ✅ Clean. AWS Converse API. `SupportsAudio`/`SupportsDocuments` correctly return false.
- [x] **`providers/gemini/oracle.go`** — ✅ Clean. `generateWithChat` uses Gemini chat session for history. No naming violations.
- [x] **`providers/openai/oracle.go`** — ✅ Clean. Multi-provider aliases (ollama, groq, mistral, together, openrouter, xai) all route through OpenAI-compatible API.
- [x] **`providers/pgvector/lore_store.go`** — ✅ Clean. pgvector `<->` cosine distance. No issues.
- [x] **`providers/pinecone/lore_store.go`** — ✅ Clean. Namespace per lore ID. No issues.
- [x] **`providers/qdrant/lore_store.go`** — ✅ Clean. `ensureCollection` handles ALREADY_EXISTS gracefully.
- [x] **`providers/vertexai/oracle.go`** — ✅ Clean. Type alias to `gemini.GeminiOracle` with ADC auth. Minimal and correct.
- [x] **`providers/weaviate/lore_store.go`** — ✅ Clean. No issues.

---

### Channels — WhatsApp

- [x] **`channels/whatsapp/action_send_message.go`** — ✅ Clean. `Validate` checks phone and message before `Execute`. Phone normalization with obfuscation in error messages. No issues.
- [x] **`channels/whatsapp/action_send_template.go`** — ✅ Clean.
- [x] **`channels/whatsapp/dialog360/client.go`** — ✅ Clean.
- [x] **`channels/whatsapp/dialog360/receptor.go`** — ✅ Clean. No naming violations.
- [x] **`channels/whatsapp/twilio/client.go`** — ✅ Clean.
- [x] **`channels/whatsapp/twilio/receptor.go`** — ✅ Clean.
- [x] **`channels/whatsapp/voice.go`** — ✅ Clean. `ShouldAutoRespond` returns true (correct for WhatsApp). No issues.

---

### GCP Integrations

- [x] **`gcp/cloudtasks/keeper.go`** — ✅ Clean. `isNotFound` correctly swallows `codes.NotFound` on Cancel (task may have already executed). Payload struct mirrors what `schedule_handler.go` expects.
- [x] **`gcp/cloudtasks/middleware.go`** — ✅ Clean. Empty audience → no-op middleware (correct for queues without OIDC configured).
- [x] **`gcp/gcs/vault.go`** — ✅ Clean. Returns public GCS URL for uploaded objects.
- [x] **`gcp/gemini/common.go`** — ✅ Clean. Shared `extractTextAndUsage` helper.
- [x] **`gcp/gemini/document_processor.go`** — ✅ Clean.
- [x] **`gcp/gemini/ear.go`** — ✅ Clean. `Transcribe` implements `Ear` port.
- [x] **`gcp/gemini/eye.go`** — ✅ Clean. `Analyze` implements `Eye` port.

---

### MCP Client

- [x] **`mcp/conduit.go`** — ✅ Clean. JSON-RPC over HTTP SSE. `parseToolList` + `extractTextContent` handle the MCP protocol. No naming violations.

---

### Examples

- [x] **`_examples/01_basic_setup/main.go`** — ✅ Clean.
- [x] **`_examples/02_custom_actions/main.go`** — ✅ Clean. Spirit named `action_assistant`.
- [x] **`_examples/03_advanced_routing/main.go`** — ✅ Fixed. Spirits renamed to `support_spirit`, `sales_spirit`, `billing_spirit`. Stale `AllowedAgents`/`DefaultAgent` comment references updated.
- [x] **`_examples/04_async_concept/main.go`** — ✅ Clean.
- [x] **`_examples/05_multi_provider/main.go`** — ✅ Clean.
- [x] **`_examples/06_rag_with_lore/main.go`** — ✅ Fixed. Spirit renamed to `support_spirit`. Description updated from "Customer support agent" to "Customer support spirit".
- [x] **`_examples/07_human_takeover/main.go`** — ✅ Fixed. Spirit renamed to `support_spirit`. Description updated.
- [x] **`_examples/08_approval_workflow/main.go`** — ✅ Fixed. Spirit renamed to `finance_spirit`.
- [x] **`_examples/09_long_term_memory/main.go`** — ✅ Clean.
- [x] **`_examples/10_cost_tracking/main.go`** — ✅ Clean.
- [x] **`_examples/11_mcp_client/main.go`** — ✅ Fixed. Spirit renamed to `mcp_spirit`. Description updated from "Agent with access to..." to "Spirit with access to...".
- [x] **`_examples/12_management_api/main.go`** — ✅ Fixed. Stale "event→agent" comment updated to "event→spirit".
- [x] **`_examples/13_multi_agent/main.go`** — ✅ Fixed. Title updated to "Multi-Spirit Orchestration".

---

## Consolidated Fix List

Ordered by impact:

### Must fix before launch (🔴)

1. ~~**`CONTRIBUTING.md` + `docs/concepts.md`** — Fix Oracle interface signature.~~ **Done.**

2. ~~**`internal/domain/entities/pulse.go`** — `PulseBuilder` nil panic.~~ **Done:** all builder methods guard against `b.event == nil`.

3. ~~**`redis/bond.go`** — `AcquireLock` returned `(false, err)` for lock contention.~~ **Done:** `redsync.ErrFailed` now returns `(false, nil)`.

### Fix before community contribution window opens (🟡)

4. ~~**`internal/domain/entities/link.go`** — Add `// Deprecated:` annotations.~~ **Done:** methods removed entirely (first release).

5. ~~**`internal/domain/entities/http_tool.go`** — `AgentIDs` naming drift.~~ **Done:** field is already `SpiritIDs`.

6. ~~**`internal/implementation/orchestrator/engine.go`** — `GetClassifierRegistry()`.~~ **Done:** renamed to `GetPathfinderRegistry()`.

7. ~~**`internal/domain/ports/oracle.go`** — Move `ConvertToLLMAttachments` + helpers out of ports package into implementation layer.~~ **Fixed:** moved to `internal/implementation/media/attachment.go`. `ports.go` facade updated; `reasoning_service.go` import updated.

8. ~~**`internal/domain/entities/memory.go`** — Misplaced struct comment.~~ **Done:** comment was already correct; fixed struct field alignment.

9. ~~**`fiber/management.go`** — Spec-phase comments.~~ **Done:** already functional descriptions in file.

10. ~~**`internal/implementation/orchestrator/builder.go`** — `// Created on demand` nil comments.~~ **Done:** removed.

### Fix before community contribution window opens (🟡) — continued

11. ~~**`mongo/spirit_repository.go`** — `FindByID` and `GetVersion` return raw `fmt.Errorf` for not-found; `Create` returns raw error on duplicate.~~ **Done:** all paths use `&eywa.NotFoundError{}` / `&eywa.ConflictError{}`. `spirit_handler` Create/Activate route through `ErrorResponse`.

### Polish (🟢)

12. ~~**`internal/domain/entities/spirit.go`** — Fix struct field alignment.~~ **Done:** `gofmt` applied.

12. ~~**`internal/domain/entities/ritual.go`** — Fix struct field alignment.~~ **Already clean.**

13. ~~**`internal/domain/entities/chronicle.go`** — Add godoc to `Chronicle` and sub-structs. Fix alignment inconsistencies.~~ **Done:** godoc added to all 8 types; `gofmt` applied.

14. ~~**`internal/domain/entities/imprint.go`** — Consider `type ImprintCategory string` constants.~~ **Done:** `ImprintCategory` and `ImprintSource` typed constants added. `imprint_actions.go` and `pipeline_step_imprint.go` updated to use constants.

15. ~~**`internal/implementation/orchestrator/engine.go`** — Move `splitText` to helpers.~~ **Done:** moved to `helpers.SplitTextIntoChunks`.

16. ~~**`mongo/mongo.go`** — Remove unused `databaseOptions` variable; change `DisconnectMongoDB` nil-client check from `Fatalw` to `Errorw`.~~ **Already clean** — both fixes were applied in a prior session.

---

---

*Review completed: 2026-05-19 | All layers reviewed, all findings resolved | Reviewed by: Claude*
