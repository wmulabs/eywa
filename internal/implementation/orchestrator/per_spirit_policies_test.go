package orchestrator

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func policySvc(t *testing.T) *ReasoningService {
	t.Helper()
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(&stubOracle{}), exec, 5)
	// Globals all enabled, so an override that disables is observable.
	svc.SetProgressPolicy(ProgressPolicy{Enabled: true, StallWindow: 2})
	svc.SetCompressionPolicy(CompressionPolicy{Enabled: true, MaxContextChars: 1000})
	svc.SetReflectionPolicy(ReflectionPolicy{Enabled: true, MaxRounds: 1})
	svc.SetGroundingPolicy(GroundingPolicy{Enabled: true})
	svc.SetPlanPolicy(PlanPolicy{Enabled: true})
	svc.SetHandoffPolicy(HandoffPolicy{Enabled: true})
	return svc
}

func TestEffectivePolicies_NilOverrides_UseGlobal(t *testing.T) {
	svc := policySvc(t)
	req := testRequest() // no overrides

	if !svc.effectiveProgress(req).Enabled ||
		!svc.effectiveCompression(req).Enabled ||
		!svc.effectiveReflection(req).Enabled ||
		!svc.effectiveGrounding(req).Enabled ||
		!svc.effectivePlan(req).Enabled ||
		!svc.effectiveHandoff(req).Enabled {
		t.Error("with no overrides, every effective policy must be the global default")
	}
}

func TestEffectivePolicies_EmptyBundle_UseGlobal(t *testing.T) {
	svc := policySvc(t)
	req := testRequest()
	req.Spirit.ReasoningOverrides = &entities.ReasoningOverrides{} // present but all-nil fields

	if !svc.effectiveReflection(req).Enabled || !svc.effectiveHandoff(req).Enabled {
		t.Error("an empty override bundle must still fall back to the global default")
	}
}

func TestEffectivePolicies_OverrideWins(t *testing.T) {
	svc := policySvc(t)
	req := testRequest()
	req.Spirit.ReasoningOverrides = &entities.ReasoningOverrides{
		Progress:    &entities.ProgressPolicy{Enabled: false},
		Compression: &entities.CompressionPolicy{Enabled: false},
		Reflection:  &entities.ReflectionPolicy{Enabled: false},
		Grounding:   &entities.GroundingPolicy{Enabled: false},
		Plan:        &entities.PlanPolicy{Enabled: false},
		Handoff:     &entities.HandoffPolicy{Enabled: false},
	}

	if svc.effectiveProgress(req).Enabled ||
		svc.effectiveCompression(req).Enabled ||
		svc.effectiveReflection(req).Enabled ||
		svc.effectiveGrounding(req).Enabled ||
		svc.effectivePlan(req).Enabled ||
		svc.effectiveHandoff(req).Enabled {
		t.Error("a per-Spirit override must win over the global default")
	}
}

func TestExecute_PerSpirit_HandoffOverrideEnablesWhenGlobalOff(t *testing.T) {
	sink := &stubHandoffSink{}
	// Global handoff stays OFF; only this Spirit enables it via an override.
	svc, _ := handoffSvc(t, "Honestly I'm not sure.", sink, HandoffPolicy{})

	req := testRequest()
	req.Spirit.ReasoningOverrides = &entities.ReasoningOverrides{
		Handoff: &entities.HandoffPolicy{
			Enabled: true, Mode: HandoffRaiseVigil, MinConfidence: ConfidenceHigh,
			UncertaintyMarkers: []string{"not sure"},
		},
	}

	result, err := svc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HandoffRaised || sink.calls != 1 {
		t.Errorf("per-Spirit override must activate handoff even with the global off (raised=%v, calls=%d)", result.HandoffRaised, sink.calls)
	}
}
