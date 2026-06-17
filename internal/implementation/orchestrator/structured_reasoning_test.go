package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

func personSchemaMap() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required":   []any{"name"},
	}
}

func structuredRequest() *ReasoningRequest {
	r := testRequest()
	r.ResponseFormat = &ports.ResponseFormat{Name: "person", Schema: personSchemaMap(), Strict: true}
	return r
}

// nativeStructuredOracle reports native structured support and returns its Structured payload.
type nativeStructuredOracle struct {
	*stubOracle
}

func (o *nativeStructuredOracle) SupportsStructuredOutput(_ string) bool { return true }

func structuredExecutor(t *testing.T) ActionExecutor {
	return NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
}

func TestFinalizeStructured_Fallback_ValidatesAndSetsStructured(t *testing.T) {
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: `{"name":"ann"}`, StopReason: ports.StopReasonComplete}}
	svc := newReasoning(t, multiFactory(oracle), structuredExecutor(t), 3)

	result, err := svc.Execute(context.Background(), structuredRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Structured) != `{"name":"ann"}` {
		t.Errorf("expected validated Structured, got %s", result.Structured)
	}
	if result.FinalResponse != `{"name":"ann"}` {
		t.Errorf("expected FinalResponse to be the JSON, got %q", result.FinalResponse)
	}
}

func TestFinalizeStructured_NativeProvider_UsesStructuredField(t *testing.T) {
	oracle := &nativeStructuredOracle{stubOracle: &stubOracle{resp: &ports.OracleResponse{
		Content:    "prose to ignore",
		Structured: json.RawMessage(`{"name":"bob"}`),
		StopReason: ports.StopReasonComplete,
	}}}
	svc := newReasoning(t, multiFactory(oracle), structuredExecutor(t), 3)

	result, err := svc.Execute(context.Background(), structuredRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Structured) != `{"name":"bob"}` {
		t.Errorf("expected native Structured payload, got %s", result.Structured)
	}
}

func TestFinalizeStructured_RepairsOnFirstInvalid(t *testing.T) {
	mo := &multiOracle{responses: []*ports.OracleResponse{
		{Content: "draft", StopReason: ports.StopReasonComplete}, // loop terminal
		{Content: "not json"},        // attempt 1: invalid
		{Content: `{"name":"cara"}`}, // attempt 2: valid after repair
	}}
	svc := newReasoning(t, multiFactory(mo), structuredExecutor(t), 3)

	result, err := svc.Execute(context.Background(), structuredRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Structured) != `{"name":"cara"}` {
		t.Errorf("expected repaired Structured, got %s", result.Structured)
	}
}

func TestFinalizeStructured_FailsAfterRepair_ReturnsRawLeavesStructuredEmpty(t *testing.T) {
	mo := &multiOracle{responses: []*ports.OracleResponse{
		{Content: "draft", StopReason: ports.StopReasonComplete},
		{Content: "nope"},
		{Content: "still bad"},
	}}
	svc := newReasoning(t, multiFactory(mo), structuredExecutor(t), 3)

	result, err := svc.Execute(context.Background(), structuredRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Structured) != 0 {
		t.Errorf("expected empty Structured on validation failure, got %s", result.Structured)
	}
	if result.FinalResponse != "still bad" {
		t.Errorf("expected the raw last answer in FinalResponse, got %q", result.FinalResponse)
	}
}

func TestFinalizeStructured_SynthesisCallError_DegradesGracefully(t *testing.T) {
	mo := &multiOracle{
		responses: []*ports.OracleResponse{{Content: "draft", StopReason: ports.StopReasonComplete}},
		errs:      []error{nil, errors.New("boom")},
	}
	svc := newReasoning(t, multiFactory(mo), structuredExecutor(t), 3)

	result, err := svc.Execute(context.Background(), structuredRequest())
	if err != nil {
		t.Fatalf("Execute must not fail when structured synthesis errors: %v", err)
	}
	if len(result.Structured) != 0 {
		t.Errorf("expected empty Structured on synthesis error, got %s", result.Structured)
	}
}

func TestProviderSupportsStructured(t *testing.T) {
	plain := &stubOracle{}
	if providerSupportsStructured(plain, "m") {
		t.Error("plain oracle should not report structured support")
	}
	native := &nativeStructuredOracle{stubOracle: &stubOracle{}}
	if !providerSupportsStructured(native, "m") {
		t.Error("native oracle should report structured support")
	}
}
