package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/implementation/pathfinders"
	"go.uber.org/zap"
)

// minimalWeave builds a Weave struct directly (same package) with only the fields needed for getter tests.
func minimalWeave() *Weave {
	return &Weave{
		appInfo:           AppInfo{Name: "test", Version: "1.0"},
		inboundConverters: map[string]ports.Receptor{},
		configCache:       NewConfigCache(nil, nil, nil),
		tracer:            noopTracer(),
		logger:            zap.NewNop().Sugar(),
	}
}

// --- GetAppInfo ---

func TestWeave_GetAppInfo(t *testing.T) {
	e := minimalWeave()
	info := e.GetAppInfo()
	if info.Name != "test" {
		t.Errorf("expected name=test, got %q", info.Name)
	}
	if info.Version != "1.0" {
		t.Errorf("expected version=1.0, got %q", info.Version)
	}
}

// --- GetLLMFactory ---

func TestWeave_GetLLMFactory_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetLLMFactory() != nil {
		t.Error("expected nil factory when not set")
	}
}

func TestWeave_GetLLMFactory_Set(t *testing.T) {
	e := minimalWeave()
	factory := &stubOracleFactory{}
	e.oracleFactory = factory
	if e.GetLLMFactory() != factory {
		t.Error("expected factory to be returned")
	}
}

// --- GetScoutRegistry ---

func TestWeave_GetScoutRegistry_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetScoutRegistry() != nil {
		t.Error("expected nil registry")
	}
}

func TestWeave_GetScoutRegistry_Set(t *testing.T) {
	e := minimalWeave()
	reg := &stubScoutRegistryForSummon{}
	e.scoutRegistry = reg
	if e.GetScoutRegistry() != reg {
		t.Error("expected registry to be returned")
	}
}

// --- GetAsyncDispatcher ---

func TestWeave_GetAsyncDispatcher_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetAsyncDispatcher() != nil {
		t.Error("expected nil dispatcher")
	}
}

func TestWeave_GetAsyncDispatcher_Set(t *testing.T) {
	e := minimalWeave()
	keeper := &stubBuilderKeeper{}
	e.asyncDispatcher = keeper
	if e.GetAsyncDispatcher() != keeper {
		t.Error("expected dispatcher to be returned")
	}
}

// --- GetRitualManager ---

func TestWeave_GetRitualManager_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetRitualManager() != nil {
		t.Error("expected nil ritual manager")
	}
}

// --- GetActionRegistry ---

func TestWeave_GetActionRegistry_NoExecutor_ReturnsNil(t *testing.T) {
	e := minimalWeave()
	if e.GetActionRegistry() != nil {
		t.Error("expected nil when no executor")
	}
}

func TestWeave_GetActionRegistry_WithExecutor(t *testing.T) {
	e := minimalWeave()
	reg := newRegistry()
	e.actionExecutor = NewActionExecutor(reg, false, testLogger(t), noopTracer())
	got := e.GetActionRegistry()
	if got == nil {
		t.Error("expected non-nil action registry")
	}
}

// --- GetReceptorNames ---

func TestWeave_GetReceptorNames_Empty(t *testing.T) {
	e := minimalWeave()
	names := e.GetReceptorNames()
	if len(names) != 0 {
		t.Errorf("expected empty, got %v", names)
	}
}

