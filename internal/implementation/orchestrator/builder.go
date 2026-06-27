package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/implementation/actions"
	"github.com/wmulabs/eywa/internal/implementation/archivists"
	"github.com/wmulabs/eywa/internal/implementation/oracles"
	"github.com/wmulabs/eywa/internal/implementation/pathfinders"
	"github.com/wmulabs/eywa/internal/implementation/registries"
	"github.com/wmulabs/eywa/internal/implementation/scouts"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/dbg"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// VigilConfig configures the human-takeover feature.
type VigilConfig struct {
	InactivityTimeout time.Duration // default 30 min; used as Redis TTL
}

type WeaveBuilder struct {
	spiritRepo           ports.SpiritRepository
	sessionRepo          ports.MemoryRepository
	messageRepo          ports.EchoRepository
	interactionLogRepo   ports.ChronicleRepository
	distributedLock      ports.Bond
	lockTTLExplicit      bool
	handoffStore         ports.HandoffStore
	idempotencyStore     ports.IdempotencyStore
	rateLimiter          ports.Limiter
	messageInbox         ports.Inbox
	asyncDispatcher      ports.Keeper
	scheduledTaskManager ports.RitualManager
	mediaStore           ports.Vault
	mediaProcessor       ports.Lens
	typingIndicator      ports.TypingIndicator

	oracleProviders map[string]ports.Oracle
	oracleFactory   ports.OracleFactory

	scoutRegistry       ports.ScoutRegistry
	pathfinderRegistry  ports.PathfinderRegistry
	actionRegistry      ports.ActionRegistry
	voiceRegistry       ports.VoiceRegistry
	logicRouterRegistry ports.LogicRouterRegistry
	channels            []Channel

	loreRepository ports.LoreRepository
	loreStore      ports.LoreStore
	loreEmbedder   ports.LoreEmbedder

	imprintRepository    ports.ImprintRepository
	imprintExtractionCfg ImprintExtractionConfig

	conduits []ports.Conduit

	ledgerRepo        ports.LedgerRepository
	costAlertHook     ports.CostAlertHook
	modelRoutingRules []entities.ModelRoutingRule

	defaultPathfinderProvider string
	defaultPathfinderModel    string
	defaultPathfinderTemp     float64

	archivist           ports.Archivist
	archivistThreshold  int
	archivistKeepRecent int // 0 = derived as threshold/2

	// Deferred Archivist creation — built in Build() from the assembled OracleFactory.
	defaultArchivistProvider string
	defaultArchivistModel    string
	defaultArchivistTemp     float64
	defaultArchivistTempSet  bool // allows 0.0 (deterministic) to be intentional
	defaultArchivistTokens   int

	logger *zap.SugaredLogger
	tracer trace.Tracer

	config          *WeaveConfig
	linkRepo        ports.LinkRepository
	weaveConfigRepo WeaveConfigRepository
	pubSub          ports.PubSub
	httpToolRepo    ports.HTTPToolRepository
	vigilRepo       ports.VigilRepository
	vigilConfig     VigilConfig
	riteRepo        ports.RiteRepository
	checkpointStore ports.CheckpointStore
	appInfo         AppInfo

	ctx context.Context
}

func NewWeaveBuilder(ctx context.Context) *WeaveBuilder {
	return &WeaveBuilder{
		ctx:                ctx,
		oracleProviders:    make(map[string]ports.Oracle),
		scoutRegistry:      nil,
		pathfinderRegistry: nil,
		actionRegistry:     nil,
		config:             DefaultWeaveConfig(),
	}
}

func (b *WeaveBuilder) WithRepositories(
	spiritRepo ports.SpiritRepository,
	sessionRepo ports.MemoryRepository,
	messageRepo ports.EchoRepository,
	interactionLogRepo ports.ChronicleRepository,
) *WeaveBuilder {
	b.spiritRepo = spiritRepo
	b.sessionRepo = sessionRepo
	b.messageRepo = messageRepo
	b.interactionLogRepo = interactionLogRepo
	return b
}

func (b *WeaveBuilder) WithSpiritRepository(repo ports.SpiritRepository) *WeaveBuilder {
	b.spiritRepo = repo
	return b
}

func (b *WeaveBuilder) WithMemoryRepository(repo ports.MemoryRepository) *WeaveBuilder {
	b.sessionRepo = repo
	return b
}

func (b *WeaveBuilder) WithEchoRepository(repo ports.EchoRepository) *WeaveBuilder {
	b.messageRepo = repo
	return b
}

func (b *WeaveBuilder) WithChronicleRepository(repo ports.ChronicleRepository) *WeaveBuilder {
	b.interactionLogRepo = repo
	return b
}

func (b *WeaveBuilder) WithBond(lock ports.Bond) *WeaveBuilder {
	b.distributedLock = lock
	return b
}

