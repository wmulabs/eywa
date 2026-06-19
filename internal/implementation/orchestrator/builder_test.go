package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/implementation/actions"
	"github.com/wmulabs/eywa/internal/implementation/registries"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

func TestNewWeaveBuilder_ReturnsNonNil(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	if b == nil {
		t.Fatal("expected non-nil builder")
	}
	if b.config == nil {
		t.Fatal("expected non-nil default config")
	}
}

func TestWeaveBuilder_WithSpiritRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubSpiritRepo{}
	got := b.WithSpiritRepository(repo)
	if got != b {
		t.Error("expected fluent builder (same pointer)")
	}
	if b.spiritRepo != repo {
		t.Error("expected spiritRepo to be set")
	}
}

func TestWeaveBuilder_WithMemoryRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubMemoryRepo{}
	got := b.WithMemoryRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.sessionRepo != repo {
		t.Error("expected sessionRepo to be set")
	}
}

func TestWeaveBuilder_WithEchoRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubEchoRepo{}
	got := b.WithEchoRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.messageRepo != repo {
		t.Error("expected messageRepo to be set")
	}
}

func TestWeaveBuilder_WithRepositories(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	spirit := &stubSpiritRepo{}
	mem := &stubMemoryRepo{}
	echo := &stubEchoRepo{}
	chronicle := &stubBuilderChronicleRepo{}
	got := b.WithRepositories(spirit, mem, echo, chronicle)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.spiritRepo != spirit || b.sessionRepo != mem || b.messageRepo != echo {
		t.Error("expected repos to be set")
	}
	if b.interactionLogRepo == nil {
		t.Error("expected interactionLogRepo to be set")
	}
}

func TestWeaveBuilder_WithBond(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	lock := &stubBond{acquired: true}
	got := b.WithBond(lock)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.distributedLock != lock {
		t.Error("expected distributedLock to be set")
	}
}

func TestWeaveBuilder_WithIdempotencyStore(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	store := NewInMemoryIdempotencyStore()
	got := b.WithIdempotencyStore(store)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.idempotencyStore != store {
		t.Error("expected idempotencyStore to be set")
	}
}

func TestWeaveBuilder_WithCheckpointStore(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	store := newMemCheckpointStore()
	got := b.WithCheckpointStore(store)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.checkpointStore != store {
		t.Error("expected checkpointStore to be set")
	}
}

func TestBuild_WithCheckpointStore_WiresReasoningService(t *testing.T) {
	b := minimalValidBuilder(t).WithCheckpointStore(newMemCheckpointStore())
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rs, ok := w.reasoningService.(*ReasoningService)
	if !ok {
		t.Fatal("expected concrete *ReasoningService")
	}
	if rs.checkpointStore == nil {
		t.Error("expected checkpoint store wired into the reasoning service")
	}
}

func TestWeaveBuilder_WithToolResultLimits(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	limits := ports.ToolResultLimits{MaxChars: 8000, Strategy: ports.ToolShapeTruncate}
	got := b.WithToolResultLimits(limits)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.ToolResultLimits.MaxChars != 8000 {
		t.Errorf("expected config ToolResultLimits.MaxChars 8000, got %d", b.config.ToolResultLimits.MaxChars)
	}
}

func TestWeaveBuilder_WithProgressPolicy(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithProgressPolicy(ProgressPolicy{Enabled: true, StallWindow: 3})
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.ProgressPolicy.Enabled || b.config.ProgressPolicy.StallWindow != 3 {
		t.Errorf("expected ProgressPolicy{Enabled:true, StallWindow:3}, got %+v", b.config.ProgressPolicy)
	}
}

func TestWeaveBuilder_WithCompressionPolicy(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithCompressionPolicy(CompressionPolicy{Enabled: true, MaxContextChars: 40000, KeepRecent: 2})
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.CompressionPolicy.Enabled || b.config.CompressionPolicy.MaxContextChars != 40000 {
		t.Errorf("expected CompressionPolicy set, got %+v", b.config.CompressionPolicy)
	}
}

func TestWeaveBuilder_WithReflectionPolicy(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithReflectionPolicy(ReflectionPolicy{Enabled: true, MaxRounds: 2})
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.ReflectionPolicy.Enabled || b.config.ReflectionPolicy.MaxRounds != 2 {
		t.Errorf("expected ReflectionPolicy set, got %+v", b.config.ReflectionPolicy)
	}
}

func TestWeaveBuilder_WithGroundingPolicy(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithGroundingPolicy(GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingReviseOnce})
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.GroundingPolicy.Enabled || b.config.GroundingPolicy.MinCitations != 1 {
		t.Errorf("expected GroundingPolicy set, got %+v", b.config.GroundingPolicy)
	}
}

func TestWeaveBuilder_WithHandoffPolicy(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithHandoffPolicy(HandoffPolicy{Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceMedium})
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.HandoffPolicy.Enabled || b.config.HandoffPolicy.Mode != HandoffRaiseVigil {
		t.Errorf("expected HandoffPolicy set, got %+v", b.config.HandoffPolicy)
	}
}

func TestWeaveBuilder_WithPlanPolicy(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithPlanPolicy(PlanPolicy{Enabled: true, MaxItems: 8})
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.PlanPolicy.Enabled || b.config.PlanPolicy.MaxItems != 8 {
		t.Errorf("expected PlanPolicy set, got %+v", b.config.PlanPolicy)
	}
}

func TestWeaveBuilder_WithRateLimiter(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	rl := &stubRateLimiter{}
	got := b.WithRateLimiter(rl)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.rateLimiter != rl {
		t.Error("expected rateLimiter to be set")
	}
}

func TestWeaveBuilder_WithAsyncDispatch(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	keeper := &stubBuilderKeeper{}
	got := b.WithAsyncDispatch(keeper)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.asyncDispatcher != keeper {
		t.Error("expected asyncDispatcher to be set")
	}
}