func TestWeave_GetReceptorNames_WithReceptors(t *testing.T) {
	e := minimalWeave()
	e.inboundConverters["whatsapp"] = &stubReceptor{}
	e.inboundConverters["sms"] = &stubReceptor{}
	names := e.GetReceptorNames()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

// --- RegisterReceptor ---

func TestWeave_RegisterReceptor(t *testing.T) {
	e := minimalWeave()
	r := &stubReceptor{}
	e.RegisterReceptor("whatsapp", r)
	if e.inboundConverters["whatsapp"] != r {
		t.Error("expected receptor to be registered")
	}
}

// --- RegisterEventConfiguration / GetEventConfiguration ---

func TestWeave_RegisterAndGetEventConfiguration(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("test-event")
	e.RegisterEventConfiguration(link)
	got := e.GetEventConfiguration("test-event")
	if got == nil {
		t.Fatal("expected link, got nil")
	}
	if got.EventType != "test-event" {
		t.Errorf("expected test-event, got %s", got.EventType)
	}
}

func TestWeave_GetEventConfiguration_NotFound_ReturnsNil(t *testing.T) {
	e := minimalWeave()
	if e.GetEventConfiguration("missing") != nil {
		t.Error("expected nil for missing config")
	}
}

// --- RegisterAction ---

func TestWeave_RegisterAction_NoExecutor_ReturnsError(t *testing.T) {
	e := minimalWeave()
	err := e.RegisterAction(&stubAction{name: "pay"})
	if err == nil {
		t.Fatal("expected error when no action executor")
	}
}

func TestWeave_RegisterAction_WithExecutor_Success(t *testing.T) {
	e := minimalWeave()
	reg := newRegistry()
	e.actionExecutor = NewActionExecutor(reg, false, testLogger(t), noopTracer())
	if err := e.RegisterAction(&stubAction{name: "pay"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- IngestLore ---

func TestWeave_IngestLore_NotConfigured_ReturnsError(t *testing.T) {
	e := minimalWeave()
	err := e.IngestLore(context.Background(), entities.LoreIngestion{LoreID: "lore-1", Text: "hello"})
	if err == nil {
		t.Fatal("expected error when lore not configured")
	}
}

// --- DeleteLore ---

func TestWeave_DeleteLore_NotConfigured_ReturnsError(t *testing.T) {
	e := minimalWeave()
	err := e.DeleteLore(context.Background(), "lore-1")
	if err == nil {
		t.Fatal("expected error when lore store not configured")
	}
}

// --- ConvertEventByType ---

func TestWeave_ConvertEventByType_EventNotFound(t *testing.T) {
	e := minimalWeave()
	_, err := e.ConvertEventByType(context.Background(), "unknown", nil)
	if err == nil {
		t.Fatal("expected error for missing event config")
	}
}

func TestWeave_ConvertEventByType_NoConverter(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat")
	link.InboundConverterName = ""
	e.RegisterEventConfiguration(link)
	_, err := e.ConvertEventByType(context.Background(), "chat", nil)
	if err == nil {
		t.Fatal("expected error for no converter")
	}
}

func TestWeave_ConvertEventByType_ConverterNotFound(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat")
	link.InboundConverterName = "missing_converter"
	e.RegisterEventConfiguration(link)
	_, err := e.ConvertEventByType(context.Background(), "chat", nil)
	if err == nil {
		t.Fatal("expected error for missing converter")
	}
}

func TestWeave_ConvertEventByType_Success(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat")
	link.InboundConverterName = "api"
	e.RegisterEventConfiguration(link)
	e.RegisterReceptor("api", &stubReceptor{})
	pulses, err := e.ConvertEventByType(context.Background(), "chat", map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = pulses
}

func TestWeave_RegisterReceptor_ConcurrentAccess(t *testing.T) {
	e := minimalWeave()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		n := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			e.RegisterReceptor(fmt.Sprintf("receptor-%d", n), &stubReceptor{})
		}()
		go func() {
			defer wg.Done()
			_ = e.GetReceptorNames()
		}()
	}
	wg.Wait()
}

// --- stub receptor ---

type stubReceptor struct{}

func (r *stubReceptor) GetName() string { return "stub" }
func (r *stubReceptor) Convert(_ context.Context, _ string, _ map[string]any) ([]*entities.Pulse, error) {
	return []*entities.Pulse{{UserMessage: "hello"}}, nil
}

// --- GetPathfinderRegistry / GetVoiceRegistry / GetLogicRouterRegistry ---

func TestWeave_GetPathfinderRegistry_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetPathfinderRegistry() != nil {
		t.Error("expected nil registry")
	}
}

func TestWeave_GetPathfinderRegistry_Set(t *testing.T) {
	e := minimalWeave()
	reg := &stubBuilderPathfinderRegistry{}
	e.pathfinderRegistry = reg
	if e.GetPathfinderRegistry() != reg {
		t.Error("expected registry to be returned")
	}
}

func TestWeave_GetVoiceRegistry_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetVoiceRegistry() != nil {
		t.Error("expected nil registry")
	}
}

func TestWeave_GetVoiceRegistry_Set(t *testing.T) {
	e := minimalWeave()
	reg := &stubBuilderVoiceRegistry{}
	e.voiceRegistry = reg
	if e.GetVoiceRegistry() != reg {
		t.Error("expected registry to be returned")
	}
}

func TestWeave_GetLogicRouterRegistry_Nil(t *testing.T) {
	e := minimalWeave()
	if e.GetLogicRouterRegistry() != nil {
		t.Error("expected nil registry")
	}
}

func TestWeave_GetLogicRouterRegistry_Set(t *testing.T) {
	e := minimalWeave()
	reg := &stubBuilderLogicRouterRegistry{}
	e.logicRouterRegistry = reg
	if e.GetLogicRouterRegistry() != reg {
		t.Error("expected registry to be returned")
	}
}

// --- wireOrchestration ---

func TestWeave_WireOrchestration_NoReasoningService_NoOp(t *testing.T) {
	e := minimalWeave()
	e.wireOrchestration() // no-op, should not panic
}

func TestWeave_WireOrchestration_WithReasoningService(t *testing.T) {
	e := minimalWeave()
	rs := NewReasoningService(nil, nil, nil, 5, "", 10, testLogger(t), noopTracer())
	e.reasoningService = rs
	e.wireOrchestration() // wires summon service
}

// --- statusFromError ---

func TestStatusFromError_PlainError(t *testing.T) {
	err := fmt.Errorf("generic error")
	if got := statusFromError(err); got != "error" {
		t.Errorf("expected 'error', got %q", got)
	}
}

func TestStatusFromError_RateLimitExceeded(t *testing.T) {
	err := ErrRateLimitExceeded("user:1")
	if got := statusFromError(err); got != "rate_limited" {
		t.Errorf("expected 'rate_limited', got %q", got)
	}
}

func TestStatusFromError_ValidationFailed(t *testing.T) {
	err := ErrValidationFailed("field", "bad input")
	if got := statusFromError(err); got != "validation_failed" {
		t.Errorf("expected 'validation_failed', got %q", got)
	}
}

func TestStatusFromError_MemoryBusy(t *testing.T) {
	err := ErrMemoryBusy("user:1")
	if got := statusFromError(err); got != "session_busy" {
		t.Errorf("expected 'session_busy', got %q", got)
	}
}

func TestStatusFromError_LockAcquisitionFailed(t *testing.T) {
	err := ErrLockAcquisitionFailed("user:1", nil)
	if got := statusFromError(err); got != "lock_acquisition_failed" {
		t.Errorf("expected 'lock_acquisition_failed', got %q", got)
	}
}

func TestStatusFromError_ReasoningFailed(t *testing.T) {
	err := ErrReasoningFailed(fmt.Errorf("llm timeout"))
	if got := statusFromError(err); got != "reasoning_failed" {
		t.Errorf("expected 'reasoning_failed', got %q", got)
	}
}

func TestStatusFromError_UnknownCode(t *testing.T) {
	err := &OrchestrationError{Code: "UNKNOWN_CODE", Message: "test"}
	if got := statusFromError(err); got != "error" {
		t.Errorf("expected 'error', got %q", got)
	}
}

// --- buildAttachmentLogs ---

func TestBuildAttachmentLogs_Empty(t *testing.T) {
	if got := buildAttachmentLogs(nil); got != nil {
		t.Error("expected nil for empty input")
	}
}

func TestBuildAttachmentLogs_WithArtifacts(t *testing.T) {
	artifacts := []*entities.Artifact{
		{Type: "image", URL: "http://example.com/img.jpg", MimeType: "image/jpeg", MediaID: "m1"},
		{Type: "audio", Data: []byte{1, 2, 3}},
	}
	logs := buildAttachmentLogs(artifacts)
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].StorageURL != "http://example.com/img.jpg" {
		t.Errorf("expected URL, got %q", logs[0].StorageURL)
	}
	if logs[0].Downloaded {
		t.Error("expected Downloaded=false when no data")
	}
	if !logs[1].Downloaded {
		t.Error("expected Downloaded=true when data present")
	}
}

