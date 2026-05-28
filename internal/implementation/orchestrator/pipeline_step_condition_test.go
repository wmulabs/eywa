package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

func notifierSpirit(conds ...entities.NotifierCondition) *entities.Spirit {
	return &entities.Spirit{
		Type:           entities.SpiritTypeNotifier,
		NotifierConfig: entities.NotifierConfig{Conditions: conds},
	}
}

func TestConditionStep_NilSpirit_Passes(t *testing.T) {
	step := NewConditionEvaluationStep(time.Second, testLogger(t))
	state := &ProcessingState{Spirit: nil, Event: &entities.Pulse{}}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("state.Skipped must be false")
	}
}

func TestConditionStep_NonNotifier_Passes(t *testing.T) {
	step := NewConditionEvaluationStep(time.Second, testLogger(t))
	spirit := &entities.Spirit{Type: entities.SpiritTypeConversational}
	state := &ProcessingState{Spirit: spirit, Event: &entities.Pulse{}}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("state.Skipped must be false")
	}
}

func TestConditionStep_NoConditions_Passes(t *testing.T) {
	step := NewConditionEvaluationStep(time.Second, testLogger(t))
	spirit := notifierSpirit()
	state := &ProcessingState{Spirit: spirit, Event: &entities.Pulse{}}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("state.Skipped must be false")
	}
}

func TestConditionStep_EqCondition_Passes(t *testing.T) {
	step := NewConditionEvaluationStep(time.Second, testLogger(t))
	spirit := notifierSpirit(entities.NotifierCondition{
		Field:    "knowledge.status",
		Operator: "eq",
		Value:    "active",
	})
	state := &ProcessingState{
		Spirit: spirit,
		Event: &entities.Pulse{
			Knowledge: map[string]any{"status": "active"},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if state.Skipped {
		t.Error("state.Skipped must be false")
	}
}

func TestConditionStep_EqCondition_Fails_SetsSkipped(t *testing.T) {
	step := NewConditionEvaluationStep(time.Second, testLogger(t))
	spirit := notifierSpirit(entities.NotifierCondition{
		Field:    "knowledge.status",
		Operator: "eq",
		Value:    "active",
	})
	state := &ProcessingState{
		Spirit: spirit,
		Event: &entities.Pulse{
			Knowledge: map[string]any{"status": "inactive"},
		},
	}
	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !state.Skipped {
		t.Error("state.Skipped must be true")
	}
	if state.ProcessingStatus != "skipped" {
		t.Errorf("state.ProcessingStatus = %q, want %q", state.ProcessingStatus, "skipped")
	}
}

func TestEvaluateCondition_Exists_NonNil(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "exists"}
	if !evaluateCondition(cond, "any_value") {
		t.Error("exists operator must return true for non-nil value")
	}
}

func TestEvaluateCondition_Exists_Nil(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "exists"}
	if evaluateCondition(cond, nil) {
		t.Error("exists operator must return false for nil value")
	}
}

func TestEvaluateCondition_NotExists_Nil(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "not_exists"}
	if !evaluateCondition(cond, nil) {
		t.Error("not_exists operator must return true for nil value")
	}
}

func TestEvaluateCondition_NotExists_NonNil(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "not_exists"}
	if evaluateCondition(cond, "any_value") {
		t.Error("not_exists operator must return false for non-nil value")
	}
}

func TestEvaluateCondition_Eq_Match(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "eq", Value: "expected"}
	if !evaluateCondition(cond, "expected") {
		t.Error("eq operator must return true when values match")
	}
}

func TestEvaluateCondition_Eq_NoMatch(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "eq", Value: "expected"}
	if evaluateCondition(cond, "different") {
		t.Error("eq operator must return false when values do not match")
	}
}

func TestEvaluateCondition_Neq_Match(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "neq", Value: "unexpected"}
	if !evaluateCondition(cond, "actual") {
		t.Error("neq operator must return true when values differ")
	}
}

func TestEvaluateCondition_Contains_Match(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "contains", Value: "world"}
	if !evaluateCondition(cond, "hello world") {
		t.Error("contains operator must return true when substring is found")
	}
}

func TestEvaluateCondition_Gt_True(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "gt", Value: 10}
	if !evaluateCondition(cond, 15) {
		t.Error("gt operator must return true when actual > expected")
	}
}

func TestEvaluateCondition_Gt_False(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "gt", Value: 10}
	if evaluateCondition(cond, 5) {
		t.Error("gt operator must return false when actual < expected")
	}
}

func TestEvaluateCondition_Gte_Equal(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "gte", Value: 10}
	if !evaluateCondition(cond, 10) {
		t.Error("gte operator must return true when actual == expected")
	}
}

func TestEvaluateCondition_Lt_True(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "lt", Value: 10}
	if !evaluateCondition(cond, 5) {
		t.Error("lt operator must return true when actual < expected")
	}
}

func TestEvaluateCondition_In_StringSlice(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "in", Value: []string{"apple", "banana", "cherry"}}
	if !evaluateCondition(cond, "banana") {
		t.Error("in operator must return true when value is in slice")
	}
}

func TestEvaluateCondition_NotIn(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "not_in", Value: []string{"apple", "banana"}}
	if !evaluateCondition(cond, "cherry") {
		t.Error("not_in operator must return true when value is not in slice")
	}
}

func TestEvaluateCondition_UnknownOperator_ReturnsFalse(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "unknown_op"}
	if evaluateCondition(cond, "any_value") {
		t.Error("unknown operator must return false")
	}
}

