package pathfinders

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubOracle struct {
	resp *ports.OracleResponse
	err  error
}

var _ ports.Oracle = (*stubOracle)(nil)

func (o *stubOracle) GetName() string                    { return "stub" }
func (o *stubOracle) GetAvailableModels() []string       { return nil }
func (o *stubOracle) IsAvailable() bool                  { return true }
func (o *stubOracle) GetConfig() map[string]any  { return nil }
func (o *stubOracle) SupportsImages(_ string) bool       { return false }
func (o *stubOracle) SupportsAudio(_ string) bool        { return false }
func (o *stubOracle) SupportsDocuments(_ string) bool    { return false }
func (o *stubOracle) GenerateResponse(_ context.Context, _ *ports.OracleRequest) (*ports.OracleResponse, error) {
	return o.resp, o.err
}

type stubOracleFactory struct {
	oracle      ports.Oracle
	getErr      error
	forModelErr error
}

var _ ports.OracleFactory = (*stubOracleFactory)(nil)

func (f *stubOracleFactory) RegisterProvider(_ string, _ ports.Oracle) error { return nil }
func (f *stubOracleFactory) GetProvider(_ string) (ports.Oracle, error) {
	return f.oracle, f.getErr
}
func (f *stubOracleFactory) GetDefaultProvider() (ports.Oracle, error) { return f.oracle, f.getErr }
func (f *stubOracleFactory) SetDefaultProvider(_ string) error          { return nil }
func (f *stubOracleFactory) ListProviders() []string                    { return nil }
func (f *stubOracleFactory) ListAvailableProviders() []string           { return nil }
func (f *stubOracleFactory) GetProviderForModel(_ string) (ports.Oracle, error) {
	return f.oracle, f.forModelErr
}

type stubSpiritRepo struct {
	spirits map[string]*entities.Spirit
	err     error
}

var _ ports.SpiritRepository = (*stubSpiritRepo)(nil)

func (r *stubSpiritRepo) Create(_ context.Context, _ *entities.Spirit) error { return nil }
func (r *stubSpiritRepo) Update(_ context.Context, _ string, _ *entities.Spirit, _ string) (*entities.Spirit, error) {
	return nil, nil
}
func (r *stubSpiritRepo) FindByID(_ context.Context, _ string) (*entities.Spirit, error) {
	return nil, nil
}
func (r *stubSpiritRepo) FindActiveByName(_ context.Context, _ string) (*entities.Spirit, error) {
	return nil, nil
}
func (r *stubSpiritRepo) FindActiveByNames(_ context.Context, _ []string) (map[string]*entities.Spirit, error) {
	return r.spirits, r.err
}
func (r *stubSpiritRepo) GetVersion(_ context.Context, _ string, _ int) (*entities.Spirit, error) {
	return nil, nil
}
func (r *stubSpiritRepo) FindVersionHistory(_ context.Context, _ string) ([]*entities.Spirit, error) {
	return nil, nil
}
func (r *stubSpiritRepo) Activate(_ context.Context, _ string, _ int) error        { return nil }
func (r *stubSpiritRepo) Deactivate(_ context.Context, _ string) error              { return nil }
func (r *stubSpiritRepo) RestoreVersion(_ context.Context, _ string) error          { return nil }
func (r *stubSpiritRepo) ListActive(_ context.Context, _, _ int) ([]*entities.Spirit, error) {
	return nil, nil
}
func (r *stubSpiritRepo) CountActive(_ context.Context) (int64, error) { return 0, nil }
func (r *stubSpiritRepo) ListAll(_ context.Context) ([]*entities.Spirit, error) {
	return nil, nil
}

// --- helpers ---

func testSpirit(provider, model string) *entities.Spirit {
	return &entities.Spirit{
		Name:  "__test_pathfinder",
		ModelConfig: entities.SpiritModel{
			Provider: provider,
			Model:    model,
		},
	}
}

func pulse(userMsg string) *entities.Pulse {
	return &entities.Pulse{
		MemoryKey:   "user:1",
		UserMessage: userMsg,
		Knowledge:   map[string]any{},
		Metadata:    map[string]any{},
	}
}

// --- NewDefaultLLMPathfinder ---

func TestNewDefaultLLMPathfinder_DefaultTemperature(t *testing.T) {
	p := NewDefaultLLMPathfinder("openai", "gpt-4o", 0, &stubOracleFactory{}, &stubSpiritRepo{})
	if p.internalSpirit.ModelConfig.Temperature != 0.2 {
		t.Errorf("expected default temperature=0.2, got %f", p.internalSpirit.ModelConfig.Temperature)
	}
	if p.name != DefaultPathfinderName {
		t.Errorf("expected name=%s, got %s", DefaultPathfinderName, p.name)
	}
}

func TestNewDefaultLLMPathfinder_CustomTemperature(t *testing.T) {
	p := NewDefaultLLMPathfinder("openai", "gpt-4o", 0.8, &stubOracleFactory{}, &stubSpiritRepo{})
	if p.internalSpirit.ModelConfig.Temperature != 0.8 {
		t.Errorf("expected temperature=0.8, got %f", p.internalSpirit.ModelConfig.Temperature)
	}
}

// --- NewLLMPathfinder ---