// WithRateLimiter enables per-session rate limiting.
// When configured, a RateLimitStep is inserted after Validation to reject requests
// that exceed the configured rate. On Redis failure the step fails open.
// Use NewRedisRateLimiter from the redis infrastructure package to create the implementation.
func (b *WeaveBuilder) WithRateLimiter(rl ports.Limiter) *WeaveBuilder {
	b.rateLimiter = rl
	return b
}

// WithIdempotencyStore enables duplicate-event suppression.
// When configured, an IdempotencyCheck step runs after Validation and drops any Pulse whose
// IdempotencyKey was already processed within WeaveConfig.IdempotencyTTL. This guards against
// webhook redelivery (e.g. WhatsApp), which the Bond alone does not cover for sequential duplicates.
// Use redis.NewIdempotencyStore for multi-instance deployments, or NewInMemoryIdempotencyStore for single-instance.
func (b *WeaveBuilder) WithIdempotencyStore(store ports.IdempotencyStore) *WeaveBuilder {
	b.idempotencyStore = store
	return b
}

// WithInputGuard configures content-level validation for incoming user messages.
// When PromptInjectionDetection is enabled, messages matching known injection patterns
// are rejected before any processing. MaxLineCount limits multi-line message abuse.
func (b *WeaveBuilder) WithInputGuard(cfg GuardConfig) *WeaveBuilder {
	b.config.InputGuard = cfg
	return b
}

// WithHandoffStore enables peer-to-peer Spirit handoff: a Spirit with HandoffConfig can transfer the
// conversation to a peer, which the store pins so subsequent turns route to it. Use the in-memory store
// for single-instance deployments, or redis/mongo adapters for multi-instance. Without it, handoff is
// disabled and behaviour is unchanged.
func (b *WeaveBuilder) WithHandoffStore(store ports.HandoffStore) *WeaveBuilder {
	b.handoffStore = store
	return b
}

// WithOutputGuard sanitises the final agent response before it is persisted, delivered, and audited:
// PII redaction and a denylist of patterns that, when matched, replace the response wholesale.
// Disabled by default. See OutputGuardConfig for the coverage caveat on streamed and notifier turns.
func (b *WeaveBuilder) WithOutputGuard(cfg OutputGuardConfig) *WeaveBuilder {
	b.config.OutputGuard = cfg
	return b
}

// WithToolResultLimits bounds how much a single Action result contributes to the reasoning context.
// Large results are shaped (truncated, or summarized via the Oracle) before re-entering the model's
// window; the full result is always kept in the audit log. A zero MaxChars disables shaping.
func (b *WeaveBuilder) WithToolResultLimits(limits ports.ToolResultLimits) *WeaveBuilder {
	b.config.ToolResultLimits = limits
	return b
}

// WithProgressPolicy enables reasoning-loop stall detection: when the model keeps repeating Action
// calls it already made, the loop forces a final synthesis instead of spinning to the iteration cap.
// Disabled by default.
func (b *WeaveBuilder) WithProgressPolicy(policy ProgressPolicy) *WeaveBuilder {
	b.config.ProgressPolicy = policy
	return b
}

// WithCompressionPolicy bounds the reasoning working-context size: when it exceeds MaxContextChars,
// the oldest completed iterations are summarized into an evidence ledger while recent iterations stay
// verbatim. Disabled by default.
func (b *WeaveBuilder) WithCompressionPolicy(policy CompressionPolicy) *WeaveBuilder {
	b.config.CompressionPolicy = policy
	return b
}

// WithReflectionPolicy enables a self-critique pass: before delivering a draft answer the model
// reviews its own output and, on a "revise" verdict, gets one more iteration to fix it (bounded by
// MaxRounds). Reflection always fails open. Disabled by default.
func (b *WeaveBuilder) WithReflectionPolicy(policy ReflectionPolicy) *WeaveBuilder {
	b.config.ReflectionPolicy = policy
	return b
}

// WithGroundingPolicy enforces source citation for RAG Spirits: when Lore is retrieved for a turn,
// the answer must cite the retrieved chunks ([chunk:<id>]). On a violation the policy revises once,
// annotates, or blocks. Disabled by default.
func (b *WeaveBuilder) WithGroundingPolicy(policy GroundingPolicy) *WeaveBuilder {
	b.config.GroundingPolicy = policy
	return b
}

// WithPlanPolicy enables a turn-scoped plan/scratchpad the model maintains via the update_plan
// action; the current plan is injected each iteration and incomplete plans block a premature stop.
// Disabled by default.
func (b *WeaveBuilder) WithPlanPolicy(policy PlanPolicy) *WeaveBuilder {
	b.config.PlanPolicy = policy
	return b
}

