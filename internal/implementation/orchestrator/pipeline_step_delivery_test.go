package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubVoice struct {
	name          string
	autoRespond   bool
	sendErr       error
	sendCalled    bool
}

var _ ports.Voice = (*stubVoice)(nil)

func (v *stubVoice) GetName() string                             { return v.name }
func (v *stubVoice) ShouldAutoRespond() bool                     { return v.autoRespond }
func (v *stubVoice) SendResponse(_ context.Context, _ *entities.Pulse, _ string) error {
	v.sendCalled = true
	return v.sendErr
}
func (v *stubVoice) GetChannelMetadata(_ *entities.Pulse) map[string]any {
	return map[string]any{}
}

type stubVoiceRegistry struct {
	voice ports.Voice
	getErr error
}

var _ ports.VoiceRegistry = (*stubVoiceRegistry)(nil)

func (r *stubVoiceRegistry) Register(_ ports.Voice) error { panic("not implemented") }
func (r *stubVoiceRegistry) RegisterMultiple(_ ...ports.Voice) error { panic("not implemented") }
func (r *stubVoiceRegistry) Get(_ string) (ports.Voice, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.voice, nil
}
func (r *stubVoiceRegistry) Has(_ string) bool { panic("not implemented") }
func (r *stubVoiceRegistry) List() []ports.Voice { panic("not implemented") }

func deliveryState(voiceName, response string, delivered bool) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{
			EventType: "user_message",
			MemoryKey: "user:1",
		},
		Spirit:            &entities.Spirit{},
		EventConfig:       &entities.Link{VoiceName: voiceName},
		Response:          response,
		ResponseDelivered: delivered,
	}
}

// --- ResponseDeliveryStep ---

func TestResponseDeliveryStep_AlreadyDelivered_NoOp(t *testing.T) {
	voice := &stubVoice{name: "whatsapp", autoRespond: true}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("whatsapp", "hello", true)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected SendResponse not called when already delivered")
	}
}

func TestResponseDeliveryStep_NoVoiceName_NoOp(t *testing.T) {
	voice := &stubVoice{name: "whatsapp", autoRespond: true}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("", "hello", false)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected no send when VoiceName is empty")
	}
}

func TestResponseDeliveryStep_NoResponse_NoOp(t *testing.T) {
	voice := &stubVoice{name: "whatsapp", autoRespond: true}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("whatsapp", "", false)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected no send when response is empty")
	}
}

func TestResponseDeliveryStep_RegistryNotFound_NoOp(t *testing.T) {
	registry := &stubVoiceRegistry{getErr: errors.New("channel not registered")}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("unknown", "hello", false)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected registry error swallowed, got: %v", err)
	}
}

func TestResponseDeliveryStep_ChannelDoesNotAutoRespond_NoOp(t *testing.T) {
	voice := &stubVoice{name: "webhook", autoRespond: false}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("webhook", "hello", false)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if voice.sendCalled {
		t.Error("expected no send when ShouldAutoRespond=false")
	}
}

func TestResponseDeliveryStep_AutoRespond_SendsAndSetsDelivered(t *testing.T) {
	voice := &stubVoice{name: "whatsapp", autoRespond: true}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("whatsapp", "hello there", false)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !voice.sendCalled {
		t.Error("expected SendResponse called")
	}
	if !state.ResponseDelivered {
		t.Error("expected ResponseDelivered=true after send")
	}
}

func TestResponseDeliveryStep_SendError_IsSwallowed(t *testing.T) {
	voice := &stubVoice{name: "whatsapp", autoRespond: true, sendErr: errors.New("timeout")}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))
	state := deliveryState("whatsapp", "hello", false)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected send error swallowed, got: %v", err)
	}
	if state.ResponseDelivered {
		t.Error("expected ResponseDelivered=false when send failed")
	}
}

func TestResponseDeliveryStep_ShouldForceResponse_FinalErrorWithIterations(t *testing.T) {
	step := &ResponseDeliveryStep{logger: testLogger(t)}
	state := &ProcessingState{
		ReasoningResult: &ReasoningResult{
			FinalError: "budget exceeded",
			Iterations: []entities.IterationLog{{Iteration: 1}},
		},
	}
	if !step.shouldForceResponse(state) {
		t.Error("expected shouldForceResponse=true when FinalError+Iterations present")
	}
}

func TestResponseDeliveryStep_ShouldForceResponse_NoReasoningResult(t *testing.T) {
	step := &ResponseDeliveryStep{logger: testLogger(t)}
	state := &ProcessingState{}
	if step.shouldForceResponse(state) {
		t.Error("expected shouldForceResponse=false when no reasoning result")
	}
}

func TestResponseDeliveryStep_ShouldForceResponse_NoFinalError(t *testing.T) {
	step := &ResponseDeliveryStep{logger: testLogger(t)}
	state := &ProcessingState{
		ReasoningResult: &ReasoningResult{
			Iterations: []entities.IterationLog{{Iteration: 1}},
		},
	}
	if step.shouldForceResponse(state) {
		t.Error("expected shouldForceResponse=false when no FinalError")
	}
}

func TestResponseDeliveryStep_Execute_ShouldForce_LogsWarn(t *testing.T) {
	voice := &stubVoice{name: "whatsapp", autoRespond: true}
	registry := &stubVoiceRegistry{voice: voice}
	step := NewResponseDeliveryStep(registry, time.Second, testLogger(t))

	state := deliveryState("whatsapp", "forced response text", false)
	state.ReasoningResult = &ReasoningResult{
		FinalError: "max iterations reached",
		Iterations: []entities.IterationLog{{Iteration: 1}},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !voice.sendCalled {
		t.Error("expected SendResponse called even on forced delivery")
	}
	if !state.ResponseDelivered {
		t.Error("expected ResponseDelivered=true after forced delivery")
	}
}
