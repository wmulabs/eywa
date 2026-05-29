package entities

import (
	"testing"
	"time"
)

func TestEcho_ToThread_MapsAllFields(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	echo := &Echo{
		Role:         "user",
		Content:      "hello",
		Timestamp:    ts,
		IsUserFacing: true,
		ToolCallID:   "tool-123",
		ImageURLs:    []string{"https://example.com/img.png"},
		AudioURLs:    []string{"https://example.com/audio.mp3"},
		Metadata:     map[string]any{"channel": "whatsapp"},
	}

	thread := echo.ToThread()

	if thread.Role != echo.Role {
		t.Errorf("expected Role '%s', got '%s'", echo.Role, thread.Role)
	}
	if thread.Content != echo.Content {
		t.Errorf("expected Content '%s', got '%s'", echo.Content, thread.Content)
	}
	if thread.Timestamp != echo.Timestamp {
		t.Errorf("expected Timestamp %v, got %v", echo.Timestamp, thread.Timestamp)
	}
	if thread.IsUserFacing != echo.IsUserFacing {
		t.Errorf("expected IsUserFacing %v, got %v", echo.IsUserFacing, thread.IsUserFacing)
	}
	if thread.ToolCallID != echo.ToolCallID {
		t.Errorf("expected ToolCallID '%s', got '%s'", echo.ToolCallID, thread.ToolCallID)
	}
	if len(thread.ImageURLs) != 1 || thread.ImageURLs[0] != echo.ImageURLs[0] {
		t.Errorf("expected ImageURLs %v, got %v", echo.ImageURLs, thread.ImageURLs)
	}
	if len(thread.AudioURLs) != 1 || thread.AudioURLs[0] != echo.AudioURLs[0] {
		t.Errorf("expected AudioURLs %v, got %v", echo.AudioURLs, thread.AudioURLs)
	}
	if thread.Metadata["channel"] != "whatsapp" {
		t.Errorf("expected Metadata channel 'whatsapp', got '%v'", thread.Metadata["channel"])
	}
}

func TestEcho_ToThread_DoesNotIncludeIDOrMemoryKey(t *testing.T) {
	echo := &Echo{
		ID:        "echo-id-123",
		MemoryKey: "user:123",
		Role:      "assistant",
		Content:   "response",
	}

	thread := echo.ToThread()

	if thread.Role != "assistant" {
		t.Errorf("expected Role 'assistant', got '%s'", thread.Role)
	}
	if thread.Content != "response" {
		t.Errorf("expected Content 'response', got '%s'", thread.Content)
	}
}

func TestEcho_ToThread_EmptyOptionalFields(t *testing.T) {
	echo := &Echo{
		Role:    "user",
		Content: "ping",
	}

	thread := echo.ToThread()

	if thread.ToolCallID != "" {
		t.Errorf("expected empty ToolCallID, got '%s'", thread.ToolCallID)
	}
	if len(thread.ImageURLs) != 0 {
		t.Errorf("expected empty ImageURLs, got %v", thread.ImageURLs)
	}
	if len(thread.AudioURLs) != 0 {
		t.Errorf("expected empty AudioURLs, got %v", thread.AudioURLs)
	}
	if thread.Metadata != nil {
		t.Errorf("expected nil Metadata, got %v", thread.Metadata)
	}
}
