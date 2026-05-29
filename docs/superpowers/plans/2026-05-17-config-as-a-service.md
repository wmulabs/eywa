# Config as a Service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `Link` (event routing config) and `WeaveConfig` (engine tuning) from Go-code registration into MongoDB, with Redis pub/sub hot-reload of Links across all running instances.

**Architecture:** A `ConfigCache` struct (in-memory map of `*entities.Link`) replaces the engine's `eventConfigs` map. At startup the builder calls `LoadAll()` to populate it from MongoDB. When a management API call saves a Link, `ConfigCache.SaveLink()` writes to MongoDB, updates the local cache, and publishes a reload notification to Redis so all other instances reload that key. `WeaveConfig` is loaded from MongoDB at `Build()` time only (restart needed to apply changes). Links are hot-reloaded without restart.

**Tech Stack:** Go, MongoDB (`go.mongodb.org/mongo-driver`), Redis (`github.com/redis/go-redis/v9` in the `redis/` sub-module), Fiber v2, go-redis pub/sub.

---

## File Map

**Root module — new:**
- `internal/domain/ports/link_repository.go` — `LinkRepository` port (CRUD for `*entities.Link`)
- `internal/domain/ports/pubsub.go` — `PubSub` port (Publish + blocking Subscribe)
- `internal/implementation/orchestrator/config_cache.go` — `ConfigCache` struct + all methods

**Root module — modified:**
- `internal/implementation/orchestrator/engine.go` — replace `eventConfigs map` with `configCache *ConfigCache`; delegate `RegisterEventConfiguration`
- `internal/implementation/orchestrator/builder.go` — add `WithLinkRepository`, `WithWeaveConfigRepository`, `WithPubSub`; wire ConfigCache in `Build()`
- `internal/implementation/orchestrator/config.go` — add `WeaveConfigRepository` interface (alongside `WeaveConfig`)
- `ports.go` — re-export `LinkRepository`, `PubSub`
- `eywa.go` — re-export `ConfigCache`, `NewConfigCache`, `WeaveConfigRepository`

**Mongo sub-module — new:**
- `mongo/link_repository.go` — `LinkRepository` implementation + `linkDocument` DTO
- `mongo/weave_config_repository.go` — `WeaveConfigRepository` implementation + singleton doc

**Redis sub-module — new:**
- `redis/pubsub.go` — `RedisPubSub` struct implementing `ports.PubSub`

**Fiber sub-module — new:**
- `fiber/link_mgmt_handler.go` — CRUD handlers for `/api/v1/event-configurations`
- `fiber/link_mgmt_handler_test.go` — tests
- `fiber/weave_config_handler.go` — GET/PUT `/api/v1/admin/engine-config` + POST `/api/v1/admin/config/reload`
- `fiber/weave_config_handler_test.go` — tests

**Fiber sub-module — modified:**
- `fiber/management.go` — add `ConfigCache`, `WeaveConfigRepo` to `ManagementDeps`; register new routes

---

## Task 1: Ports — LinkRepository + PubSub

**Files:**
- Create: `internal/domain/ports/link_repository.go`
- Create: `internal/domain/ports/pubsub.go`

- [ ] **Step 1.1 — Create `internal/domain/ports/link_repository.go`**

```go
package ports

import (
	"context"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

type LinkRepository interface {
	FindAll(ctx context.Context) ([]*entities.Link, error)
	FindByKey(ctx context.Context, eventType string) (*entities.Link, error)
	Save(ctx context.Context, link *entities.Link) error
	Delete(ctx context.Context, eventType string) error
}
```

- [ ] **Step 1.2 — Create `internal/domain/ports/pubsub.go`**

```go
package ports

import "context"

// PubSub is a publish/subscribe notifier used for cross-instance config reload.
// Subscribe blocks until ctx is cancelled, calling handler for each received message.
type PubSub interface {
	Publish(ctx context.Context, channel, message string) error
	Subscribe(ctx context.Context, channel string, handler func(msg string)) error
}
```

- [ ] **Step 1.3 — Re-export from `ports.go`**

Add to the existing type aliases block in `ports.go`:

```go
LinkRepository = ports.LinkRepository
PubSub        = ports.PubSub
```

- [ ] **Step 1.4 — Build root module**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 1.5 — Commit**

```bash
git add internal/domain/ports/link_repository.go internal/domain/ports/pubsub.go ports.go
git commit -m "feat(config): LinkRepository and PubSub ports"
```

---

## Task 2: WeaveConfigRepository interface

**Files:**
- Modify: `internal/implementation/orchestrator/config.go`
- Modify: `eywa.go`

- [ ] **Step 2.1 — Add `WeaveConfigRepository` to `config.go`**

Append to the end of `internal/implementation/orchestrator/config.go`:

```go
// WeaveConfigRepository persists and retrieves WeaveConfig from a durable store.
// Find returns DefaultWeaveConfig() if no document exists yet.
type WeaveConfigRepository interface {
	Find(ctx context.Context) (*WeaveConfig, error)
	Save(ctx context.Context, config *WeaveConfig) error
}
```

Add `"context"` to the imports at the top of config.go (it currently only imports `"time"`):

```go
import (
	"context"
	"time"
)
```

- [ ] **Step 2.2 — Re-export from `eywa.go`**

Find the existing type-alias block in `eywa.go` and add:

```go
WeaveConfigRepository = orchestrator.WeaveConfigRepository
```

- [ ] **Step 2.3 — Build root module**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 2.4 — Commit**

```bash
git add internal/implementation/orchestrator/config.go eywa.go
git commit -m "feat(config): WeaveConfigRepository interface"
```

---

## Task 3: ConfigCache — core (no writes, no Redis yet)

**Files:**
- Create: `internal/implementation/orchestrator/config_cache.go`

- [ ] **Step 3.1 — Create `internal/implementation/orchestrator/config_cache.go`**

