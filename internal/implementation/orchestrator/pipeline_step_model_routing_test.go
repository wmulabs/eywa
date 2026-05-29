package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func routingState(msg string, spirit *entities.Spirit) *ProcessingState {
	return &ProcessingState{
		Event:       &entities.Pulse{EventType: "user_message", MemoryKey: "user:1", UserMessage: msg},
		Spirit:      spirit,
		EventConfig: &entities.Link{},
	}
}

func TestModelRoutingStep_NoRules_NoOp(t *testing.T) {
	step := NewModelRoutingStep(nil, time.Second, testLogger(t))
	spirit := &entities.Spirit{ModelConfig: entities.SpiritModel{Model: "gpt-4"}}
	state := routingState("hi", spirit)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "gpt-4" {
		t.Error("expected model unchanged when no rules")
	}
}

func TestModelRoutingStep_NilSpirit_NoOp(t *testing.T) {
	rules := []entities.ModelRoutingRule{{
		Name:      "always",
		Condition: entities.ModelRoutingCondition{},
		Model:     entities.SpiritModel{Model: "gpt-3"},
	}}
	step := NewModelRoutingStep(rules, time.Second, testLogger(t))
	state := routingState("hi", nil)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelRoutingStep_MatchingRule_ReplacesModel(t *testing.T) {
	rules := []entities.ModelRoutingRule{{
		Name:      "long-input",
		Condition: entities.ModelRoutingCondition{InputLengthGte: 10},
		Model:     entities.SpiritModel{Model: "claude-3"},
	}}
	step := NewModelRoutingStep(rules, time.Second, testLogger(t))
	spirit := &entities.Spirit{ModelConfig: entities.SpiritModel{Model: "gpt-4"}}
	state := routingState("this is a long message", spirit)

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Spirit.ModelConfig.Model != "claude-3" {
		t.Errorf("expected model updated to claude-3, got %s", state.Spirit.ModelConfig.Model)
	}
}

func TestModelRoutingStep_FirstMatchWins(t *testing.T) {
	rules := []entities.ModelRoutingRule{
		{
			Name:      "long-input",
			Condition: entities.ModelRoutingCondition{InputLengthGte: 5},
			Model:     entities.SpiritModel{Model: "first-match"},
		},
		{
			Name:      "also-long",
			Condition: entities.ModelRoutingCondition{InputLengthGte: 5},
			Model:     entities.SpiritModel{Model: "second-match"},
		},
	}
	step := NewModelRoutingStep(rules, time.Second, testLogger(t))
	spirit := &entities.Spirit{ModelConfig: entities.SpiritModel{Model: "default"}}
	state := routingState("this is long enough", spirit)

	_ = step.Execute(context.Background(), state)

	if state.Spirit.ModelConfig.Model != "first-match" {
		t.Errorf("expected first matching rule to win, got %s", state.Spirit.ModelConfig.Model)
	}
}

func TestModelRoutingStep_NoMatch_ModelUnchanged(t *testing.T) {
	rules := []entities.ModelRoutingRule{{
		Name:      "short-only",
		Condition: entities.ModelRoutingCondition{InputLengthLt: 5},
		Model:     entities.SpiritModel{Model: "cheap"},
	}}
	step := NewModelRoutingStep(rules, time.Second, testLogger(t))
	spirit := &entities.Spirit{ModelConfig: entities.SpiritModel{Model: "premium"}}
	state := routingState("this message is too long for the rule", spirit)

	_ = step.Execute(context.Background(), state)

	if state.Spirit.ModelConfig.Model != "premium" {
		t.Errorf("expected model unchanged on no match, got %s", state.Spirit.ModelConfig.Model)
	}
}

// --- matchesRoutingCondition ---

func TestMatchesRoutingCondition_InputLengthGte_Pass(t *testing.T) {
	cond := entities.ModelRoutingCondition{InputLengthGte: 5}
	state := routingState("hello world", &entities.Spirit{})
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected condition to match for long input")
	}
}

