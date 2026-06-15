package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// tierOracle records the model used on each call. The draft model returns tool calls for the first
// maxToolTurns then a draft terminal; the primary (final) model returns a distinct final answer.
type tierOracle struct {
	mu           sync.Mutex
	models       []string
	toolTurns    int
	maxToolTurns int
	primaryModel string
	primaryErr   bool
}

func (o *tierOracle) GetName() string                 { return "tier" }
func (o *tierOracle) GetAvailableModels() []string    { return nil }
func (o *tierOracle) IsAvailable() bool               { return true }
func (o *tierOracle) GetConfig() map[string]any       { return nil }
func (o *tierOracle) SupportsImages(_ string) bool    { return false }
func (o *tierOracle) SupportsAudio(_ string) bool     { return false }
func (o *tierOracle) SupportsDocuments(_ string) bool { return false }
func (o *tierOracle) GenerateResponse(_ context.Context, req *ports.OracleRequest) (*ports.OracleResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.models = append(o.models, req.Model)
	if req.Model == o.primaryModel {
		if o.primaryErr {
			return nil, errors.New("primary model unavailable")
		}
		return &ports.OracleResponse{Content: "PRIMARY FINAL", StopReason: ports.StopReasonComplete, TokensUsed: ports.OracleUsage{TotalTokens: 10}}, nil
	}
	// draft model
	o.toolTurns++
	if o.toolTurns <= o.maxToolTurns {
		return &ports.OracleResponse{
			StopReason: ports.StopReasonToolCalls,
			ToolCalls:  []ports.OracleToolCall{{ID: "c", Name: "do", Arguments: map[string]any{}}},
			TokensUsed: ports.OracleUsage{TotalTokens: 1},
		}, nil
	}
	return &ports.OracleResponse{Content: "draft final", StopReason: ports.StopReasonComplete, TokensUsed: ports.OracleUsage{TotalTokens: 1}}, nil
}

func (o *tierOracle) usedModel(name string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, m := range o.models {
		if m == name {
			return true
		}
	}
	return false
}

func tierReq(model, draft string) *ReasoningRequest {
	return &ReasoningRequest{
		Event:  &entities.Pulse{MemoryKey: "user:1", ID: "evt", UserMessage: "hi"},
		Spirit: &entities.Spirit{ModelConfig: entities.SpiritModel{Model: model, DraftModel: draft}},
	}
}

func TestTierDraftModel_FallsBackToPrimary(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)
	if got := svc.tierDraftModel(tierReq("primary", "")); got != "primary" {
		t.Errorf("draft must fall back to primary, got %q", got)
	}
	if got := svc.tierDraftModel(tierReq("primary", "cheap")); got != "cheap" {
		t.Errorf("expected configured draft 'cheap', got %q", got)
	}
}

func TestTieringActive_OnlyWhenDistinctDraftSet(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)

	if svc.tieringActive(tierReq("m", "")) {
		t.Error("no draft model -> inactive")
	}
	if svc.tieringActive(tierReq("m", "m")) {
		t.Error("draft equal to primary -> inactive")
	}
	if !svc.tieringActive(tierReq("m", "cheap")) {
		t.Error("distinct draft -> active")
	}
}

func TestAccumulateModelTokens(t *testing.T) {
	r := &ReasoningResult{}
	r.accumulateModelTokens("cheap", ports.OracleUsage{TotalTokens: 5})
	r.accumulateModelTokens("cheap", ports.OracleUsage{TotalTokens: 3})
	r.accumulateModelTokens("primary", ports.OracleUsage{TotalTokens: 10})

	if r.TokensByModel["cheap"].TotalTokens != 8 {
		t.Errorf("expected cheap=8, got %d", r.TokensByModel["cheap"].TotalTokens)
	}
	if r.TokensByModel["primary"].TotalTokens != 10 {
		t.Errorf("expected primary=10, got %d", r.TokensByModel["primary"].TotalTokens)
	}
}

func TestAccrueTokens_TieringEmptyModelDefaultsToPrimary(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)
	req := tierReq("gpt-primary", "cheap")

	result := &ReasoningResult{}
	svc.accrueTokens(req, result, "", ports.OracleUsage{TotalTokens: 7})
	if result.TokensByModel["gpt-primary"].TotalTokens != 7 {
		t.Errorf("empty model under tiering must bucket to primary, got %+v", result.TokensByModel)
	}
}

func TestExecute_Tiering_DraftToolsPrimaryFinal(t *testing.T) {
	action := &stubAction{name: "do", execResult: "ok", category: ports.ActionGeneral}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	oracle := &tierOracle{maxToolTurns: 1, primaryModel: "gpt-4o"}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	req := tierReq("gpt-4o", "cheap")
	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "PRIMARY FINAL" {
		t.Errorf("final answer must come from the primary model, got %q", result.FinalResponse)
	}
	if !oracle.usedModel("cheap") || !oracle.usedModel("gpt-4o") {
		t.Errorf("expected both draft and primary models used, got %v", oracle.models)
	}
	if result.TokensByModel["cheap"].TotalTokens == 0 || result.TokensByModel["gpt-4o"].TotalTokens == 0 {
		t.Errorf("expected per-model token accounting, got %+v", result.TokensByModel)
	}
}

func TestExecute_Tiering_PrimarySynthesisFails_KeepsDraft(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &tierOracle{maxToolTurns: 0, primaryModel: "gpt-4o", primaryErr: true}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	result, err := svc.Execute(context.Background(), tierReq("gpt-4o", "cheap"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "draft final" {
		t.Errorf("a failed primary synthesis must keep the draft answer, got %q", result.FinalResponse)
	}
}

func TestExecute_NoTiering_SingleModel(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &tierOracle{maxToolTurns: 0, primaryModel: "gpt-4o"}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	// No DraftModel -> single model, no re-synthesis.
	result, err := svc.Execute(context.Background(), tierReq("gpt-4o", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "PRIMARY FINAL" {
		t.Errorf("without tiering, expected the single-model answer, got %q", result.FinalResponse)
	}
	if result.TokensByModel != nil {
		t.Errorf("TokensByModel must be empty when tiering off, got %+v", result.TokensByModel)
	}
}
