package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func leaderSpirit(targets ...string) *entities.Spirit {
	return &entities.Spirit{
		Name:          "triage",
		Type:          entities.SpiritTypeConversational,
		HandoffConfig: entities.HandoffConfig{AllowedTargets: targets},
	}
}

func handoffParentPulse() *entities.Pulse {
	return &entities.Pulse{ID: "p1", MemoryKey: "user:1", UserMessage: "I need a refund", Knowledge: map[string]any{"k": "v"}}
}

func newHandoffSvc(t *testing.T, repo ports.SpiritRepository, reasoning ReasoningExecutor, store ports.HandoffStore, arch ports.Archivist) *HandoffService {
	return NewHandoffService(repo, nil, reasoning, store, arch, testLogger(t))
}

// --- validateHandoff ---

func TestValidateHandoff_NotAllowed_BusinessError(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	err := svc.validateHandoff("billing", handoffParentPulse(), leaderSpirit("sales"))
	if err == nil || !domainerrors.IsBusinessError(err) {
		t.Fatalf("expected business error, got %v", err)
	}
}

func TestValidateHandoff_LoopCap_BusinessError(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	pulse := handoffParentPulse()
	pulse.HandoffCount = 3 // equals default cap
	err := svc.validateHandoff("billing", pulse, leaderSpirit("billing"))
	if err == nil || !domainerrors.IsBusinessError(err) {
		t.Fatalf("expected loop-cap business error, got %v", err)
	}
}

