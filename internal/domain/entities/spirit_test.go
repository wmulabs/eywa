package entities

import "testing"

func TestSpirit_IsConversational_EmptyType(t *testing.T) {
	s := Spirit{Type: ""}

	if !s.IsConversational() {
		t.Error("expected empty type to be conversational")
	}
}

func TestSpirit_IsConversational_ExplicitConversational(t *testing.T) {
	s := Spirit{Type: SpiritTypeConversational}

	if !s.IsConversational() {
		t.Error("expected conversational type to be conversational")
	}
}

func TestSpirit_IsConversational_Executor(t *testing.T) {
	s := Spirit{Type: SpiritTypeExecutor}

	if s.IsConversational() {
		t.Error("expected executor type to not be conversational")
	}
}

func TestSpirit_IsConversational_Notifier(t *testing.T) {
	s := Spirit{Type: SpiritTypeNotifier}

	if s.IsConversational() {
		t.Error("expected notifier type to not be conversational")
	}
}

func TestSpirit_IsConversational_Orchestrator(t *testing.T) {
	s := Spirit{Type: SpiritTypeOrchestrator}

	if s.IsConversational() {
		t.Error("expected orchestrator type to not be conversational")
	}
}

func TestSpirit_IsExecutor_True(t *testing.T) {
	s := Spirit{Type: SpiritTypeExecutor}

	if !s.IsExecutor() {
		t.Error("expected executor type to be executor")
	}
}

func TestSpirit_IsExecutor_False(t *testing.T) {
	s := Spirit{Type: SpiritTypeConversational}

	if s.IsExecutor() {
		t.Error("expected conversational type to not be executor")
	}
}

func TestSpirit_IsNotifier_True(t *testing.T) {
	s := Spirit{Type: SpiritTypeNotifier}

	if !s.IsNotifier() {
		t.Error("expected notifier type to be notifier")
	}
}

func TestSpirit_IsNotifier_False(t *testing.T) {
	s := Spirit{Type: SpiritTypeConversational}

	if s.IsNotifier() {
		t.Error("expected conversational type to not be notifier")
	}
}

func TestSpirit_IsOrchestrator_True(t *testing.T) {
	s := Spirit{Type: SpiritTypeOrchestrator}

	if !s.IsOrchestrator() {
		t.Error("expected orchestrator type to be orchestrator")
	}
}

func TestSpirit_IsOrchestrator_False(t *testing.T) {
	s := Spirit{Type: SpiritTypeConversational}

	if s.IsOrchestrator() {
		t.Error("expected conversational type to not be orchestrator")
	}
}

func TestSpirit_HasHandoff(t *testing.T) {
	if (Spirit{}).HasHandoff() {
		t.Error("expected no handoff without allowed targets")
	}
	s := Spirit{HandoffConfig: HandoffConfig{AllowedTargets: []string{"billing"}}}
	if !s.HasHandoff() {
		t.Error("expected handoff when allowed targets are set")
	}
}

func TestSpirit_NeedsSession_Conversational(t *testing.T) {
	s := Spirit{Type: SpiritTypeConversational}

	if !s.NeedsSession() {
		t.Error("expected conversational to need session")
	}
}

func TestSpirit_NeedsSession_Orchestrator(t *testing.T) {
	s := Spirit{Type: SpiritTypeOrchestrator}

	if !s.NeedsSession() {
		t.Error("expected orchestrator to need session")
	}
}

func TestSpirit_NeedsSession_ExecutorWithMemory(t *testing.T) {
	s := Spirit{
		Type: SpiritTypeExecutor,
		ExecutorConfig: ExecutorConfig{
			WithSessionMemory: true,
		},
	}

	if !s.NeedsSession() {
		t.Error("expected executor with session memory to need session")
	}
}

func TestSpirit_NeedsSession_ExecutorWithoutMemory(t *testing.T) {
	s := Spirit{
		Type: SpiritTypeExecutor,
		ExecutorConfig: ExecutorConfig{
			WithSessionMemory: false,
		},
	}

	if s.NeedsSession() {
		t.Error("expected executor without session memory to not need session")
	}
}

func TestSpirit_NeedsSession_Notifier(t *testing.T) {
	s := Spirit{Type: SpiritTypeNotifier}

	if s.NeedsSession() {
		t.Error("expected notifier to not need session")
	}
}

func TestSpirit_NeedsMessageCoalescing_Conversational(t *testing.T) {
	s := Spirit{Type: SpiritTypeConversational}

	if !s.NeedsMessageCoalescing() {
		t.Error("expected conversational to need message coalescing")
	}
}

func TestSpirit_NeedsMessageCoalescing_Executor(t *testing.T) {
	s := Spirit{Type: SpiritTypeExecutor}

	if s.NeedsMessageCoalescing() {
		t.Error("expected executor to not need message coalescing")
	}
}

func TestSpirit_NeedsMessageCoalescing_Notifier(t *testing.T) {
	s := Spirit{Type: SpiritTypeNotifier}

	if s.NeedsMessageCoalescing() {
		t.Error("expected notifier to not need message coalescing")
	}
}

func TestSpirit_NeedsMessageCoalescing_Orchestrator(t *testing.T) {
	s := Spirit{Type: SpiritTypeOrchestrator}

	if s.NeedsMessageCoalescing() {
		t.Error("expected orchestrator to not need message coalescing")
	}
}
