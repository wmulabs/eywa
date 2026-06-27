package orchestrator

import (
	"testing"
	"time"
)

// These tests call Name() and Timeout() on every pipeline step struct.
// The methods are trivial one-liners but they were unreachable via existing tests.

func TestSpiritSelectionStep_NameTimeout(t *testing.T) {
	s := &SpiritSelectionStep{timeout: 5 * time.Second}
	if s.Name() != "SpiritSelection" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestSpiritLoadStep_NameTimeout(t *testing.T) {
	s := &SpiritLoadStep{timeout: 3 * time.Second}
	if s.Name() != "SpiritLoad" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestConditionEvaluationStep_NameTimeout(t *testing.T) {
	s := &ConditionEvaluationStep{timeout: 2 * time.Second}
	if s.Name() != "ConditionEvaluation" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 2*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestCostEnforcementStep_NameTimeout(t *testing.T) {
	s := &CostEnforcementStep{timeout: 4 * time.Second}
	if s.Name() != "CostEnforcement" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 4*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestLedgerUpdateStep_NameTimeout(t *testing.T) {
	s := &LedgerUpdateStep{timeout: 3 * time.Second}
	if s.Name() != "LedgerUpdate" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestResponseDeliveryStep_NameTimeout(t *testing.T) {
	s := &ResponseDeliveryStep{timeout: 10 * time.Second}
	if s.Name() != "ResponseDelivery" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 10*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestOutputGuardStep_NameTimeout(t *testing.T) {
	s := &OutputGuardStep{timeout: 6 * time.Second}
	if s.Name() != "OutputGuard" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 6*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestTypingStartStep_NameTimeout(t *testing.T) {
	s := &TypingStartStep{}
	if s.Name() != "TypingStart" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestTypingStopStep_NameTimeout(t *testing.T) {
	s := &TypingStopStep{}
	if s.Name() != "TypingStop" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestLockAcquisitionStep_NameTimeout(t *testing.T) {
	s := &LockAcquisitionStep{timeout: 5 * time.Second}
	if s.Name() != "LockAcquisition" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestLockReleaseStep_NameTimeout(t *testing.T) {
	s := &LockReleaseStep{timeout: 3 * time.Second}
	if s.Name() != "LockRelease" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestPersistenceStep_NameTimeout(t *testing.T) {
	s := &PersistenceStep{timeout: 5 * time.Second}
	if s.Name() != "Persistence" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestAuditLogStep_NameTimeout(t *testing.T) {
	s := &AuditLogStep{timeout: 5 * time.Second}
	if s.Name() != "AuditLog" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestSessionSetupStep_NameTimeout(t *testing.T) {
	s := &SessionSetupStep{timeout: 5 * time.Second}
	if s.Name() != "SessionSetup" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestArchivistStep_NameTimeout(t *testing.T) {
	s := &ArchivistStep{timeout: 5 * time.Second}
	if s.Name() != "Archivist" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestMessageCoalescingStep_NameTimeout(t *testing.T) {
	s := &MessageCoalescingStep{timeout: 5 * time.Second}
	if s.Name() != "MessageCoalescing" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestVigilCheckStep_NameTimeout(t *testing.T) {
	s := &VigilCheckStep{}
	if s.Name() != "VigilCheck" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestModelRoutingStep_NameTimeout(t *testing.T) {
	s := &ModelRoutingStep{timeout: 2 * time.Second}
	if s.Name() != "ModelRouting" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 2*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestValidationStep_NameTimeout(t *testing.T) {
	s := &ValidationStep{timeout: 2 * time.Second}
	if s.Name() != "Validation" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 2*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestRateLimitStep_NameTimeout(t *testing.T) {
	s := &RateLimitStep{timeout: 2 * time.Second}
	if s.Name() != "RateLimit" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 2*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestFilterStep_NameTimeout(t *testing.T) {
	s := &FilterStep{timeout: 2 * time.Second}
	if s.Name() != "Filter" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 2*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestHTTPToolLoadStep_NameTimeout(t *testing.T) {
	s := &HTTPToolLoadStep{timeout: 3 * time.Second}
	if s.Name() != "HTTPToolLoad" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestImprintExtractionStep_NameTimeout(t *testing.T) {
	s := &ImprintExtractionStep{}
	if s.Name() != "ImprintExtraction" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 0 {
		t.Errorf("Timeout = %v, want 0 (async)", s.Timeout())
	}
}

func TestLinkScoutStep_NameTimeout(t *testing.T) {
	s := &LinkScoutStep{timeout: 4 * time.Second}
	if s.Name() != "LinkScout" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 4*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestSpiritScoutStep_NameTimeout(t *testing.T) {
	s := &SpiritScoutStep{timeout: 4 * time.Second}
	if s.Name() != "SpiritScout" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 4*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestOrchestratorRoutingStep_NameTimeout(t *testing.T) {
	s := &OrchestratorRoutingStep{timeout: 5 * time.Second}
	if s.Name() != "OrchestratorRouting" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestReasoningStep_NameTimeout(t *testing.T) {
	s := &ReasoningStep{timeout: 30 * time.Second}
	if s.Name() != "Reasoning" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 30*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestMediaVaultStep_NameTimeout(t *testing.T) {
	s := &MediaVaultStep{timeout: 10 * time.Second}
	if s.Name() != "MediaVault" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 10*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestMediaProcessingStep_NameTimeout(t *testing.T) {
	s := &MediaProcessingStep{timeout: 10 * time.Second}
	if s.Name() != "MediaProcessing" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 10*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestNotificationStep_NameTimeout(t *testing.T) {
	s := &NotificationStep{timeout: 5 * time.Second}
	if s.Name() != "Notification" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestRitualMarkStep_NameTimeout(t *testing.T) {
	s := &RitualMarkStep{timeout: 3 * time.Second}
	if s.Name() != "RitualMark" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}

func TestMarkExecutedStep_NameTimeout(t *testing.T) {
	s := &MarkExecutedStep{timeout: 3 * time.Second}
	if s.Name() != "MarkExecuted" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("Timeout = %v", s.Timeout())
	}
}
