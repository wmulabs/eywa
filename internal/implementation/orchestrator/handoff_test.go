package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func TestVigilHandoffSink_AcquiresWhenNoSeat(t *testing.T) {
	repo := &stubVigilRepo{}             // no seat, no error
	sink := newVigilHandoffSink(repo, 0) // 0 -> default TTL
	if err := sink.RaiseTakeover(context.Background(), "user:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVigilHandoffSink_SkipsWhenSeatHeld(t *testing.T) {
	// A held seat short-circuits before Acquire: even with the repo set to error, RaiseTakeover
	// returns nil because Acquire is never called.
	repo := &stubVigilRepo{vigil: &entities.Vigil{MemoryKey: "user:1"}, err: errors.New("would fail if called")}
	sink := newVigilHandoffSink(repo, time.Minute)
	if err := sink.RaiseTakeover(context.Background(), "user:1"); err != nil {
		t.Fatalf("held seat must short-circuit without acquiring, got %v", err)
	}
}

func TestVigilHandoffSink_WrapsAcquireError(t *testing.T) {
	repo := &stubVigilRepo{err: errors.New("redis down")} // no seat -> Acquire attempted -> errors
	sink := newVigilHandoffSink(repo, time.Minute)
	if err := sink.RaiseTakeover(context.Background(), "user:1"); err == nil {
		t.Error("expected the acquire error to surface")
	}
}

type stubHandoffSink struct {
	calls int
	err   error
}

func (s *stubHandoffSink) RaiseTakeover(_ context.Context, _ string) error {
	s.calls++
	return s.err
}

func handoffSvc(t *testing.T, content string, sink HandoffSink, policy HandoffPolicy) (*ReasoningService, *stubHandoffSink) {
	t.Helper()
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: content, StopReason: ports.StopReasonComplete}}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)
	svc.SetHandoffPolicy(policy)
	if sink != nil {
		svc.SetHandoffSink(sink)
	}
	ss, _ := sink.(*stubHandoffSink)
	return svc, ss
}

func TestExecute_Handoff_RaiseVigil_LowConfidence(t *testing.T) {
	sink := &stubHandoffSink{}
	policy := HandoffPolicy{Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceHigh, UncertaintyMarkers: []string{"not sure"}}
	svc, ss := handoffSvc(t, "Honestly I'm not sure about that.", sink, policy)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HandoffRaised {
		t.Error("expected a takeover to be raised on low confidence")
	}
	if ss.calls != 1 {
		t.Errorf("expected exactly 1 takeover call, got %d", ss.calls)
	}
	if result.FinalResponse != defaultHoldingMessage {
		t.Errorf("expected the holding message delivered, got %q", result.FinalResponse)
	}
}

func TestExecute_Handoff_AnnotateOnly_DeliversAnswer(t *testing.T) {
	policy := HandoffPolicy{Enabled: true, Mode: HandoffAnnotateOnly, MinConfidence: ConfidenceHigh, UncertaintyMarkers: []string{"not sure"}}
	svc, _ := handoffSvc(t, "I'm not sure, but maybe 42.", nil, policy)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HandoffRaised {
		t.Error("annotate-only must not raise a takeover")
	}
	if result.FinalResponse != "I'm not sure, but maybe 42." {
		t.Errorf("annotate-only must deliver the answer, got %q", result.FinalResponse)
	}
	if result.Confidence != ConfidenceMedium {
		t.Errorf("expected low-confidence band recorded, got %q", result.Confidence)
	}
}

func TestExecute_Handoff_NoSink_DegradesToDelivery(t *testing.T) {
	policy := HandoffPolicy{Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceHigh, UncertaintyMarkers: []string{"not sure"}}
	svc, _ := handoffSvc(t, "I'm not sure.", nil, policy) // no sink wired

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HandoffRaised {
		t.Error("must not claim a handoff when no sink is available")
	}
	if result.FinalResponse != "I'm not sure." {
		t.Errorf("must degrade to delivering the answer, got %q", result.FinalResponse)
	}
}

func TestExecute_Handoff_SinkError_Degrades(t *testing.T) {
	sink := &stubHandoffSink{err: context.DeadlineExceeded}
	policy := HandoffPolicy{Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceHigh, UncertaintyMarkers: []string{"not sure"}}
	svc, ss := handoffSvc(t, "I'm not sure.", sink, policy)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HandoffRaised {
		t.Error("a failed takeover must not claim a handoff")
	}
	if ss.calls != 1 {
		t.Errorf("expected the sink to be attempted once, got %d", ss.calls)
	}
	if result.FinalResponse != "I'm not sure." {
		t.Errorf("must degrade to delivering the answer, got %q", result.FinalResponse)
	}
}

