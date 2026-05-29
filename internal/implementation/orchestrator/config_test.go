package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestDefaultWeaveConfig_LockTTLGreaterThanReasoningTimeout(t *testing.T) {
	cfg := DefaultWeaveConfig()

	minExpected := cfg.ReasoningTimeout + 15*time.Second
	if cfg.LockTTL < minExpected {
		t.Errorf("LockTTL (%s) must be >= ReasoningTimeout (%s) + 15s, got less",
			cfg.LockTTL, cfg.ReasoningTimeout)
	}
}

func TestWeaveConfig_Validate_LockTTLTooShort_ReturnsError(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.LockTTL = cfg.ReasoningTimeout - time.Second

	err := cfg.Validate()

	if err == nil {
		t.Error("expected error when LockTTL < ReasoningTimeout + 15s, got nil")
	}
}

func TestWeaveConfig_Validate_LockTTLSufficient_ReturnsNil(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.LockTTL = cfg.ReasoningTimeout + 15*time.Second

	err := cfg.Validate()

	if err != nil {
		t.Errorf("expected no error with sufficient LockTTL, got: %v", err)
	}
}

func TestWeaveBuilder_WithConfig_ThenWithReasoningTimeout_PreservesLockTTL(t *testing.T) {
	cfg := DefaultWeaveConfig()
	cfg.LockTTL = 300 * time.Second
	cfg.ReasoningTimeout = 60 * time.Second

	builder := NewWeaveBuilder(context.Background())
	builder.WithConfig(cfg)
	builder.WithReasoningTimeout(90 * time.Second)

	if builder.config.LockTTL != 300*time.Second {
		t.Errorf("WithConfig LockTTL should be preserved after WithReasoningTimeout, got %s", builder.config.LockTTL)
	}
}

func TestWeaveBuilder_WithReasoningTimeout_WithoutExplicitLockTTL_DerivesLockTTL(t *testing.T) {
	builder := NewWeaveBuilder(context.Background())
	builder.WithReasoningTimeout(60 * time.Second)

	expected := 60*time.Second + 30*time.Second
	if builder.config.LockTTL != expected {
		t.Errorf("WithReasoningTimeout should auto-derive LockTTL=%s, got %s", expected, builder.config.LockTTL)
	}
}
