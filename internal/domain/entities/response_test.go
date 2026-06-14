package entities

import (
	"testing"
)

func TestNewResponse(t *testing.T) {
	r := NewResponse("evt-1", "chan:user", "spirit-a", []string{"action1", "action2"})
	if r == nil {
		t.Fatal("NewResponse returned nil")
	}
	if r.Status != ResponseSuccess {
		t.Errorf("Status = %q, want %q", r.Status, ResponseSuccess)
	}
	if r.EventID != "evt-1" {
		t.Errorf("EventID = %q, want %q", r.EventID, "evt-1")
	}
	if r.MemoryKey != "chan:user" {
		t.Errorf("MemoryKey = %q, want %q", r.MemoryKey, "chan:user")
	}
	if r.SpiritUsed != "spirit-a" {
		t.Errorf("SpiritUsed = %q, want %q", r.SpiritUsed, "spirit-a")
	}
	if len(r.ActionsExecuted) != 2 {
		t.Errorf("ActionsExecuted len = %d, want 2", len(r.ActionsExecuted))
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNewPartialResponse(t *testing.T) {
	results := []ActionResult{
		{ActionName: "send_msg", Success: false, Error: "timeout", ErrorType: "infrastructure"},
	}
	r := NewPartialResponse("evt-2", "chan:user", "spirit-b", []string{"send_msg"}, results)
	if r == nil {
		t.Fatal("NewPartialResponse returned nil")
	}
	if r.Status != ResponsePartial {
		t.Errorf("Status = %q, want %q", r.Status, ResponsePartial)
	}
	if r.EventID != "evt-2" {
		t.Errorf("EventID = %q, want %q", r.EventID, "evt-2")
	}
	if len(r.ActionResults) != 1 {
		t.Errorf("ActionResults len = %d, want 1", len(r.ActionResults))
	}
	if r.ActionResults[0].ActionName != "send_msg" {
		t.Errorf("ActionResults[0].ActionName = %q, want %q", r.ActionResults[0].ActionName, "send_msg")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNewDuplicateResponse(t *testing.T) {
	r := NewDuplicateResponse("evt-dup", "chan:user")
	if r == nil {
		t.Fatal("NewDuplicateResponse returned nil")
	}
	if r.Status != ResponseDuplicate {
		t.Errorf("Status = %q, want %q", r.Status, ResponseDuplicate)
	}
	if r.EventID != "evt-dup" {
		t.Errorf("EventID = %q, want %q", r.EventID, "evt-dup")
	}
	if r.MemoryKey != "chan:user" {
		t.Errorf("MemoryKey = %q, want %q", r.MemoryKey, "chan:user")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNewErrorResponse(t *testing.T) {
	r := NewErrorResponse("evt-3", "chan:user", "something went wrong")
	if r == nil {
		t.Fatal("NewErrorResponse returned nil")
	}
	if r.Status != ResponseFailed {
		t.Errorf("Status = %q, want %q", r.Status, ResponseFailed)
	}
	if r.EventID != "evt-3" {
		t.Errorf("EventID = %q, want %q", r.EventID, "evt-3")
	}
	if r.Error != "something went wrong" {
		t.Errorf("Error = %q, want %q", r.Error, "something went wrong")
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}