```go
package orchestrator

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

const configReloadChannel = "eywa:config:reload"

type ConfigCache struct {
	mu       sync.RWMutex
	links    map[string]*entities.Link
	linkRepo ports.LinkRepository // nil = no persistence
	pubSub   ports.PubSub        // nil = no pub/sub
	logger   *zap.SugaredLogger
}

func NewConfigCache(linkRepo ports.LinkRepository, pubSub ports.PubSub, logger *zap.SugaredLogger) *ConfigCache {
	return &ConfigCache{
		links:    make(map[string]*entities.Link),
		linkRepo: linkRepo,
		pubSub:   pubSub,
		logger:   logger,
	}
}

// RegisterLink adds a link directly to the in-memory cache without persisting to MongoDB.
// Use this for static registrations in tests or when no repository is configured.
func (c *ConfigCache) RegisterLink(link *entities.Link) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.links[link.EventType] = link
}

func (c *ConfigCache) GetLink(eventType string) (*entities.Link, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	link, ok := c.links[eventType]
	return link, ok
}

func (c *ConfigCache) GetAllLinks() []*entities.Link {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*entities.Link, 0, len(c.links))
	for _, l := range c.links {
		result = append(result, l)
	}
	return result
}

// LoadAll loads all Links from the repository into the in-memory cache.
// No-op if linkRepo is nil.
func (c *ConfigCache) LoadAll(ctx context.Context) error {
	if c.linkRepo == nil {
		return nil
	}
	links, err := c.linkRepo.FindAll(ctx)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range links {
		c.links[l.EventType] = l
	}
	return nil
}
```

- [ ] **Step 3.2 — Build root module**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3.3 — Commit**

```bash
git add internal/implementation/orchestrator/config_cache.go
git commit -m "feat(config): ConfigCache core — in-memory link map with LoadAll"
```

---

## Task 4: Engine — replace eventConfigs with ConfigCache

**Files:**
- Modify: `internal/implementation/orchestrator/engine.go`

The engine currently has:
- Field `eventConfigs map[string]*entities.Link` (line ~73)
- Init `eventConfigs: make(map[string]*entities.Link)` in `newWeaveWithConfig` (line ~142)
- `RegisterEventConfiguration(config *entities.Link)` (line ~275) — writes to map
- `GetEventConfiguration(eventType string)` (line ~279) — reads from map
- `e.eventConfigs[eventType]` in `ConvertEventByType` (line ~291)
- `e.eventConfigs[eventKey]` in `ProcessEventByKey` (line ~329)
- `e.eventConfigs[eventKey]` in `ProcessMultipleEventsByKey` (line ~349)

- [ ] **Step 4.1 — Replace the field**

Find `eventConfigs      map[string]*entities.Link` in the `Weave` struct and replace with:

```go
configCache       *ConfigCache
```

- [ ] **Step 4.2 — Update the constructor**

In `newWeaveWithConfig`, find the struct literal and:
- Remove: `eventConfigs: make(map[string]*entities.Link),`
- Add: `configCache: NewConfigCache(nil, nil, logger),`

- [ ] **Step 4.3 — Update RegisterEventConfiguration and GetEventConfiguration**

Replace both methods:

```go
func (e *Weave) RegisterEventConfiguration(config *entities.Link) {
	e.configCache.RegisterLink(config)
}

func (e *Weave) GetEventConfiguration(eventType string) *entities.Link {
	link, _ := e.configCache.GetLink(eventType)
	return link
}
```

- [ ] **Step 4.4 — Update ConvertEventByType**

Find `eventConfig, exists := e.eventConfigs[eventType]` in `ConvertEventByType` and replace with:

```go
eventConfig, exists := e.configCache.GetLink(eventType)
```

- [ ] **Step 4.5 — Update ProcessEventByKey**

Find `eventConfig, exists := e.eventConfigs[eventKey]` in `ProcessEventByKey` and replace with:

```go
eventConfig, exists := e.configCache.GetLink(eventKey)
```

- [ ] **Step 4.6 — Update ProcessMultipleEventsByKey**

Find `eventConfig, exists := e.eventConfigs[eventKey]` in `ProcessMultipleEventsByKey` and replace with:

```go
eventConfig, exists := e.configCache.GetLink(eventKey)
```

- [ ] **Step 4.7 — Build and test**

```bash
go build ./...
go test ./...
```

Expected: no errors. The orchestrator tests don't use RegisterEventConfiguration directly, so they should pass unchanged.

- [ ] **Step 4.8 — Commit**

```bash
git add internal/implementation/orchestrator/engine.go
git commit -m "feat(config): engine uses ConfigCache instead of eventConfigs map"
```

---

## Task 5: Builder — wire ConfigCache and WeaveConfig loading

**Files:**
- Modify: `internal/implementation/orchestrator/builder.go`
- Modify: `eywa.go`

- [ ] **Step 5.1 — Add fields to WeaveBuilder struct**

Find the `WeaveBuilder` struct in `builder.go` and add three new fields (alongside the existing `config *WeaveConfig` field):

```go
linkRepo        ports.LinkRepository
weaveConfigRepo WeaveConfigRepository
pubSub          ports.PubSub
```

Ensure `"github.com/wmulabs/eywa/internal/domain/ports"` is in the imports (it already is).

- [ ] **Step 5.2 — Add three new builder methods**

Add after the existing `WithConfig` method:

```go
func (b *WeaveBuilder) WithLinkRepository(repo ports.LinkRepository) *WeaveBuilder {
	b.linkRepo = repo
	return b
}

func (b *WeaveBuilder) WithWeaveConfigRepository(repo WeaveConfigRepository) *WeaveBuilder {
	b.weaveConfigRepo = repo
	return b
}

func (b *WeaveBuilder) WithPubSub(ps ports.PubSub) *WeaveBuilder {
	b.pubSub = ps
	return b
}
```

- [ ] **Step 5.3 — Wire in Build()**

