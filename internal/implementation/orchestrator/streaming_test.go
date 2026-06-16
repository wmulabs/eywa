package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// streamStubOracle implements both Oracle and StreamingOracle, replaying a scripted sequence of
// StreamEvents per call.
type streamStubOracle struct {
	stubOracle
	scripts [][]ports.StreamEvent
	call    int
}

var _ ports.StreamingOracle = (*streamStubOracle)(nil)

func (o *streamStubOracle) GenerateStream(_ context.Context, _ *ports.OracleRequest) (<-chan ports.StreamEvent, error) {
	evs := o.scripts[o.call]
	o.call++
	ch := make(chan ports.StreamEvent, len(evs))
	for _, e := range evs {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func collectEvents(ch <-chan ReasoningEvent) (deltas, tools []string, done *ReasoningResult, gotErr error) {
	for ev := range ch {
		switch ev.Type {
		case ReasoningEventDelta:
			deltas = append(deltas, ev.Delta)
		case ReasoningEventToolStatus:
			tools = append(tools, ev.ToolName)
		case ReasoningEventDone:
			done = ev.Result
		case ReasoningEventError:
			gotErr = ev.Err
		}
	}
	return
}

func TestExecuteStream_StreamsFinalAnswerTokens(t *testing.T) {
	oracle := &streamStubOracle{scripts: [][]ports.StreamEvent{
		{
			{Type: ports.StreamEventDelta, Delta: "Hel"},
			{Type: ports.StreamEventDelta, Delta: "lo"},
			{Type: ports.StreamEventDone, StopReason: ports.StopReasonComplete, Usage: ports.OracleUsage{TotalTokens: 3}},
		},
	}}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	ch, err := svc.ExecuteStream(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deltas, _, done, gotErr := collectEvents(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if strings.Join(deltas, "") != "Hello" {
		t.Errorf("expected streamed deltas to assemble to 'Hello', got %v", deltas)
	}
	if done == nil || done.FinalResponse != "Hello" {
		t.Errorf("expected Done with assembled final 'Hello', got %+v", done)
	}
}

func TestExecuteStream_ToolThenAnswer(t *testing.T) {
	action := &stubAction{name: "lookup", execResult: "data", category: ports.ActionRetrieval}
	exec := NewActionExecutor(newRegistry(action), false, testLogger(t), noopTracer())
	oracle := &streamStubOracle{scripts: [][]ports.StreamEvent{
		{ // iteration 1: tool call, no text
			{Type: ports.StreamEventDone, StopReason: ports.StopReasonToolCalls,
				ToolCalls: []ports.OracleToolCall{{ID: "c", Name: "lookup", Arguments: map[string]any{}}}},
		},
		{ // iteration 2: streamed final answer
			{Type: ports.StreamEventDelta, Delta: "done!"},
			{Type: ports.StreamEventDone, StopReason: ports.StopReasonComplete},
		},
	}}
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	ch, err := svc.ExecuteStream(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deltas, tools, done, gotErr := collectEvents(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if len(tools) != 1 || tools[0] != "lookup" {
		t.Errorf("expected one tool-status for lookup, got %v", tools)
	}
	if strings.Join(deltas, "") != "done!" || done.FinalResponse != "done!" {
		t.Errorf("expected final 'done!', deltas=%v done=%+v", deltas, done)
	}
}

func TestExecuteStream_FallbackNonStreamingProvider(t *testing.T) {
	// Plain stubOracle does not implement StreamingOracle -> buffered; the answer arrives in Done.
	oracle := &stubOracle{resp: &ports.OracleResponse{Content: "buffered answer", StopReason: ports.StopReasonComplete}}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	ch, err := svc.ExecuteStream(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, done, gotErr := collectEvents(ch)
	if gotErr != nil {
		t.Fatalf("stream error: %v", gotErr)
	}
	if done == nil || done.FinalResponse != "buffered answer" {
		t.Errorf("fallback must still deliver the answer in Done, got %+v", done)
	}
}

func TestExecuteStream_ProviderError_EmitsError(t *testing.T) {
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	factory := &stubOracleFactory{err: errors.New("no provider")}
	svc := newReasoning(t, factory, exec, 5)

	ch, err := svc.ExecuteStream(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("ExecuteStream itself must not fail synchronously: %v", err)
	}
	_, _, _, gotErr := collectEvents(ch)
	if gotErr == nil {
		t.Error("expected an error event when the provider cannot be resolved")
	}
}

func TestExecuteStream_MidStreamError_EmitsError(t *testing.T) {
	oracle := &streamStubOracle{scripts: [][]ports.StreamEvent{
		{
			{Type: ports.StreamEventDelta, Delta: "partial"},
			{Type: ports.StreamEventError, Err: errors.New("stream broke")},
		},
	}}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())
	svc := newReasoning(t, multiFactory(oracle), exec, 5)

	ch, _ := svc.ExecuteStream(context.Background(), testRequest())
	_, _, _, gotErr := collectEvents(ch)
	if gotErr == nil {
		t.Error("expected an error event when the stream fails mid-flight")
	}
}

func TestExecuteStream_MatchesExecuteResult(t *testing.T) {
	// Golden: streamed assembled result equals the buffered Execute result for the same content.
	newOracle := func() ports.Oracle {
		return &stubOracle{resp: &ports.OracleResponse{Content: "same answer", StopReason: ports.StopReasonComplete}}
	}
	exec := NewActionExecutor(newRegistry(), false, testLogger(t), noopTracer())

	bufSvc := newReasoning(t, multiFactory(newOracle()), exec, 5)
	bufResult, err := bufSvc.Execute(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	strSvc := newReasoning(t, multiFactory(newOracle()), exec, 5)
	ch, _ := strSvc.ExecuteStream(context.Background(), testRequest())
	_, _, streamResult, _ := collectEvents(ch)

	if streamResult.FinalResponse != bufResult.FinalResponse {
		t.Errorf("stream result %q must match buffered %q", streamResult.FinalResponse, bufResult.FinalResponse)
	}
}
