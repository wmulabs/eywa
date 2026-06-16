package orchestrator

import (
	"context"
	"strings"

	"github.com/wmulabs/eywa/internal/domain/ports"
)

// ReasoningEventType classifies an incremental event surfaced by ExecuteStream.
type ReasoningEventType string

const (
	// ReasoningEventDelta carries a chunk of the model's generated text.
	ReasoningEventDelta ReasoningEventType = "delta"
	// ReasoningEventToolStatus signals that an Action is being executed (UI: "calling X…").
	ReasoningEventToolStatus ReasoningEventType = "tool_status"
	// ReasoningEventDone is the final event; it carries the assembled ReasoningResult.
	ReasoningEventDone ReasoningEventType = "done"
	// ReasoningEventError reports a terminal failure for the turn.
	ReasoningEventError ReasoningEventType = "error"
)

// ReasoningEvent is one incremental event from a streaming reasoning turn.
type ReasoningEvent struct {
	Type     ReasoningEventType
	Delta    string           // ReasoningEventDelta
	ToolName string           // ReasoningEventToolStatus
	Result   *ReasoningResult // ReasoningEventDone
	Err      error            // ReasoningEventError
}

// reasoningEmitter receives events during a streaming turn. It is nil for the buffered Execute path.
type reasoningEmitter func(ReasoningEvent)

const streamEventBuffer = 32

// ExecuteStream runs a reasoning turn and streams events as they occur. The terminal answer's tokens
// arrive as ReasoningEventDelta; tool executions arrive as ReasoningEventToolStatus; the turn ends
// with exactly one ReasoningEventDone (carrying the assembled ReasoningResult) or ReasoningEventError.
// Post-reasoning pipeline steps run on the assembled result in Done — identical to Execute.
func (r *ReasoningService) ExecuteStream(ctx context.Context, req *ReasoningRequest) (<-chan ReasoningEvent, error) {
	events := make(chan ReasoningEvent, streamEventBuffer)

	go func() {
		defer close(events)
		emit := func(ev ReasoningEvent) {
			select {
			case events <- ev:
			case <-ctx.Done():
			}
		}
		result, err := r.run(ctx, req, emit)
		if err != nil {
			emit(ReasoningEvent{Type: ReasoningEventError, Err: err, Result: result})
			return
		}
		emit(ReasoningEvent{Type: ReasoningEventDone, Result: result})
	}()

	return events, nil
}

// assembleStream consumes a provider stream, forwarding text deltas to emit and assembling the full
// OracleResponse (content + tool calls + usage) for the loop to process exactly as a buffered call.
func assembleStream(ch <-chan ports.StreamEvent, emit reasoningEmitter) (*ports.OracleResponse, error) {
	var content strings.Builder
	resp := &ports.OracleResponse{}
	for ev := range ch {
		switch ev.Type {
		case ports.StreamEventDelta:
			content.WriteString(ev.Delta)
			if emit != nil {
				emit(ReasoningEvent{Type: ReasoningEventDelta, Delta: ev.Delta})
			}
		case ports.StreamEventDone:
			resp.ToolCalls = ev.ToolCalls
			resp.TokensUsed = ev.Usage
			resp.StopReason = ev.StopReason
		case ports.StreamEventError:
			return nil, ev.Err
		}
	}
	resp.Content = content.String()
	return resp, nil
}
