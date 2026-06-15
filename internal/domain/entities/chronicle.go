package entities

import "time"

// ActionCallLog records a single Action execution within a reasoning iteration.
type ActionCallLog struct {
	ActionName   string         `bson:"action_name"`
	Arguments    map[string]any `bson:"arguments,omitempty"`
	Result       string         `bson:"result,omitempty"`
	IsError      bool           `bson:"is_error"`
	ErrorMessage string         `bson:"error_message,omitempty"`
	IsCritical   bool           `bson:"is_critical"`
	DurationMs   int64          `bson:"duration_ms"`
}

// IterationLog records one full Oracle request-response cycle, including all Action calls made.
type IterationLog struct {
	Iteration        int             `bson:"iteration"`
	OracleResponse   string          `bson:"oracle_response"`
	ActionCalls      []ActionCallLog `bson:"action_calls,omitempty"`
	BannedActions    []string        `bson:"banned_actions,omitempty"`
	ReflectionIssues []string        `bson:"reflection_issues,omitempty"`
	Errors           []string        `bson:"errors,omitempty"`
	PromptTokens     int             `bson:"prompt_tokens"`
	CompletionTokens int             `bson:"completion_tokens"`
	TotalTokens      int             `bson:"total_tokens"`
	DurationMs       int64           `bson:"duration_ms"`
}

// AttachmentLog records media artifacts received with the inbound Pulse.
type AttachmentLog struct {
	Type       ArtifactType `bson:"type"`
	MediaID    string       `bson:"media_id,omitempty"`
	MimeType   string       `bson:"mime_type,omitempty"`
	StorageURL string       `bson:"storage_url,omitempty"`
	Downloaded bool         `bson:"downloaded"`
}

// TokenUsageBreakdown holds prompt, completion, and total token counts for a single phase.
type TokenUsageBreakdown struct {
	PromptTokens     int `bson:"prompt_tokens"`
	CompletionTokens int `bson:"completion_tokens"`
	TotalTokens      int `bson:"total_tokens"`
}

// InteractionTokenUsage aggregates token usage across reasoning and media processing phases.
type InteractionTokenUsage struct {
	Reasoning TokenUsageBreakdown `bson:"reasoning"`
	Media     TokenUsageBreakdown `bson:"media"`
	Total     TokenUsageBreakdown `bson:"total"`
}

// InteractionEventLog captures the inbound Pulse state and the context forwarded to the LLM.
type InteractionEventLog struct {
	ID             string          `bson:"id"`
	Type           string          `bson:"type"`
	Source         string          `bson:"source"`
	SubType        string          `bson:"sub_type,omitempty"`
	IdempotencyKey string          `bson:"idempotency_key,omitempty"`
	ContactPhone   string          `bson:"contact_phone,omitempty"`
	SubjectKey     string          `bson:"subject_key,omitempty"`
	UserInput      string          `bson:"user_input"`
	Attachments    []AttachmentLog `bson:"attachments,omitempty"`
	Knowledge      map[string]any  `bson:"knowledge,omitempty"`
	ScoutsUsed     []string        `bson:"scouts_used,omitempty"`
	Payload        map[string]any  `bson:"payload,omitempty"`
	Metadata       map[string]any  `bson:"metadata,omitempty"`
}

// InteractionSpiritLog captures the Spirit and model that handled the Pulse.
type InteractionSpiritLog struct {
	ID          string  `bson:"id"`
	Name        string  `bson:"name"`
	Version     string  `bson:"version"`
	Provider    string  `bson:"provider"`
	Model       string  `bson:"model"`
	Temperature float64 `bson:"temperature"`
}

// InteractionProcessingLog records pipeline execution details: timings, iterations, and final outcome.
type InteractionProcessingLog struct {
	Status           string `bson:"status"` // "success" | "rate_limited" | "validation_failed" | ...
	ProcessingTimeMs int64  `bson:"processing_time_ms"`

	PipelineFailedAtStep string `bson:"pipeline_failed_at_step,omitempty"`
	PathfinderUsed       string `bson:"pathfinder_used,omitempty"`
	ArchivistApplied     bool   `bson:"archivist_applied,omitempty"`

	SessionMessagesCount   int `bson:"session_messages_count"`
	CoalescedMessagesCount int `bson:"coalesced_messages_count,omitempty"`

	Iterations      []IterationLog `bson:"iterations"`
	IterationsUsed  int            `bson:"iterations_used"`
	FinalResponse   string         `bson:"final_response"`
	FinalError      string         `bson:"final_error,omitempty"`
	ExecutionErrors []string       `bson:"execution_errors,omitempty"`

	ResponseDelivered bool   `bson:"response_delivered"`
	ResponseChannel   string `bson:"response_channel,omitempty"`
}

// Chronicle is the immutable audit record for a single Pulse processing cycle.
// One Chronicle is written per Pulse regardless of success or failure.
type Chronicle struct {
	ID        string    `bson:"_id,omitempty"`
	MemoryKey string    `bson:"memory_key"`
	EventKey  string    `bson:"event_key"`
	Timestamp time.Time `bson:"timestamp"`

	Event      InteractionEventLog      `bson:"event"`
	Spirit     InteractionSpiritLog     `bson:"spirit"`
	Processing InteractionProcessingLog `bson:"processing"`
	TokenUsage InteractionTokenUsage    `bson:"token_usage"`

	Metadata map[string]any `bson:"metadata,omitempty"`
}
