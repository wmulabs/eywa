# 🌿 Contributing to Eywa

Thank you for wanting to make the Weave stronger.

Eywa is built on hexagonal architecture — the entire engine is wired through interfaces (ports). This means **every contribution is a new adapter**, and adapters are completely independent sub-modules. You don't need to understand the engine internals to contribute. You just need to implement one interface and publish it.

---

## What can be contributed

### 🔮 New LLM Oracle providers

Implement `eywa.Oracle` to add support for any LLM provider:

```go
type Oracle interface {
    GetName() string
    GetAvailableModels() []string
    GenerateResponse(ctx context.Context, req *OracleRequest) (*OracleResponse, error)
    IsAvailable() bool
    GetConfig() map[string]any

    // model is the specific model string from Spirit.ModelConfig.Model.
    SupportsImages(model string) bool
    SupportsAudio(model string) bool
    SupportsDocuments(model string) bool
}
```

**Ideas:** Cohere, Mistral (native), DeepSeek, Together AI, Fireworks AI, Azure OpenAI, Hugging Face Inference Endpoints, any local model via llama.cpp HTTP server.

Sub-module template: `providers/<provider-name>/`

---

### 🔍 New Vector Store adapters (Lore)

Implement `eywa.LoreStore` for semantic vector search:

```go
type LoreStore interface {
    Upsert(ctx context.Context, loreID string, chunks []LoreChunk) error
    Search(ctx context.Context, loreID string, embedding []float32, limit int, minScore float64) ([]LoreChunk, error)
    Delete(ctx context.Context, loreID string) error
}
```

**Ideas:** Chroma, Milvus, OpenSearch, Typesense, MongoDB Atlas Vector Search, Supabase pgvector (managed), Redis Vector Sets (Redis 8+).

Sub-module template: `providers/<store-name>/`

---

### 📣 New channel integrations (Voice + Receptor)

Add support for a new messaging channel. Two interfaces:

```go
// Inbound — convert raw webhook payloads to Pulses
type Receptor interface {
    GetName() string
    Convert(ctx context.Context, eventType string, raw map[string]any) ([]*Pulse, error)
}

// Outbound — deliver the Spirit's response
type Voice interface {
    GetName() string
    ShouldAutoRespond() bool
    SendResponse(ctx context.Context, event *Pulse, response string) error
    GetChannelMetadata(event *Pulse) map[string]any
}
```

**Ideas:** Telegram, Instagram DM, Slack, Discord, SMS (Twilio, Vonage), Email (SendGrid), WeChat, LINE, Viber, RCS.

Sub-module template: `channels/<channel-name>/`

---

### 🗄️ New infrastructure adapters

New database or cache backends for the core repositories:

```go
// Implement any of these ports with a new backend
eywa.SpiritRepository
eywa.EchoRepository
eywa.ChronicleRepository
eywa.MemoryRepository
eywa.Bond
eywa.ImprintRepository
eywa.RiteRepository
eywa.LoreRepository
eywa.LedgerRepository
```

**Ideas:** PostgreSQL, DynamoDB, Firestore, CockroachDB, ScyllaDB, Valkey, Momento (for Memory/Bond).

Sub-module template: `<db-name>/` (e.g. `postgres/`, `dynamodb/`)

---

### ☁️ New cloud provider integrations

New Vault (object storage) or Lens (media processing) implementations:

```go
type Vault interface {
    Upload(ctx context.Context, name string, data []byte, mime string) (string, error)
    Delete(ctx context.Context, url string) error
}

type Lens interface {
    Analyze(ctx context.Context, data []byte, mime string) (string, OracleUsage, error)
    Transcribe(ctx context.Context, data []byte, mime string) (string, OracleUsage, error)
    Extract(ctx context.Context, data []byte, mime string) (string, OracleUsage, error)
}
```

**Ideas:** AWS S3 Vault, Azure Blob Vault, R2 Vault, AWS Transcribe Lens, Azure Speech Lens.

Sub-module template: `<cloud>/`

---

### 🧪 Examples and use cases

Working examples for real-world patterns not yet covered — healthcare triage bots, e-commerce support, document processing pipelines, multi-tenant setups, WhatsApp flows with Lore + Imprint.

Add to `_examples/` following the existing structure.

---

### 📖 Documentation improvements

Fixes, clarifications, additional examples in `docs/`. The bar is low — if something confused you, fix it.

---

## How to contribute

### 1. For new sub-modules (adapters, providers, channels)

New sub-modules live as independent Go modules. The structure:

```
providers/my-provider/
├── go.mod          # module github.com/wmulabs/eywa/providers/my-provider
├── go.sum
├── my_provider.go  # implementation
└── *_test.go       # tests — required
```

The `go.mod` should only import `github.com/wmulabs/eywa` plus the provider's SDK. Never import `eywa/internal`.