// WithHandoffPolicy escalates low-confidence turns to a human: instead of shipping a weak answer,
// the loop raises a Vigil takeover (RaiseVigil, needs a VigilRepository) or flags the turn
// (AnnotateOnly). Disabled by default.
func (b *WeaveBuilder) WithHandoffPolicy(policy HandoffPolicy) *WeaveBuilder {
	b.config.HandoffPolicy = policy
	return b
}

// WithMessageInbox enables message coalescing by providing a MessageInbox implementation.
// When configured, messages arriving while a session is locked are buffered in the inbox
// and merged into a single user turn at the start of the next processing cycle.
// Use NewRedisMessageInbox from the redis infrastructure package to create the implementation.
func (b *WeaveBuilder) WithMessageInbox(inbox ports.Inbox) *WeaveBuilder {
	b.messageInbox = inbox
	return b
}

// WithInboxMinWindow sets the minimum accumulation window for message coalescing.
// The MessageCoalescingStep waits until at least this duration has elapsed since pipeline
// start before draining the inbox. Pipeline steps (enrichment, session setup, etc.) count
// toward the window — only the remainder is an actual sleep.
//
// Recommended: 3–5 seconds for WhatsApp-style multi-message conversations.
// Default: 0 (no wait; coalescing still happens for messages already in the inbox).
func (b *WeaveBuilder) WithInboxMinWindow(d time.Duration) *WeaveBuilder {
	b.config.InboxMinWindow = d
	return b
}

// WithRitualManager enables scheduled event execution.
// When configured, the engine exposes the /schedule API and the built-in Actions
// (schedule_event, list_scheduled_tasks, cancel_scheduled_task) can be registered.
// Use NewRitualService from the services package to create the implementation.
func (b *WeaveBuilder) WithRitualManager(manager ports.RitualManager) *WeaveBuilder {
	b.scheduledTaskManager = manager
	return b
}

// WithAsyncDispatch enables asynchronous webhook processing via a TaskScheduler.
// When configured, POST /webhooks/:event_name dispatches converted events to the
// task queue for immediate execution (scheduleTime = now) instead of processing inline.
// The same scheduler instance can be shared with WithRitualManager.
func (b *WeaveBuilder) WithAsyncDispatch(scheduler ports.Keeper) *WeaveBuilder {
	b.asyncDispatcher = scheduler
	return b
}

func (b *WeaveBuilder) WithMediaStore(store ports.Vault) *WeaveBuilder {
	b.mediaStore = store
	return b
}

func (b *WeaveBuilder) WithMediaProcessor(mp ports.Lens) *WeaveBuilder {
	b.mediaProcessor = mp
	return b
}

func (b *WeaveBuilder) WithTypingIndicator(ti ports.TypingIndicator) *WeaveBuilder {
	b.typingIndicator = ti
	return b
}

// AddOracle registers an Oracle provider (e.g. anthropic.NewOracle(...), openai.NewOracle(...)).
// The provider's GetName() return value is used as the key for Spirit.ModelConfig.Provider routing.
func (b *WeaveBuilder) AddOracle(provider ports.Oracle) *WeaveBuilder {
	b.oracleProviders[provider.GetName()] = provider
	return b
}

// WithOracleFactory sets a pre-built OracleFactory, bypassing provider auto-assembly.
func (b *WeaveBuilder) WithOracleFactory(factory ports.OracleFactory) *WeaveBuilder {
	b.oracleFactory = factory
	return b
}

// WithScoutRegistry sets a custom Scout registry (optional).
// Scouts add contextual data to Pulses before Spirit processing.
func (b *WeaveBuilder) WithScoutRegistry(registry ports.ScoutRegistry) *WeaveBuilder {
	b.scoutRegistry = registry
	return b
}

// WithPathfinderRegistry sets a custom Pathfinder registry (optional).
// Pathfinders select the best Spirit when multiple Spirits are available.
func (b *WeaveBuilder) WithPathfinderRegistry(registry ports.PathfinderRegistry) *WeaveBuilder {
	b.pathfinderRegistry = registry
	return b
}

func (b *WeaveBuilder) WithActionRegistry(registry ports.ActionRegistry) *WeaveBuilder {
	b.actionRegistry = registry
	return b
}

// WithVoiceRegistry sets a custom response channel registry for automatic response delivery.
// Use this when adding channels that are not covered by WithChannel (e.g. a fully custom Voice).
//
// Example:
//
//	registry := registries.NewVoiceRegistry()
//	registry.Register(channels.NewWhatsAppResponseChannel(whatsappClient))
//	builder.WithVoiceRegistry(registry)
func (b *WeaveBuilder) WithVoiceRegistry(registry ports.VoiceRegistry) *WeaveBuilder {
	b.voiceRegistry = registry
	return b
}