func TestValidateHandoff_Success(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	if err := svc.validateHandoff("billing", handoffParentPulse(), leaderSpirit("billing")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- buildChildPulse ---

func TestHandoffBuildChildPulse_TransfersMessageAndIncrementsCount(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	child := svc.buildChildPulse(handoffParentPulse())
	if child.UserMessage != "I need a refund" {
		t.Errorf("expected user message transferred, got %q", child.UserMessage)
	}
	if child.HandoffCount != 1 {
		t.Errorf("expected HandoffCount=1, got %d", child.HandoffCount)
	}
	if child.Knowledge["k"] != "v" {
		t.Error("expected knowledge copied")
	}
}

// --- Handoff ---

func TestHandoff_Success_PinsTargetAndReturnsResult(t *testing.T) {
	ctx := context.Background()
	repo := &stubSpiritRepo{spirit: &entities.Spirit{Name: "billing", Type: entities.SpiritTypeConversational}}
	reasoning := &stubReasoningExec{result: &ReasoningResult{FinalResponse: "refund started"}}
	store := NewInMemoryHandoffStore()
	svc := newHandoffSvc(t, repo, reasoning, store, nil)

	res, err := svc.Handoff(ctx, "billing", handoffParentPulse(), leaderSpirit("billing"), &entities.Memory{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalResponse != "refund started" {
		t.Errorf("expected target response, got %q", res.FinalResponse)
	}
	if active, _ := store.GetActiveSpirit(ctx, "user:1"); active != "billing" {
		t.Errorf("expected pin set to billing, got %q", active)
	}
}

func TestHandoff_NotAllowed_NoPin(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryHandoffStore()
	svc := newHandoffSvc(t, &stubSpiritRepo{}, &stubReasoningExec{}, store, nil)

	if _, err := svc.Handoff(ctx, "billing", handoffParentPulse(), leaderSpirit("sales"), &entities.Memory{}, nil); err == nil {
		t.Fatal("expected error for disallowed target")
	}
	if active, _ := store.GetActiveSpirit(ctx, "user:1"); active != "" {
		t.Errorf("pin must not be set on a rejected handoff, got %q", active)
	}
}

func TestHandoff_SpiritNotFound_InfraError(t *testing.T) {
	repo := &stubSpiritRepo{err: errors.New("db down")}
	svc := newHandoffSvc(t, repo, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	_, err := svc.Handoff(context.Background(), "billing", handoffParentPulse(), leaderSpirit("billing"), &entities.Memory{}, nil)
	if err == nil || !domainerrors.IsInfrastructureError(err) {
		t.Fatalf("expected infra error, got %v", err)
	}
}

func TestHandoff_ReasoningFails_InfraError(t *testing.T) {
	repo := &stubSpiritRepo{spirit: &entities.Spirit{Name: "billing", Type: entities.SpiritTypeConversational}}
	reasoning := &stubReasoningExec{err: errors.New("llm timeout")}
	_, err := newHandoffSvc(t, repo, reasoning, NewInMemoryHandoffStore(), nil).
		Handoff(context.Background(), "billing", handoffParentPulse(), leaderSpirit("billing"), &entities.Memory{}, nil)
	if err == nil || !domainerrors.IsInfrastructureError(err) {
		t.Fatalf("expected infra error, got %v", err)
	}
}

type errSetHandoffStore struct{}

func (errSetHandoffStore) GetActiveSpirit(context.Context, string) (string, error) { return "", nil }
func (errSetHandoffStore) SetActiveSpirit(context.Context, string, string) error {
	return errors.New("store down")
}
func (errSetHandoffStore) ClearActiveSpirit(context.Context, string) error { return nil }

func TestHandoff_PinError_InfraError(t *testing.T) {
	repo := &stubSpiritRepo{spirit: &entities.Spirit{Name: "billing", Type: entities.SpiritTypeConversational}}
	svc := newHandoffSvc(t, repo, &stubReasoningExec{}, errSetHandoffStore{}, nil)
	_, err := svc.Handoff(context.Background(), "billing", handoffParentPulse(), leaderSpirit("billing"), &entities.Memory{}, nil)
	if err == nil || !domainerrors.IsInfrastructureError(err) {
		t.Fatalf("expected infra error when pin fails, got %v", err)
	}
}

// --- transferContext ---

func TestTransferContext_Session_PassesFullHistory(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	spirit := leaderSpirit("billing")
	spirit.HandoffConfig.ContextTransfer = entities.HandoffContextSession
	parentSession := &entities.Memory{MemoryKey: "user:1", Threads: []entities.Thread{{Role: "user", Content: "hi"}}}
	parentCtx := []ports.OracleMessage{{Role: ports.RoleUser, Content: "hi"}}

	session, ctx, addendum := svc.transferContext(context.Background(), spirit, parentSession, parentCtx)
	if session != parentSession || len(ctx) != 1 || addendum != "" {
		t.Errorf("session mode must pass full history unchanged")
	}
}

func TestTransferContext_Summary_UsesArchivist(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), &stubArchivist{summary: "user wants refund"})
	spirit := leaderSpirit("billing")
	spirit.HandoffConfig.ContextTransfer = entities.HandoffContextSummary
	parentSession := &entities.Memory{MemoryKey: "user:1", Threads: []entities.Thread{{Role: "user", Content: "refund please"}}}

	session, ctx, addendum := svc.transferContext(context.Background(), spirit, parentSession, nil)
	if !strings.Contains(addendum, "user wants refund") {
		t.Errorf("summary must be injected into the prompt addendum, got %q", addendum)
	}
	if len(session.Threads) != 0 || ctx != nil {
		t.Error("summary mode must not pass raw history")
	}
}

func TestTransferContext_Summary_FallsBackWhenNoArchivist(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	spirit := leaderSpirit("billing")
	spirit.HandoffConfig.ContextTransfer = entities.HandoffContextSummary
	parentSession := &entities.Memory{MemoryKey: "user:1", Threads: []entities.Thread{{Role: "user", Content: "hi"}}}
	parentCtx := []ports.OracleMessage{{Role: ports.RoleUser, Content: "hi"}}

	session, ctx, addendum := svc.transferContext(context.Background(), spirit, parentSession, parentCtx)
	if session != parentSession || len(ctx) != 1 || addendum != "" {
		t.Error("summary without archivist must fall back to full session")
	}
}

func TestTransferContext_None_EmptyContext(t *testing.T) {
	svc := newHandoffSvc(t, nil, &stubReasoningExec{}, NewInMemoryHandoffStore(), nil)
	spirit := leaderSpirit("billing") // ContextTransfer unset = none
	parentSession := &entities.Memory{MemoryKey: "user:1", Threads: []entities.Thread{{Role: "user", Content: "hi"}}}

	session, ctx, addendum := svc.transferContext(context.Background(), spirit, parentSession, nil)
	if len(session.Threads) != 0 || ctx != nil || addendum != "" {
		t.Error("none mode must start the target with no prior context")
	}
}