func TestWeaveBuilder_WithScoutRegistry(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	reg := &stubScoutRegistryForSummon{}
	got := b.WithScoutRegistry(reg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.scoutRegistry != reg {
		t.Error("expected scoutRegistry to be set")
	}
}

func TestWeaveBuilder_WithActionRegistry(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	reg := newRegistry()
	got := b.WithActionRegistry(reg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.actionRegistry != reg {
		t.Error("expected actionRegistry to be set")
	}
}

func TestWeaveBuilder_WithOracleFactory(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	factory := &stubOracleFactory{}
	got := b.WithOracleFactory(factory)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.oracleFactory != factory {
		t.Error("expected oracleFactory to be set")
	}
}

func TestWeaveBuilder_WithLockTTL(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	ttl := 5 * time.Minute
	got := b.WithLockTTL(ttl)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.LockTTL != ttl {
		t.Errorf("expected LockTTL=%v, got %v", ttl, b.config.LockTTL)
	}
}

func TestWeaveBuilder_WithMaxIterations(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithMaxIterations(10)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.MaxReasoningIterations != 10 {
		t.Errorf("expected MaxReasoningIterations=10, got %d", b.config.MaxReasoningIterations)
	}
}

func TestWeaveBuilder_WithMaxMemoryMessages(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithMaxMemoryMessages(20)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.MaxMemoryMessages != 20 {
		t.Errorf("expected MaxMemoryMessages=20, got %d", b.config.MaxMemoryMessages)
	}
}

func TestWeaveBuilder_WithMemoryReconstruction(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithMemoryReconstruction(false, 15)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.EnableMemoryReconstruction != false {
		t.Error("expected EnableMemoryReconstruction=false")
	}
	if b.config.MemoryReconstructionLimit != 15 {
		t.Errorf("expected limit=15, got %d", b.config.MemoryReconstructionLimit)
	}
}

func TestWeaveBuilder_WithMemoryReconstruction_ZeroLimitIgnored(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.config.MemoryReconstructionLimit = 10
	b.WithMemoryReconstruction(true, 0)
	if b.config.MemoryReconstructionLimit != 10 {
		t.Error("zero limit should be ignored")
	}
}

func TestWeaveBuilder_WithParallelActionExecution(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithParallelActionExecution(true)
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.ParallelActionExecution {
		t.Error("expected ParallelActionExecution=true")
	}
}

func TestWeaveBuilder_WithActionRetry(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithActionRetry(3, 100*time.Millisecond)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.ActionRetryMaxAttempts != 3 {
		t.Errorf("expected 3 attempts, got %d", b.config.ActionRetryMaxAttempts)
	}
	if b.config.ActionRetryBaseDelay != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", b.config.ActionRetryBaseDelay)
	}
}

func TestWeaveBuilder_WithReasoningTimeout(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithReasoningTimeout(30 * time.Second)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.ReasoningTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", b.config.ReasoningTimeout)
	}
}

func TestWeaveBuilder_WithScoutTimeout(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithScoutTimeout(5 * time.Second)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.ScoutTimeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", b.config.ScoutTimeout)
	}
}

func TestWeaveBuilder_WithLockAcquireTimeout(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithLockAcquireTimeout(2 * time.Second)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.LockAcquireTimeout != 2*time.Second {
		t.Errorf("expected 2s, got %v", b.config.LockAcquireTimeout)
	}
}

func TestWeaveBuilder_WithMaxIterationsMessage(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithMaxIterationsMessage("too many")
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.MaxIterationsMessage != "too many" {
		t.Errorf("expected 'too many', got %q", b.config.MaxIterationsMessage)
	}
}

func TestWeaveBuilder_WithSpiritSelectionTimeout(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithSpiritSelectionTimeout(3 * time.Second)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.SpiritSelectionTimeout != 3*time.Second {
		t.Errorf("expected 3s, got %v", b.config.SpiritSelectionTimeout)
	}
}

func TestWeaveBuilder_WithValidationLimits(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithValidationLimits(100, 500, 20)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.MaxSessionKeyLength != 100 || b.config.MaxUserMessageLength != 500 || b.config.MaxEventContextSize != 20 {
		t.Error("expected validation limits to be set")
	}
}

func TestWeaveBuilder_WithLogger(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	logger := zap.NewNop().Sugar()
	got := b.WithLogger(logger)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.logger != logger {
		t.Error("expected logger to be set")
	}
}

func TestWeaveBuilder_WithTracer(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	tracer := noop.NewTracerProvider().Tracer("test")
	got := b.WithTracer(tracer)
	if got != b {
		t.Error("expected fluent builder")
	}
}

func TestWeaveBuilder_Validate_MissingRepos(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	if err := b.validate(); err == nil {
		t.Fatal("expected error for missing spiritRepo")
	}
}

func TestWeaveBuilder_Validate_MissingOracle(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	b.messageRepo = &stubEchoRepo{}
	b.interactionLogRepo = &stubBuilderChronicleRepo{}
	b.distributedLock = &stubBond{acquired: true}
	if err := b.validate(); err == nil {
		t.Fatal("expected error for missing oracle")
	}
}

// --- minimal stubs for builder tests ---

type stubRateLimiter struct{}

func (s *stubRateLimiter) Allow(_ context.Context, _ string) (bool, error) { return true, nil }

type stubBuilderKeeper struct{}

func (s *stubBuilderKeeper) Schedule(_ context.Context, _ string, _ *entities.Pulse, _ time.Time) (string, error) {
	return "", nil
}
func (s *stubBuilderKeeper) Cancel(_ context.Context, _ string) error { return nil }

type stubBuilderChronicleRepo struct{}

func (r *stubBuilderChronicleRepo) LogInteraction(_ context.Context, _ *entities.Chronicle) error {
	return nil
}
func (r *stubBuilderChronicleRepo) FindByMemoryKey(_ context.Context, _ string) ([]*entities.Chronicle, error) {
	return nil, nil
}
func (r *stubBuilderChronicleRepo) FindByEventKey(_ context.Context, _ string) ([]*entities.Chronicle, error) {
	return nil, nil
}
func (r *stubBuilderChronicleRepo) FindByTimeRange(_ context.Context, _, _ time.Time) ([]*entities.Chronicle, error) {
	return nil, nil
}

func TestWeaveBuilder_WithChronicleRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderChronicleRepo{}
	got := b.WithChronicleRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.interactionLogRepo == nil {
		t.Error("expected interactionLogRepo to be set")
	}
}