In `Build()`, immediately after the logger and tracer are set up (before the `newWeaveWithConfig` call), add:

```go
// Load WeaveConfig from MongoDB if a repository is provided.
if b.weaveConfigRepo != nil {
	cfg, err := b.weaveConfigRepo.Find(b.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load engine config: %w", err)
	}
	b.config = cfg
}
```

Then, after `engine, err := newWeaveWithConfig(...)` and the `engine.appInfo` assignment, add:

```go
// Build and populate ConfigCache.
cache := NewConfigCache(b.linkRepo, b.pubSub, engine.logger)
if b.linkRepo != nil {
	if err := cache.LoadAll(b.ctx); err != nil {
		return nil, fmt.Errorf("failed to load link config cache: %w", err)
	}
}
engine.configCache = cache
if b.pubSub != nil {
	go cache.Subscribe(b.ctx)
}
```

Note: `cache.Subscribe` does not exist yet (Task 7). For now the `go cache.Subscribe(b.ctx)` line will fail to compile. Add a placeholder that the linter won't complain about — stub `Subscribe` in config_cache.go temporarily:

```go
// Placeholder — implemented in Task 7.
func (c *ConfigCache) Subscribe(ctx context.Context) {}
```

- [ ] **Step 5.4 — Re-export from eywa.go**

Add to `eywa.go`:

```go
ConfigCache    = orchestrator.ConfigCache
NewConfigCache = orchestrator.NewConfigCache
```

- [ ] **Step 5.5 — Build and test**

```bash
go build ./...
go test ./...
```

Expected: no errors.

- [ ] **Step 5.6 — Commit**

```bash
git add internal/implementation/orchestrator/builder.go internal/implementation/orchestrator/config_cache.go eywa.go
git commit -m "feat(config): builder wires ConfigCache and loads WeaveConfig from repo at startup"
```

---

## Task 6: Mongo — LinkRepository + WeaveConfigRepository

**Files:**
- Create: `mongo/link_repository.go`
- Create: `mongo/weave_config_repository.go`

- [ ] **Step 6.1 — Create `mongo/link_repository.go`**

```go
package mongo

import (
	"context"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Compile-time interface satisfaction check.
var _ eywa.LinkRepository = (*LinkRepository)(nil)

type LinkRepository struct {
	collection *mongodriver.Collection
}

func NewLinkRepository(database *mongodriver.Database) *LinkRepository {
	return &LinkRepository{
		collection: database.Collection("event_configurations"),
	}
}

type linkDocument struct {
	EventType            string                 `bson:"_id"`
	InboundConverterName string                 `bson:"inbound_converter_name"`
	RequireScouts        []string               `bson:"require_scouts"`
	PathfinderName       string                 `bson:"pathfinder_name"`
	AllowedAgents        []string               `bson:"allowed_agents"`
	DefaultAgent         string                 `bson:"default_agent"`
	VoiceName            string                 `bson:"voice_name"`
	ChannelName          string                 `bson:"channel_name"`
	IngestionTimeoutNs   int64                  `bson:"ingestion_timeout_ns"`
	ProcessingTimeoutNs  int64                  `bson:"processing_timeout_ns"`
	Guards               []guardDocument        `bson:"guards"`
	Metadata             map[string]interface{} `bson:"metadata"`
	UpdatedAt            time.Time              `bson:"updated_at"`
}

type guardDocument struct {
	Field     string   `bson:"field"`
	BlockList []string `bson:"block_list"`
	AllowList []string `bson:"allow_list"`
}

// eywa.Link, eywa.Guard, eywa.NewLink are re-exported from internal/domain/entities
// via the root package's entities.go — use eywa.* throughout this file.

func linkToDocument(link *eywa.Link) linkDocument {
	guards := make([]guardDocument, len(link.Guards))
	for i, g := range link.Guards {
		guards[i] = guardDocument{
			Field:     g.Field,
			BlockList: g.BlockList,
			AllowList: g.AllowList,
		}
	}
	return linkDocument{
		EventType:            link.EventType,
		InboundConverterName: link.InboundConverterName,
		RequireScouts:        link.RequireScouts,
		PathfinderName:       link.PathfinderName,
		AllowedAgents:        link.AllowedAgents,
		DefaultAgent:         link.DefaultAgent,
		VoiceName:            link.VoiceName,
		ChannelName:          link.ChannelName,
		IngestionTimeoutNs:   link.IngestionTimeout.Nanoseconds(),
		ProcessingTimeoutNs:  link.ProcessingTimeout.Nanoseconds(),
		Guards:               guards,
		Metadata:             link.Metadata,
		UpdatedAt:            time.Now().UTC(),
	}
}

func documentToLink(doc linkDocument) *eywa.Link {
	guards := make([]eywa.Guard, len(doc.Guards))
	for i, g := range doc.Guards {
		guards[i] = eywa.Guard{
			Field:     g.Field,
			BlockList: g.BlockList,
			AllowList: g.AllowList,
		}
	}
	link := eywa.NewLink(doc.EventType).
		WithInboundConverter(doc.InboundConverterName).
		WithScouts(doc.RequireScouts...).
		WithPathfinder(doc.PathfinderName).
		WithAgents(doc.AllowedAgents...).
		WithDefaultAgent(doc.DefaultAgent).
		WithVoice(doc.VoiceName).
		WithMetadata(doc.Metadata).
		WithGuards(guards...).
		Build()
	link.ChannelName = doc.ChannelName
	link.IngestionTimeout = time.Duration(doc.IngestionTimeoutNs)
	link.ProcessingTimeout = time.Duration(doc.ProcessingTimeoutNs)
	return link
}

func (r *LinkRepository) FindAll(ctx context.Context) ([]*eywa.Link, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []linkDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	result := make([]*eywa.Link, len(docs))
	for i, d := range docs {
		result[i] = documentToLink(d)
	}
	return result, nil
}

func (r *LinkRepository) FindByKey(ctx context.Context, eventType string) (*eywa.Link, error) {
	var doc linkDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": eventType}).Decode(&doc)
	if err == mongodriver.ErrNoDocuments {
		return nil, eywa.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return documentToLink(doc), nil
}

func (r *LinkRepository) Save(ctx context.Context, link *eywa.Link) error {
	doc := linkToDocument(link)
	opts := options.Replace().SetUpsert(true)
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": link.EventType}, doc, opts)
	return err
}

func (r *LinkRepository) Delete(ctx context.Context, eventType string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": eventType})
	return err
}
```

