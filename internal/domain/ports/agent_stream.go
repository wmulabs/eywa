package ports

// AgentStreamEventType classifies an incremental event from a streaming agent turn.
type AgentStreamEventType string

const (
	// AgentStreamDelta carries a chunk of the agent's generated answer.
	AgentStreamDelta AgentStreamEventType = "delta"
	// AgentStreamToolStatus signals that an Action is being executed (UI: "calling X…").
	AgentStreamToolStatus AgentStreamEventType = "tool_status"
	// AgentStreamDone is the final event; it carries the assembled answer.
	AgentStreamDone AgentStreamEventType = "done"
	// AgentStreamError reports a terminal failure for the turn.
	AgentStreamError AgentStreamEventType = "error"
)

// AgentStreamEvent is one incremental event from a streaming agent turn, surfaced by the streaming
// Weave entry point. Token deltas arrive as AgentStreamDelta; tool executions as AgentStreamToolStatus;
// the turn ends with exactly one AgentStreamDone (carrying the assembled answer) or AgentStreamError.
type AgentStreamEvent struct {
	Type     AgentStreamEventType
	Delta    string // AgentStreamDelta
	ToolName string // AgentStreamToolStatus
	Response string // AgentStreamDone — the full assembled answer
	Err      error  // AgentStreamError
}
