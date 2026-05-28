package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

type stubVoiceRegistry struct {
	voice ports.Voice
	err   error
}

var _ ports.VoiceRegistry = (*stubVoiceRegistry)(nil)

func (r *stubVoiceRegistry) Get(_ string) (ports.Voice, error) { return r.voice, r.err }
func (r *stubVoiceRegistry) Register(_ ports.Voice) error      { panic("not implemented") }
func (r *stubVoiceRegistry) RegisterMultiple(_ ...ports.Voice) error {
	panic("not implemented")
}
func (r *stubVoiceRegistry) Has(_ string) bool   { panic("not implemented") }
func (r *stubVoiceRegistry) List() []ports.Voice { panic("not implemented") }

type stubVoice struct {
	sendErr error
	sentTo  string
	sentMsg string
}

var _ ports.Voice = (*stubVoice)(nil)

func (v *stubVoice) SendResponse(_ context.Context, pulse *entities.Pulse, msg string) error {
	v.sentTo = pulse.ContactPhone
	v.sentMsg = msg
	return v.sendErr
}
func (v *stubVoice) GetName() string                                             { return "stub" }
func (v *stubVoice) ShouldAutoRespond() bool                                     { return false }
func (v *stubVoice) GetChannelMetadata(_ *entities.Pulse) map[string]any { return nil }

func linkAndPulseCtx(voiceName, contactPhone string) context.Context {
	link := &entities.Link{VoiceName: voiceName}
	pulse := &entities.Pulse{}
	pulse.ContactPhone = contactPhone
	ctx := context.WithValue(context.Background(), ports.LinkContextKey{}, link)
	ctx = context.WithValue(ctx, ports.PulseContextKey{}, pulse)
	return ctx
}

func TestSendToContactAction_Validate_EmptyMessage(t *testing.T) {
	a := NewSendToContactAction(&stubVoiceRegistry{})
	err := a.Validate(map[string]any{"message": ""})
	if err == nil {
		t.Errorf("expected error for empty message, got nil")
	}
}

func TestSendToContactAction_Validate_ValidMessage(t *testing.T) {
	a := NewSendToContactAction(&stubVoiceRegistry{})
	err := a.Validate(map[string]any{"message": "hello"})
	if err != nil {
		t.Errorf("expected no error for valid message, got %v", err)
	}
}

func TestSendToContactAction_Execute_NoLink(t *testing.T) {
	a := NewSendToContactAction(&stubVoiceRegistry{})
	result, err := a.Execute(context.Background(), map[string]any{"message": "hi"})
	if err == nil {
		t.Errorf("expected error when no link in context, got nil")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestSendToContactAction_Execute_NoVoiceOnLink(t *testing.T) {
	a := NewSendToContactAction(&stubVoiceRegistry{})
	ctx := linkAndPulseCtx("", "+5511999999999")
	_, err := a.Execute(ctx, map[string]any{"message": "hi"})
	if err == nil {
		t.Errorf("expected error when link has no VoiceName, got nil")
	}
}

func TestSendToContactAction_Execute_VoiceNotFound(t *testing.T) {
	registry := &stubVoiceRegistry{err: errors.New("not found")}
	a := NewSendToContactAction(registry)
	ctx := linkAndPulseCtx("whatsapp", "+5511999999999")
	_, err := a.Execute(ctx, map[string]any{"message": "hi"})
	if err == nil {
		t.Errorf("expected error when voice not found in registry, got nil")
	}
}

func TestSendToContactAction_Execute_SendsMessage(t *testing.T) {
	voice := &stubVoice{}
	a := NewSendToContactAction(&stubVoiceRegistry{voice: voice})
	ctx := linkAndPulseCtx("whatsapp", "+5511999999999")
	result, err := a.Execute(ctx, map[string]any{"message": "hello"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == "" {
		t.Errorf("expected non-empty result")
	}
	if len(result) < 9 || result[:9] != "Message d" {
		t.Errorf("expected result to contain 'delivered', got %q", result)
	}
}

func TestSendToContactAction_Execute_ContactKeyOverride(t *testing.T) {
	voice := &stubVoice{}
	a := NewSendToContactAction(&stubVoiceRegistry{voice: voice})
	ctx := linkAndPulseCtx("whatsapp", "+5511000000000")
	result, err := a.Execute(ctx, map[string]any{
		"message":     "hello",
		"contact_key": "override_phone",
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if voice.sentTo != "override_phone" {
		t.Errorf("expected sentTo to be 'override_phone', got %q", voice.sentTo)
	}
	found := false
	for i := 0; i+len("override_phone") <= len(result); i++ {
		if result[i:i+len("override_phone")] == "override_phone" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected result to contain 'override_phone', got %q", result)
	}
}

func TestSendToContactAction_Execute_SendError(t *testing.T) {
	voice := &stubVoice{sendErr: errors.New("send failed")}
	a := NewSendToContactAction(&stubVoiceRegistry{voice: voice})
	ctx := linkAndPulseCtx("whatsapp", "+5511999999999")
	_, err := a.Execute(ctx, map[string]any{"message": "hello"})
	if err == nil {
		t.Errorf("expected error when SendResponse fails, got nil")
	}
}

func TestSendToContactAction_Execute_NilPulseNoContactKey(t *testing.T) {
	voice := &stubVoice{}
	a := NewSendToContactAction(&stubVoiceRegistry{voice: voice})
	link := &entities.Link{VoiceName: "whatsapp"}
	ctx := context.WithValue(context.Background(), ports.LinkContextKey{}, link)
	_, err := a.Execute(ctx, map[string]any{"message": "hello"})
	if err == nil {
		t.Errorf("expected error when pulse is nil and no contact_key override, got nil")
	}
}

func TestSendToContactAction_Metadata(t *testing.T) {
	a := NewSendToContactAction(&stubVoiceRegistry{})
	if a.GetName() != "send_to_contact" {
		t.Errorf("GetName = %q, want %q", a.GetName(), "send_to_contact")
	}
	if a.GetDescription() == "" {
		t.Error("GetDescription returned empty")
	}
	if a.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if !a.IsCritical() {
		t.Error("IsCritical should return true")
	}
	if a.GetCategory() != ports.ActionDelivery {
		t.Errorf("GetCategory = %v, want ActionDelivery", a.GetCategory())
	}
}
