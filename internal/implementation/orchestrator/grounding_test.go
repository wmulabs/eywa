package orchestrator

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func groundedRequest(loreCtx string) *ReasoningRequest {
	req := testRequest()
	req.Event.Knowledge = map[string]any{entities.LoreContextKnowledgeKey: loreCtx}
	return req
}

const testLoreCtx = `<lore name="Policy" id="c1">refund window is 30 days</lore>`

func TestExecute_Grounding_ReviseOnceThenCite(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		{Content: "The window is 30 days.", StopReason: ports.StopReasonComplete},            // no citation -> revise
		{Content: "The window is 30 days [chunk:c1].", StopReason: ports.StopReasonComplete}, // cited
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetGroundingPolicy(GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingReviseOnce})

	result, err := svc.Execute(context.Background(), groundedRequest(testLoreCtx))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "The window is 30 days [chunk:c1]." {
		t.Errorf("expected cited revision delivered, got %q", result.FinalResponse)
	}
	if len(result.Citations) != 1 || result.Citations[0] != "c1" {
		t.Errorf("expected Citations [c1], got %v", result.Citations)
	}
}

func TestExecute_Grounding_BlockOnViolation(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		{Content: "Some unsourced claim.", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetGroundingPolicy(GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingBlock})

	result, err := svc.Execute(context.Background(), groundedRequest(testLoreCtx))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != defaultBlockedMessage {
		t.Errorf("expected blocked fallback message, got %q", result.FinalResponse)
	}
}

func TestExecute_Grounding_SufficientOnFirstDraft(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		{Content: "30 days [chunk:c1].", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetGroundingPolicy(GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingReviseOnce})

	result, err := svc.Execute(context.Background(), groundedRequest(testLoreCtx))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "30 days [chunk:c1]." {
		t.Errorf("expected first draft delivered, got %q", result.FinalResponse)
	}
	if len(result.Citations) != 1 {
		t.Errorf("expected 1 citation recorded, got %v", result.Citations)
	}
}

func TestExecute_Grounding_NoLore_Inert(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	oracle := &multiOracle{responses: []*ports.OracleResponse{
		{Content: "plain answer", StopReason: ports.StopReasonComplete},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetGroundingPolicy(GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingBlock})

	// testRequest has no lore context -> grounding must be inert.
	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "plain answer" {
		t.Errorf("grounding must be inert without retrieved Lore, got %q", result.FinalResponse)
	}
}

func groundingSvc(t *testing.T, policy GroundingPolicy) *ReasoningService {
	t.Helper()
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)
	svc.SetGroundingPolicy(policy)
	return svc
}

func TestRetrievedLoreContext_NonStringValue_Empty(t *testing.T) {
	req := testRequest()
	req.Event.Knowledge = map[string]any{entities.LoreContextKnowledgeKey: 123}
	if got := retrievedLoreContext(req); got != "" {
		t.Errorf("non-string lore context must yield empty, got %q", got)
	}
}

func TestEnforceGrounding_NoChunkIDsInContext_Inert(t *testing.T) {
	svc := groundingSvc(t, GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingReviseOnce})
	req := groundedRequest(`<lore name="x">text with no id attribute</lore>`)

	revise, blocked := svc.enforceGrounding(req, "uncited answer", &ReasoningResult{})
	if revise || blocked {
		t.Errorf("with no citable chunk ids grounding must be inert, got revise=%v blocked=%v", revise, blocked)
	}
}

func TestEnforceGrounding_AnnotateOnViolation(t *testing.T) {
	svc := groundingSvc(t, GroundingPolicy{Enabled: true, MinCitations: 1, OnViolation: GroundingAnnotate})
	req := groundedRequest(testLoreCtx)
	result := &ReasoningResult{}

	revise, blocked := svc.enforceGrounding(req, "answer without citation", result)
	if revise || blocked {
		t.Errorf("annotate must deliver (no revise/block), got revise=%v blocked=%v", revise, blocked)
	}
	if result.FinalError == "" {
		t.Error("annotate must flag the violation in FinalError")
	}
}

func TestEnforceGrounding_MinCitationsDefaultsToOne(t *testing.T) {
	svc := groundingSvc(t, GroundingPolicy{Enabled: true, MinCitations: 0, OnViolation: GroundingReviseOnce})
	req := groundedRequest(testLoreCtx)
	result := &ReasoningResult{}

	revise, blocked := svc.enforceGrounding(req, "the answer [chunk:c1]", result)
	if revise || blocked {
		t.Errorf("a single citation must satisfy the default minimum, got revise=%v blocked=%v", revise, blocked)
	}
	if len(result.Citations) != 1 {
		t.Errorf("expected 1 recorded citation, got %v", result.Citations)
	}
}

func TestParseCitations_ExtractsChunkIDs(t *testing.T) {
	draft := "The refund window is 30 days [chunk:c1]. Returns are free [chunk:c2]."
	got := parseCitations(draft)
	if len(got) != 2 || !contains(got, "c1") || !contains(got, "c2") {
		t.Errorf("expected [c1 c2], got %v", got)
	}
}

func TestParseCitations_Deduplicates(t *testing.T) {
	got := parseCitations("a [chunk:c1] b [chunk:c1]")
	if len(got) != 1 {
		t.Errorf("expected deduped single citation, got %v", got)
	}
}

func TestParseCitations_NoneFound(t *testing.T) {
	if got := parseCitations("no sources here"); len(got) != 0 {
		t.Errorf("expected no citations, got %v", got)
	}
}

func TestExtractLoreChunkIDs_FromContext(t *testing.T) {
	loreCtx := `<lore name="Policy" id="c1">refund text</lore>
<lore name="FAQ" id="c2">returns text</lore>`
	ids := extractLoreChunkIDs(loreCtx)
	if !ids["c1"] || !ids["c2"] || len(ids) != 2 {
		t.Errorf("expected {c1,c2}, got %v", ids)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