func TestMatchesRoutingCondition_InputLengthGte_Fail(t *testing.T) {
	cond := entities.ModelRoutingCondition{InputLengthGte: 100}
	state := routingState("short", &entities.Spirit{})
	if matchesRoutingCondition(cond, state) {
		t.Error("expected condition to not match for short input")
	}
}

func TestMatchesRoutingCondition_InputLengthLt_Pass(t *testing.T) {
	cond := entities.ModelRoutingCondition{InputLengthLt: 100}
	state := routingState("short", &entities.Spirit{})
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected condition to match for input shorter than limit")
	}
}

func TestMatchesRoutingCondition_InputLengthLt_Fail(t *testing.T) {
	cond := entities.ModelRoutingCondition{InputLengthLt: 3}
	state := routingState("this is longer than three", &entities.Spirit{})
	if matchesRoutingCondition(cond, state) {
		t.Error("expected condition to not match when input >= limit")
	}
}

func TestMatchesRoutingCondition_HasAttachments_Pass(t *testing.T) {
	cond := entities.ModelRoutingCondition{HasAttachments: true}
	state := routingState("", &entities.Spirit{})
	state.Event.Attachments = []*entities.Artifact{{URL: "https://img.com/a.png"}}
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected condition to match when attachments present")
	}
}

func TestMatchesRoutingCondition_HasAttachments_Fail(t *testing.T) {
	cond := entities.ModelRoutingCondition{HasAttachments: true}
	state := routingState("", &entities.Spirit{})
	if matchesRoutingCondition(cond, state) {
		t.Error("expected condition to not match when no attachments")
	}
}

func TestMatchesRoutingCondition_EventType_Match(t *testing.T) {
	cond := entities.ModelRoutingCondition{EventType: "order_placed"}
	state := routingState("", &entities.Spirit{})
	state.Event.EventType = "order_placed"
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected condition to match on event type")
	}
}

func TestMatchesRoutingCondition_EventType_NoMatch(t *testing.T) {
	cond := entities.ModelRoutingCondition{EventType: "order_placed"}
	state := routingState("", &entities.Spirit{})
	state.Event.EventType = "user_message"
	if matchesRoutingCondition(cond, state) {
		t.Error("expected condition to not match on different event type")
	}
}

func TestMatchesRoutingCondition_TopicPrefix_Match(t *testing.T) {
	cond := entities.ModelRoutingCondition{TopicPrefix: "support:"}
	state := routingState("", &entities.Spirit{})
	state.Event.SubjectKey = "support:billing"
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected condition to match on topic prefix")
	}
}

func TestMatchesRoutingCondition_UserTier_Match(t *testing.T) {
	cond := entities.ModelRoutingCondition{UserTier: "premium"}
	state := routingState("", &entities.Spirit{})
	state.Event.Knowledge = map[string]any{"user_tier": "premium"}
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected condition to match on user tier")
	}
}

func TestMatchesRoutingCondition_UserTier_NoMatch(t *testing.T) {
	cond := entities.ModelRoutingCondition{UserTier: "premium"}
	state := routingState("", &entities.Spirit{})
	state.Event.Knowledge = map[string]any{"user_tier": "free"}
	if matchesRoutingCondition(cond, state) {
		t.Error("expected condition to not match on different user tier")
	}
}

func TestMatchesRoutingCondition_TopicPrefix_NoMatch(t *testing.T) {
	cond := entities.ModelRoutingCondition{TopicPrefix: "support:"}
	state := routingState("any", &entities.Spirit{})
	state.Event.SubjectKey = "billing:invoice" // different prefix
	if matchesRoutingCondition(cond, state) {
		t.Error("expected condition to not match when topic prefix doesn't match")
	}
}

func TestMatchesRoutingCondition_EmptyCondition_AlwaysMatches(t *testing.T) {
	cond := entities.ModelRoutingCondition{}
	state := routingState("any message", &entities.Spirit{})
	if !matchesRoutingCondition(cond, state) {
		t.Error("expected empty condition to always match")
	}
}