func TestExecute_Handoff_DefaultMinConfidence(t *testing.T) {
	// Empty MinConfidence -> defaults to Medium. A High-confidence turn delivers normally.
	policy := HandoffPolicy{Enabled: true, Mode: HandoffAnnotateOnly}
	svc, _ := handoffSvc(t, "The answer is 42.", nil, policy)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence != ConfidenceHigh {
		t.Errorf("expected High confidence recorded, got %q", result.Confidence)
	}
	if result.FinalResponse != "The answer is 42." {
		t.Errorf("expected normal delivery, got %q", result.FinalResponse)
	}
}

func TestExecute_Handoff_NonConversationalSpirit_NeverHandsOff(t *testing.T) {
	sink := &stubHandoffSink{}
	policy := HandoffPolicy{Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceHigh, UncertaintyMarkers: []string{"not sure"}}
	svc, ss := handoffSvc(t, "I'm not sure.", sink, policy)

	req := testRequest()
	req.Spirit.Type = entities.SpiritTypeExecutor // non-conversational

	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HandoffRaised || ss.calls != 0 {
		t.Error("a non-conversational Spirit must never raise a takeover")
	}
	if result.FinalResponse != "I'm not sure." {
		t.Errorf("expected the answer delivered, got %q", result.FinalResponse)
	}
}

func TestExecute_Handoff_HighConfidence_NoHandoff(t *testing.T) {
	sink := &stubHandoffSink{}
	policy := HandoffPolicy{Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceHigh}
	svc, ss := handoffSvc(t, "The answer is 42.", sink, policy)

	result, err := svc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HandoffRaised {
		t.Error("a confident turn must not hand off")
	}
	if ss.calls != 0 {
		t.Error("a confident turn must not call the takeover sink")
	}
	if result.FinalResponse != "The answer is 42." {
		t.Errorf("expected the answer delivered, got %q", result.FinalResponse)
	}
	if result.Confidence != ConfidenceHigh {
		t.Errorf("expected High confidence, got %q", result.Confidence)
	}
}

func TestScoreConfidence(t *testing.T) {
	cases := []struct {
		name string
		sig  confidenceSignals
		want Confidence
	}{
		{"clean turn", confidenceSignals{}, ConfidenceHigh},
		{"grounded clean", confidenceSignals{grounded: true}, ConfidenceHigh},
		{"one critical error", confidenceSignals{criticalErrors: 1}, ConfidenceMedium},
		{"two critical errors", confidenceSignals{criticalErrors: 2}, ConfidenceLow},
		{"reflection failed", confidenceSignals{reflectionFailed: true}, ConfidenceLow},
		{"uncertain content", confidenceSignals{uncertainContent: true}, ConfidenceMedium},
		{"error offset by grounding", confidenceSignals{criticalErrors: 1, grounded: true}, ConfidenceHigh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scoreConfidence(tc.sig); got != tc.want {
				t.Errorf("scoreConfidence(%+v) = %q, want %q", tc.sig, got, tc.want)
			}
		})
	}
}

func TestConfidenceRank_Ordering(t *testing.T) {
	if confidenceRank(ConfidenceLow) >= confidenceRank(ConfidenceMedium) {
		t.Error("Low must rank below Medium")
	}
	if confidenceRank(ConfidenceMedium) >= confidenceRank(ConfidenceHigh) {
		t.Error("Medium must rank below High")
	}
	if confidenceRank(Confidence("bogus")) != confidenceRank(ConfidenceMedium) {
		t.Error("unknown confidence must rank as Medium")
	}
}

func TestMatchesUncertainty(t *testing.T) {
	markers := []string{"i'm not sure", "i cannot confirm"}
	if !matchesUncertainty("Well, I'm not sure about that.", markers) {
		t.Error("expected uncertainty match (case-insensitive)")
	}
	if matchesUncertainty("The refund window is 30 days.", markers) {
		t.Error("did not expect a match")
	}
	if matchesUncertainty("anything", nil) {
		t.Error("no markers must never match")
	}
}
