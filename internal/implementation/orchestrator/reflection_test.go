package orchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// reflectionOracle returns sequential drafts for reasoning requests and sequential verdicts for
// critique requests (SystemPrompt carries the reflection instruction).
type reflectionOracle struct {
	mu          sync.Mutex
	drafts      []string
	verdicts    []string
	draftTurn   int
	verdictTurn int
}

func (o *reflectionOracle) GetName() string                 { return "reflect" }
func (o *reflectionOracle) GetAvailableModels() []string    { return nil }
func (o *reflectionOracle) IsAvailable() bool               { return true }
func (o *reflectionOracle) GetConfig() map[string]any       { return nil }
func (o *reflectionOracle) SupportsImages(_ string) bool    { return false }
func (o *reflectionOracle) SupportsAudio(_ string) bool     { return false }
func (o *reflectionOracle) SupportsDocuments(_ string) bool { return false }
func (o *reflectionOracle) GenerateResponse(_ context.Context, req *ports.OracleRequest) (*ports.OracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if strings.Contains(req.SystemPrompt, "reviewing a draft") {
		v := o.verdicts[o.verdictTurn]
		o.verdictTurn++
		return &ports.OracleResponse{Content: v, StopReason: ports.StopReasonComplete}, nil
	}
	d := o.drafts[o.draftTurn]
	o.draftTurn++
	return &ports.OracleResponse{Content: d, StopReason: ports.StopReasonComplete}, nil
}

func TestExecute_Reflection_ReviseThenDeliver(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &reflectionOracle{
		drafts:   []string{"DRAFT", "REVISED"},
		verdicts: []string{`{"pass": false, "issues": ["did not answer"]}`},
	}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetReflectionPolicy(ReflectionPolicy{Enabled: true, MaxRounds: 1})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "REVISED" {
		t.Errorf("expected revised answer delivered, got %q", result.FinalResponse)
	}
	if oracle.verdictTurn != 1 {
		t.Errorf("expected exactly 1 critique (MaxRounds=1), got %d", oracle.verdictTurn)
	}
}

func TestExecute_Reflection_PassDeliversImmediately(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &reflectionOracle{
		drafts:   []string{"GOOD ANSWER"},
		verdicts: []string{`{"pass": true}`},
	}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetReflectionPolicy(ReflectionPolicy{Enabled: true, MaxRounds: 1})

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "GOOD ANSWER" {
		t.Errorf("expected the passing draft delivered, got %q", result.FinalResponse)
	}
}

func TestExecute_Reflection_Disabled_NoCritique(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &reflectionOracle{drafts: []string{"DRAFT"}, verdicts: []string{`{"pass": false}`}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	// No SetReflectionPolicy -> disabled.

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "DRAFT" {
		t.Errorf("expected raw draft when reflection disabled, got %q", result.FinalResponse)
	}
	if oracle.verdictTurn != 0 {
		t.Errorf("reflection must not run when disabled, got %d critiques", oracle.verdictTurn)
	}
}

func TestParseReflectionVerdict_CleanPass(t *testing.T) {
	pass, issues := parseReflectionVerdict(`{"pass": true}`)
	if !pass {
		t.Error("expected pass=true")
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestParseReflectionVerdict_FailWithIssues(t *testing.T) {
	pass, issues := parseReflectionVerdict(`{"pass": false, "issues": ["ignored a tool error", "did not answer"]}`)
	if pass {
		t.Error("expected pass=false")
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %v", issues)
	}
}

func TestParseReflectionVerdict_EmbeddedInProse(t *testing.T) {
	content := "Here is my review:\n```json\n{\"pass\": false, \"issues\": [\"unsupported claim\"]}\n```\nDone."
	pass, issues := parseReflectionVerdict(content)
	if pass {
		t.Error("expected pass=false from embedded JSON")
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %v", issues)
	}
}

func TestParseReflectionVerdict_GarbageFailsOpen(t *testing.T) {
	pass, issues := parseReflectionVerdict("not json at all")
	if !pass {
		t.Error("unparseable verdict must fail open (pass=true) to never block delivery")
	}
	if issues != nil {
		t.Errorf("expected nil issues on fail-open, got %v", issues)
	}
}

func TestParseReflectionVerdict_MalformedJSONObject_FailsOpen(t *testing.T) {
	// Has braces (so a JSON object is extracted) but is not valid JSON -> unmarshal error path.
	pass, issues := parseReflectionVerdict(`{pass: true, issues: [}`)
	if !pass {
		t.Error("malformed JSON object must fail open")
	}
	if issues != nil {
		t.Errorf("expected nil issues, got %v", issues)
	}
}

func TestReflect_AppliesModelOverrideAndCriteria(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: `{"pass": true}`}}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetReflectionPolicy(ReflectionPolicy{
		Enabled:   true,
		MaxRounds: 1,
		Model:     "critic-model",
		Criteria:  []string{"check tone"},
	})

	pass, _, _ := svc.reflect(context.Background(), oracle, testRequest(), []ports.OracleMessage{{Role: ports.RoleUser, Content: "draft"}})
	if !pass {
		t.Error("expected pass verdict")
	}
	if oracle.gotReq.Model != "critic-model" {
		t.Errorf("expected critic model override, got %q", oracle.gotReq.Model)
	}
	if !strings.Contains(oracle.gotReq.SystemPrompt, "check tone") {
		t.Errorf("expected criteria injected into critique prompt, got %q", oracle.gotReq.SystemPrompt)
	}
}

func TestReflect_OracleError_FailsOpen(t *testing.T) {
	oracle := &stubOracle{err: errors.New("critic down")}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetReflectionPolicy(ReflectionPolicy{Enabled: true, MaxRounds: 1})

	pass, issues, _ := svc.reflect(context.Background(), oracle, testRequest(), []ports.OracleMessage{{Role: ports.RoleUser, Content: "draft"}})
	if !pass {
		t.Error("oracle error must fail open (pass=true)")
	}
	if issues != nil {
		t.Errorf("expected nil issues on error, got %v", issues)
	}
}