- [ ] **Step 6.2 — Create `mongo/weave_config_repository.go`**

```go
package mongo

import (
	"context"
	"time"

	eywa "github.com/wmulabs/eywa"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Compile-time interface satisfaction check.
var _ eywa.WeaveConfigRepository = (*WeaveConfigRepository)(nil)

type WeaveConfigRepository struct {
	collection *mongodriver.Collection
}

func NewWeaveConfigRepository(database *mongodriver.Database) *WeaveConfigRepository {
	return &WeaveConfigRepository{
		collection: database.Collection("engine_config"),
	}
}

type weaveConfigDocument struct {
	ID                         string        `bson:"_id"`
	LockTTLNs                  int64         `bson:"lock_ttl_ns"`
	LockAcquireTimeoutNs       int64         `bson:"lock_acquire_timeout_ns"`
	ScoutTimeoutNs             int64         `bson:"scout_timeout_ns"`
	SpiritSelectionTimeoutNs   int64         `bson:"spirit_selection_timeout_ns"`
	SpiritLoadTimeoutNs        int64         `bson:"spirit_load_timeout_ns"`
	SessionTimeoutNs           int64         `bson:"session_timeout_ns"`
	ReasoningTimeoutNs         int64         `bson:"reasoning_timeout_ns"`
	PersistenceTimeoutNs       int64         `bson:"persistence_timeout_ns"`
	MaxReasoningIterations     int           `bson:"max_reasoning_iterations"`
	MaxMemoryMessages          int           `bson:"max_memory_messages"`
	MaxIterationsMessage       string        `bson:"max_iterations_message"`
	EnableMemoryReconstruction bool          `bson:"enable_memory_reconstruction"`
	MemoryReconstructionLimit  int           `bson:"memory_reconstruction_limit"`
	ParallelActionExecution    bool          `bson:"parallel_action_execution"`
	ActionRetryMaxAttempts     int           `bson:"action_retry_max_attempts"`
	ActionRetryBaseDelayNs     int64         `bson:"action_retry_base_delay_ns"`
	MaxActionsPerCycle         int           `bson:"max_actions_per_cycle"`
	InboxMinWindowNs           int64         `bson:"inbox_min_window_ns"`
	MessageCoalescingTimeoutNs int64         `bson:"message_coalescing_timeout_ns"`
	MaxSessionKeyLength        int           `bson:"max_session_key_length"`
	MaxUserMessageLength       int           `bson:"max_user_message_length"`
	MaxEventContextSize        int           `bson:"max_event_context_size"`
	PromptInjectionDetection   bool          `bson:"prompt_injection_detection"`
	MaxLineCount               int           `bson:"max_line_count"`
	UpdatedAt                  time.Time     `bson:"updated_at"`
}

func configToDocument(cfg *eywa.WeaveConfig) weaveConfigDocument {
	return weaveConfigDocument{
		ID:                         "default",
		LockTTLNs:                  cfg.LockTTL.Nanoseconds(),
		LockAcquireTimeoutNs:       cfg.LockAcquireTimeout.Nanoseconds(),
		ScoutTimeoutNs:             cfg.ScoutTimeout.Nanoseconds(),
		SpiritSelectionTimeoutNs:   cfg.SpiritSelectionTimeout.Nanoseconds(),
		SpiritLoadTimeoutNs:        cfg.SpiritLoadTimeout.Nanoseconds(),
		SessionTimeoutNs:           cfg.SessionTimeout.Nanoseconds(),
		ReasoningTimeoutNs:         cfg.ReasoningTimeout.Nanoseconds(),
		PersistenceTimeoutNs:       cfg.PersistenceTimeout.Nanoseconds(),
		MaxReasoningIterations:     cfg.MaxReasoningIterations,
		MaxMemoryMessages:          cfg.MaxMemoryMessages,
		MaxIterationsMessage:       cfg.MaxIterationsMessage,
		EnableMemoryReconstruction: cfg.EnableMemoryReconstruction,
		MemoryReconstructionLimit:  cfg.MemoryReconstructionLimit,
		ParallelActionExecution:    cfg.ParallelActionExecution,
		ActionRetryMaxAttempts:     cfg.ActionRetryMaxAttempts,
		ActionRetryBaseDelayNs:     cfg.ActionRetryBaseDelay.Nanoseconds(),
		MaxActionsPerCycle:         cfg.MaxActionsPerCycle,
		InboxMinWindowNs:           cfg.InboxMinWindow.Nanoseconds(),
		MessageCoalescingTimeoutNs: cfg.MessageCoalescingTimeout.Nanoseconds(),
		MaxSessionKeyLength:        cfg.MaxSessionKeyLength,
		MaxUserMessageLength:       cfg.MaxUserMessageLength,
		MaxEventContextSize:        cfg.MaxEventContextSize,
		PromptInjectionDetection:   cfg.InputGuard.PromptInjectionDetection,
		MaxLineCount:               cfg.InputGuard.MaxLineCount,
		UpdatedAt:                  time.Now().UTC(),
	}
}

func documentToConfig(doc weaveConfigDocument) *eywa.WeaveConfig {
	return &eywa.WeaveConfig{
		LockTTL:                    time.Duration(doc.LockTTLNs),
		LockAcquireTimeout:         time.Duration(doc.LockAcquireTimeoutNs),
		ScoutTimeout:               time.Duration(doc.ScoutTimeoutNs),
		SpiritSelectionTimeout:     time.Duration(doc.SpiritSelectionTimeoutNs),
		SpiritLoadTimeout:          time.Duration(doc.SpiritLoadTimeoutNs),
		SessionTimeout:             time.Duration(doc.SessionTimeoutNs),
		ReasoningTimeout:           time.Duration(doc.ReasoningTimeoutNs),
		PersistenceTimeout:         time.Duration(doc.PersistenceTimeoutNs),
		MaxReasoningIterations:     doc.MaxReasoningIterations,
		MaxMemoryMessages:          doc.MaxMemoryMessages,
		MaxIterationsMessage:       doc.MaxIterationsMessage,
		EnableMemoryReconstruction: doc.EnableMemoryReconstruction,
		MemoryReconstructionLimit:  doc.MemoryReconstructionLimit,
		ParallelActionExecution:    doc.ParallelActionExecution,
		ActionRetryMaxAttempts:     doc.ActionRetryMaxAttempts,
		ActionRetryBaseDelay:       time.Duration(doc.ActionRetryBaseDelayNs),
		MaxActionsPerCycle:         doc.MaxActionsPerCycle,
		InboxMinWindow:             time.Duration(doc.InboxMinWindowNs),
		MessageCoalescingTimeout:   time.Duration(doc.MessageCoalescingTimeoutNs),
		MaxSessionKeyLength:        doc.MaxSessionKeyLength,
		MaxUserMessageLength:       doc.MaxUserMessageLength,
		MaxEventContextSize:        doc.MaxEventContextSize,
		InputGuard: eywa.GuardConfig{
			PromptInjectionDetection: doc.PromptInjectionDetection,
			MaxLineCount:             doc.MaxLineCount,
		},
	}
}

func (r *WeaveConfigRepository) Find(ctx context.Context) (*eywa.WeaveConfig, error) {
	var doc weaveConfigDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": "default"}).Decode(&doc)
	if err == mongodriver.ErrNoDocuments {
		return eywa.DefaultWeaveConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	return documentToConfig(doc), nil
}

func (r *WeaveConfigRepository) Save(ctx context.Context, config *eywa.WeaveConfig) error {
	doc := configToDocument(config)
	opts := options.Replace().SetUpsert(true)
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": "default"}, doc, opts)
	return err
}
```