// --- statusFromError remaining cases ---

func TestStatusFromError_SessionHeld(t *testing.T) {
	err := &OrchestrationError{Code: "SESSION_HELD", Message: "held"}
	if got := statusFromError(err); got != "session_held" {
		t.Errorf("expected 'session_held', got %q", got)
	}
}

func TestStatusFromError_EnrichmentFailed(t *testing.T) {
	err := &OrchestrationError{Code: "ENRICHMENT_FAILED", Message: "scout fail"}
	if got := statusFromError(err); got != "enrichment_failed" {
		t.Errorf("expected 'enrichment_failed', got %q", got)
	}
}

func TestStatusFromError_SpiritNotFound(t *testing.T) {
	err := ErrSpiritNotFound("bot", nil)
	if got := statusFromError(err); got != "spirit_not_found" {
		t.Errorf("expected 'spirit_not_found', got %q", got)
	}
}

func TestStatusFromError_SessionOperationFailed(t *testing.T) {
	err := &OrchestrationError{Code: "SESSION_OPERATION_FAILED", Message: "test"}
	if got := statusFromError(err); got != "session_operation_failed" {
		t.Errorf("expected 'session_operation_failed', got %q", got)
	}
}

func TestStatusFromError_PersistenceFailed(t *testing.T) {
	err := ErrPersistenceFailed("save", nil)
	if got := statusFromError(err); got != "persistence_failed" {
		t.Errorf("expected 'persistence_failed', got %q", got)
	}
}

func TestStatusFromError_TimeoutExceeded(t *testing.T) {
	err := ErrTimeoutExceeded("reasoning")
	if got := statusFromError(err); got != "timeout_exceeded" {
		t.Errorf("expected 'timeout_exceeded', got %q", got)
	}
}

func TestStatusFromError_ToolBudgetExceeded(t *testing.T) {
	err := ErrToolBudgetExceeded(5)
	if got := statusFromError(err); got != "tool_budget_exceeded" {
		t.Errorf("expected 'tool_budget_exceeded', got %q", got)
	}
}

