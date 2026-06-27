package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func TestSpiritSelectionStep_HonorsHandoffPin(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryHandoffStore()
	_ = store.SetActiveSpirit(ctx, "user:1", "billing")

	selector := &stubSpiritSelectorImpl{spiritName: "triage"}
	step := NewSpiritSelectionStep(selector, store, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: &entities.Link{AllowedSpirits: []string{"triage"}},
	}

	if err := step.Execute(ctx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.SpiritName != "billing" {
		t.Errorf("expected pinned spirit 'billing', got %q", state.SpiritName)
	}
	if state.PathfinderUsed != "handoff" {
		t.Errorf("expected PathfinderUsed 'handoff', got %q", state.PathfinderUsed)
	}
}

func TestSpiritSelectionStep_NoPin_FallsBackToSelector(t *testing.T) {
	selector := &stubSpiritSelectorImpl{spiritName: "triage"}
	step := NewSpiritSelectionStep(selector, NewInMemoryHandoffStore(), time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{MemoryKey: "user:1"},
		EventConfig: &entities.Link{AllowedSpirits: []string{"triage"}},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.SpiritName != "triage" {
		t.Errorf("expected selector result 'triage', got %q", state.SpiritName)
	}
}

func newReasoningServiceForHandoff(t *testing.T, hs *HandoffService) *ReasoningService {
	rs := NewReasoningService(nil, nil, nil, 5, "max", 0, testLogger(t), nil)
	rs.SetSpiritHandoffService(hs)
	return rs
}

func TestAvailableActions_InjectsHandoffTool(t *testing.T) {
	rs := newReasoningServiceForHandoff(t, &HandoffService{})
	req := &ReasoningRequest{Spirit: leaderSpirit("billing")}

	found := false
	for _, a := range rs.availableActions(req) {
		if a.Name == handoffSpiritActionName {
			found = true
		}
	}
	if !found {
		t.Error("expected handoff_spirit tool to be injected for a Spirit with handoff targets")
	}
}

func TestAvailableActions_NoHandoffTool_WhenNoTargets(t *testing.T) {
	rs := newReasoningServiceForHandoff(t, &HandoffService{})
	req := &ReasoningRequest{Spirit: &entities.Spirit{Name: "plain", Type: entities.SpiritTypeConversational}}

	for _, a := range rs.availableActions(req) {
		if a.Name == handoffSpiritActionName {
			t.Error("handoff tool must not appear for a Spirit without handoff targets")
		}
	}
}

func TestFindHandoffCall(t *testing.T) {
	hs := &HandoffService{}
	rsWired := newReasoningServiceForHandoff(t, hs)
	rsBare := NewReasoningService(nil, nil, nil, 5, "max", 0, testLogger(t), nil)

	handoffCall := ports.OracleToolCall{Name: handoffSpiritActionName}
	otherCall := ports.OracleToolCall{Name: "get_time"}

	if _, ok := rsBare.findHandoffCall(leaderSpirit("billing"), []ports.OracleToolCall{handoffCall}); ok {
		t.Error("no handoff service wired: must not find a handoff call")
	}
	if _, ok := rsWired.findHandoffCall(&entities.Spirit{}, []ports.OracleToolCall{handoffCall}); ok {
		t.Error("spirit without handoff targets: must not find a handoff call")
	}
	if _, ok := rsWired.findHandoffCall(leaderSpirit("billing"), []ports.OracleToolCall{otherCall}); ok {
		t.Error("no handoff_spirit call present: must not find one")
	}
	if _, ok := rsWired.findHandoffCall(leaderSpirit("billing"), []ports.OracleToolCall{otherCall, handoffCall}); !ok {
		t.Error("expected to find the handoff_spirit call")
	}
}

func TestTransferContext_Summary_ArchivistError_FallsBack(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), &stubArchivist{err: context.DeadlineExceeded})
	spirit := leaderSpirit("billing")
	spirit.HandoffConfig.ContextTransfer = entities.HandoffContextSummary
	parentSession := &entities.Memory{MemoryKey: "user:1", Threads: []entities.Thread{{Role: "user", Content: "hi"}}}
	parentCtx := []ports.OracleMessage{{Role: ports.RoleUser, Content: "hi"}}

	session, ctx, addendum := svc.transferContext(context.Background(), spirit, parentSession, parentCtx)
	if session != parentSession || len(ctx) != 1 || addendum != "" {
		t.Error("archivist error must fall back to the full session")
	}
}