- [ ] **Step 6.3 — Build mongo module**

```bash
cd mongo && go build ./... && cd ..
```

Expected: no errors.

- [ ] **Step 6.4 — Commit**

```bash
git add mongo/link_repository.go mongo/weave_config_repository.go
git commit -m "feat(config): mongo LinkRepository and WeaveConfigRepository"
```

---

## Task 7: ConfigCache — writes + Redis pub/sub

**Files:**
- Modify: `internal/implementation/orchestrator/config_cache.go`

- [ ] **Step 7.1 — Add write methods and real Subscribe**

Replace the stub `Subscribe` method and add `SaveLink`, `DeleteLink`, `ForceReload`:

```go
// SaveLink persists link to MongoDB, updates the local cache, and notifies other instances.
func (c *ConfigCache) SaveLink(ctx context.Context, link *entities.Link) error {
	if c.linkRepo != nil {
		if err := c.linkRepo.Save(ctx, link); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.links[link.EventType] = link
	c.mu.Unlock()
	if c.pubSub != nil {
		_ = c.pubSub.Publish(ctx, configReloadChannel, "link:"+link.EventType)
	}
	return nil
}

// DeleteLink removes a link from MongoDB and the local cache, and notifies other instances.
func (c *ConfigCache) DeleteLink(ctx context.Context, eventType string) error {
	if c.linkRepo != nil {
		if err := c.linkRepo.Delete(ctx, eventType); err != nil {
			return err
		}
	}
	c.mu.Lock()
	delete(c.links, eventType)
	c.mu.Unlock()
	if c.pubSub != nil {
		_ = c.pubSub.Publish(ctx, configReloadChannel, "link:"+eventType)
	}
	return nil
}

// ForceReload reloads all Links from MongoDB and notifies other instances.
func (c *ConfigCache) ForceReload(ctx context.Context) error {
	if err := c.LoadAll(ctx); err != nil {
		return err
	}
	if c.pubSub != nil {
		_ = c.pubSub.Publish(ctx, configReloadChannel, "link:*")
	}
	return nil
}

// Subscribe listens for reload notifications and reloads the affected Link from MongoDB.
// Blocks until ctx is cancelled. Call in a goroutine.
func (c *ConfigCache) Subscribe(ctx context.Context) {
	if c.pubSub == nil {
		return
	}
	err := c.pubSub.Subscribe(ctx, configReloadChannel, func(msg string) {
		if msg == "link:*" {
			if err := c.LoadAll(ctx); err != nil && c.logger != nil {
				c.logger.Errorw("config cache reload all failed", "error", err)
			}
			return
		}
		if len(msg) > 5 && msg[:5] == "link:" {
			eventType := msg[5:]
			if c.linkRepo == nil {
				return
			}
			link, err := c.linkRepo.FindByKey(ctx, eventType)
			if err != nil {
				// Link may have been deleted — remove from cache.
				c.mu.Lock()
				delete(c.links, eventType)
				c.mu.Unlock()
				return
			}
			c.mu.Lock()
			c.links[link.EventType] = link
			c.mu.Unlock()
		}
	})
	if err != nil && ctx.Err() == nil && c.logger != nil {
		c.logger.Errorw("config cache subscription ended unexpectedly", "error", err)
	}
}
```

- [ ] **Step 7.2 — Build and test**

```bash
go build ./...
go test ./...
```

Expected: no errors.

- [ ] **Step 7.3 — Commit**

```bash
git add internal/implementation/orchestrator/config_cache.go
git commit -m "feat(config): ConfigCache write methods and Redis pub/sub Subscribe"
```

