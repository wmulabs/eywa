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

type stubLimiter struct {
	allowed bool
	err     error
}

var _ ports.Limiter = (*stubLimiter)(nil)

func (l *stubLimiter) Allow(_ context.Context, _ string) (bool, error) {
	return l.allowed, l.err
}

// --- RateLimitStep ---

func TestRateLimitStep_Allowed_NoError(t *testing.T) {
	step := NewRateLimitStep(&stubLimiter{allowed: true}, time.Second, testLogger(t))
	state := &ProcessingState{Event: &entities.Pulse{MemoryKey: "user:1"}}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimitStep_NotAllowed_ReturnsRateLimitError(t *testing.T) {
	step := NewRateLimitStep(&stubLimiter{allowed: false}, time.Second, testLogger(t))
	state := &ProcessingState{Event: &entities.Pulse{MemoryKey: "user:1"}}

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Error("expected error when rate limit exceeded")
	}
	if !IsRateLimited(err) {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestRateLimitStep_LimiterError_FailsOpen(t *testing.T) {
	step := NewRateLimitStep(&stubLimiter{err: errors.New("redis down")}, time.Second, testLogger(t))
	state := &ProcessingState{Event: &entities.Pulse{MemoryKey: "user:1"}}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected fail-open on limiter error, got: %v", err)
	}
}

// --- resolveEventField ---

func TestResolveEventField_TopLevelFields(t *testing.T) {
	event := &entities.Pulse{
		ContactPhone:   "+5511999",
		MemoryKey:      "user:1",
		Source:         "whatsapp",
		SubType:        "express",
		EventType:      "order_placed",
		SubjectKey:     "billing",
		IdempotencyKey: "idem-123",
		UserMessage:    "hello",
	}
	cases := []struct {
		field    string
		expected string
	}{
		{"contact_phone", "+5511999"},
		{"memory_key", "user:1"},
		{"source", "whatsapp"},
		{"sub_type", "express"},
		{"event_type", "order_placed"},
		{"subject_key", "billing"},
		{"idempotency_key", "idem-123"},
		{"user_message", "hello"},
		{"unknown_field", ""},
	}
	for _, tc := range cases {
		got := resolveEventField(event, tc.field)
		if got != tc.expected {
			t.Errorf("resolveEventField(%q) = %q, want %q", tc.field, got, tc.expected)
		}
	}
}

func TestResolveEventField_KnowledgeKey(t *testing.T) {
	event := &entities.Pulse{Knowledge: map[string]any{"plan": "premium"}}
	got := resolveEventField(event, "knowledge.plan")
	if got != "premium" {
		t.Errorf("expected 'premium', got %q", got)
	}
}

func TestResolveEventField_MetadataKey(t *testing.T) {
	event := &entities.Pulse{Metadata: map[string]any{"channel": "mobile"}}
	got := resolveEventField(event, "metadata.channel")
	if got != "mobile" {
		t.Errorf("expected 'mobile', got %q", got)
	}
}

func TestResolveEventField_PayloadKey(t *testing.T) {
	event := &entities.Pulse{Payload: map[string]any{"order_id": "ORD-42"}}
	got := resolveEventField(event, "payload.order_id")
	if got != "ORD-42" {
		t.Errorf("expected 'ORD-42', got %q", got)
	}
}

func TestResolveEventField_UnknownPrefix_ReturnsEmpty(t *testing.T) {
	event := &entities.Pulse{}
	got := resolveEventField(event, "headers.x-custom")
	if got != "" {
		t.Errorf("expected empty for unknown prefix, got %q", got)
	}
}

// --- resolveMapPath ---

func TestResolveMapPath_StringValue(t *testing.T) {
	m := map[string]any{"city": "São Paulo"}
	got := resolveMapPath(m, "city")
	if got != "São Paulo" {
		t.Errorf("expected 'São Paulo', got %q", got)
	}
}

func TestResolveMapPath_MissingKey_ReturnsEmpty(t *testing.T) {
	m := map[string]any{}
	got := resolveMapPath(m, "missing")
	if got != "" {
		t.Errorf("expected empty for missing key, got %q", got)
	}
}

func TestResolveMapPath_NilValue_ReturnsEmpty(t *testing.T) {
	m := map[string]any{"key": nil}
	got := resolveMapPath(m, "key")
	if got != "" {
		t.Errorf("expected empty for nil value, got %q", got)
	}
}

func TestResolveMapPath_NonStringNonStringer_ReturnsEmpty(t *testing.T) {
	m := map[string]any{"count": 42}
	got := resolveMapPath(m, "count")
	if got != "" {
		t.Errorf("expected empty for non-string non-Stringer value, got %q", got)
	}
}

func TestResolveMapPath_NestedMap(t *testing.T) {
	m := map[string]any{
		"address": map[string]any{
			"city": "Bogotá",
		},
	}
	got := resolveMapPath(m, "address.city")
	if got != "Bogotá" {
		t.Errorf("expected 'Bogotá', got %q", got)
	}
}

func TestResolveMapPath_NestedPath_NonMapIntermediate_ReturnsEmpty(t *testing.T) {
	m := map[string]any{"address": "flat-string-not-a-map"}
	got := resolveMapPath(m, "address.city")
	if got != "" {
		t.Errorf("expected empty when intermediate is not a map, got %q", got)
	}
}

type stringerVal struct{ s string }

func (sv stringerVal) String() string { return sv.s }

func TestResolveMapPath_StringerValue(t *testing.T) {
	m := map[string]any{"status": stringerVal{"active"}}
	got := resolveMapPath(m, "status")
	if got != "active" {
		t.Errorf("expected 'active' from Stringer, got %q", got)
	}
}

// --- NewArchivistStep keepRecent clamp ---

func TestArchivistStep_KeepRecentBelowOne_ClampsToOne(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: "summary"}, 3, 0, time.Second, testLogger(t))
	session := archivistSession(5)
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Session: session,
	}

	_ = step.Execute(context.Background(), state)

	// keepRecent was 0, clamped to 1 → 4 summarized, 1 kept
	if len(state.Session.Threads) != 1 {
		t.Errorf("expected 1 thread kept after keepRecent clamp, got %d", len(state.Session.Threads))
	}
}

// --- ValidationStep ---

type stubValidator struct{ err error }

var _ EventValidatorIface = (*stubValidator)(nil)

func (v *stubValidator) Validate(_ context.Context, _ *entities.Pulse) error {
	return v.err
}

func (v *stubValidator) ValidateEventConfiguration(_ *entities.Link) error {
	return v.err
}

func TestValidationStep_Execute_Success(t *testing.T) {
	step := NewValidationStep(&stubValidator{}, time.Second, testLogger(t))
	if step.Name() != "Validation" {
		t.Errorf("Name = %q", step.Name())
	}
	state := &ProcessingState{Event: &entities.Pulse{}}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidationStep_Execute_Error(t *testing.T) {
	step := NewValidationStep(&stubValidator{err: errors.New("invalid")}, time.Second, testLogger(t))
	state := &ProcessingState{Event: &entities.Pulse{}}
	if err := step.Execute(context.Background(), state); err == nil {
		t.Error("expected error, got nil")
	}
}
