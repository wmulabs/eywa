package actions

import (
	"context"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

func sessionCtx(subjectKey string) context.Context {
	mem := &entities.Memory{SubjectKey: subjectKey}
	return context.WithValue(context.Background(), ports.SessionContextKey{}, mem)
}

func TestUpdateSubjectTool_Validate_MissingKey(t *testing.T) {
	tool := &UpdateSubjectTool{}
	err := tool.Validate(map[string]any{})
	if err == nil {
		t.Error("expected error for missing subject_key")
	}
}

func TestUpdateSubjectTool_Validate_EmptyKey(t *testing.T) {
	tool := &UpdateSubjectTool{}
	err := tool.Validate(map[string]any{"subject_key": ""})
	if err == nil {
		t.Error("expected error for empty subject_key")
	}
}

func TestUpdateSubjectTool_Validate_ValidKey(t *testing.T) {
	tool := &UpdateSubjectTool{}
	err := tool.Validate(map[string]any{"subject_key": "shipment:123"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateSubjectTool_Execute_NoSession(t *testing.T) {
	tool := &UpdateSubjectTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"subject_key": "shipment:123",
	})
	if err == nil {
		t.Error("expected error when session missing from context")
	}
	if result != "" {
		t.Errorf("expected empty result on error, got %q", result)
	}
}

func TestUpdateSubjectTool_Execute_FirstTimeSet(t *testing.T) {
	tool := &UpdateSubjectTool{}
	ctx := sessionCtx("")
	session := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)

	result, err := tool.Execute(ctx, map[string]any{
		"subject_key": "shipment:123",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if session.SubjectKey != "shipment:123" {
		t.Errorf("want subject_key=shipment:123, got %s", session.SubjectKey)
	}
	want := "subject updated to 'shipment:123'"
	if result != want {
		t.Errorf("want result=%q, got %q", want, result)
	}
}

func TestUpdateSubjectTool_Execute_TopicSwitch(t *testing.T) {
	tool := &UpdateSubjectTool{}
	ctx := sessionCtx("shipment:001")
	session := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	session.Summary = "previous summary"
	session.TopicFacts = map[string]any{"key": "value"}

	result, err := tool.Execute(ctx, map[string]any{
		"subject_key": "shipment:002",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if session.SubjectKey != "shipment:002" {
		t.Errorf("want subject_key=shipment:002, got %s", session.SubjectKey)
	}
	if session.Summary != "" {
		t.Errorf("want summary reset to empty string, got %q", session.Summary)
	}
	if len(session.TopicFacts) != 0 {
		t.Errorf("want topic_facts reset to empty, got %v", session.TopicFacts)
	}
	if want := "subject switched from 'shipment:001' to 'shipment:002'"; result != want {
		t.Errorf("want result=%q, got %q", want, result)
	}
}

func TestUpdateSubjectTool_Execute_MergesFacts(t *testing.T) {
	tool := &UpdateSubjectTool{}
	ctx := sessionCtx("shipment:123")
	session := ctx.Value(ports.SessionContextKey{}).(*entities.Memory)
	session.TopicFacts = map[string]any{"status": "pending"}

	result, err := tool.Execute(ctx, map[string]any{
		"subject_key": "shipment:123",
		"facts": map[string]any{
			"destination": "NYC",
			"weight":      "500kg",
		},
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if session.SubjectKey != "shipment:123" {
		t.Errorf("want subject_key=shipment:123, got %s", session.SubjectKey)
	}
	if session.TopicFacts["status"] != "pending" {
		t.Errorf("want existing fact preserved, got %v", session.TopicFacts)
	}
	if session.TopicFacts["destination"] != "NYC" {
		t.Errorf("want destination=NYC, got %v", session.TopicFacts["destination"])
	}
	if session.TopicFacts["weight"] != "500kg" {
		t.Errorf("want weight=500kg, got %v", session.TopicFacts["weight"])
	}
	if want := "subject updated to 'shipment:123'"; result != want {
		t.Errorf("want result=%q, got %q", want, result)
	}
}

func TestUpdateSubjectTool_Metadata(t *testing.T) {
	action := NewUpdateSubjectAction()
	if action == nil {
		t.Fatal("NewUpdateSubjectAction returned nil")
	}
	tool := &UpdateSubjectTool{}
	if tool.GetName() != "update_subject" {
		t.Errorf("GetName = %q, want %q", tool.GetName(), "update_subject")
	}
	if tool.GetDescription() == "" {
		t.Error("GetDescription returned empty string")
	}
	if tool.GetParameters() == nil {
		t.Error("GetParameters returned nil")
	}
	if tool.IsCritical() {
		t.Error("IsCritical should return false")
	}
	if tool.GetCategory() != ports.ActionGeneral {
		t.Errorf("GetCategory = %v, want ActionGeneral", tool.GetCategory())
	}
}

func TestUpdateSubjectTool_ApplyTopicUpdate_NilTopicFacts_WithFacts(t *testing.T) {
	tool := &UpdateSubjectTool{}
	session := &entities.Memory{SubjectKey: "order:1", TopicFacts: nil}
	tool.applyTopicUpdate(session, "order:1", false, map[string]any{
		"facts": map[string]any{"status": "active"},
	})
	if session.TopicFacts == nil {
		t.Fatal("TopicFacts should be initialized")
	}
	if session.TopicFacts["status"] != "active" {
		t.Errorf("TopicFacts[\"status\"] = %v, want %q", session.TopicFacts["status"], "active")
	}
}