---

## Task 8: Redis — PubSub implementation

**Files:**
- Create: `redis/pubsub.go`

- [ ] **Step 8.1 — Create `redis/pubsub.go`**

```go
package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
	eywa "github.com/wmulabs/eywa"
)

// Compile-time interface satisfaction check.
var _ eywa.PubSub = (*RedisPubSub)(nil)

type RedisPubSub struct {
	client *redis.Client
}

func NewRedisPubSub(client *redis.Client) *RedisPubSub {
	return &RedisPubSub{client: client}
}

func (r *RedisPubSub) Publish(ctx context.Context, channel, message string) error {
	return r.client.Publish(ctx, channel, message).Err()
}

// Subscribe blocks until ctx is cancelled, calling handler for each received message.
func (r *RedisPubSub) Subscribe(ctx context.Context, channel string, handler func(msg string)) error {
	sub := r.client.Subscribe(ctx, channel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			handler(msg.Payload)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
```

- [ ] **Step 8.2 — Build redis module**

```bash
cd redis && go build ./... && cd ..
```

Expected: no errors.

- [ ] **Step 8.3 — Commit**

```bash
git add redis/pubsub.go
git commit -m "feat(config): RedisPubSub implements PubSub port for config hot-reload"
```

---

## Task 9: Fiber — link management handlers (TDD)

**Files:**
- Create: `fiber/link_mgmt_handler_test.go`
- Create: `fiber/link_mgmt_handler.go`
- Modify: `fiber/management.go`

The handler uses `*eywa.ConfigCache` directly (concrete type) — no additional interface needed. The test stubs a `*eywa.ConfigCache` built with `eywa.NewConfigCache(stubRepo, nil, nil)`.

### 9a — Stub and test helper

- [ ] **Step 9.1 — Create `fiber/link_mgmt_handler_test.go` with stubs and tests**

```go
package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

// stubLinkRepo implements eywa.LinkRepository in memory.
// eywa.Link is the re-exported alias for entities.Link — use eywa.Link throughout.
type stubLinkRepo struct {
	links map[string]*eywa.Link
	err   error
}

func newStubLinkRepo(links ...*eywa.Link) *stubLinkRepo {
	s := &stubLinkRepo{links: make(map[string]*eywa.Link)}
	for _, l := range links {
		s.links[l.EventType] = l
	}
	return s
}

func (s *stubLinkRepo) FindAll(_ context.Context) ([]*eywa.Link, error) {
	result := make([]*eywa.Link, 0, len(s.links))
	for _, l := range s.links {
		result = append(result, l)
	}
	return result, s.err
}
func (s *stubLinkRepo) FindByKey(_ context.Context, eventType string) (*eywa.Link, error) {
	l, ok := s.links[eventType]
	if !ok {
		return nil, eywa.ErrNotFound
	}
	return l, s.err
}
func (s *stubLinkRepo) Save(_ context.Context, link *eywa.Link) error {
	s.links[link.EventType] = link
	return s.err
}
func (s *stubLinkRepo) Delete(_ context.Context, eventType string) error {
	delete(s.links, eventType)
	return s.err
}

func linkCacheDeps(repo *stubLinkRepo) ManagementDeps {
	cache := eywa.NewConfigCache(repo, nil, nil)
	_ = cache.LoadAll(context.Background())
	return ManagementDeps{
		APIKeys:    map[string]string{"test-key": "admin"},
		ConfigCache: cache,
	}
}

// Tests

func TestLinkMgmtHandler_List_Returns200(t *testing.T) {
	repo := newStubLinkRepo(
		eywa.NewLink("whatsapp_message").WithDefaultAgent("support").Build(),
	)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []map[string]interface{} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Errorf("want 1 link, got %d", len(body.Items))
	}
}

func TestLinkMgmtHandler_List_ReturnsEmptySlice(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Items []interface{} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items must be [] not null")
	}
}

func TestLinkMgmtHandler_GetByKey_Returns200(t *testing.T) {
	repo := newStubLinkRepo(
		eywa.NewLink("whatsapp_message").WithDefaultAgent("support").Build(),
	)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations/whatsapp_message", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["event_type"] != "whatsapp_message" {
		t.Errorf("want event_type whatsapp_message, got %v", body["event_type"])
	}
}

func TestLinkMgmtHandler_GetByKey_NotFound_Returns404(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	req := httptest.NewRequest("GET", "/api/v1/event-configurations/unknown", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_Returns200(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	body := map[string]interface{}{
		"event_type":    "sms_message",
		"default_agent": "support",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms_message", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Save_MismatchedKey_Returns400(t *testing.T) {
	app := buildMgmtTestApp(linkCacheDeps(newStubLinkRepo()))

	body := map[string]interface{}{"event_type": "other_event"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/api/v1/event-configurations/sms_message", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 400 {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestLinkMgmtHandler_Delete_Returns204(t *testing.T) {
	repo := newStubLinkRepo(
		eywa.NewLink("whatsapp_message").Build(),
	)
	app := buildMgmtTestApp(linkCacheDeps(repo))

	req := httptest.NewRequest("DELETE", "/api/v1/event-configurations/whatsapp_message", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 204 {
		t.Errorf("want 204, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 9.2 — Run tests to verify they fail**

```bash
cd fiber && go test ./... -run TestLinkMgmtHandler -v 2>&1 | head -40
```

Expected: compile errors (handler and ManagementDeps.ConfigCache don't exist yet).

- [ ] **Step 9.3 — Create `fiber/link_mgmt_handler.go`**

```go
package fiber

import (
	"time"

	eywa "github.com/wmulabs/eywa"
	"github.com/wmulabs/eywa/internal/domain/entities"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
)

type linkHandler struct {
	cache *eywa.ConfigCache
}

func newLinkHandler(cache *eywa.ConfigCache) *linkHandler {
	return &linkHandler{cache: cache}
}