func TestHandoff_RunsTargetScouts(t *testing.T) {
	target := &entities.Spirit{Name: "billing", Type: entities.SpiritTypeConversational, RequireScouts: []string{"geo"}}
	repo := &stubSpiritRepo{spirit: target}
	reasoning := &stubReasoningExec{result: &ReasoningResult{FinalResponse: "ok"}}
	registry := &stubScoutRegistryForSummon{scout: &stubScoutForSummon{}}
	svc := NewHandoffService(repo, registry, reasoning, NewInMemoryHandoffStore(), nil, testLogger(t))

	if _, err := svc.Handoff(context.Background(), "billing", handoffParentPulse(), leaderSpirit("billing"), &entities.Memory{}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func handoffTurnState(spirit *entities.Spirit) *turnState {
	return &turnState{
		req: &ReasoningRequest{
			Event:  &entities.Pulse{MemoryKey: "user:1", UserMessage: "refund"},
			Spirit: spirit,
		},
		result: &ReasoningResult{},
	}
}

func TestExecuteHandoff_Terminal_FinalizesWithTargetResponse(t *testing.T) {
	repo := &stubSpiritRepo{spirit: &entities.Spirit{Name: "billing", Type: entities.SpiritTypeConversational}}
	reasoning := &stubReasoningExec{result: &ReasoningResult{
		FinalResponse: "refund issued",
		TokensUsed:    ports.OracleUsage{TotalTokens: 42},
	}}
	hs := NewHandoffService(repo, nil, reasoning, NewInMemoryHandoffStore(), nil, testLogger(t))
	rs := newReasoningServiceForHandoff(t, hs)

	ts := handoffTurnState(leaderSpirit("billing"))
	call := ports.OracleToolCall{ID: "c1", Name: handoffSpiritActionName, Arguments: map[string]any{"target_spirit": "billing"}}

	done, err := rs.executeHandoff(context.Background(), ts, call, &entities.IterationLog{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("a successful handoff must be terminal (done=true)")
	}
	if ts.result.FinalResponse != "refund issued" {
		t.Errorf("expected target response, got %q", ts.result.FinalResponse)
	}
	if ts.result.TokensUsed.TotalTokens != 42 {
		t.Errorf("expected target tokens accrued, got %d", ts.result.TokensUsed.TotalTokens)
	}
}

func TestExecuteHandoff_DisallowedTarget_ContinuesNonTerminal(t *testing.T) {
	hs := NewHandoffService(&stubSpiritRepo{}, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil, testLogger(t))
	rs := newReasoningServiceForHandoff(t, hs)

	ts := handoffTurnState(leaderSpirit("sales")) // target 'billing' not allowed
	call := ports.OracleToolCall{ID: "c1", Name: handoffSpiritActionName, Arguments: map[string]any{"target_spirit": "billing"}}

	done, err := rs.executeHandoff(context.Background(), ts, call, &entities.IterationLog{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("a rejected handoff must not terminate the turn; the current Spirit continues")
	}
	if ts.result.FinalResponse != "" {
		t.Errorf("rejected handoff must not set a final response, got %q", ts.result.FinalResponse)
	}
}

func TestExecuteIterationActions_HandoffIsTerminal(t *testing.T) {
	repo := &stubSpiritRepo{spirit: &entities.Spirit{Name: "billing", Type: entities.SpiritTypeConversational}}
	reasoning := &stubReasoningExec{result: &ReasoningResult{FinalResponse: "refund issued"}}
	hs := NewHandoffService(repo, nil, reasoning, NewInMemoryHandoffStore(), nil, testLogger(t))
	rs := newReasoningServiceForHandoff(t, hs)

	ts := handoffTurnState(leaderSpirit("billing"))
	llmResp := &ports.OracleResponse{ToolCalls: []ports.OracleToolCall{
		{ID: "c1", Name: handoffSpiritActionName, Arguments: map[string]any{"target_spirit": "billing"}},
	}}

	done, err := rs.executeIterationActions(context.Background(), ts, llmResp, &entities.IterationLog{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("a handoff call must make the iteration terminal")
	}
	if ts.result.FinalResponse != "refund issued" {
		t.Errorf("expected target response, got %q", ts.result.FinalResponse)
	}
}

func TestExecuteHandoff_MissingTarget_ContinuesNonTerminal(t *testing.T) {
	hs := NewHandoffService(&stubSpiritRepo{}, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil, testLogger(t))
	rs := newReasoningServiceForHandoff(t, hs)

	ts := handoffTurnState(leaderSpirit("billing"))
	call := ports.OracleToolCall{ID: "c1", Name: handoffSpiritActionName, Arguments: map[string]any{}}

	done, err := rs.executeHandoff(context.Background(), ts, call, &entities.IterationLog{})
	if err != nil || done {
		t.Errorf("missing target_spirit must continue non-terminally, got done=%v err=%v", done, err)
	}
}