// WithChannel registers a named Receptor+Voice pair so a Link can reference both via ChannelName.
func (b *WeaveBuilder) WithChannel(ch Channel) *WeaveBuilder {
	b.channels = append(b.channels, ch)
	return b
}

func (b *WeaveBuilder) WithLogicRouterRegistry(registry ports.LogicRouterRegistry) *WeaveBuilder {
	b.logicRouterRegistry = registry
	return b
}

func (b *WeaveBuilder) WithLoreRepository(repo ports.LoreRepository) *WeaveBuilder {
	b.loreRepository = repo
	return b
}

func (b *WeaveBuilder) WithLoreStore(store ports.LoreStore) *WeaveBuilder {
	b.loreStore = store
	return b
}

func (b *WeaveBuilder) WithLoreEmbedder(embedder ports.LoreEmbedder) *WeaveBuilder {
	b.loreEmbedder = embedder
	return b
}

func (b *WeaveBuilder) WithImprintRepository(repo ports.ImprintRepository) *WeaveBuilder {
	b.imprintRepository = repo
	return b
}

func (b *WeaveBuilder) WithImprintExtraction(cfg ImprintExtractionConfig) *WeaveBuilder {
	b.imprintExtractionCfg = cfg
	return b
}

// WithConduit registers an MCP Conduit. On Build(), the engine connects to it,
// discovers its tools, and registers each as a ConduitActionAdapter in the ActionRegistry.
func (b *WeaveBuilder) WithConduit(conduit ports.Conduit) *WeaveBuilder {
	b.conduits = append(b.conduits, conduit)
	return b
}

func (b *WeaveBuilder) WithLedgerRepository(repo ports.LedgerRepository) *WeaveBuilder {
	b.ledgerRepo = repo
	return b
}

func (b *WeaveBuilder) WithCostAlertHook(hook ports.CostAlertHook) *WeaveBuilder {
	b.costAlertHook = hook
	return b
}

func (b *WeaveBuilder) WithModelRoutingRules(rules []entities.ModelRoutingRule) *WeaveBuilder {
	b.modelRoutingRules = rules
	return b
}

func (b *WeaveBuilder) WithConfig(config *WeaveConfig) *WeaveBuilder {
	b.config = config
	b.lockTTLExplicit = true // caller supplied full config; don't auto-derive LockTTL
	return b
}

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

func (b *WeaveBuilder) WithHTTPToolRepository(repo ports.HTTPToolRepository) *WeaveBuilder {
	b.httpToolRepo = repo
	return b
}

func (b *WeaveBuilder) WithVigilRepository(repo ports.VigilRepository) *WeaveBuilder {
	b.vigilRepo = repo
	return b
}

func (b *WeaveBuilder) WithVigilConfig(cfg VigilConfig) *WeaveBuilder {
	b.vigilConfig = cfg
	return b
}

// WithCheckpointStore enables durable execution: reasoning-turn state is checkpointed after each
// iteration so a turn interrupted mid-flight (deploy, crash, request timeout) resumes from where it
// stopped instead of restarting. Off by default; without it the synchronous path is unchanged.
func (b *WeaveBuilder) WithCheckpointStore(store ports.CheckpointStore) *WeaveBuilder {
	b.checkpointStore = store
	return b
}

func (b *WeaveBuilder) WithRiteRepository(repo ports.RiteRepository) *WeaveBuilder {
	b.riteRepo = repo
	return b
}

func (b *WeaveBuilder) WithLockTTL(ttl time.Duration) *WeaveBuilder {
	b.config.LockTTL = ttl
	b.lockTTLExplicit = true
	return b
}

func (b *WeaveBuilder) WithLockAcquireTimeout(timeout time.Duration) *WeaveBuilder {
	b.config.LockAcquireTimeout = timeout
	return b
}

func (b *WeaveBuilder) WithScoutTimeout(timeout time.Duration) *WeaveBuilder {
	b.config.ScoutTimeout = timeout
	return b
}

func (b *WeaveBuilder) WithReasoningTimeout(timeout time.Duration) *WeaveBuilder {
	b.config.ReasoningTimeout = timeout
	if !b.lockTTLExplicit {
		b.config.LockTTL = timeout + 30*time.Second
	}
	return b
}

func (b *WeaveBuilder) WithMaxIterations(max int) *WeaveBuilder {
	b.config.MaxReasoningIterations = max
	return b
}

func (b *WeaveBuilder) WithMaxMemoryMessages(max int) *WeaveBuilder {
	b.config.MaxMemoryMessages = max
	return b
}

