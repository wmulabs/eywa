package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs for imprint extraction ---

type stubImprintRepo struct {
	imprints   []entities.Imprint
	getErr     error
	saveErr    error
	saveCalled int
	pruned     bool
}

var _ ports.ImprintRepository = (*stubImprintRepo)(nil)

func (r *stubImprintRepo) Save(_ context.Context, _ entities.Imprint) error {
	r.saveCalled++
	return r.saveErr
}
func (r *stubImprintRepo) GetByUserKey(_ context.Context, _, _ string) ([]entities.Imprint, error) {
	return r.imprints, r.getErr
}
func (r *stubImprintRepo) List(_ context.Context, _ ports.ImprintListOptions) ([]entities.Imprint, int64, error) {
	panic("not implemented")
}
func (r *stubImprintRepo) Delete(_ context.Context, _ string) error { panic("not implemented") }
func (r *stubImprintRepo) Prune(_ context.Context, _, _ string, _ int) error {
	r.pruned = true
	return nil
}

type stubOracle struct {
	resp   *ports.OracleResponse
	err    error
	gotReq *ports.OracleRequest // captures the last request for assertions
}

var _ ports.Oracle = (*stubOracle)(nil)

func (o *stubOracle) GetName() string                 { return "stub" }
func (o *stubOracle) GetAvailableModels() []string    { return nil }
func (o *stubOracle) IsAvailable() bool               { return true }
func (o *stubOracle) GetConfig() map[string]any       { return nil }
func (o *stubOracle) SupportsImages(_ string) bool    { return false }
func (o *stubOracle) SupportsAudio(_ string) bool     { return false }
func (o *stubOracle) SupportsDocuments(_ string) bool { return false }
func (o *stubOracle) GenerateResponse(_ context.Context, req *ports.OracleRequest) (*ports.OracleResponse, error) {
	o.gotReq = req
	return o.resp, o.err
}

type stubOracleFactory struct {
	oracle ports.Oracle
	err    error
}

var _ ports.OracleFactory = (*stubOracleFactory)(nil)

func (f *stubOracleFactory) RegisterProvider(_ string, _ ports.Oracle) error { return nil }
func (f *stubOracleFactory) GetProvider(_ string) (ports.Oracle, error) {
	return f.oracle, f.err
}
func (f *stubOracleFactory) GetDefaultProvider() (ports.Oracle, error) { return f.oracle, f.err }
func (f *stubOracleFactory) SetDefaultProvider(_ string) error         { return nil }
func (f *stubOracleFactory) ListProviders() []string                   { return nil }
func (f *stubOracleFactory) ListAvailableProviders() []string          { return nil }
func (f *stubOracleFactory) GetProviderForModel(_ string) (ports.Oracle, error) {
	return f.oracle, f.err
}

// --- ImprintExtractionStep.Execute guard tests ---

func TestImprintExtractionStep_Execute_Disabled_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	factory := &stubOracleFactory{}
	cfg := ImprintExtractionConfig{Enabled: false}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: "+1234", UserMessage: "hello"},
		Spirit:   &entities.Spirit{Type: entities.SpiritTypeConversational},
		Response: "response",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImprintExtractionStep_Execute_NilRepo_NoOp(t *testing.T) {
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(nil, &stubOracleFactory{}, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: "+1234", UserMessage: "hello"},
		Spirit:   &entities.Spirit{Type: entities.SpiritTypeConversational},
		Response: "response",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImprintExtractionStep_Execute_NilSpirit_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	factory := &stubOracleFactory{oracle: &stubOracle{}}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: "+1234"},
		Spirit:   nil,
		Response: "response",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImprintExtractionStep_Execute_NotifierSpirit_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	factory := &stubOracleFactory{oracle: &stubOracle{}}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: "+1234"},
		Spirit:   &entities.Spirit{Type: entities.SpiritTypeNotifier},
		Response: "response",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImprintExtractionStep_Execute_EmptyResponse_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	factory := &stubOracleFactory{oracle: &stubOracle{}}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: "+1234"},
		Spirit:   &entities.Spirit{Type: entities.SpiritTypeConversational},
		Response: "",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImprintExtractionStep_Execute_HappyPath_SpawnsGoroutine(t *testing.T) {
	doneCh := make(chan struct{})
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "[preference] Likes tea"}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &signalImprintRepo{doneCh: doneCh}
	cfg := ImprintExtractionConfig{Enabled: true, MaxImprints: 10}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: "+1234", UserMessage: "hello"},
		Spirit:   &entities.Spirit{Type: entities.SpiritTypeConversational, Name: "bot"},
		Response: "I can help with that",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// wait for goroutine to complete (signals after Save)
	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Error("goroutine did not complete in time")
	}
	if repo.saveCalled != 1 {
		t.Errorf("expected 1 save, got %d", repo.saveCalled)
	}
}