```go
module github.com/wmulabs/eywa/providers/my-provider

go 1.22

require (
    github.com/wmulabs/eywa v0.x.x
    github.com/my-provider/go-sdk v1.x.x
)
```

### 2. Fork → branch → PR

```bash
git checkout -b feat/providers-my-provider
# implement + tests
git push origin feat/providers-my-provider
# open PR against main
```

### 3. PR requirements

- [ ] Tests pass (`go test ./...` in the sub-module)
- [ ] Builds clean (`go build ./...`)
- [ ] No `internal/` imports from sub-modules
- [ ] Exported types documented (no godoc on self-explanatory functions)
- [ ] Example added or existing example updated if it adds a new capability

---

## Testing

Tests are not optional. Every PR that adds or changes behavior must include tests covering that behavior. CI will fail without them.

### Running tests

```bash
make test           # all tests
make coverage       # tests + coverage summary
make coverage-html  # interactive HTML report (opens in browser)
```

### Conventions

The codebase uses a strict, uniform test style. Deviations will be rejected in review.

| Rule | Detail |
|------|--------|
| **Same package** | `package foo` not `package foo_test` — white-box access |
| **Flat functions** | No `t.Run`, no table-driven tests, no subtests |
| **Naming** | `TestType_Method_Scenario` — e.g. `TestSpirit_IsExecutor_True` |
| **Assertions** | Standard `testing` only — `t.Errorf` preferred, `t.Fatalf` only when nil dereference would follow |
| **No external test libs** | No testify, gomock, or any other test library |
| **Stubs over mocks** | Hand-written structs implementing port interfaces; unused methods `panic("not implemented")` |
| **Interface guards** | Every stub must have `var _ ports.X = (*stubX)(nil)` |
| **No test comments** | No section headers, no inline comments explaining what the code does |

### Example

```go
func TestRememberFactAction_Execute_NoPulseInContext(t *testing.T) {
    action := NewRememberFactAction(&stubImprintRepo{}, "assistant")

    result, err := action.Execute(context.Background(), map[string]any{
        "fact": "Likes coffee",
    })

    if err == nil {
        t.Error("expected error when pulse missing from context")
    }
    if result != "" {
        t.Errorf("expected empty result, got %q", result)
    }
}
```

---

## Code style

- Go standard formatting (`gofmt`)
- No speculative abstractions — implement the interface, nothing more
- Comments only when the code cannot explain itself (the *why*, not the *what*)
- Error classification: use `eywa.NewBusinessError` for user-facing failures, `eywa.NewInfrastructureError` for transient/infra failures
- Sub-modules: single responsibility — one provider per module

---

## Publishing your contribution

Once merged, the sub-module is published as `github.com/wmulabs/eywa/<path>@vX.Y.Z` and available to the entire community via `go get`.

If you build something useful that doesn't fit the core repo (a full application template, a CLI tool, a monitoring integration), publish it as a separate module and open a PR adding it to the README under a "Community" section.

---

## Public API stability contract

The root `eywa` package exposes **type aliases** (`type Weave = orchestrator.Weave`, `type Spirit = entities.Spirit`, etc.). This means:

- **Every exported field, method, and type in the aliased internal types IS the public API.** External consumers depend on them directly.
- **Any change to an aliased type is a breaking change** — even adding a field, renaming a method, or changing a parameter type.
- Removing or renaming an exported field in `internal/domain/entities/`, `internal/domain/ports/`, or `internal/implementation/orchestrator/` will break every downstream consumer without any Go compile-time warning at the `internal/` boundary.

### Rules for contributors

| Change | Required action |
|--------|----------------|
| Add a field to an aliased struct (e.g. `entities.Spirit`) | MINOR version bump; mention in CHANGELOG |
| Remove or rename a field in an aliased struct | MAJOR version bump; deprecate in previous minor first |
| Change a method signature on an aliased type | MAJOR version bump |
| Add a method to a port interface (e.g. `ports.Bond`) | MAJOR version bump (breaks all existing implementations) |
| Add a new type to `eywa.go`/`ports.go` | MINOR version bump |

When in doubt: **adding is MINOR, changing or removing is MAJOR**.

Before opening a PR that changes any aliased type, run:

```bash
# Snapshot the current API surface
go doc -all github.com/wmulabs/eywa > /tmp/api_before.txt

# After your change
go doc -all github.com/wmulabs/eywa > /tmp/api_after.txt

diff /tmp/api_before.txt /tmp/api_after.txt
```

Any diff that is not purely additive requires a changelog entry and the correct semver bump.

---

## Questions?

Open an issue. If you're unsure whether something is worth contributing, ask first — a short issue is cheaper than a full PR that needs major rework.

---

<p align="center">
  <sub>🌿 Every adapter you write extends the Weave. The community grows the forest.</sub>
</p>
