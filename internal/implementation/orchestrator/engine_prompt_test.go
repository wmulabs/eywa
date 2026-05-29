package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func TestBuildSystemPrompt_BasePromptOnly(t *testing.T) {
	spirit := &entities.Spirit{SystemPrompt: "You are helpful."}
	pulse := &entities.Pulse{EventType: "user_message", Source: "api"}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "You are helpful.") {
		t.Errorf("expected base prompt in result, got: %s", result)
	}
}

func TestBuildSystemPrompt_IncludesEventContext(t *testing.T) {
	spirit := &entities.Spirit{SystemPrompt: "base"}
	pulse := &entities.Pulse{EventType: "order_placed", Source: "webhook", SubType: "express"}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "order_placed") {
		t.Error("expected EventType in system prompt")
	}
	if !strings.Contains(result, "webhook") {
		t.Error("expected Source in system prompt")
	}
	if !strings.Contains(result, "express") {
		t.Error("expected SubType in system prompt")
	}
}

func TestBuildSystemPrompt_IncludesKnowledge(t *testing.T) {
	spirit := &entities.Spirit{SystemPrompt: "base"}
	pulse := &entities.Pulse{
		EventType: "user_message",
		Knowledge: map[string]any{"plan": "premium"},
	}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "premium") {
		t.Error("expected knowledge value in system prompt")
	}
}

func TestBuildSystemPrompt_EnforceVoiceDelivery_DefaultInstructions(t *testing.T) {
	spirit := &entities.Spirit{
		SystemPrompt:         "base",
		EnforceVoiceDelivery: true,
	}
	pulse := &entities.Pulse{EventType: "user_message"}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "CRITICAL INSTRUCTION") {
		t.Error("expected default voice delivery instructions when EnforceVoiceDelivery=true")
	}
}

func TestBuildSystemPrompt_EnforceVoiceDelivery_CustomInstructions(t *testing.T) {
	spirit := &entities.Spirit{
		SystemPrompt:              "base",
		EnforceVoiceDelivery:      true,
		VoiceDeliveryInstructions: "Always call send_message.",
	}
	pulse := &entities.Pulse{EventType: "user_message"}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "Always call send_message.") {
		t.Error("expected custom voice delivery instructions")
	}
}

func TestBuildSystemPrompt_NoEnforceVoiceDelivery_OmitsSection(t *testing.T) {
	spirit := &entities.Spirit{
		SystemPrompt:         "base",
		EnforceVoiceDelivery: false,
	}
	pulse := &entities.Pulse{EventType: "user_message"}

	result := buildSystemPrompt(spirit, pulse)

	if strings.Contains(result, "CRITICAL INSTRUCTION") {
		t.Error("expected no voice delivery section when EnforceVoiceDelivery=false")
	}
}

func TestBuildSystemPrompt_WithAllowedActions_IncludesBusinessErrorSection(t *testing.T) {
	spirit := &entities.Spirit{
		SystemPrompt:   "base",
		AllowedActions: []entities.AllowedAction{{Name: "search_lore"}},
	}
	pulse := &entities.Pulse{EventType: "user_message"}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "Business Error") {
		t.Error("expected business error section when spirit has AllowedActions")
	}
}

func TestBuildSystemPrompt_NoAllowedActions_OmitsBusinessErrorSection(t *testing.T) {
	spirit := &entities.Spirit{SystemPrompt: "base"}
	pulse := &entities.Pulse{EventType: "user_message"}

	result := buildSystemPrompt(spirit, pulse)

	if strings.Contains(result, "Business Error") {
		t.Error("expected no business error section for action-less spirit")
	}
}

func TestBuildSystemPrompt_CustomBusinessErrorInstructions(t *testing.T) {
	spirit := &entities.Spirit{
		SystemPrompt:              "base",
		AllowedActions:            []entities.AllowedAction{{Name: "search"}},
		BusinessErrorInstructions: "Custom error handling.",
	}
	pulse := &entities.Pulse{EventType: "user_message"}

	result := buildSystemPrompt(spirit, pulse)

	if !strings.Contains(result, "Custom error handling.") {
		t.Error("expected custom business error instructions")
	}
}

func TestBuildMessageHistory_MapsThreadsToOracleMessages(t *testing.T) {
	ts := time.Now()
	session := &entities.Memory{
		Threads: []entities.Thread{
			{Role: ports.RoleUser, Content: "hello", IsUserFacing: true},
			{Role: ports.RoleAssistant, Content: "hi there", IsUserFacing: true, ImageURLs: []string{"https://img.com/a.png"}},
		},
	}
	_ = ts

	messages := buildMessageHistory(session)

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != ports.RoleUser || messages[0].Content != "hello" {
		t.Errorf("first message mismatch: %+v", messages[0])
	}
	if messages[1].Role != ports.RoleAssistant || messages[1].Content != "hi there" {
		t.Errorf("second message mismatch: %+v", messages[1])
	}
	if len(messages[1].ImageURLs) != 1 || messages[1].ImageURLs[0] != "https://img.com/a.png" {
		t.Errorf("expected ImageURLs to be propagated, got %v", messages[1].ImageURLs)
	}
}

func TestBuildMessageHistory_EmptySession_ReturnsEmptySlice(t *testing.T) {
	session := &entities.Memory{Threads: []entities.Thread{}}
	messages := buildMessageHistory(session)
	if len(messages) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(messages))
	}
}