// WithMemoryReconstruction enables/disables automatic memory reconstruction from the message store.
// When enabled, if a Redis session expires, the engine rebuilds short-term memory from the last N
// persisted messages, scoped to the active context_key when available.
// Parameters:
//   - enabled: Whether to enable automatic reconstruction (default: true)
//   - limit: Number of messages to load into short-term memory (default: 10)
func (b *WeaveBuilder) WithMemoryReconstruction(enabled bool, limit int) *WeaveBuilder {
	b.config.EnableMemoryReconstruction = enabled
	if limit > 0 {
		b.config.MemoryReconstructionLimit = limit
	}
	return b
}

// WithSpiritSelectionTimeout sets the timeout for Spirit selection, which may invoke an LLM Pathfinder.
func (b *WeaveBuilder) WithSpiritSelectionTimeout(timeout time.Duration) *WeaveBuilder {
	b.config.SpiritSelectionTimeout = timeout
	return b
}

func (b *WeaveBuilder) WithMaxIterationsMessage(msg string) *WeaveBuilder {
	b.config.MaxIterationsMessage = msg
	return b
}

func (b *WeaveBuilder) WithParallelActionExecution(enabled bool) *WeaveBuilder {
	b.config.ParallelActionExecution = enabled
	return b
}

// WithActionRetry configures exponential-backoff retry for transient (InfrastructureError) Action
// failures. maxAttempts is the total attempt count (1 = no retry); baseDelay doubles each retry.
// Business errors are never retried regardless of these values.
func (b *WeaveBuilder) WithActionRetry(maxAttempts int, baseDelay time.Duration) *WeaveBuilder {
	b.config.ActionRetryMaxAttempts = maxAttempts
	b.config.ActionRetryBaseDelay = baseDelay
	return b
}

func (b *WeaveBuilder) WithValidationLimits(maxSessionKeyLength, maxUserMessageLength, maxEventContextSize int) *WeaveBuilder {
	b.config.MaxSessionKeyLength = maxSessionKeyLength
	b.config.MaxUserMessageLength = maxUserMessageLength
	b.config.MaxEventContextSize = maxEventContextSize
	return b
}

// WithLogger sets a custom logger instance.
// If not provided, a default logger will be created.
func (b *WeaveBuilder) WithLogger(logger *zap.SugaredLogger) *WeaveBuilder {
	b.logger = logger
	return b
}

// WithTracer sets a custom tracer instance.
// If not provided, a no-op tracer will be used.
func (b *WeaveBuilder) WithTracer(tracer trace.Tracer) *WeaveBuilder {
	b.tracer = tracer
	return b
}

// WithAppInfo sets application metadata (name and version) for health checks and monitoring.
// If not provided, defaults to "eywa" and "unknown".
func (b *WeaveBuilder) WithAppInfo(name, version string) *WeaveBuilder {
	b.appInfo = AppInfo{
		Name:    name,
		Version: version,
	}
	return b
}

// WithArchivist enables automatic conversation summarization.
// When Threads reaches threshold, the oldest half is summarized and stored in session.Summary,
// then injected into event.Context["conversation_summary"] for the Oracle.
//
// Parameters:
//   - archivist: Implementation (e.g., NewLLMArchivist)
//   - threshold: Message count that triggers summarization (e.g., 20)
//
// Example:
//
//	archivist := archivists.NewLLMArchivist("anthropic", "claude-3-5-haiku-20241022", oracleFactory)
//	builder.WithArchivist(archivist, 20)
func (b *WeaveBuilder) WithArchivist(s ports.Archivist, threshold int) *WeaveBuilder {
	b.archivist = s
	b.archivistThreshold = threshold
	return b
}

// WithDefaultLLMArchivist enables automatic conversation summarization using the engine's
// own LLM factory — no need to create a factory or archivist manually. This solves the
// chicken-and-egg problem of needing an LLMFactory before the engine is built.
//
// The archivist is built during Build() using the same providers registered via
// AddAnthropic(), AddOpenAI(), AddGemini(), etc.
//
// Parameters:
//   - provider: Oracle provider name (e.g., "anthropic", "openai", "gemini")
//   - model: Model to use (e.g., "claude-3-5-haiku-20241022", "gpt-4o-mini")
//   - threshold: Number of messages that triggers summarization (e.g., 20)
//
// Optional: Customize temperature and max tokens using WithArchivistConfig() after this call.
//
// For custom archivist implementations, use WithArchivist() instead.
//
// Example:
//
//	builder.WithDefaultLLMArchivist("anthropic", "claude-3-5-haiku-20241022", 20)
//
// With custom config:
//
//	builder.WithDefaultLLMArchivist("anthropic", "claude-3-5-haiku-20241022", 20).
//	    WithArchivistConfig(0.1, 1024)
func (b *WeaveBuilder) WithDefaultLLMArchivist(provider, model string, threshold int) *WeaveBuilder {
	b.defaultArchivistProvider = provider
	b.defaultArchivistModel = model
	b.archivistThreshold = threshold
	// Defaults will be set in Build() if not overridden
	if b.defaultArchivistTemp == 0 {
		b.defaultArchivistTemp = 0.2
	}
	if b.defaultArchivistTokens == 0 {
		b.defaultArchivistTokens = 512
	}
	return b
}