type guardDTO struct {
	Field     string   `json:"field"`
	BlockList []string `json:"block_list"`
	AllowList []string `json:"allow_list"`
}

type linkDTO struct {
	EventType            string                 `json:"event_type"`
	InboundConverterName string                 `json:"inbound_converter_name"`
	RequireScouts        []string               `json:"require_scouts"`
	PathfinderName       string                 `json:"pathfinder_name"`
	AllowedAgents        []string               `json:"allowed_agents"`
	DefaultAgent         string                 `json:"default_agent"`
	VoiceName            string                 `json:"voice_name"`
	ChannelName          string                 `json:"channel_name"`
	IngestionTimeoutMs   int64                  `json:"ingestion_timeout_ms"`
	ProcessingTimeoutMs  int64                  `json:"processing_timeout_ms"`
	Guards               []guardDTO             `json:"guards"`
	Metadata             map[string]interface{} `json:"metadata"`
}

func linkToDTO(l *entities.Link) linkDTO {
	guards := make([]guardDTO, len(l.Guards))
	for i, g := range l.Guards {
		guards[i] = guardDTO{Field: g.Field, BlockList: g.BlockList, AllowList: g.AllowList}
	}
	return linkDTO{
		EventType:            l.EventType,
		InboundConverterName: l.InboundConverterName,
		RequireScouts:        l.RequireScouts,
		PathfinderName:       l.PathfinderName,
		AllowedAgents:        l.AllowedAgents,
		DefaultAgent:         l.DefaultAgent,
		VoiceName:            l.VoiceName,
		ChannelName:          l.ChannelName,
		IngestionTimeoutMs:   l.IngestionTimeout.Milliseconds(),
		ProcessingTimeoutMs:  l.ProcessingTimeout.Milliseconds(),
		Guards:               guards,
		Metadata:             l.Metadata,
	}
}

func dtoToLink(dto linkDTO) *entities.Link {
	guards := make([]entities.Guard, len(dto.Guards))
	for i, g := range dto.Guards {
		guards[i] = entities.Guard{Field: g.Field, BlockList: g.BlockList, AllowList: g.AllowList}
	}
	return entities.NewLink(dto.EventType).
		WithInboundConverter(dto.InboundConverterName).
		WithScouts(dto.RequireScouts...).
		WithPathfinder(dto.PathfinderName).
		WithAgents(dto.AllowedAgents...).
		WithDefaultAgent(dto.DefaultAgent).
		WithVoice(dto.VoiceName).
		WithMetadata(dto.Metadata).
		WithGuards(guards...).
		Build()
}

func (h *linkHandler) list(c *fiberlib.Ctx) error {
	links := h.cache.GetAllLinks()
	dtos := make([]linkDTO, len(links))
	for i, l := range links {
		dtos[i] = linkToDTO(l)
	}
	return c.JSON(fiberlib.Map{"items": dtos})
}

func (h *linkHandler) getByKey(c *fiberlib.Ctx) error {
	key := c.Params("eventType")
	link, ok := h.cache.GetLink(key)
	if !ok {
		return c.Status(fiberlib.StatusNotFound).JSON(fiberlib.Map{"error": "event configuration not found"})
	}
	return c.JSON(linkToDTO(link))
}

func (h *linkHandler) save(c *fiberlib.Ctx) error {
	key := c.Params("eventType")
	var dto linkDTO
	if err := c.BodyParser(&dto); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if dto.EventType == "" {
		dto.EventType = key
	}
	if dto.EventType != key {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "event_type in body must match URL parameter"})
	}
	link := dtoToLink(dto)
	link.IngestionTimeout = time.Duration(dto.IngestionTimeoutMs) * time.Millisecond
	link.ProcessingTimeout = time.Duration(dto.ProcessingTimeoutMs) * time.Millisecond
	link.ChannelName = dto.ChannelName
	if err := h.cache.SaveLink(c.Context(), link); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(linkToDTO(link))
}

func (h *linkHandler) delete(c *fiberlib.Ctx) error {
	key := c.Params("eventType")
	if err := h.cache.DeleteLink(c.Context(), key); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.SendStatus(fiberlib.StatusNoContent)
}
```

- [ ] **Step 9.4 — Add ConfigCache to ManagementDeps and register routes**

In `fiber/management.go`, add to `ManagementDeps`:

```go
// Phase 5 — Config as a Service (SPEC_03)
ConfigCache    *eywa.ConfigCache
WeaveConfigRepo eywa.WeaveConfigRepository
```

And in `RegisterManagementRoutes`, after the echo block:

```go
if deps.ConfigCache != nil {
    lh := newLinkHandler(deps.ConfigCache)
    cfgs := api.Group("/event-configurations")
    cfgs.Get("", lh.list)
    cfgs.Get("/:eventType", lh.getByKey)
    cfgs.Put("/:eventType", lh.save)
    cfgs.Delete("/:eventType", lh.delete)
}
```

- [ ] **Step 9.5 — Run tests**

```bash
cd fiber && go test ./... -run TestLinkMgmtHandler -v
```

Expected: all 7 link handler tests pass.

- [ ] **Step 9.6 — Run full fiber suite**

```bash
cd fiber && go test ./...
```

Expected: all tests pass (previous 25 + 7 new = 32+).

- [ ] **Step 9.7 — Commit**

```bash
git add fiber/link_mgmt_handler.go fiber/link_mgmt_handler_test.go fiber/management.go
git commit -m "feat(config): link management CRUD handlers with tests"
```

---

## Task 10: Fiber — WeaveConfig handler (TDD)

**Files:**
- Create: `fiber/weave_config_handler_test.go`
- Create: `fiber/weave_config_handler.go`

- [ ] **Step 10.1 — Create `fiber/weave_config_handler_test.go`**

```go
package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	eywa "github.com/wmulabs/eywa"
)

type stubWeaveConfigRepo struct {
	config *eywa.WeaveConfig
	err    error
}