func TestWeaveBuilder_WithInputGuard(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	cfg := GuardConfig{PromptInjectionDetection: true}
	got := b.WithInputGuard(cfg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.config.InputGuard.PromptInjectionDetection {
		t.Error("expected InputGuard to be set")
	}
}

func TestWeaveBuilder_WithMessageInbox(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	inbox := &stubBuilderInbox{}
	got := b.WithMessageInbox(inbox)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.messageInbox != inbox {
		t.Error("expected messageInbox to be set")
	}
}

func TestWeaveBuilder_WithInboxMinWindow(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithInboxMinWindow(3 * time.Second)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.InboxMinWindow != 3*time.Second {
		t.Errorf("expected 3s, got %v", b.config.InboxMinWindow)
	}
}

func TestWeaveBuilder_WithRitualManager(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	mgr := &stubBuilderRitualManager{}
	got := b.WithRitualManager(mgr)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.scheduledTaskManager != mgr {
		t.Error("expected scheduledTaskManager to be set")
	}
}

func TestWeaveBuilder_WithMediaStore(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	store := &stubBuilderVault{}
	got := b.WithMediaStore(store)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.mediaStore != store {
		t.Error("expected mediaStore to be set")
	}
}

func TestWeaveBuilder_WithMediaProcessor(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	lens := &stubBuilderLens{}
	got := b.WithMediaProcessor(lens)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.mediaProcessor != lens {
		t.Error("expected mediaProcessor to be set")
	}
}

func TestWeaveBuilder_WithTypingIndicator(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	ti := &stubBuilderTypingIndicator{}
	got := b.WithTypingIndicator(ti)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.typingIndicator != ti {
		t.Error("expected typingIndicator to be set")
	}
}

func TestWeaveBuilder_AddOracle(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	oracle := &stubOracle{}
	got := b.AddOracle(oracle)
	if got != b {
		t.Error("expected fluent builder")
	}
	if _, ok := b.oracleProviders[oracle.GetName()]; !ok {
		t.Error("expected oracle to be registered")
	}
}

func TestWeaveBuilder_WithPathfinderRegistry(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	reg := &stubBuilderPathfinderRegistry{}
	got := b.WithPathfinderRegistry(reg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.pathfinderRegistry != reg {
		t.Error("expected pathfinderRegistry to be set")
	}
}

func TestWeaveBuilder_WithVoiceRegistry(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	reg := &stubBuilderVoiceRegistry{}
	got := b.WithVoiceRegistry(reg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.voiceRegistry != reg {
		t.Error("expected voiceRegistry to be set")
	}
}

func TestWeaveBuilder_WithLogicRouterRegistry(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	reg := &stubBuilderLogicRouterRegistry{}
	got := b.WithLogicRouterRegistry(reg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.logicRouterRegistry != reg {
		t.Error("expected logicRouterRegistry to be set")
	}
}

func TestWeaveBuilder_WithLoreRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderLoreRepo{}
	got := b.WithLoreRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.loreRepository != repo {
		t.Error("expected loreRepository to be set")
	}
}

func TestWeaveBuilder_WithLoreStore(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	store := &stubBuilderLoreStore{}
	got := b.WithLoreStore(store)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.loreStore != store {
		t.Error("expected loreStore to be set")
	}
}

func TestWeaveBuilder_WithLoreEmbedder(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	emb := &stubBuilderLoreEmbedder{}
	got := b.WithLoreEmbedder(emb)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.loreEmbedder != emb {
		t.Error("expected loreEmbedder to be set")
	}
}

func TestWeaveBuilder_WithImprintRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubImprintRepo{}
	got := b.WithImprintRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.imprintRepository != repo {
		t.Error("expected imprintRepository to be set")
	}
}

func TestWeaveBuilder_WithImprintExtraction(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	cfg := ImprintExtractionConfig{Enabled: true, MaxImprints: 5}
	got := b.WithImprintExtraction(cfg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if !b.imprintExtractionCfg.Enabled || b.imprintExtractionCfg.MaxImprints != 5 {
		t.Error("expected imprintExtractionCfg to be set")
	}
}

func TestWeaveBuilder_WithConduit(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	c := &stubBuilderConduit{}
	got := b.WithConduit(c)
	if got != b {
		t.Error("expected fluent builder")
	}
	if len(b.conduits) != 1 {
		t.Errorf("expected 1 conduit, got %d", len(b.conduits))
	}
}

func TestWeaveBuilder_WithLedgerRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderLedgerRepo{}
	got := b.WithLedgerRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.ledgerRepo != repo {
		t.Error("expected ledgerRepo to be set")
	}
}

func TestWeaveBuilder_WithCostAlertHook(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	hook := &stubBuilderCostAlertHook{}
	got := b.WithCostAlertHook(hook)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.costAlertHook != hook {
		t.Error("expected costAlertHook to be set")
	}
}

func TestWeaveBuilder_WithModelRoutingRules(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	rules := []entities.ModelRoutingRule{{Name: "support"}}
	got := b.WithModelRoutingRules(rules)
	if got != b {
		t.Error("expected fluent builder")
	}
	if len(b.modelRoutingRules) != 1 {
		t.Error("expected 1 routing rule")
	}
}

func TestWeaveBuilder_WithConfig(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	cfg := &WeaveConfig{MaxReasoningIterations: 99}
	got := b.WithConfig(cfg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.config.MaxReasoningIterations != 99 {
		t.Error("expected custom config to be set")
	}
}

func TestWeaveBuilder_WithLinkRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderLinkRepo{}
	got := b.WithLinkRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.linkRepo != repo {
		t.Error("expected linkRepo to be set")
	}
}

func TestWeaveBuilder_WithPubSub(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	ps := &stubBuilderPubSub{}
	got := b.WithPubSub(ps)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.pubSub != ps {
		t.Error("expected pubSub to be set")
	}
}

func TestWeaveBuilder_WithHTTPToolRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderHTTPToolRepo{}
	got := b.WithHTTPToolRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.httpToolRepo != repo {
		t.Error("expected httpToolRepo to be set")
	}
}

func TestWeaveBuilder_WithVigilConfig(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	cfg := VigilConfig{InactivityTimeout: 30 * time.Minute}
	got := b.WithVigilConfig(cfg)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.vigilConfig.InactivityTimeout != 30*time.Minute {
		t.Error("expected vigilConfig to be set")
	}
}

func TestWeaveBuilder_WithRiteRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderRiteRepo{}
	got := b.WithRiteRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.riteRepo != repo {
		t.Error("expected riteRepo to be set")
	}
}

func TestWeaveBuilder_WithAppInfo(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithAppInfo("myapp", "2.0")
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.appInfo.Name != "myapp" || b.appInfo.Version != "2.0" {
		t.Error("expected appInfo to be set")
	}
}

func TestWeaveBuilder_WithArchivist(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	archivist := &stubBuilderArchivist{}
	got := b.WithArchivist(archivist, 20)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.archivist != archivist || b.archivistThreshold != 20 {
		t.Error("expected archivist and threshold to be set")
	}
}

