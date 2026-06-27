package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/helpers"
)

func outputGuardState(response string) *ProcessingState {
	return &ProcessingState{
		Event:    &entities.Pulse{MemoryKey: "session:123"},
		Response: response,
	}
}

func TestOutputGuardStep_RedactsPII(t *testing.T) {
	step := NewOutputGuardStep(OutputGuardConfig{RedactPII: true}, time.Second, testLogger(t))
	state := outputGuardState("your agent is bob@acme.com, call +55 11 98765-4321")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "your agent is [REDACTED], call [REDACTED]"; state.Response != want {
		t.Errorf("response = %q, want %q", state.Response, want)
	}
}

func TestOutputGuardStep_PIIKindFilter(t *testing.T) {
	step := NewOutputGuardStep(
		OutputGuardConfig{RedactPII: true, PIIKinds: []helpers.PIIKind{helpers.PIIEmail}},
		time.Second, testLogger(t),
	)
	state := outputGuardState("mail bob@acme.com phone +55 11 98765-4321")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "mail [REDACTED] phone +55 11 98765-4321"; state.Response != want {
		t.Errorf("response = %q, want %q", state.Response, want)
	}
}

func TestOutputGuardStep_CustomMask(t *testing.T) {
	step := NewOutputGuardStep(OutputGuardConfig{RedactPII: true, RedactionMask: "##"}, time.Second, testLogger(t))
	state := outputGuardState("mail bob@acme.com")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "mail ##"; state.Response != want {
		t.Errorf("response = %q, want %q", state.Response, want)
	}
}

func TestOutputGuardStep_BlocksOnPattern(t *testing.T) {
	step := NewOutputGuardStep(
		OutputGuardConfig{BlockedPatterns: []string{`(?i)\bssn\b`}, BlockedMessage: "blocked"},
		time.Second, testLogger(t),
	)
	state := outputGuardState("your SSN is 123-45-6789")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Response != "blocked" {
		t.Errorf("response = %q, want blocked", state.Response)
	}
}

func TestOutputGuardStep_BlockTakesPrecedenceOverRedaction(t *testing.T) {
	step := NewOutputGuardStep(
		OutputGuardConfig{RedactPII: true, BlockedPatterns: []string{`(?i)forbidden`}},
		time.Second, testLogger(t),
	)
	state := outputGuardState("forbidden content for bob@acme.com")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Response != defaultOutputBlockedMessage {
		t.Errorf("response = %q, want default blocked message", state.Response)
	}
}

func TestOutputGuardStep_NoPII_Unchanged(t *testing.T) {
	step := NewOutputGuardStep(OutputGuardConfig{RedactPII: true}, time.Second, testLogger(t))
	state := outputGuardState("a perfectly ordinary answer")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Response != "a perfectly ordinary answer" {
		t.Errorf("response unexpectedly changed: %q", state.Response)
	}
}

func TestOutputGuardStep_EmptyResponse_NoOp(t *testing.T) {
	step := NewOutputGuardStep(OutputGuardConfig{RedactPII: true, BlockedPatterns: []string{".+"}}, time.Second, testLogger(t))
	state := outputGuardState("")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Response != "" {
		t.Errorf("empty response should stay empty, got %q", state.Response)
	}
}

func TestOutputGuardStep_SkipsInvalidPattern(t *testing.T) {
	// Validate normally rejects bad patterns; the constructor defends against any that slip through
	// by skipping them rather than panicking.
	step := NewOutputGuardStep(
		OutputGuardConfig{BlockedPatterns: []string{"(", `(?i)bad`}},
		time.Second, testLogger(t),
	)
	state := outputGuardState("this is bad")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Response != defaultOutputBlockedMessage {
		t.Errorf("valid pattern should still block: %q", state.Response)
	}
}

func TestOutputGuardConfig_Enabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  OutputGuardConfig
		want bool
	}{
		{"off", OutputGuardConfig{}, false},
		{"redact", OutputGuardConfig{RedactPII: true}, true},
		{"blocklist", OutputGuardConfig{BlockedPatterns: []string{"x"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.enabled(); got != c.want {
				t.Errorf("enabled() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestWeaveConfig_Validate_RejectsBadBlockedPattern(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.OutputGuard.BlockedPatterns = []string{"("}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid blocked pattern")
	}
}