// WithArchivistConfig sets temperature and max tokens for the default LLM Archivist.
// Only applies when using WithDefaultLLMArchivist().
//
// Example:
//
//	builder.WithDefaultLLMArchivist("anthropic", "claude-3-5-haiku-20241022", 20).
//	    WithArchivistConfig(0.1, 1024)
func (b *WeaveBuilder) WithArchivistConfig(temperature float64, maxTokens int) *WeaveBuilder {
	b.defaultArchivistTemp = temperature
	b.defaultArchivistTempSet = true
	b.defaultArchivistTokens = maxTokens
	return b
}

// WithArchivistKeepRecent sets how many recent messages are preserved after summarization.
// The oldest (threshold - keepRecent) messages are summarized; the newest keepRecent are kept verbatim.
// Default: threshold / 2. Must be called after WithArchivist or WithDefaultLLMArchivist.
func (b *WeaveBuilder) WithArchivistKeepRecent(keepRecent int) *WeaveBuilder {
	b.archivistKeepRecent = keepRecent
	return b
}

// resolveKeepRecent returns the configured keepRecent value, defaulting to threshold/2.
func (b *WeaveBuilder) resolveKeepRecent() int {
	if b.archivistKeepRecent > 0 {
		return b.archivistKeepRecent
	}
	return b.archivistThreshold / 2
}

// WithDefaultLLMPathfinder configures the built-in LLM Pathfinder for Spirit routing.
// Automatically used when a Link has multiple Spirits and no explicit Pathfinder is configured.
//
// If no PathfinderRegistry has been provided via WithPathfinderRegistry(), an empty registry will be
// automatically created.
//
// Parameters:
//   - provider: Oracle provider name (e.g., "anthropic", "openai", "gemini")
//   - model: Model to use (e.g., "claude-3-5-haiku-20241022", "gpt-4o-mini", "gemini-2.5-flash")
//   - temperature: Optional temperature for routing decisions (0.0-1.0); defaults to 0.2
func (b *WeaveBuilder) WithDefaultLLMPathfinder(provider, model string, temperature ...float64) *WeaveBuilder {
	b.defaultPathfinderProvider = provider
	b.defaultPathfinderModel = model

	if len(temperature) > 0 {
		b.defaultPathfinderTemp = temperature[0]
	} else {
		b.defaultPathfinderTemp = 0.2 // low temperature for consistent routing
	}

	return b
}

