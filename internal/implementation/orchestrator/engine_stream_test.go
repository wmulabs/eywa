package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// stubStreamingStep populates a successful state and, when streaming, emits answer deltas and a
// tool-status event through the sink — mirroring what a streaming Reasoning step would surface.
type stubStreamingStep struct{}

func (s *stubStreamingStep) Name() string           { return "stub_streaming" }
func (s *stubStreamingStep) Timeout() time.Duration { return 0 }
func (s *stubStreamingStep) Execute(_ context.Context, state *ProcessingState) error {
	state.Spirit = &entities.Spirit{Name: "test-spirit"}
	if state.streamSink != nil {
		state.streamSink(ReasoningEvent{Type: ReasoningEventDelta, Delta: "All "})
		state.streamSink(ReasoningEvent{Type: ReasoningEventToolStatus, ToolName: "greet"})
		state.streamSink(ReasoningEvent{Type: ReasoningEventDelta, Delta: "good"})
	}
	state.ReasoningResult = &ReasoningResult{IterationsUsed: 1}
	state.Response = "All good"
	return nil
}

func collectAgentFrames(ch <-chan ports.AgentStreamEvent) (deltas, tools []string, done, errFrame *ports.AgentStreamEvent) {
	for ev := range ch {
		e := ev
		switch ev.Type {
		case ports.AgentStreamDelta:
			deltas = append(deltas, ev.Delta)
		case ports.AgentStreamToolStatus:
			tools = append(tools, ev.ToolName)
		case ports.AgentStreamDone:
			done = &e
		case ports.AgentStreamError:
			errFrame = &e
		}
	}
	return
}

func newStreamingWeave(t *testing.T) *Weave {
	t.Helper()
	e := newPipelineWeave(t)
	p := NewPipeline(testLogger(t), noopTracer())
	p.AddStep(&stubStreamingStep{})
	e.pipeline = p
	return e
}

func TestProcessEventByKeyStream_EventNotFound(t *testing.T) {
	e := minimalWeave()
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1"}
	if _, err := e.ProcessEventByKeyStream(context.Background(), "unknown", pulse); err == nil {
		t.Fatal("expected error for missing event config")
	}
}

func TestProcessEventByKeyStream_Success_StreamsDeltasThenDone(t *testing.T) {
	e := newStreamingWeave(t)
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-ok", UserMessage: "hello"}

	ch, err := e.ProcessEventByKeyStream(context.Background(), "test_event", pulse)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deltas, tools, done, errFrame := collectAgentFrames(ch)
	if errFrame != nil {
		t.Fatalf("unexpected error frame: %v", errFrame.Err)
	}
	if strings.Join(deltas, "") != "All good" {
		t.Errorf("expected deltas to assemble to 'All good', got %v", deltas)
	}
	if len(tools) != 1 || tools[0] != "greet" {
		t.Errorf("expected one tool status 'greet', got %v", tools)
	}
	if done == nil || done.Response != "All good" {
		t.Errorf("expected Done carrying the assembled answer, got %+v", done)
	}
}

func TestProcessEventByKeyStream_PipelineError_EmitsErrorFrame(t *testing.T) {
	e := newPipelineWeave(t) // fails at SpiritLoad
	pulse := &entities.Pulse{MemoryKey: "user:1", ID: "evt-1", UserMessage: "hi"}

	ch, err := e.ProcessEventByKeyStream(context.Background(), "test_event", pulse)
	if err != nil {
		t.Fatalf("ProcessEventByKeyStream must not fail synchronously: %v", err)
	}

	_, _, done, errFrame := collectAgentFrames(ch)
	if errFrame == nil {
		t.Fatal("expected an error frame when the pipeline fails")
	}
	if done != nil {
		t.Error("must not emit Done after an error frame")
	}
}