func TestWeaveBuilder_WithArchivistConfig(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithArchivistConfig(0.1, 1024)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.defaultArchivistTemp != 0.1 || b.defaultArchivistTokens != 1024 {
		t.Error("expected archivist config to be set")
	}
	if !b.defaultArchivistTempSet {
		t.Error("expected tempSet flag to be true")
	}
}

func TestWeaveBuilder_WithArchivistKeepRecent(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithArchivistKeepRecent(5)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.archivistKeepRecent != 5 {
		t.Error("expected keepRecent to be set")
	}
}

func TestWeaveBuilder_ResolveKeepRecent_Configured(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.archivistKeepRecent = 7
	b.archivistThreshold = 20
	if got := b.resolveKeepRecent(); got != 7 {
		t.Errorf("expected 7, got %d", got)
	}
}

func TestWeaveBuilder_ResolveKeepRecent_Default(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.archivistThreshold = 20
	if got := b.resolveKeepRecent(); got != 10 {
		t.Errorf("expected 10 (threshold/2), got %d", got)
	}
}

func TestWeaveBuilder_WithDefaultLLMArchivist(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithDefaultLLMArchivist("anthropic", "claude-3-5-haiku-20241022", 20)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.defaultArchivistProvider != "anthropic" || b.defaultArchivistModel != "claude-3-5-haiku-20241022" {
		t.Error("expected archivist provider/model to be set")
	}
	if b.archivistThreshold != 20 {
		t.Error("expected threshold=20")
	}
	if b.defaultArchivistTemp == 0 || b.defaultArchivistTokens == 0 {
		t.Error("expected defaults to be populated")
	}
}

// --- additional stubs for builder tests ---

type stubBuilderInbox struct{}

func (s *stubBuilderInbox) Push(_ context.Context, _ string, _ string) error     { return nil }
func (s *stubBuilderInbox) PopAll(_ context.Context, _ string) ([]string, error) { return nil, nil }

type stubBuilderRitualManager struct{}

func (s *stubBuilderRitualManager) Schedule(_ context.Context, _ ports.RitualRequest) (*entities.Ritual, error) {
	return nil, nil
}
func (s *stubBuilderRitualManager) Cancel(_ context.Context, _, _ string) error    { return nil }
func (s *stubBuilderRitualManager) MarkRunning(_ context.Context, _ string) error  { return nil }
func (s *stubBuilderRitualManager) MarkExecuted(_ context.Context, _ string) error { return nil }
func (s *stubBuilderRitualManager) MarkFailed(_ context.Context, _ string, _ string) error {
	return nil
}
func (s *stubBuilderRitualManager) ListPendingByMemoryKey(_ context.Context, _ string, _, _ int) ([]*entities.Ritual, error) {
	return nil, nil
}
func (s *stubBuilderRitualManager) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

type stubBuilderVault struct{}

func (s *stubBuilderVault) Store(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return "", nil
}

type stubBuilderLens struct{}

func (s *stubBuilderLens) Process(_ context.Context, _ *entities.Pulse) (ports.OracleUsage, error) {
	return ports.OracleUsage{}, nil
}

type stubBuilderTypingIndicator struct{}

func (s *stubBuilderTypingIndicator) StartTyping(_ context.Context, _ string) error { return nil }
func (s *stubBuilderTypingIndicator) StopTyping(_ context.Context, _ string) error  { return nil }

type stubBuilderPathfinderRegistry struct{}

func (s *stubBuilderPathfinderRegistry) Register(_ ports.Pathfinder) error            { return nil }
func (s *stubBuilderPathfinderRegistry) RegisterMultiple(_ ...ports.Pathfinder) error { return nil }
func (s *stubBuilderPathfinderRegistry) Get(_ string) ports.Pathfinder                { return nil }
func (s *stubBuilderPathfinderRegistry) List() []string                               { return nil }

type stubBuilderVoiceRegistry struct{}

func (s *stubBuilderVoiceRegistry) Register(_ ports.Voice) error            { return nil }
func (s *stubBuilderVoiceRegistry) RegisterMultiple(_ ...ports.Voice) error { return nil }
func (s *stubBuilderVoiceRegistry) Get(_ string) (ports.Voice, error)       { return nil, nil }
func (s *stubBuilderVoiceRegistry) Has(_ string) bool                       { return false }
func (s *stubBuilderVoiceRegistry) List() []ports.Voice                     { return nil }

type stubBuilderLogicRouterRegistry struct{}

func (s *stubBuilderLogicRouterRegistry) Register(_ ports.LogicRouter) error      { return nil }
func (s *stubBuilderLogicRouterRegistry) Get(_ string) (ports.LogicRouter, error) { return nil, nil }
func (s *stubBuilderLogicRouterRegistry) List() []string                          { return nil }

type stubBuilderLoreRepo struct{}

func (s *stubBuilderLoreRepo) Create(_ context.Context, _ entities.Lore) error { return nil }
func (s *stubBuilderLoreRepo) GetByID(_ context.Context, _ string) (entities.Lore, error) {
	return entities.Lore{}, nil
}
func (s *stubBuilderLoreRepo) GetByName(_ context.Context, _ string) (entities.Lore, error) {
	return entities.Lore{}, nil
}
func (s *stubBuilderLoreRepo) GetBySpiritID(_ context.Context, _ string) ([]entities.Lore, error) {
	return nil, nil
}
func (s *stubBuilderLoreRepo) GetByIDs(_ context.Context, _ []string) ([]entities.Lore, error) {
	return nil, nil
}
func (s *stubBuilderLoreRepo) Update(_ context.Context, _ entities.Lore) error { return nil }
func (s *stubBuilderLoreRepo) Delete(_ context.Context, _ string) error        { return nil }

type stubBuilderLoreStore struct{}

func (s *stubBuilderLoreStore) Upsert(_ context.Context, _ []entities.LoreChunk) error { return nil }
func (s *stubBuilderLoreStore) Search(_ context.Context, _ string, _ []float32, _ int, _ float64) ([]entities.LoreChunk, error) {
	return nil, nil
}
func (s *stubBuilderLoreStore) Delete(_ context.Context, _ string) error { return nil }

type stubBuilderLoreEmbedder struct{}

func (s *stubBuilderLoreEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, nil
}
func (s *stubBuilderLoreEmbedder) Dimensions() int { return 0 }

type stubBuilderConduit struct{}

