package entities

// Reasoning policy configuration. These are plain data structs (no behavior) describing how the
// reasoning loop should behave. They live in the domain so a Spirit can carry per-agent overrides
// (see ReasoningOverrides); the loop logic that consumes them stays in the orchestrator.

// ProgressPolicy configures stall detection for the reasoning loop.
type ProgressPolicy struct {
	Enabled     bool `bson:"enabled" json:"enabled"`
	StallWindow int  `bson:"stall_window" json:"stall_window"`
}

// CompressionPolicy bounds the reasoning working-context size by summarizing the oldest completed
// iterations into an evidence ledger.
type CompressionPolicy struct {
	Enabled         bool `bson:"enabled" json:"enabled"`
	MaxContextChars int  `bson:"max_context_chars" json:"max_context_chars"`
	KeepRecent      int  `bson:"keep_recent" json:"keep_recent"`
}

// ReflectionPolicy enables a self-critique pass before a draft answer is delivered.
type ReflectionPolicy struct {
	Enabled   bool     `bson:"enabled" json:"enabled"`
	MaxRounds int      `bson:"max_rounds" json:"max_rounds"`
	Model     string   `bson:"model,omitempty" json:"model,omitempty"`
	Criteria  []string `bson:"criteria,omitempty" json:"criteria,omitempty"`
}

// GroundingViolationAction selects what happens when a RAG answer fails to cite its sources.
type GroundingViolationAction string

const (
	GroundingReviseOnce GroundingViolationAction = "revise_once"
	GroundingAnnotate   GroundingViolationAction = "annotate"
	GroundingBlock      GroundingViolationAction = "block"
)

// GroundingPolicy enforces source citation for Spirits that retrieve Lore (RAG).
type GroundingPolicy struct {
	Enabled        bool                     `bson:"enabled" json:"enabled"`
	MinCitations   int                      `bson:"min_citations" json:"min_citations"`
	OnViolation    GroundingViolationAction `bson:"on_violation,omitempty" json:"on_violation,omitempty"`
	BlockedMessage string                   `bson:"blocked_message,omitempty" json:"blocked_message,omitempty"`
}

// PlanPolicy enables a turn-scoped plan/scratchpad the model maintains via the update_plan action.
type PlanPolicy struct {
	Enabled  bool `bson:"enabled" json:"enabled"`
	MaxItems int  `bson:"max_items" json:"max_items"`
	Required bool `bson:"required" json:"required"`
}

// Confidence is a coarse, rule-based assessment of how trustworthy a turn's answer is.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// HandoffMode selects what happens when a turn's confidence falls below the threshold.
type HandoffMode string

const (
	HandoffRaiseVigil   HandoffMode = "raise_vigil"
	HandoffAnnotateOnly HandoffMode = "annotate"
)

// HandoffPolicy escalates low-confidence turns to a human instead of shipping a weak answer.
type HandoffPolicy struct {
	Enabled            bool        `bson:"enabled" json:"enabled"`
	MinConfidence      Confidence  `bson:"min_confidence,omitempty" json:"min_confidence,omitempty"`
	Mode               HandoffMode `bson:"mode,omitempty" json:"mode,omitempty"`
	HoldingMessage     string      `bson:"holding_message,omitempty" json:"holding_message,omitempty"`
	UncertaintyMarkers []string    `bson:"uncertainty_markers,omitempty" json:"uncertainty_markers,omitempty"`
}

// ReasoningOverrides lets a Spirit override any subset of the reasoning policies. A nil field uses
// the global WeaveConfig default; a nil bundle means the Spirit uses all global defaults.
type ReasoningOverrides struct {
	Progress    *ProgressPolicy    `bson:"progress,omitempty" json:"progress,omitempty"`
	Compression *CompressionPolicy `bson:"compression,omitempty" json:"compression,omitempty"`
	Reflection  *ReflectionPolicy  `bson:"reflection,omitempty" json:"reflection,omitempty"`
	Grounding   *GroundingPolicy   `bson:"grounding,omitempty" json:"grounding,omitempty"`
	Plan        *PlanPolicy        `bson:"plan,omitempty" json:"plan,omitempty"`
	Handoff     *HandoffPolicy     `bson:"handoff,omitempty" json:"handoff,omitempty"`
}