func TestEvaluateCondition_Neq_NoMatch(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "neq", Value: "same"}
	if evaluateCondition(cond, "same") {
		t.Error("neq operator must return false when values are equal")
	}
}

func TestEvaluateCondition_Contains_NoMatch(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "contains", Value: "missing"}
	if evaluateCondition(cond, "hello world") {
		t.Error("contains operator must return false when substring is absent")
	}
}

func TestEvaluateCondition_Lte_Equal(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "lte", Value: 10}
	if !evaluateCondition(cond, 10) {
		t.Error("lte operator must return true when actual == expected")
	}
}

func TestEvaluateCondition_In_InterfaceSlice(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "in", Value: []any{"red", "green", "blue"}}
	if !evaluateCondition(cond, "green") {
		t.Error("in operator must return true when value is in []any slice")
	}
}

func TestEvaluateCondition_In_InterfaceSlice_NotFound(t *testing.T) {
	cond := entities.NotifierCondition{Operator: "in", Value: []any{"red", "green"}}
	if evaluateCondition(cond, "blue") {
		t.Error("in operator must return false when value is not in []any slice")
	}
}

func TestToFloat_Float32(t *testing.T) {
	v, ok := toFloat(float32(3.14))
	if !ok {
		t.Error("expected ok=true for float32")
	}
	if v == 0 {
		t.Error("expected non-zero float64 from float32 input")
	}
}

func TestToFloat_Int(t *testing.T) {
	v, ok := toFloat(int(42))
	if !ok {
		t.Error("expected ok=true for int")
	}
	if v != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestToFloat_Int32(t *testing.T) {
	v, ok := toFloat(int32(7))
	if !ok {
		t.Error("expected ok=true for int32")
	}
	if v != 7 {
		t.Errorf("expected 7, got %v", v)
	}
}

func TestToFloat_Int64(t *testing.T) {
	v, ok := toFloat(int64(100))
	if !ok {
		t.Error("expected ok=true for int64")
	}
	if v != 100 {
		t.Errorf("expected 100, got %v", v)
	}
}

func TestToFloat_UnknownType_ReturnsFalse(t *testing.T) {
	_, ok := toFloat("not_a_number")
	if ok {
		t.Error("expected ok=false for string input")
	}
}

func TestResolveKnowledgeField_UserMessage(t *testing.T) {
	pulse := &entities.Pulse{UserMessage: "hello"}
	val := resolveKnowledgeField(pulse, "user_message")
	if val != "hello" {
		t.Errorf("expected 'hello', got '%v'", val)
	}
}

func TestResolveKnowledgeField_EventType(t *testing.T) {
	pulse := &entities.Pulse{EventType: "user_message"}
	val := resolveKnowledgeField(pulse, "event_type")
	if val != "user_message" {
		t.Errorf("expected 'user_message', got '%v'", val)
	}
}

func TestResolveKnowledgeField_Source(t *testing.T) {
	pulse := &entities.Pulse{Source: "whatsapp"}
	val := resolveKnowledgeField(pulse, "source")
	if val != "whatsapp" {
		t.Errorf("expected 'whatsapp', got '%v'", val)
	}
}

func TestResolveKnowledgeField_ContactPhone(t *testing.T) {
	pulse := &entities.Pulse{ContactPhone: "+5511999999999"}
	val := resolveKnowledgeField(pulse, "contact_phone")
	if val != "+5511999999999" {
		t.Errorf("expected '+5511999999999', got '%v'", val)
	}
}

func TestResolveKnowledgeField_BareKey_Found(t *testing.T) {
	pulse := &entities.Pulse{
		Knowledge: map[string]any{"plan": "premium"},
	}
	val := resolveKnowledgeField(pulse, "plan")
	if val != "premium" {
		t.Errorf("expected 'premium', got '%v'", val)
	}
}

func TestResolveKnowledgeField_BareKey_Missing(t *testing.T) {
	pulse := &entities.Pulse{
		Knowledge: map[string]any{},
	}
	val := resolveKnowledgeField(pulse, "missing_key")
	if val != nil {
		t.Errorf("expected nil for missing key, got '%v'", val)
	}
}

func TestResolveKnowledgeField_KnowledgeDotKey(t *testing.T) {
	pulse := &entities.Pulse{
		Knowledge: map[string]any{"user_id": "abc123"},
	}
	val := resolveKnowledgeField(pulse, "knowledge.user_id")
	if val != "abc123" {
		t.Errorf("expected 'abc123', got '%v'", val)
	}
}

func TestResolveKnowledgeField_KnowledgeWithoutSubkey(t *testing.T) {
	knowledge := map[string]any{"a": 1}
	pulse := &entities.Pulse{Knowledge: knowledge}
	val := resolveKnowledgeField(pulse, "knowledge")
	asMap, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", val)
	}
	if asMap["a"] != 1 {
		t.Errorf("expected knowledge map with a=1, got %v", asMap)
	}
}

func TestToFloat_Float64(t *testing.T) {
	got, ok := toFloat(float64(3.14))
	if !ok || got != 3.14 {
		t.Errorf("toFloat(float64(3.14)) = %v %v, want 3.14 true", got, ok)
	}
}

func TestCompareNumeric_NonNumericValues_ReturnsFalse(t *testing.T) {
	if compareNumeric("gt", "not-a-number", "also-not") {
		t.Error("compareNumeric with non-numeric values should return false")
	}
}

func TestCompareNumeric_UnknownOperator_ReturnsFalse(t *testing.T) {
	if compareNumeric("unknown_op", 5, 3) {
		t.Error("compareNumeric with unknown operator should return false")
	}
}