func (s *stubBuilderConduit) GetName() string                 { return "stub" }
func (s *stubBuilderConduit) Connect(_ context.Context) error { return nil }
func (s *stubBuilderConduit) ListTools(_ context.Context) ([]entities.ActionDefinition, error) {
	return nil, nil
}
func (s *stubBuilderConduit) Call(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}
func (s *stubBuilderConduit) Close() error { return nil }

type stubBuilderLedgerRepo struct{}

func (s *stubBuilderLedgerRepo) IncrementUsage(_ context.Context, _ string, _ int64, _ float64) error {
	return nil
}
func (s *stubBuilderLedgerRepo) GetMonthUsage(_ context.Context, _, _ string) (entities.LedgerEntry, error) {
	return entities.LedgerEntry{}, nil
}
func (s *stubBuilderLedgerRepo) GetBudget(_ context.Context, _ string) (entities.TokenBudget, error) {
	return entities.TokenBudget{}, nil
}
func (s *stubBuilderLedgerRepo) SetBudget(_ context.Context, _ entities.TokenBudget) error {
	return nil
}

type stubBuilderCostAlertHook struct{}

func (s *stubBuilderCostAlertHook) OnBudgetThreshold(_ context.Context, _ string, _ float64, _ entities.LedgerEntry) {
}
func (s *stubBuilderCostAlertHook) OnBudgetExceeded(_ context.Context, _ string, _ string, _ entities.LedgerEntry) {
}

type stubBuilderLinkRepo struct{}

func (s *stubBuilderLinkRepo) FindAll(_ context.Context) ([]*entities.Link, error) { return nil, nil }
func (s *stubBuilderLinkRepo) FindByKey(_ context.Context, _ string) (*entities.Link, error) {
	return nil, nil
}
func (s *stubBuilderLinkRepo) Save(_ context.Context, _ *entities.Link) error { return nil }
func (s *stubBuilderLinkRepo) Delete(_ context.Context, _ string) error       { return nil }

type stubBuilderPubSub struct{}

func (s *stubBuilderPubSub) Publish(_ context.Context, _, _ string) error                { return nil }
func (s *stubBuilderPubSub) Subscribe(_ context.Context, _ string, _ func(string)) error { return nil }

type stubBuilderHTTPToolRepo struct{}

func (s *stubBuilderHTTPToolRepo) List(_ context.Context) ([]entities.HTTPTool, error) {
	return nil, nil
}
func (s *stubBuilderHTTPToolRepo) FindBySpiritID(_ context.Context, _ string) ([]entities.HTTPTool, error) {
	return nil, nil
}
func (s *stubBuilderHTTPToolRepo) FindByID(_ context.Context, _ string) (*entities.HTTPTool, error) {
	return nil, nil
}
func (s *stubBuilderHTTPToolRepo) Save(_ context.Context, _ entities.HTTPTool) error   { return nil }
func (s *stubBuilderHTTPToolRepo) Update(_ context.Context, _ entities.HTTPTool) error { return nil }
func (s *stubBuilderHTTPToolRepo) Delete(_ context.Context, _ string) error            { return nil }

type stubBuilderRiteRepo struct{}

func (s *stubBuilderRiteRepo) Create(_ context.Context, _ *entities.Rite) error { return nil }
func (s *stubBuilderRiteRepo) FindByID(_ context.Context, _ string) (*entities.Rite, error) {
	return nil, nil
}
func (s *stubBuilderRiteRepo) List(_ context.Context, _ ports.RiteListOptions) ([]*entities.Rite, int64, error) {
	return nil, 0, nil
}
func (s *stubBuilderRiteRepo) Decide(_ context.Context, _, _ string, _ entities.RiteStatus) error {
	return nil
}

type stubBuilderArchivist struct{}

func (s *stubBuilderArchivist) Summarize(_ context.Context, _, _ string, _ []entities.Thread) (string, error) {
	return "", nil
}

func TestWeaveBuilder_WithChannel(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	ch := Channel{Name: "whatsapp", Receptor: &stubReceptor{}}
	got := b.WithChannel(ch)
	if got != b {
		t.Error("expected fluent builder")
	}
	if len(b.channels) != 1 || b.channels[0].Name != "whatsapp" {
		t.Error("expected channel to be appended")
	}
}

func TestWeaveBuilder_WithWeaveConfigRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderWeaveConfigRepo{}
	got := b.WithWeaveConfigRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.weaveConfigRepo != repo {
		t.Error("expected weaveConfigRepo to be set")
	}
}

func TestWeaveBuilder_WithVigilRepository(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	repo := &stubBuilderVigilRepo{}
	got := b.WithVigilRepository(repo)
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.vigilRepo != repo {
		t.Error("expected vigilRepo to be set")
	}
}

func TestWeaveBuilder_WithDefaultLLMPathfinder(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	got := b.WithDefaultLLMPathfinder("anthropic", "claude-3-5-haiku")
	if got != b {
		t.Error("expected fluent builder")
	}
	if b.defaultPathfinderProvider != "anthropic" || b.defaultPathfinderModel != "claude-3-5-haiku" {
		t.Error("expected provider/model to be set")
	}
	if b.defaultPathfinderTemp != 0.2 {
		t.Errorf("expected default temp=0.2, got %v", b.defaultPathfinderTemp)
	}
}

func TestWeaveBuilder_WithDefaultLLMPathfinder_CustomTemp(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.WithDefaultLLMPathfinder("openai", "gpt-4o-mini", 0.5)
	if b.defaultPathfinderTemp != 0.5 {
		t.Errorf("expected temp=0.5, got %v", b.defaultPathfinderTemp)
	}
}

// --- stubs for the remaining builder tests ---

type stubBuilderWeaveConfigRepo struct{}

func (r *stubBuilderWeaveConfigRepo) Find(_ context.Context) (*WeaveConfig, error) {
	return DefaultWeaveConfig(), nil
}
func (r *stubBuilderWeaveConfigRepo) Save(_ context.Context, _ *WeaveConfig) error { return nil }

type stubBuilderVigilRepo struct{}

