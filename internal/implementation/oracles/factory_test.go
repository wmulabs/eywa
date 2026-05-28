package oracles

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stub oracle ---

type stubOracle struct {
	name      string
	models    []string
	available bool
}

var _ ports.Oracle = (*stubOracle)(nil)

func (o *stubOracle) GetName() string                    { return o.name }
func (o *stubOracle) GetAvailableModels() []string       { return o.models }
func (o *stubOracle) IsAvailable() bool                  { return o.available }
func (o *stubOracle) GetConfig() map[string]any  { return nil }
func (o *stubOracle) SupportsImages(_ string) bool       { return false }
func (o *stubOracle) SupportsAudio(_ string) bool        { return false }
func (o *stubOracle) SupportsDocuments(_ string) bool    { return false }
func (o *stubOracle) GenerateResponse(_ context.Context, _ *ports.OracleRequest) (*ports.OracleResponse, error) {
	return nil, nil
}

// --- tests ---

func TestNewFactory_Empty(t *testing.T) {
	f := NewFactory()
	if f == nil {
		t.Fatal("expected non-nil factory")
	}
}

func TestRegisterProvider_Success(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "openai", available: true}
	if err := f.RegisterProvider("openai", oracle); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterProvider_EmptyName_Error(t *testing.T) {
	f := NewFactory()
	if err := f.RegisterProvider("", &stubOracle{}); err == nil {
		t.Error("expected error for empty provider name")
	}
}

func TestRegisterProvider_NilProvider_Error(t *testing.T) {
	f := NewFactory()
	if err := f.RegisterProvider("openai", nil); err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRegisterProvider_Duplicate_Error(t *testing.T) {
	f := NewFactory()
	f.RegisterProvider("openai", &stubOracle{name: "openai"})
	if err := f.RegisterProvider("openai", &stubOracle{name: "openai2"}); err == nil {
		t.Error("expected error for duplicate provider")
	}
}

func TestRegisterProvider_FirstBecomesDefault(t *testing.T) {
	f := NewFactory()
	f.RegisterProvider("openai", &stubOracle{name: "openai", available: true})
	p, err := f.GetDefaultProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected default=openai, got %s", p.GetName())
	}
}

func TestGetProvider_Found(t *testing.T) {
	f := NewFactory()
	f.RegisterProvider("openai", &stubOracle{name: "openai"})
	p, err := f.GetProvider("openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected openai, got %s", p.GetName())
	}
}

func TestGetProvider_NotFound_Error(t *testing.T) {
	f := NewFactory()
	_, err := f.GetProvider("unknown")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestGetDefaultProvider_Empty_Error(t *testing.T) {
	f := NewFactory()
	_, err := f.GetDefaultProvider()
	if err == nil {
		t.Error("expected error when no providers registered")
	}
}

func TestSetDefaultProvider_Success(t *testing.T) {
	f := NewFactory()
	f.RegisterProvider("openai", &stubOracle{name: "openai"})
	f.RegisterProvider("anthropic", &stubOracle{name: "anthropic"})
	if err := f.SetDefaultProvider("anthropic"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, _ := f.GetDefaultProvider()
	if p.GetName() != "anthropic" {
		t.Errorf("expected default=anthropic, got %s", p.GetName())
	}
}

func TestSetDefaultProvider_NotRegistered_Error(t *testing.T) {
	f := NewFactory()
	if err := f.SetDefaultProvider("unknown"); err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestListProviders_ReturnsAll(t *testing.T) {
	f := NewFactory()
	f.RegisterProvider("openai", &stubOracle{name: "openai"})
	f.RegisterProvider("anthropic", &stubOracle{name: "anthropic"})
	providers := f.ListProviders()
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestListAvailableProviders_FiltersUnavailable(t *testing.T) {
	f := NewFactory()
	f.RegisterProvider("openai", &stubOracle{name: "openai", available: true})
	f.RegisterProvider("anthropic", &stubOracle{name: "anthropic", available: false})
	available := f.ListAvailableProviders()
	if len(available) != 1 || available[0] != "openai" {
		t.Errorf("expected [openai], got %v", available)
	}
}

// --- GetProviderForModel ---

func TestGetProviderForModel_ExactMatch(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "openai", models: []string{"gpt-4o", "gpt-3.5-turbo"}, available: true}
	f.RegisterProvider("openai", oracle)
	p, err := f.GetProviderForModel("gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected openai, got %s", p.GetName())
	}
}

func TestGetProviderForModel_CaseInsensitiveMatch(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "openai", models: []string{"GPT-4O"}, available: true}
	f.RegisterProvider("openai", oracle)
	p, err := f.GetProviderForModel("gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected openai, got %s", p.GetName())
	}
}

func TestGetProviderForModel_GPTPrefix_FallsBackToOpenAI(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "openai", available: true}
	f.RegisterProvider("openai", oracle)
	p, err := f.GetProviderForModel("gpt-4-turbo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected openai, got %s", p.GetName())
	}
}

func TestGetProviderForModel_O1Prefix_FallsBackToOpenAI(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "openai", available: true}
	f.RegisterProvider("openai", oracle)
	p, err := f.GetProviderForModel("o1-mini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected openai, got %s", p.GetName())
	}
}

func TestGetProviderForModel_ClaudePrefix_FallsBackToAnthropic(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "anthropic", available: true}
	f.RegisterProvider("anthropic", oracle)
	p, err := f.GetProviderForModel("claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "anthropic" {
		t.Errorf("expected anthropic, got %s", p.GetName())
	}
}

func TestGetProviderForModel_GeminiPrefix_FallsBackToGemini(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "gemini", available: true}
	f.RegisterProvider("gemini", oracle)
	p, err := f.GetProviderForModel("gemini-2.5-flash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "gemini" {
		t.Errorf("expected gemini, got %s", p.GetName())
	}
}

func TestGetProviderForModel_UnknownModel_FallsBackToDefault(t *testing.T) {
	f := NewFactory()
	oracle := &stubOracle{name: "openai", available: true}
	f.RegisterProvider("openai", oracle)
	p, err := f.GetProviderForModel("unknown-model-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.GetName() != "openai" {
		t.Errorf("expected default=openai, got %s", p.GetName())
	}
}

func TestGetProviderForModel_NoProviders_Error(t *testing.T) {
	f := NewFactory()
	_, err := f.GetProviderForModel("gpt-4")
	if err == nil {
		t.Error("expected error when no providers")
	}
}

func TestGetDefaultProvider_NotFoundInMap(t *testing.T) {
	// Set defaultProvider directly without registering — triggers line 69-71.
	f := &Factory{
		providers:       make(map[string]ports.Oracle),
		defaultProvider: "ghost",
	}
	_, err := f.GetDefaultProvider()
	if err == nil {
		t.Fatal("expected error when default provider not in map")
	}
}