// signalImprintRepo signals doneCh after the first Save call.
type signalImprintRepo struct {
	stubImprintRepo
	doneCh chan struct{}
	once   sync.Once
}

func (r *signalImprintRepo) Save(ctx context.Context, imp entities.Imprint) error {
	err := r.stubImprintRepo.Save(ctx, imp)
	r.once.Do(func() { close(r.doneCh) })
	return err
}

// signalOracle wraps another Oracle and signals a channel after GenerateResponse.
type signalOracle struct {
	wrapped ports.Oracle
	done    chan struct{}
}

var _ ports.Oracle = (*signalOracle)(nil)

func (o *signalOracle) GetName() string                 { return o.wrapped.GetName() }
func (o *signalOracle) GetAvailableModels() []string    { return nil }
func (o *signalOracle) IsAvailable() bool               { return true }
func (o *signalOracle) GetConfig() map[string]any       { return nil }
func (o *signalOracle) SupportsImages(_ string) bool    { return false }
func (o *signalOracle) SupportsAudio(_ string) bool     { return false }
func (o *signalOracle) SupportsDocuments(_ string) bool { return false }
func (o *signalOracle) GenerateResponse(ctx context.Context, req *ports.OracleRequest) (*ports.OracleResponse, error) {
	resp, err := o.wrapped.GenerateResponse(ctx, req)
	close(o.done)
	return resp, err
}

func TestImprintExtractionStep_Execute_EmptyContactPhone_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	factory := &stubOracleFactory{oracle: &stubOracle{}}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:    &entities.Pulse{ContactPhone: ""},
		Spirit:   &entities.Spirit{Type: entities.SpiritTypeConversational},
		Response: "hello there",
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ImprintExtractionStep.extract direct tests ---

func TestImprintExtractionStep_Extract_ProviderNotFound_NoOp(t *testing.T) {
	repo := &stubImprintRepo{}
	factory := &stubOracleFactory{err: errors.New("provider not found")}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "User: hi\nAssistant: hello")

	if repo.saveCalled != 0 {
		t.Error("expected no save when provider not found")
	}
}

func TestImprintExtractionStep_Extract_GenerateResponseError_NoOp(t *testing.T) {
	oracle := &stubOracle{err: errors.New("llm error")}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 0 {
		t.Error("expected no save on oracle error")
	}
}

func TestImprintExtractionStep_Extract_EmptyContent_NoOp(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: ""}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 0 {
		t.Error("expected no save when oracle returns empty content")
	}
}

func TestImprintExtractionStep_Extract_SavesNewFacts(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "[preference] Likes coffee\n[personal] Name is Alice"}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 2 {
		t.Errorf("expected 2 saves, got %d", repo.saveCalled)
	}
	if !repo.pruned {
		t.Error("expected Prune called")
	}
}

func TestImprintExtractionStep_Extract_DuplicateFact_Skipped(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "[preference] Likes coffee"}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{
		imprints: []entities.Imprint{{Fact: "Likes coffee"}}, // already exists
	}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 0 {
		t.Errorf("expected no save for duplicate fact, got %d", repo.saveCalled)
	}
}