// Build creates and returns a ready-to-use Weave instance.
//
// Side effect: Build calls dbg.SetLogger to set the process-global logger to the
// logger configured for this builder. In processes running multiple Weave instances,
// the last Build() call wins — earlier instances will use the logger set by the later
// builder. Use WithLogger to configure the logger for each builder independently,
// and be aware of this global contamination when running multiple instances.
func (b *WeaveBuilder) Build() (*Weave, error) {
	if err := b.validate(); err != nil {
		return nil, fmt.Errorf("engine builder validation failed: %w", err)
	}

	if b.oracleFactory == nil {
		factory, err := b.buildOracleFactory()
		if err != nil {
			return nil, fmt.Errorf("failed to build Oracle factory: %w", err)
		}
		b.oracleFactory = factory
	}

	if b.logger == nil {
		config := zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.LevelKey = "severity"
		config.EncoderConfig.TimeKey = "timestamp"
		log, _ := config.Build()
		b.logger = log.Sugar().Named("eywa")
	}

	// Set the process-global logger so sub-packages that call dbg.GetLogger() (e.g. redis)
	// use this Weave's configured logger. Side effect: overwrites any previous Weave's global logger.
	dbg.SetLogger(b.logger)

	if b.tracer == nil {
		b.tracer = noop.NewTracerProvider().Tracer("eywa")
	}

	if b.appInfo.Name == "" {
		b.appInfo.Name = "eywa"
	}
	if b.appInfo.Version == "" {
		b.appInfo.Version = "unknown"
	}

	// Load WeaveConfig from MongoDB if a repository is provided.
	if b.weaveConfigRepo != nil {
		cfg, err := b.weaveConfigRepo.Find(b.ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load engine config: %w", err)
		}
		b.config = cfg
	}

	// Auto-create action registry if HTTP tools or Rite are configured but no registry was provided.
	if (b.httpToolRepo != nil || b.riteRepo != nil) && b.actionRegistry == nil {
		b.actionRegistry = registries.NewActionRegistry()
	}

	engine, err := newWeaveWithConfig(
		b.scoutRegistry,
		b.pathfinderRegistry,
		b.voiceRegistry,
		b.spiritRepo,
		b.sessionRepo,
		b.messageRepo,
		b.interactionLogRepo,
		b.oracleFactory,
		b.actionRegistry,
		b.distributedLock,
		b.rateLimiter,
		b.messageInbox,
		b.asyncDispatcher,
		b.config,
		b.logger,
		b.tracer,
	)
	if err != nil {
		return nil, err
	}

	engine.appInfo = b.appInfo

	// Build and populate ConfigCache.
	cache := NewConfigCache(b.linkRepo, b.pubSub, engine.logger)
	if b.linkRepo != nil {
		if err := cache.LoadAll(b.ctx); err != nil {
			return nil, fmt.Errorf("failed to load link config cache: %w", err)
		}
	}
	engine.configCache = cache
	if b.httpToolRepo != nil {
		engine.httpToolRepo = b.httpToolRepo
	}
	if b.vigilRepo != nil {
		engine.vigilRepo = b.vigilRepo
		// Wire the handoff sink so a low-confidence turn can auto-raise a Vigil takeover.
		if rs, ok := engine.reasoningService.(*ReasoningService); ok {
			rs.SetHandoffSink(newVigilHandoffSink(b.vigilRepo, b.vigilConfig.InactivityTimeout))
		}
	}
	if b.idempotencyStore != nil {
		engine.idempotencyStore = b.idempotencyStore
	}
	if b.checkpointStore != nil {
		if rs, ok := engine.reasoningService.(*ReasoningService); ok {
			rs.SetCheckpointStore(b.checkpointStore)
		}
	}
	if b.riteRepo != nil {
		if err := engine.RegisterAction(actions.NewRequestRiteAction(b.riteRepo, b.pubSub)); err != nil {
			b.logger.Warnw("failed to register request_rite action", "error", err)
		}
	}
	if b.pubSub != nil {
		// Create an internal context so Close() can stop the Subscribe goroutine.
		subCtx, subCancel := context.WithCancel(context.Background())
		engine.closeCancel = subCancel
		go cache.Subscribe(subCtx)
	}

	if b.scheduledTaskManager != nil {
		engine.scheduledTaskManager = b.scheduledTaskManager
	}

	if b.mediaStore != nil {
		engine.mediaStore = b.mediaStore
	}

	if b.mediaProcessor != nil {
		engine.mediaProcessor = b.mediaProcessor
	}

	if b.typingIndicator != nil {
		engine.typingIndicator = b.typingIndicator
	}

	if b.archivist != nil {
		engine.archivist = b.archivist
		engine.archivistThreshold = b.archivistThreshold
		engine.archivistKeepRecent = b.resolveKeepRecent()
	} else if b.defaultArchivistProvider != "" && b.defaultArchivistModel != "" {
		b.buildDefaultLLMArchivist(engine)
	}

	if len(b.channels) > 0 {
		if engine.voiceRegistry == nil {
			engine.voiceRegistry = registries.NewVoiceRegistry()
		}
		for _, ch := range b.channels {
			engine.RegisterReceptor(ch.Name, ch.Receptor)
			if err := engine.voiceRegistry.Register(ch.Voice); err != nil {
				return nil, fmt.Errorf("register voice for channel %q: %w", ch.Name, err)
			}
		}
	}

	if b.logicRouterRegistry != nil {
		engine.logicRouterRegistry = b.logicRouterRegistry
	}

	if b.loreRepository != nil && b.loreStore != nil && b.loreEmbedder != nil {
		engine.loreHarvester = scouts.NewLoreScout(b.loreRepository, b.loreStore, b.loreEmbedder, 5, 0.7)
		engine.loreRepository = b.loreRepository
		engine.loreStore = b.loreStore
		engine.loreEmbedder = b.loreEmbedder
	}

	if b.imprintRepository != nil {
		engine.imprintHarvester = scouts.NewImprintScout(b.imprintRepository, b.imprintExtractionCfg.maxImprints())
		engine.imprintRepository = b.imprintRepository
		engine.imprintExtractionCfg = b.imprintExtractionCfg
	}

	if len(b.conduits) > 0 {
		b.wireConduits(b.ctx, engine)
	}

	if b.ledgerRepo != nil {
		engine.ledgerRepo = b.ledgerRepo
		engine.costAlertHook = b.costAlertHook
	}

	if len(b.modelRoutingRules) > 0 {
		engine.modelRoutingRules = b.modelRoutingRules
	}

	if b.handoffStore != nil {
		engine.handoffStore = b.handoffStore
	}

	engine.wireOrchestration()

	// Rebuild with optional fields (scheduledTaskManager, mediaStore, mediaProcessor, archivist) now wired.
	engine.pipeline = engine.buildProcessingPipeline()

	if b.defaultPathfinderProvider != "" && b.defaultPathfinderModel != "" {
		b.registerDefaultLLMPathfinder(engine)
	}

	return engine, nil
}