func TestNewLLMPathfinder_GetName(t *testing.T) {
	p := NewLLMPathfinder("my-pathfinder", testSpirit("openai", "gpt-4o"), &stubOracleFactory{}, &stubSpiritRepo{})
	if p.GetName() != "my-pathfinder" {
		t.Errorf("expected name=my-pathfinder, got %s", p.GetName())
	}
}

// --- SelectSpirit ---

func TestSelectSpirit_NoSpirits_ReturnsEmpty(t *testing.T) {
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), &stubOracleFactory{}, &stubSpiritRepo{})
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSelectSpirit_OneSpirit_SkipsLLM(t *testing.T) {
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), &stubOracleFactory{}, &stubSpiritRepo{})
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support"})
	if result != "support" {
		t.Errorf("expected support, got %q", result)
	}
}

func TestSelectSpirit_FetchDescriptionsFails_ReturnsEmpty(t *testing.T) {
	repo := &stubSpiritRepo{err: errors.New("db down")}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), &stubOracleFactory{}, repo)
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support", "billing"})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSelectSpirit_GetOracleFails_ReturnsEmpty(t *testing.T) {
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{
		getErr:      errors.New("no provider"),
		forModelErr: errors.New("no model"),
	}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support", "billing"})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSelectSpirit_GetOracleProviderFallsBackToModel(t *testing.T) {
	// GetProvider fails, GetProviderForModel succeeds with oracle that returns invalid spirit
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "unknown-spirit"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{
		getErr: errors.New("no provider"),
		oracle: oracle,
		// forModelErr = nil → fallback succeeds
	}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support", "billing"})
	// "unknown-spirit" not in available list → invalid → returns ""
	if result != "" {
		t.Errorf("expected empty for invalid spirit, got %q", result)
	}
}

func TestSelectSpirit_LLMCallFails_ReturnsEmpty(t *testing.T) {
	oracle := &stubOracle{err: errors.New("llm timeout")}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support", "billing"})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSelectSpirit_InvalidSpiritName_ReturnsEmpty(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "does-not-exist"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support", "billing"})
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSelectSpirit_ValidSpiritReturned(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "  support  "}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{
		"support": {Name: "support", Description: "handles support requests"},
		"billing": {Name: "billing", Description: "handles billing"},
	}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("I need help"), []string{"support", "billing"})
	if result != "support" {
		t.Errorf("expected support, got %q", result)
	}
}

func TestSelectSpirit_CaseInsensitiveMatch(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "SUPPORT"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("help"), []string{"support", "billing"})
	// isValidSpirit uses EqualFold, so "SUPPORT" is valid and returned as-is from LLM
	if result != "SUPPORT" {
		t.Errorf("expected SUPPORT (case-insensitive match returns LLM string), got %q", result)
	}
}

func TestSelectSpirit_MissingSpiritInRepo_UsesPlaceholder(t *testing.T) {
	// Repo returns only "billing", "support" is missing → addMissingSpirits creates placeholder
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "support"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{
		"billing": {Name: "billing", Description: "billing"},
	}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("hello"), []string{"support", "billing"})
	if result != "support" {
		t.Errorf("expected support, got %q", result)
	}
}

// --- buildRoutingPrompt (via SelectSpirit) ---

func TestSelectSpirit_WithKnowledgeAndMetadata_BuildsPrompt(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "sales"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	ev := &entities.Pulse{
		MemoryKey:   "user:2",
		UserMessage: "I want to buy",
		Knowledge:   map[string]any{"tier": "gold"},
		Metadata:    map[string]any{"channel": "web"},
	}
	result := p.SelectSpirit(context.Background(), ev, []string{"sales", "support"})
	if result != "sales" {
		t.Errorf("expected sales, got %q", result)
	}
}

func TestSelectSpirit_EmptyUserMessage_BuildsPrompt(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "support"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	ev := &entities.Pulse{
		MemoryKey:   "user:3",
		UserMessage: "", // → "(No message provided)"
		Knowledge:   map[string]any{},
		Metadata:    map[string]any{},
	}
	result := p.SelectSpirit(context.Background(), ev, []string{"support", "billing"})
	if result != "support" {
		t.Errorf("expected support, got %q", result)
	}
}

func TestSelectSpirit_SpiritWithSpecialization_IncludedInPrompt(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "sales"}}
	repo := &stubSpiritRepo{spirits: map[string]*entities.Spirit{
		"sales": {Name: "sales", Description: "sales team", Specialization: "B2B enterprise"},
	}}
	factory := &stubOracleFactory{oracle: oracle}
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), factory, repo)
	result := p.SelectSpirit(context.Background(), pulse("contract"), []string{"sales", "support"})
	if result != "sales" {
		t.Errorf("expected sales, got %q", result)
	}
}

// --- extractSpiritName ---

func TestExtractSpiritName_TrimsSpacesAndQuotes(t *testing.T) {
	p := NewLLMPathfinder("p", testSpirit("openai", "gpt-4o"), &stubOracleFactory{}, &stubSpiritRepo{})
	cases := []struct {
		input    string
		expected string
	}{
		{`"support"`, "support"},
		{`'billing'`, "billing"},
		{"  sales  ", "sales"},
		{`  "support"  `, "support"},
	}
	for _, tc := range cases {
		got := p.extractSpiritName(tc.input)
		if got != tc.expected {
			t.Errorf("extractSpiritName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