func TestStatusFromError_FilterBlocked(t *testing.T) {
	err := ErrFilterBlocked("field", "val")
	if got := statusFromError(err); got != "filter_blocked" {
		t.Errorf("expected 'filter_blocked', got %q", got)
	}
}

func TestStatusFromError_FilterNotAllowed(t *testing.T) {
	err := ErrFilterNotAllowed("field", "val")
	if got := statusFromError(err); got != "filter_not_allowed" {
		t.Errorf("expected 'filter_not_allowed', got %q", got)
	}
}

func TestStatusFromError_PromptInjection(t *testing.T) {
	err := ErrPromptInjectionDetected()
	if got := statusFromError(err); got != "validation_failed" {
		t.Errorf("expected 'validation_failed', got %q", got)
	}
}

// --- DeleteLore success path ---

func TestWeave_DeleteLore_Success(t *testing.T) {
	e := minimalWeave()
	e.loreStore = &stubBuilderLoreStore{}
	if err := e.DeleteLore(context.Background(), "lore-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- IngestLore branches ---

func TestWeave_IngestLore_LoreNotFound(t *testing.T) {
	e := minimalWeave()
	e.loreRepository = &stubBuilderLoreRepo{}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubBuilderLoreEmbedder{}
	// stubBuilderLoreRepo.GetByID returns empty Lore, not error — set error variant
	e.loreRepository = &stubErrLoreRepo{err: fmt.Errorf("not found")}
	err := e.IngestLore(context.Background(), entities.LoreIngestion{LoreID: "x", Text: "hello"})
	if err == nil {
		t.Fatal("expected error for lore not found")
	}
}

func TestWeave_IngestLore_NoTextOrFile(t *testing.T) {
	e := minimalWeave()
	e.loreRepository = &stubBuilderLoreRepo{}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubBuilderLoreEmbedder{}
	err := e.IngestLore(context.Background(), entities.LoreIngestion{LoreID: "x"})
	if err == nil {
		t.Fatal("expected error for no text and no filepath")
	}
}

func TestWeave_IngestLore_EmbedFails(t *testing.T) {
	e := minimalWeave()
	e.loreRepository = &stubBuilderLoreRepo{}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubErrLoreEmbedder{err: fmt.Errorf("embed fail")}
	err := e.IngestLore(context.Background(), entities.LoreIngestion{LoreID: "x", Text: "hello world"})
	if err == nil {
		t.Fatal("expected error from embedder")
	}
}

func TestWeave_IngestLore_Success(t *testing.T) {
	e := minimalWeave()
	e.loreRepository = &stubBuilderLoreRepo{}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubSuccessLoreEmbedder{}
	err := e.IngestLore(context.Background(), entities.LoreIngestion{LoreID: "x", Text: "hello world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- helper stubs ---

type stubErrLoreRepo struct{ err error }

func (s *stubErrLoreRepo) Create(_ context.Context, _ entities.Lore) error { return nil }
func (s *stubErrLoreRepo) GetByID(_ context.Context, _ string) (entities.Lore, error) {
	return entities.Lore{}, s.err
}
func (s *stubErrLoreRepo) GetByName(_ context.Context, _ string) (entities.Lore, error) {
	return entities.Lore{}, s.err
}
func (s *stubErrLoreRepo) GetBySpiritID(_ context.Context, _ string) ([]entities.Lore, error) {
	return nil, s.err
}
func (s *stubErrLoreRepo) GetByIDs(_ context.Context, _ []string) ([]entities.Lore, error) {
	return nil, s.err
}
func (s *stubErrLoreRepo) List(_ context.Context) ([]entities.Lore, error) {
	return nil, nil
}

func (s *stubErrLoreRepo) Update(_ context.Context, _ entities.Lore) error { return s.err }
func (s *stubErrLoreRepo) Delete(_ context.Context, _ string) error        { return s.err }

type stubErrLoreEmbedder struct{ err error }

func (s *stubErrLoreEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, s.err
}
func (s *stubErrLoreEmbedder) Dimensions() int { return 0 }

type stubSuccessLoreEmbedder struct{}

func (s *stubSuccessLoreEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0.1, 0.2}
	}
	return result, nil
}
func (s *stubSuccessLoreEmbedder) Dimensions() int { return 2 }

// stubLoreRepoWithData returns a Lore with configurable fields.
type stubLoreRepoWithData struct {
	lore entities.Lore
}

func (s *stubLoreRepoWithData) Create(_ context.Context, _ entities.Lore) error { return nil }
func (s *stubLoreRepoWithData) GetByID(_ context.Context, _ string) (entities.Lore, error) {
	return s.lore, nil
}
func (s *stubLoreRepoWithData) GetByName(_ context.Context, _ string) (entities.Lore, error) {
	return s.lore, nil
}
func (s *stubLoreRepoWithData) GetBySpiritID(_ context.Context, _ string) ([]entities.Lore, error) {
	return nil, nil
}
func (s *stubLoreRepoWithData) GetByIDs(_ context.Context, _ []string) ([]entities.Lore, error) {
	return nil, nil
}
func (s *stubLoreRepoWithData) List(_ context.Context) ([]entities.Lore, error) {
	return nil, nil
}

func (s *stubLoreRepoWithData) Update(_ context.Context, _ entities.Lore) error { return nil }
func (s *stubLoreRepoWithData) Delete(_ context.Context, _ string) error        { return nil }

func TestWeave_IngestLore_FilePath_ReadError(t *testing.T) {
	e := minimalWeave()
	e.loreRepository = &stubBuilderLoreRepo{}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubSuccessLoreEmbedder{}
	err := e.IngestLore(context.Background(), entities.LoreIngestion{
		LoreID:   "x",
		FilePath: "/nonexistent/path/to/file.txt",
	})
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
}

func TestWeave_IngestLore_FilePath_Success(t *testing.T) {
	// Write a temp file and ingest via FilePath.
	f, err := os.CreateTemp("", "lore-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString("hello from file")
	f.Close()

	e := minimalWeave()
	e.loreRepository = &stubBuilderLoreRepo{}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubSuccessLoreEmbedder{}
	err = e.IngestLore(context.Background(), entities.LoreIngestion{
		LoreID:   "x",
		FilePath: f.Name(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWeave_IngestLore_NegativeOverlap_Clamped(t *testing.T) {
	e := minimalWeave()
	e.loreRepository = &stubLoreRepoWithData{lore: entities.Lore{Overlap: -5, ChunkSize: 100}}
	e.loreStore = &stubBuilderLoreStore{}
	e.loreEmbedder = &stubSuccessLoreEmbedder{}
	// negative overlap should not cause panic; clamped to 0
	err := e.IngestLore(context.Background(), entities.LoreIngestion{LoreID: "x", Text: "some text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ConvertEventByType additional branches ---

type stubErrReceptor struct{}

func (r *stubErrReceptor) GetName() string { return "err-receptor" }
func (r *stubErrReceptor) Convert(_ context.Context, _ string, _ map[string]any) ([]*entities.Pulse, error) {
	return nil, fmt.Errorf("convert failed")
}

func TestWeave_ConvertEventByType_ConvertError(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat").WithInboundConverter("err-receptor")
	e.configCache.RegisterLink(link)
	e.inboundConverters["err-receptor"] = &stubErrReceptor{}

	_, err := e.ConvertEventByType(context.Background(), "chat", nil)
	if err == nil {
		t.Fatal("expected error from failing converter")
	}
}

// --- selectSpirit ---

type stubPathfinderForSelect struct {
	result string
}

func (s *stubPathfinderForSelect) GetName() string { return "test-pathfinder" }
func (s *stubPathfinderForSelect) SelectSpirit(_ context.Context, _ *entities.Pulse, _ []string) string {
	return s.result
}

type stubPathfinderRegistryForSelect struct {
	pfs map[string]ports.Pathfinder
}

func (r *stubPathfinderRegistryForSelect) Register(_ ports.Pathfinder) error            { return nil }
func (r *stubPathfinderRegistryForSelect) RegisterMultiple(_ ...ports.Pathfinder) error { return nil }
func (r *stubPathfinderRegistryForSelect) Get(name string) ports.Pathfinder {
	return r.pfs[name]
}
func (r *stubPathfinderRegistryForSelect) List() []string { return nil }

func TestSelectSpirit_SingleSpirit(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat").WithSpirits("bot-a")
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-a" {
		t.Errorf("expected bot-a, got %s", got)
	}
}

func TestSelectSpirit_NoSpirits_DefaultSet(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat").WithDefaultSpirit("default-bot")
	link.AllowedSpirits = nil
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "default-bot" {
		t.Errorf("expected default-bot, got %s", got)
	}
}

func TestSelectSpirit_NoSpirits_NoDefault_ReturnsDefault(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat")
	link.AllowedSpirits = nil
	link.DefaultSpirit = ""
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "DEFAULT" {
		t.Errorf("expected DEFAULT, got %s", got)
	}
}

func TestSelectSpirit_MultipleSpirits_NoPathfinder_UsesDefault(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b").WithDefaultSpirit("bot-a")
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-a" {
		t.Errorf("expected bot-a, got %s", got)
	}
}

func TestSelectSpirit_MultipleSpirits_NoPathfinder_NoDefault_UsesFirst(t *testing.T) {
	e := minimalWeave()
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b")
	link.DefaultSpirit = ""
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-a" {
		t.Errorf("expected bot-a, got %s", got)
	}
}

func TestSelectSpirit_NamedPathfinder_Found(t *testing.T) {
	e := minimalWeave()
	pf := &stubPathfinderForSelect{result: "bot-b"}
	e.pathfinderRegistry = &stubPathfinderRegistryForSelect{
		pfs: map[string]ports.Pathfinder{"named-pf": pf},
	}
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b")
	link.PathfinderName = "named-pf"
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-b" {
		t.Errorf("expected bot-b, got %s", got)
	}
}

func TestSelectSpirit_NamedPathfinder_NotInRegistry_FallsBackToDefault(t *testing.T) {
	e := minimalWeave()
	e.pathfinderRegistry = &stubPathfinderRegistryForSelect{pfs: map[string]ports.Pathfinder{}}
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b").WithDefaultSpirit("bot-a")
	link.PathfinderName = "missing-pf"
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-a" {
		t.Errorf("expected fallback to bot-a, got %s", got)
	}
}

func TestSelectSpirit_DefaultLLMPathfinder_Used(t *testing.T) {
	e := minimalWeave()
	defaultPF := &stubPathfinderForSelect{result: "bot-b"}
	e.pathfinderRegistry = &stubPathfinderRegistryForSelect{
		pfs: map[string]ports.Pathfinder{
			pathfinders.DefaultPathfinderName: defaultPF,
		},
	}
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b")
	link.PathfinderName = "" // no named pathfinder
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-b" {
		t.Errorf("expected bot-b from default pathfinder, got %s", got)
	}
}

func TestSelectSpirit_Pathfinder_ReturnsEmpty_FallsBackToDefault(t *testing.T) {
	e := minimalWeave()
	emptyPF := &stubPathfinderForSelect{result: ""}
	e.pathfinderRegistry = &stubPathfinderRegistryForSelect{
		pfs: map[string]ports.Pathfinder{
			pathfinders.DefaultPathfinderName: emptyPF,
		},
	}
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b").WithDefaultSpirit("bot-a")
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-a" {
		t.Errorf("expected bot-a as fallback, got %s", got)
	}
}

func TestSelectSpirit_Pathfinder_ReturnsEmpty_NoDefault_UsesFirst(t *testing.T) {
	e := minimalWeave()
	emptyPF := &stubPathfinderForSelect{result: ""}
	e.pathfinderRegistry = &stubPathfinderRegistryForSelect{
		pfs: map[string]ports.Pathfinder{
			pathfinders.DefaultPathfinderName: emptyPF,
		},
	}
	link := entities.NewLink("chat").WithSpirits("bot-a", "bot-b")
	link.DefaultSpirit = ""
	pulse := &entities.Pulse{MemoryKey: "user:1"}

	got := e.selectSpirit(context.Background(), pulse, link)
	if got != "bot-a" {
		t.Errorf("expected bot-a (first) as fallback, got %s", got)
	}
}

// --- logInteraction ---

type stubChronicleRepoForLog struct {
	logged *entities.Chronicle
	err    error
}

func (r *stubChronicleRepoForLog) LogInteraction(_ context.Context, c *entities.Chronicle) error {
	r.logged = c
	return r.err
}
func (r *stubChronicleRepoForLog) FindByMemoryKey(_ context.Context, _ string) ([]*entities.Chronicle, error) {
	return nil, nil
}
func (r *stubChronicleRepoForLog) FindByEventKey(_ context.Context, _ string) ([]*entities.Chronicle, error) {
	return nil, nil
}
func (r *stubChronicleRepoForLog) FindByTimeRange(_ context.Context, _, _ time.Time) ([]*entities.Chronicle, error) {
	return nil, nil
}

func TestLogInteraction_Success(t *testing.T) {
	e := minimalWeave()
	repo := &stubChronicleRepoForLog{}
	e.interactionLogRepo = repo

	state := &ProcessingState{
		Event:     &entities.Pulse{MemoryKey: "user:1", ID: "evt-1"},
		EventKey:  "chat",
		StartTime: time.Now(),
	}
	err := e.logInteraction(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.logged == nil {
		t.Fatal("expected a chronicle to be logged")
	}
}

func TestLogInteraction_WithSpirit_StatusFromFinalError(t *testing.T) {
	e := minimalWeave()
	repo := &stubChronicleRepoForLog{}
	e.interactionLogRepo = repo

	state := &ProcessingState{
		Event:      &entities.Pulse{MemoryKey: "user:1", ID: "evt-1"},
		EventKey:   "chat",
		StartTime:  time.Now(),
		FinalError: "something went wrong",
		Spirit: &entities.Spirit{
			ID: "spirit-1", Name: "bot",
		},
		Session: &entities.Memory{SubjectKey: "topic-1"},
		ReasoningResult: &ReasoningResult{
			IterationsUsed: 2,
		},
		EventConfig: entities.NewLink("chat").WithPathfinder("pf1").WithVoice("sms"),
	}
	err := e.logInteraction(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogInteraction_RepoError(t *testing.T) {
	e := minimalWeave()
	repo := &stubChronicleRepoForLog{err: fmt.Errorf("db down")}
	e.interactionLogRepo = repo

	state := &ProcessingState{
		Event:     &entities.Pulse{MemoryKey: "user:1"},
		EventKey:  "chat",
		StartTime: time.Now(),
	}
	err := e.logInteraction(context.Background(), state)
	if err == nil {
		t.Fatal("expected error from repo")
	}
}

// --- newWeaveWithConfig ---

func TestNewWeaveWithConfig_NilConfig_UsesDefault(t *testing.T) {
	w, err := newWeaveWithConfig(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, // config = nil → DefaultWeaveConfig()
		testLogger(t), noopTracer(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Weave")
	}
}

func TestNewWeaveWithConfig_InvalidConfig_ReturnsError(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.MaxReasoningIterations = -1 // triggers Validate() failure

	_, err := newWeaveWithConfig(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		cfg,
		testLogger(t), noopTracer(),
	)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

// --- ProcessEventByKey / ProcessMultipleEventsByKey ---

func TestProcessEventByKey_EventNotFound(t *testing.T) {
	e := minimalWeave()
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1"}
	resp, err := e.ProcessEventByKey(context.Background(), "unknown", pulse)
	if err == nil {
		t.Fatal("expected error for missing event config")
	}
	if resp == nil {
		t.Fatal("expected error response to be non-nil")
	}
}

func TestProcessMultipleEventsByKey_EventNotFound(t *testing.T) {
	e := minimalWeave()
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1"}
	_, err := e.ProcessMultipleEventsByKey(context.Background(), "unknown", []*entities.Pulse{pulse})
	if err == nil {
		t.Fatal("expected error for missing event config")
	}
}

// newPipelineWeave builds a real Weave that runs the pipeline but fails at SpiritLoad.
// Useful for testing processEventWithConfig error/success paths without LLM.
func newPipelineWeave(t *testing.T) *Weave {
	t.Helper()
	e, err := newWeaveWithConfig(
		nil, // scoutRegistry
		nil, // pathfinderRegistry
		nil, // voiceRegistry
		&stubSpiritRepo{err: errors.New("spirit not found")},
		nil, // sessionRepo
		nil, // messageRepo
		&stubBuilderChronicleRepo{},
		&stubOracleFactory{},
		newRegistry(),
		&stubBond{acquired: true},
		nil, // rateLimiter
		nil, // messageInbox
		nil, // asyncDispatcher
		nil, // config — uses DefaultWeaveConfig
		testLogger(t),
		noopTracer(),
	)
	if err != nil {
		t.Fatalf("unexpected newWeaveWithConfig error: %v", err)
	}
	e.configCache.RegisterLink(entities.NewLink("test_event"))
	return e
}

// TestProcessEventByKey_PipelineError exercises processEventWithConfig's error path:
// lock acquired → pipeline fails at SpiritLoad → error response returned.
func TestProcessEventByKey_PipelineError_ReturnsErrorResponse(t *testing.T) {
	e := newPipelineWeave(t)
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1", UserMessage: "hello"}
	resp, pipelineErr := e.ProcessEventByKey(context.Background(), "test_event", pulse)
	if pipelineErr == nil {
		t.Fatal("expected pipeline error from SpiritLoad failure")
	}
	if resp == nil {
		t.Fatal("expected non-nil error response")
	}
}

// TestProcessEventByKey_DuplicateEvent_Dropped proves a redelivered Pulse (same IdempotencyKey)
// is dropped at the IdempotencyCheck step — before SpiritLoad, which would otherwise fail.
func TestProcessEventByKey_DuplicateEvent_Dropped(t *testing.T) {
	e := newPipelineWeave(t)
	e.idempotencyStore = NewInMemoryIdempotencyStore()
	e.pipeline = e.buildProcessingPipeline()

	first := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1", IdempotencyKey: "wamid.ABC", UserMessage: "hi"}
	if _, firstErr := e.ProcessEventByKey(context.Background(), "test_event", first); firstErr == nil {
		t.Fatal("expected first delivery to fail downstream at SpiritLoad")
	}

	redelivery := &entities.Pulse{MemoryKey: "user:1", ID: "evt-2", IdempotencyKey: "wamid.ABC", UserMessage: "hi"}
	resp, err := e.ProcessEventByKey(context.Background(), "test_event", redelivery)
	if err != nil {
		t.Fatalf("duplicate must be an idempotent no-op (nil error), got %v", err)
	}
	if resp.Status != entities.ResponseDuplicate {
		t.Errorf("expected ResponseDuplicate, got %s", resp.Status)
	}
}

// TestProcessMultipleEventsByKey_PipelineError exercises the goroutine fan-out path:
// each event fails at SpiritLoad, results collected via WaitGroup.
func TestProcessMultipleEventsByKey_PipelineError_ReturnsResults(t *testing.T) {
	e := newPipelineWeave(t)
	pulses := []*entities.Pulse{
		{MemoryKey: "user:1", ID: "evt-1", UserMessage: "hello"},
		{MemoryKey: "user:2", ID: "evt-2", UserMessage: "world"},
	}
	results, err := e.ProcessMultipleEventsByKey(context.Background(), "test_event", pulses)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Errorf("result[%d] is nil", i)
		}
	}
}

// TestProcessEventByKey_LockReleaseError exercises the "failed to release lock in cleanup" log branch.
func TestProcessEventByKey_LockReleaseError_StillReturnsError(t *testing.T) {
	e, err := newWeaveWithConfig(
		nil, nil, nil,
		&stubSpiritRepo{err: errors.New("spirit not found")},
		nil, nil,
		&stubBuilderChronicleRepo{},
		&stubOracleFactory{},
		newRegistry(),
		&stubBond{acquired: true, releaseErr: errors.New("lock server down")},
		nil, nil, nil, nil,
		testLogger(t), noopTracer(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e.configCache.RegisterLink(entities.NewLink("test_event"))

	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1", UserMessage: "hi"}
	resp, pipelineErr := e.ProcessEventByKey(context.Background(), "test_event", pulse)
	if pipelineErr == nil {
		t.Fatal("expected pipeline error")
	}
	if resp == nil {
		t.Fatal("expected non-nil error response")
	}
}

// TestLogInteraction_WithScouts covers the scoutsUsed branch (RequireScouts non-empty).
func TestLogInteraction_WithScouts(t *testing.T) {
	chronicleRepo := &stubBuilderChronicleRepo{}
	e, err := newWeaveWithConfig(
		nil, nil, nil, nil, nil, nil,
		chronicleRepo,
		&stubOracleFactory{},
		newRegistry(),
		&stubBond{acquired: true},
		nil, nil, nil, nil,
		testLogger(t), noopTracer(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	link := entities.NewLink("test_event")
	link.RequireScouts = []string{"scout-a"}

	state := &ProcessingState{
		Event:            &entities.Pulse{MemoryKey: "user:1", ID: "evt-2"},
		EventKey:         "test_event",
		EventConfig:      link,
		ProcessingStatus: "success",
	}

	if err := e.logInteraction(context.Background(), state); err != nil {
		t.Errorf("unexpected error from logInteraction: %v", err)
	}
}

// stubSuccessStep satisfies ProcessingStep; populates the state so processEventWithConfig
// succeeds without needing a real LLM or DB.
type stubSuccessStep struct{}

func (s *stubSuccessStep) Name() string           { return "stub_success" }
func (s *stubSuccessStep) Timeout() time.Duration { return 0 }
func (s *stubSuccessStep) Execute(_ context.Context, state *ProcessingState) error {
	state.Spirit = &entities.Spirit{Name: "test-spirit"}
	state.ReasoningResult = &ReasoningResult{
		IterationsUsed: 1,
		Iterations: []entities.IterationLog{
			{ActionCalls: []entities.ActionCallLog{{ActionName: "greet"}}},
		},
	}
	state.Response = "All good"
	return nil
}

func newSuccessWeave(t *testing.T) *Weave {
	t.Helper()
	e := newPipelineWeave(t)
	// Replace the real pipeline with a single stub step that always succeeds.
	p := NewPipeline(testLogger(t), noopTracer())
	p.AddStep(&stubSuccessStep{})
	e.pipeline = p
	return e
}

// TestProcessEventByKey_Success exercises processEventWithConfig's success path (lines 444-476).
func TestProcessEventByKey_Success_ReturnsResponse(t *testing.T) {
	e := newSuccessWeave(t)
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-ok", UserMessage: "hello"}
	resp, err := e.ProcessEventByKey(context.Background(), "test_event", pulse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.FinalResponse != "All good" {
		t.Errorf("expected 'All good', got %q", resp.FinalResponse)
	}
}

// TestProcessMultipleEventsByKey_Success exercises the goroutine fan-out success path (line 388).
func TestProcessMultipleEventsByKey_Success_ReturnsResponses(t *testing.T) {
	e := newSuccessWeave(t)
	pulses := []*entities.Pulse{
		{MemoryKey: "user:1", ID: "evt-1", UserMessage: "hello"},
		{MemoryKey: "user:2", ID: "evt-2", UserMessage: "world"},
	}
	results, err := e.ProcessMultipleEventsByKey(context.Background(), "test_event", pulses)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Errorf("result[%d] is nil", i)
		}
		if r.FinalResponse != "All good" {
			t.Errorf("result[%d].FinalResponse = %q, want 'All good'", i, r.FinalResponse)
		}
	}
}