func (b *WeaveBuilder) registerDefaultLLMPathfinder(engine *Weave) {
	// Auto-create so WithDefaultLLMPathfinder works without requiring WithPathfinderRegistry.
	if engine.pathfinderRegistry == nil {
		b.logger.Infow("Pathfinder registry not provided, creating empty registry for default LLM Pathfinder",
			"provider", b.defaultPathfinderProvider,
			"model", b.defaultPathfinderModel)

		engine.pathfinderRegistry = registries.NewPathfinderRegistry()
	}

	pathfinder := pathfinders.NewDefaultLLMPathfinder(
		b.defaultPathfinderProvider,
		b.defaultPathfinderModel,
		b.defaultPathfinderTemp,
		engine.GetLLMFactory(),
		engine.spiritRepo,
	)

	if err := engine.pathfinderRegistry.Register(pathfinder); err != nil {
		b.logger.Errorw("failed to register default LLM Pathfinder",
			"provider", b.defaultPathfinderProvider,
			"model", b.defaultPathfinderModel,
			"error", err)
	} else {
		b.logger.Infow("default LLM Pathfinder registered",
			"provider", b.defaultPathfinderProvider,
			"model", b.defaultPathfinderModel,
			"temperature", b.defaultPathfinderTemp)
	}
}

func (b *WeaveBuilder) buildDefaultLLMArchivist(engine *Weave) {
	archivist := archivists.NewLLMArchivist(
		b.defaultArchivistProvider,
		b.defaultArchivistModel,
		engine.GetLLMFactory(),
	)

	// Use the explicit-set flag so that 0.0 (deterministic mode) is honoured rather than ignored.
	if b.defaultArchivistTempSet {
		archivist.WithTemperature(b.defaultArchivistTemp)
	}
	if b.defaultArchivistTokens > 0 {
		archivist.WithMaxTokens(b.defaultArchivistTokens)
	}

	engine.archivist = archivist
	engine.archivistThreshold = b.archivistThreshold
	engine.archivistKeepRecent = b.resolveKeepRecent()

	b.logger.Infow("default Archivist registered",
		"provider", b.defaultArchivistProvider,
		"model", b.defaultArchivistModel,
		"threshold", b.archivistThreshold,
		"temperature", b.defaultArchivistTemp,
		"max_tokens", b.defaultArchivistTokens)
}

func (b *WeaveBuilder) validate() error {
	if b.spiritRepo == nil {
		return fmt.Errorf("spirit repository is required")
	}
	if b.sessionRepo == nil {
		return fmt.Errorf("session repository is required")
	}
	if b.messageRepo == nil {
		return fmt.Errorf("message repository is required")
	}
	if b.interactionLogRepo == nil {
		return fmt.Errorf("interaction log repository is required")
	}
	if b.distributedLock == nil {
		return fmt.Errorf("distributed lock is required")
	}

	if b.oracleFactory == nil && len(b.oracleProviders) == 0 {
		return fmt.Errorf("at least one Oracle provider must be configured")
	}

	if (b.archivist != nil || b.defaultArchivistProvider != "") && b.archivistThreshold < 2 {
		return fmt.Errorf("archivist threshold must be at least 2, got %d", b.archivistThreshold)
	}

	return nil
}

func (b *WeaveBuilder) buildOracleFactory() (ports.OracleFactory, error) {
	if len(b.oracleProviders) == 0 {
		return nil, fmt.Errorf("no Oracle providers registered")
	}

	factory := oracles.NewFactory()

	for name, provider := range b.oracleProviders {
		if err := factory.RegisterProvider(name, provider); err != nil {
			return nil, fmt.Errorf("failed to register provider %s: %w", name, err)
		}
	}

	return factory, nil
}

func (b *WeaveBuilder) wireConduits(ctx context.Context, engine *Weave) {
	for _, conduit := range b.conduits {
		if err := conduit.Connect(ctx); err != nil {
			b.logger.Warnw("conduit connect failed, skipping", "conduit", conduit.GetName(), "error", err)
			continue
		}
		tools, err := conduit.ListTools(ctx)
		if err != nil {
			b.logger.Warnw("conduit list tools failed, skipping", "conduit", conduit.GetName(), "error", err)
			continue
		}
		for _, tool := range tools {
			adapter := actions.NewConduitActionAdapter(conduit, tool)
			if err := engine.RegisterAction(adapter); err != nil {
				b.logger.Warnw("conduit tool registration failed",
					"conduit", conduit.GetName(),
					"tool", tool.Name,
					"error", err)
			}
		}
		b.logger.Infow("conduit connected and tools registered",
			"conduit", conduit.GetName(),
			"tool_count", len(tools))
	}
}