func (r *stubBuilderVigilRepo) Acquire(_ context.Context, _, _ string, _ time.Duration) error {
	return nil
}
func (r *stubBuilderVigilRepo) Release(_ context.Context, _ string) error { return nil }
func (r *stubBuilderVigilRepo) Get(_ context.Context, _ string) (*entities.Vigil, error) {
	return nil, nil
}
func (r *stubBuilderVigilRepo) Refresh(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
func (r *stubBuilderVigilRepo) ListAll(_ context.Context) ([]*entities.Vigil, error) { return nil, nil }

func TestWeaveBuilder_Validate_MissingSessionRepo(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	if err := b.validate(); err == nil {
		t.Fatal("expected error for missing sessionRepo")
	}
}

func TestWeaveBuilder_Validate_MissingMessageRepo(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	if err := b.validate(); err == nil {
		t.Fatal("expected error for missing messageRepo")
	}
}

func TestWeaveBuilder_Validate_MissingInteractionLogRepo(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	b.messageRepo = &stubEchoRepo{}
	if err := b.validate(); err == nil {
		t.Fatal("expected error for missing interactionLogRepo")
	}
}

func TestWeaveBuilder_Validate_MissingDistributedLock(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	b.messageRepo = &stubEchoRepo{}
	b.interactionLogRepo = &stubBuilderChronicleRepo{}
	if err := b.validate(); err == nil {
		t.Fatal("expected error for missing distributedLock")
	}
}

func TestWeaveBuilder_Validate_ArchivistThresholdTooLow(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	b.messageRepo = &stubEchoRepo{}
	b.interactionLogRepo = &stubBuilderChronicleRepo{}
	b.distributedLock = &stubBond{acquired: true}
	b.oracleFactory = &stubOracleFactory{}
	b.archivist = &stubBuilderArchivist{}
	b.archivistThreshold = 1 // must be >= 2
	if err := b.validate(); err == nil {
		t.Fatal("expected error for low archivist threshold")
	}
}

func TestWeaveBuilder_Validate_DefaultArchivistProviderThresholdTooLow(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	b.messageRepo = &stubEchoRepo{}
	b.interactionLogRepo = &stubBuilderChronicleRepo{}
	b.distributedLock = &stubBond{acquired: true}
	b.oracleFactory = &stubOracleFactory{}
	b.defaultArchivistProvider = "anthropic"
	b.archivistThreshold = 1
	if err := b.validate(); err == nil {
		t.Fatal("expected error for low archivist threshold via default provider")
	}
}

func TestWeaveBuilder_Validate_AllRequired_Success(t *testing.T) {
	b := NewWeaveBuilder(context.Background())
	b.spiritRepo = &stubSpiritRepo{}
	b.sessionRepo = &stubMemoryRepo{}
	b.messageRepo = &stubEchoRepo{}
	b.interactionLogRepo = &stubBuilderChronicleRepo{}
	b.distributedLock = &stubBond{acquired: true}
	b.oracleFactory = &stubOracleFactory{}
	if err := b.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- wireConduits ---

type stubConduit struct {
	name         string
	connectErr   error
	listToolsErr error
	tools        []entities.ActionDefinition
}

func (c *stubConduit) GetName() string { return c.name }
func (c *stubConduit) Connect(_ context.Context) error {
	return c.connectErr
}
func (c *stubConduit) ListTools(_ context.Context) ([]entities.ActionDefinition, error) {
	return c.tools, c.listToolsErr
}
func (c *stubConduit) Call(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}
func (c *stubConduit) Close() error { return nil }

func minimalWeaveForConduit(t *testing.T) *Weave {
	t.Helper()
	w, err := newWeaveWithConfig(
		nil, nil, nil, nil, nil, nil, nil, nil,
		newRegistry(), // actionRegistry so RegisterAction works
		nil, nil, nil, nil,
		nil,
		testLogger(t), noopTracer(),
	)
	if err != nil {
		t.Fatalf("newWeaveWithConfig: %v", err)
	}
	return w
}

func TestWireConduits_ConnectError_Skips(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.conduits = []ports.Conduit{&stubConduit{name: "mcp-fail", connectErr: errors.New("refused")}}

	w := minimalWeaveForConduit(t)
	b.wireConduits(context.Background(), w) // must not panic
}

func TestWireConduits_ListToolsError_Skips(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.conduits = []ports.Conduit{&stubConduit{name: "mcp-list-fail", listToolsErr: errors.New("timeout")}}

	w := minimalWeaveForConduit(t)
	b.wireConduits(context.Background(), w)
}

func TestWireConduits_Success_RegistersTools(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.conduits = []ports.Conduit{&stubConduit{
		name:  "mcp-ok",
		tools: []entities.ActionDefinition{{Name: "search", Description: "web search"}},
	}}

	w := minimalWeaveForConduit(t)
	b.wireConduits(context.Background(), w)
}

func TestWireConduits_NoConduits_NoOp(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	// b.conduits is nil
	w := minimalWeaveForConduit(t)
	b.wireConduits(context.Background(), w) // must not panic
}

// --- buildOracleFactory ---

func TestBuildOracleFactory_NoProviders_Error(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	// no oracle providers set
	_, err := b.buildOracleFactory()
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestBuildOracleFactory_WithProviders_Success(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.oracleProviders = map[string]ports.Oracle{"openai": &stubOracle{}}

	factory, err := b.buildOracleFactory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
}

// --- registerDefaultLLMPathfinder ---

func TestRegisterDefaultLLMPathfinder_NilRegistry_CreatesOne(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.defaultPathfinderProvider = "openai"
	b.defaultPathfinderModel = "gpt-4"

	w := minimalWeaveForConduit(t)
	w.pathfinderRegistry = nil // force auto-create
	b.registerDefaultLLMPathfinder(w)
	if w.pathfinderRegistry == nil {
		t.Fatal("expected pathfinder registry to be created")
	}
}

func TestRegisterDefaultLLMPathfinder_ExistingRegistry(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.defaultPathfinderProvider = "anthropic"
	b.defaultPathfinderModel = "claude-3"

	w := minimalWeaveForConduit(t)
	w.pathfinderRegistry = &stubBuilderPathfinderRegistry{}
	b.registerDefaultLLMPathfinder(w) // must not panic
}

// --- buildDefaultLLMArchivist ---

func TestBuildDefaultLLMArchivist_SetsEngine(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.defaultArchivistProvider = "openai"
	b.defaultArchivistModel = "gpt-4"
	b.archivistThreshold = 10
	b.defaultArchivistTempSet = true
	b.defaultArchivistTemp = 0.5
	b.defaultArchivistTokens = 500

	w := minimalWeaveForConduit(t)
	b.buildDefaultLLMArchivist(w)
	if w.archivist == nil {
		t.Fatal("expected archivist to be set")
	}
	if w.archivistThreshold != 10 {
		t.Errorf("expected threshold=10, got %d", w.archivistThreshold)
	}
}

// --- Build() ---

func minimalValidBuilder(t *testing.T) *WeaveBuilder {
	t.Helper()
	b := NewWeaveBuilder(context.Background()).
		WithLogger(testLogger(t)).
		WithSpiritRepository(&stubSpiritRepo{}).
		WithMemoryRepository(&stubMemoryRepo{}).
		WithEchoRepository(&stubEchoRepo{}).
		WithChronicleRepository(&stubBuilderChronicleRepo{}).
		WithBond(&stubBond{acquired: true}).
		WithOracleFactory(&stubOracleFactory{})
	return b
}

func TestBuild_ValidationError_ReturnsError(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	// missing required fields → validate() will fail
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error for invalid builder")
	}
}

func TestBuild_Success_Minimal(t *testing.T) {
	b := minimalValidBuilder(t)
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

func TestBuild_NilLogger_CreatesDefault(t *testing.T) {
	b := minimalValidBuilder(t)
	b.logger = nil // force logger creation
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

func TestBuild_WithOracleProviders_BuildsFactory(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).
		WithLogger(testLogger(t)).
		WithSpiritRepository(&stubSpiritRepo{}).
		WithMemoryRepository(&stubMemoryRepo{}).
		WithEchoRepository(&stubEchoRepo{}).
		WithChronicleRepository(&stubBuilderChronicleRepo{}).
		WithBond(&stubBond{acquired: true})
	// No WithOracleFactory — use providers instead
	b.oracleProviders = map[string]ports.Oracle{"openai": &stubOracle{}}

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

func TestBuild_WithHTTPToolRepo_AutoCreatesActionRegistry(t *testing.T) {
	b := minimalValidBuilder(t)
	b.httpToolRepo = &stubBuilderHTTPToolRepo{}

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.GetActionRegistry() == nil {
		t.Error("expected action registry to be auto-created")
	}
}

func TestBuild_WithDefaultPathfinder_RegistersIt(t *testing.T) {
	b := minimalValidBuilder(t)
	b.defaultPathfinderProvider = "openai"
	b.defaultPathfinderModel = "gpt-4"

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.pathfinderRegistry == nil {
		t.Error("expected pathfinder registry to be created")
	}
}

func TestBuild_WithDefaultArchivist_SetsIt(t *testing.T) {
	b := minimalValidBuilder(t)
	b.defaultArchivistProvider = "openai"
	b.defaultArchivistModel = "gpt-4"
	b.archivistThreshold = 10

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.archivist == nil {
		t.Error("expected archivist to be set via default provider")
	}
}

func TestBuild_WithExplicitArchivist_SetsIt(t *testing.T) {
	b := minimalValidBuilder(t)
	b.archivist = &stubBuilderArchivist{}
	b.archivistThreshold = 5

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.archivist == nil {
		t.Error("expected explicit archivist to be set")
	}
}

func TestBuild_WithLinkRepo_LoadsCache(t *testing.T) {
	b := minimalValidBuilder(t)
	b.linkRepo = &stubBuilderLinkRepo{}

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

func TestBuild_WithPubSub_StartsSubscribe(t *testing.T) {
	b := minimalValidBuilder(t)
	b.pubSub = &stubBuilderPubSub{}
	b.linkRepo = &stubBuilderLinkRepo{}

	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

// stubWeaveConfigRepoErr returns an error on Find.
type stubWeaveConfigRepoErr struct{}

func (r *stubWeaveConfigRepoErr) Find(_ context.Context) (*WeaveConfig, error) {
	return nil, errors.New("config load error")
}
func (r *stubWeaveConfigRepoErr) Save(_ context.Context, _ *WeaveConfig) error { return nil }

// stubLinkRepoErr returns error on FindAll (used by cache.LoadAll).
type stubLinkRepoErr struct{}

func (s *stubLinkRepoErr) FindAll(_ context.Context) ([]*entities.Link, error) {
	return nil, errors.New("db error")
}
func (s *stubLinkRepoErr) FindByKey(_ context.Context, _ string) (*entities.Link, error) {
	return nil, errors.New("db error")
}
func (s *stubLinkRepoErr) Save(_ context.Context, _ *entities.Link) error { return nil }
func (s *stubLinkRepoErr) Delete(_ context.Context, _ string) error       { return nil }

func TestBuild_WeaveConfigRepo_LoadError_ReturnsError(t *testing.T) {
	b := minimalValidBuilder(t)
	b.weaveConfigRepo = &stubWeaveConfigRepoErr{}
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error from weaveConfigRepo.Find")
	}
}

func TestBuild_WeaveConfigRepo_Success(t *testing.T) {
	b := minimalValidBuilder(t)
	b.weaveConfigRepo = &stubBuilderWeaveConfigRepo{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

func TestBuild_LinkRepo_LoadAllError_ReturnsError(t *testing.T) {
	b := minimalValidBuilder(t)
	b.linkRepo = &stubLinkRepoErr{}
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error from cache.LoadAll")
	}
}

func TestBuild_WithVigilRepo(t *testing.T) {
	b := minimalValidBuilder(t)
	b.vigilRepo = &stubBuilderVigilRepo{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.vigilRepo == nil {
		t.Error("expected vigilRepo to be set")
	}
}

func TestBuild_WithRiteRepo(t *testing.T) {
	b := minimalValidBuilder(t)
	b.riteRepo = &stubBuilderRiteRepo{}
	b.actionRegistry = newRegistry()
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = w
}

func TestBuild_WithScheduledTaskManager(t *testing.T) {
	b := minimalValidBuilder(t)
	b.scheduledTaskManager = &stubBuilderRitualManager{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.scheduledTaskManager == nil {
		t.Error("expected scheduledTaskManager to be set")
	}
}

func TestBuild_WithMediaStore(t *testing.T) {
	b := minimalValidBuilder(t)
	b.mediaStore = &stubBuilderVault{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.mediaStore == nil {
		t.Error("expected mediaStore to be set")
	}
}

func TestBuild_WithMediaProcessor(t *testing.T) {
	b := minimalValidBuilder(t)
	b.mediaProcessor = &stubBuilderLens{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.mediaProcessor == nil {
		t.Error("expected mediaProcessor to be set")
	}
}

func TestBuild_WithTypingIndicator(t *testing.T) {
	b := minimalValidBuilder(t)
	b.typingIndicator = &stubBuilderTypingIndicator{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.typingIndicator == nil {
		t.Error("expected typingIndicator to be set")
	}
}

func TestBuild_WithLore(t *testing.T) {
	b := minimalValidBuilder(t)
	b.loreRepository = &stubBuilderLoreRepo{}
	b.loreStore = &stubBuilderLoreStore{}
	b.loreEmbedder = &stubBuilderLoreEmbedder{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.loreRepository == nil {
		t.Error("expected loreRepository to be set")
	}
}

func TestBuild_WithImprint(t *testing.T) {
	b := minimalValidBuilder(t)
	b.imprintRepository = &stubImprintRepo{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.imprintRepository == nil {
		t.Error("expected imprintRepository to be set")
	}
}

func TestBuild_WithLedgerRepo(t *testing.T) {
	b := minimalValidBuilder(t)
	b.ledgerRepo = &stubBuilderLedgerRepo{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.ledgerRepo == nil {
		t.Error("expected ledgerRepo to be set")
	}
}

func TestBuild_WithLogicRouter(t *testing.T) {
	b := minimalValidBuilder(t)
	b.logicRouterRegistry = &stubBuilderLogicRouterRegistry{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.logicRouterRegistry == nil {
		t.Error("expected logicRouterRegistry to be set")
	}
}

func TestBuild_WithModelRoutingRules(t *testing.T) {
	b := minimalValidBuilder(t)
	b.modelRoutingRules = []entities.ModelRoutingRule{{Name: "rule-1"}}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.modelRoutingRules) != 1 {
		t.Errorf("expected 1 routing rule, got %d", len(w.modelRoutingRules))
	}
}

func TestBuild_WithChannels(t *testing.T) {
	b := minimalValidBuilder(t)
	b.channels = []Channel{{
		Name:     "whatsapp",
		Receptor: &stubReceptor{},
		Voice:    &stubVoice{},
	}}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := w.GetReceptorNames()
	if len(names) != 1 {
		t.Errorf("expected 1 receptor, got %d", len(names))
	}
}

func TestBuild_WithConduits(t *testing.T) {
	b := minimalValidBuilder(t)
	b.conduits = []ports.Conduit{&stubConduit{name: "mcp-1"}}
	b.actionRegistry = newRegistry() // needed so RegisterAction works
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = w
}

// --- Additional gap coverage ---

// stubErrorPathfinderRegistry returns an error from Register to cover the error log branch.
type stubErrorPathfinderRegistry struct{}

func (s *stubErrorPathfinderRegistry) Register(_ ports.Pathfinder) error {
	return errors.New("already registered")
}
func (s *stubErrorPathfinderRegistry) RegisterMultiple(_ ...ports.Pathfinder) error { return nil }
func (s *stubErrorPathfinderRegistry) Get(_ string) ports.Pathfinder                { return nil }
func (s *stubErrorPathfinderRegistry) List() []string                               { return nil }

func TestRegisterDefaultLLMPathfinder_RegisterError_LogsWarn(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.defaultPathfinderProvider = "openai"
	b.defaultPathfinderModel = "gpt-4o"

	w := minimalWeaveForConduit(t)
	w.pathfinderRegistry = &stubErrorPathfinderRegistry{}
	b.registerDefaultLLMPathfinder(w) // must not panic; error logged
}

// TestBuildOracleFactory_EmptyProviderName triggers RegisterProvider("", ...) error.
func TestBuildOracleFactory_EmptyProviderName(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.oracleProviders = map[string]ports.Oracle{"": &stubOracle{}}
	_, err := b.buildOracleFactory()
	if err == nil {
		t.Fatal("expected error when provider name is empty")
	}
}

// stubNonDefaultActionExecutor satisfies ActionExecutor without being *DefaultActionExecutor.
type stubNonDefaultActionExecutor struct{}

func (e *stubNonDefaultActionExecutor) ExecuteAll(_ context.Context, _ []ports.OracleToolCall) []ActionResult {
	return nil
}
func (e *stubNonDefaultActionExecutor) GetAction(_ string) (ports.Action, error) { return nil, nil }

func TestWireConduits_RegisterActionFails_LogsWarn(t *testing.T) {
	b := NewWeaveBuilder(context.Background()).WithLogger(testLogger(t))
	b.conduits = []ports.Conduit{&stubConduit{
		name:  "mcp-reg-fail",
		tools: []entities.ActionDefinition{{Name: "tool1", Description: "test tool"}},
	}}

	w := minimalWeaveForConduit(t)
	// Replace with non-*DefaultActionExecutor so RegisterAction returns an error.
	w.actionExecutor = &stubNonDefaultActionExecutor{}
	b.wireConduits(context.Background(), w) // must not panic; warn logged
}

// TestBuild_WithRateLimiter_Pipeline ensures the rateLimiter branch in buildProcessingPipeline is hit.
func TestBuild_WithRateLimiter_Pipeline(t *testing.T) {
	b := minimalValidBuilder(t)
	b.rateLimiter = &stubRateLimiter{}
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = w
}

// TestBuild_OracleFactoryError_ReturnsError covers builder.go:574-576.
// oracleFactory=nil + empty provider name → buildOracleFactory errors.
func TestBuild_OracleFactoryError_ReturnsError(t *testing.T) {
	b := minimalValidBuilder(t)
	b.oracleFactory = nil
	b.oracleProviders = map[string]ports.Oracle{"": &stubOracle{}} // empty name → error
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error from buildOracleFactory")
	}
}

// TestBuild_InvalidWeaveConfig_ReturnsError covers builder.go:636-638.
// Passing an invalid WeaveConfig causes newWeaveWithConfig to fail.
func TestBuild_InvalidWeaveConfig_ReturnsError(t *testing.T) {
	b := minimalValidBuilder(t)
	b.config = &WeaveConfig{LockTTL: -1} // invalid: LockTTL must be positive
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected error from invalid WeaveConfig")
	}
}

// TestBuild_RiteRepo_RegisterError_LogsWarn covers builder.go:657-659.
// Pre-register "request_rite" so the second Register call fails, hitting the warn log.
func TestBuild_RiteRepo_RegisterError_LogsWarn(t *testing.T) {
	b := minimalValidBuilder(t)
	b.riteRepo = &stubBuilderRiteRepo{}

	// Use a real action registry with "request_rite" pre-registered.
	reg := registries.NewActionRegistry()
	if err := reg.Register(actions.NewRequestRiteAction(b.riteRepo, nil)); err != nil {
		t.Fatalf("pre-register failed: %v", err)
	}
	b.actionRegistry = reg

	// Build: riteRepo != nil → tries to register "request_rite" again → duplicate → warn logged.
	w, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = w
}