func (s *stubWeaveConfigRepo) Find(_ context.Context) (*eywa.WeaveConfig, error) {
	if s.config == nil {
		return eywa.DefaultWeaveConfig(), s.err
	}
	return s.config, s.err
}

func (s *stubWeaveConfigRepo) Save(_ context.Context, cfg *eywa.WeaveConfig) error {
	s.config = cfg
	return s.err
}

func weaveConfigDeps(repo *stubWeaveConfigRepo) ManagementDeps {
	return ManagementDeps{
		APIKeys:        map[string]string{"test-key": "admin"},
		WeaveConfigRepo: repo,
	}
}

func TestWeaveConfigHandler_Get_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{config: eywa.DefaultWeaveConfig()}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	req := httptest.NewRequest("GET", "/api/v1/admin/engine-config", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["max_actions_per_cycle"] == nil {
		t.Error("want max_actions_per_cycle in response")
	}
}

func TestWeaveConfigHandler_Save_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	cfg := eywa.DefaultWeaveConfig()
	cfg.MaxActionsPerCycle = 42
	b, _ := json.Marshal(cfg)

	req := httptest.NewRequest("PUT", "/api/v1/admin/engine-config", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if repo.config == nil || repo.config.MaxActionsPerCycle != 42 {
		t.Error("expected config to be saved with MaxActionsPerCycle=42")
	}
}

func TestWeaveConfigHandler_Reload_Returns200(t *testing.T) {
	repo := &stubWeaveConfigRepo{}
	app := buildMgmtTestApp(weaveConfigDeps(repo))

	req := httptest.NewRequest("POST", "/api/v1/admin/config/reload", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 10.2 — Run tests to verify they fail**

```bash
cd fiber && go test ./... -run TestWeaveConfigHandler -v 2>&1 | head -20
```

Expected: compile errors (handler doesn't exist yet).

- [ ] **Step 10.3 — Create `fiber/weave_config_handler.go`**

```go
package fiber

import (
	eywa "github.com/wmulabs/eywa"
	resthttp "github.com/wmulabs/eywa/fiber/http"
	fiberlib "github.com/gofiber/fiber/v2"
)

type weaveConfigHandler struct {
	repo        eywa.WeaveConfigRepository
	configCache *eywa.ConfigCache // nil if not set
}

func newWeaveConfigHandler(repo eywa.WeaveConfigRepository, cache *eywa.ConfigCache) *weaveConfigHandler {
	return &weaveConfigHandler{repo: repo, configCache: cache}
}

func (h *weaveConfigHandler) get(c *fiberlib.Ctx) error {
	cfg, err := h.repo.Find(c.Context())
	if err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(cfg)
}

func (h *weaveConfigHandler) save(c *fiberlib.Ctx) error {
	var cfg eywa.WeaveConfig
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": "invalid request body"})
	}
	if err := cfg.Validate(); err != nil {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": err.Error()})
	}
	if err := h.repo.Save(c.Context(), &cfg); err != nil {
		return resthttp.ErrorResponse(c, err)
	}
	return c.JSON(&cfg)
}

func (h *weaveConfigHandler) reload(c *fiberlib.Ctx) error {
	if h.configCache != nil {
		if err := h.configCache.ForceReload(c.Context()); err != nil {
			return resthttp.ErrorResponse(c, err)
		}
	}
	return c.JSON(fiberlib.Map{"status": "reloaded"})
}
```

- [ ] **Step 10.4 — Register routes in `fiber/management.go`**

Add to the `RegisterManagementRoutes` function, after the event-configurations block:

```go
if deps.WeaveConfigRepo != nil {
    wh := newWeaveConfigHandler(deps.WeaveConfigRepo, deps.ConfigCache)
    admin := api.Group("/admin")
    admin.Get("/engine-config", wh.get)
    admin.Put("/engine-config", wh.save)
    admin.Post("/config/reload", wh.reload)
}
```

- [ ] **Step 10.5 — Run tests**

```bash
cd fiber && go test ./... -run TestWeaveConfigHandler -v
```

Expected: all 3 tests pass.

- [ ] **Step 10.6 — Run full fiber suite**

```bash
cd fiber && go test ./...
```

Expected: all tests pass.

- [ ] **Step 10.7 — Commit**

```bash
git add fiber/weave_config_handler.go fiber/weave_config_handler_test.go fiber/management.go
git commit -m "feat(config): WeaveConfig GET/PUT + force reload handler with tests"
```

---

## Task 11: Full verification

- [ ] **Step 11.1 — Run all root module tests**

```bash
go test ./...
```

Expected: all pass, no errors.

- [ ] **Step 11.2 — Run all fiber module tests**

```bash
cd fiber && go test ./...
```

Expected: all pass.

- [ ] **Step 11.3 — Build mongo module**

```bash
cd mongo && go build ./...
```

Expected: no errors.

- [ ] **Step 11.4 — Build redis module**

```bash
cd redis && go build ./...
```

Expected: no errors.

- [ ] **Step 11.5 — Commit if any uncommitted changes**

```bash
git status
```

If clean, no commit needed.

---

## Notes for implementer

- `eywa.GuardConfig` is already re-exported from `orchestrator.GuardConfig` — use it directly in the mongo repo.
- `eywa.ErrNotFound` is the sentinel used by the existing mongo repos — use it in `FindByKey`.
- `entities.NewLink(eventType).WithScouts()` called with no arguments sets `RequireScouts = []string{}` — this is correct when the stored scouts slice is empty.
- The `buildMgmtTestApp` helper is already defined in `fiber/chronicle_handler_test.go` — do not redefine it.
- `WeaveConfig` is a struct (not pointer) in most places but `Find()` returns `*WeaveConfig`. The `BodyParser` target must be a value `var cfg eywa.WeaveConfig` (not pointer), then pass `&cfg` to Save and return `c.JSON(&cfg)`.
