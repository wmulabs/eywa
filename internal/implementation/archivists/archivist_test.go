package archivists

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

func (o *stubOracle) GetName() string                 { return "stub" }
func (o *stubOracle) GetAvailableModels() []string    { return nil }
func (o *stubOracle) IsAvailable() bool               { return true }
func (o *stubOracle) GetConfig() map[string]any       { return nil }
func (o *stubOracle) SupportsImages(_ string) bool    { return false }
func (o *stubOracle) SupportsAudio(_ string) bool     { return false }
func (o *stubOracle) SupportsDocuments(_ string) bool { return false }
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
func (f *stubOracleFactory) SetDefaultProvider(_ string) error         { return nil }
func (f *stubOracleFactory) ListProviders() []string                   { return nil }
func (f *stubOracleFactory) ListAvailableProviders() []string          { return nil }
func (f *stubOracleFactory) GetProviderForModel(_ string) (ports.Oracle, error) {
	return f.oracle, f.forModelErr
}

func threads(n int) []entities.Thread {
	result := make([]entities.Thread, n)
	for i := range result {
		if i%2 == 0 {
			result[i] = entities.Thread{Role: ports.RoleUser, Content: "user msg"}
		} else {
			result[i] = entities.Thread{Role: ports.RoleAssistant, Content: "assistant reply"}
		}
	}
	return result
}

// --- LLMArchivist ---

func TestNewLLMArchivist_WithTemperatureAndMaxTokens(t *testing.T) {
	a := NewLLMArchivist("openai", "gpt-4o", &stubOracleFactory{}).
		WithTemperature(0.5).
		WithMaxTokens(256)
	if a.temp != 0.5 {
		t.Errorf("expected temp=0.5, got %f", a.temp)
	}
	if a.maxTokens != 256 {
		t.Errorf("expected maxTokens=256, got %d", a.maxTokens)
	}
}

func TestLLMArchivist_Summarize_EmptyMessages_ReturnsEmpty(t *testing.T) {
	a := NewLLMArchivist("openai", "gpt-4o", &stubOracleFactory{})
	summary, err := a.Summarize(context.Background(), "user:1", "topic", []entities.Thread{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
}

func TestLLMArchivist_Summarize_ProviderFound_ReturnsSummary(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "  User asked about billing.  "}}
	factory := &stubOracleFactory{oracle: oracle}
	a := NewLLMArchivist("openai", "gpt-4o", factory)

	summary, err := a.Summarize(context.Background(), "user:1", "billing", threads(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "User asked about billing." {
		t.Errorf("expected trimmed summary, got %q", summary)
	}
}

func TestLLMArchivist_Summarize_ProviderNotFound_FallsBackToModel(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "summary"}}
	factory := &stubOracleFactory{
		getErr: errors.New("provider not found"), // GetProvider fails
		oracle: oracle,                           // GetProviderForModel succeeds
	}
	a := NewLLMArchivist("openai", "gpt-4o", factory)

	summary, err := a.Summarize(context.Background(), "user:1", "topic", threads(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "summary" {
		t.Errorf("unexpected summary: %q", summary)
	}
}

func TestLLMArchivist_Summarize_BothProviderAndModelFail_ReturnsError(t *testing.T) {
	factory := &stubOracleFactory{
		getErr:      errors.New("not found"),
		forModelErr: errors.New("no model"),
	}
	a := NewLLMArchivist("openai", "gpt-4o", factory)

	_, err := a.Summarize(context.Background(), "user:1", "topic", threads(2))
	if err == nil {
		t.Error("expected error when both provider lookups fail")
	}
}

func TestLLMArchivist_Summarize_OracleCallFails_ReturnsError(t *testing.T) {
	oracle := &stubOracle{err: errors.New("llm timeout")}
	factory := &stubOracleFactory{oracle: oracle}
	a := NewLLMArchivist("openai", "gpt-4o", factory)

	_, err := a.Summarize(context.Background(), "user:1", "topic", threads(2))
	if err == nil {
		t.Error("expected error when oracle fails")
	}
}

func TestLLMArchivist_BuildPrompt_AllRoles(t *testing.T) {
	a := NewLLMArchivist("openai", "gpt-4o", &stubOracleFactory{})
	msgs := []entities.Thread{
		{Role: ports.RoleUser, Content: "hello"},
		{Role: ports.RoleAssistant, Content: "hi there"},
		{Role: ports.RoleTool, Content: "tool result"},
		{Role: "unknown", Content: "other"},
	}
	prompt := a.buildPrompt(msgs)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	for _, label := range []string{"User", "Assistant", "Tool"} {
		if !contains(prompt, label) {
			t.Errorf("expected label %q in prompt", label)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