func TestImprintExtractionStep_Extract_EmptyLinesSkipped(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "\n\n[preference] Likes tea\n\n"}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 1 {
		t.Errorf("expected 1 save (empty lines skipped), got %d", repo.saveCalled)
	}
}

func TestImprintExtractionStep_Extract_EmptyFactAfterParse_Skipped(t *testing.T) {
	// "[preference]" parses to empty fact → skip
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "[preference]"}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 0 {
		t.Errorf("expected no save for empty fact, got %d", repo.saveCalled)
	}
}

func TestParseFact_NoBracketPrefix_ReturnsContext(t *testing.T) {
	category, fact := parseFact("Likes spicy food")
	if category != entities.ImprintCategoryContext {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryContext, category)
	}
	if fact != "Likes spicy food" {
		t.Errorf("expected fact %q, got %q", "Likes spicy food", fact)
	}
}

func TestParseFact_PreferencePrefix(t *testing.T) {
	category, fact := parseFact("[preference] Likes coffee")
	if category != entities.ImprintCategoryPreference {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryPreference, category)
	}
	if fact != "Likes coffee" {
		t.Errorf("expected fact %q, got %q", "Likes coffee", fact)
	}
}

func TestParseFact_PersonalPrefix(t *testing.T) {
	category, fact := parseFact("[personal] Name is Alice")
	if category != entities.ImprintCategoryPersonal {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryPersonal, category)
	}
	if fact != "Name is Alice" {
		t.Errorf("expected fact %q, got %q", "Name is Alice", fact)
	}
}

func TestParseFact_BusinessPrefix(t *testing.T) {
	category, fact := parseFact("[business] Works at Acme")
	if category != entities.ImprintCategoryBusiness {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryBusiness, category)
	}
	if fact != "Works at Acme" {
		t.Errorf("expected fact %q, got %q", "Works at Acme", fact)
	}
}

func TestParseFact_ContextPrefix(t *testing.T) {
	category, fact := parseFact("[context] In Mexico City")
	if category != entities.ImprintCategoryContext {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryContext, category)
	}
	if fact != "In Mexico City" {
		t.Errorf("expected fact %q, got %q", "In Mexico City", fact)
	}
}

func TestParseFact_UnclosedBracket_ReturnsContext(t *testing.T) {
	category, fact := parseFact("[preference Likes coffee")
	if category != entities.ImprintCategoryContext {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryContext, category)
	}
	if fact != "[preference Likes coffee" {
		t.Errorf("expected fact %q, got %q", "[preference Likes coffee", fact)
	}
}

func TestParseFact_EmptyFactAfterBracket(t *testing.T) {
	category, fact := parseFact("[preference]")
	if category != entities.ImprintCategoryPreference {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryPreference, category)
	}
	if fact != "" {
		t.Errorf("expected empty fact, got %q", fact)
	}
}

func TestParseFact_CaseInsensitiveCategory(t *testing.T) {
	category, fact := parseFact("[PREFERENCE] Morning delivery")
	if category != entities.ImprintCategoryPreference {
		t.Errorf("expected category %q, got %q", entities.ImprintCategoryPreference, category)
	}
	if fact != "Morning delivery" {
		t.Errorf("expected fact %q, got %q", "Morning delivery", fact)
	}
}

func TestImprintExtractionStep_Extract_InnerEmptyLine_Skipped(t *testing.T) {
	// Inner empty line between facts: TrimSpace on whole string preserves it.
	// Split gives ["[preference] Likes tea", "", "[preference] Drinks coffee"]
	// The middle empty string hits the `if line == ""` continue branch.
	content := "[preference] Likes tea\n\n[preference] Drinks coffee"
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: content}}
	factory := &stubOracleFactory{oracle: oracle}
	repo := &stubImprintRepo{}
	cfg := ImprintExtractionConfig{Enabled: true}
	step := NewImprintExtractionStep(repo, factory, cfg, time.Second, testLogger(t))

	step.extract(context.Background(), "+1234", "bot", "conversation")

	if repo.saveCalled != 2 {
		t.Errorf("expected 2 saves (inner empty line skipped), got %d", repo.saveCalled)
	}
}
