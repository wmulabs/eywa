package eywa

import (
	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/implementation/orchestrator"
)

type (
	// Spirit is an AI assistant definition stored in the SpiritRepository. It carries the
	// system prompt, model configuration, and the list of Actions the Oracle may invoke.
	// Spirits are versioned — every update creates a new version, allowing rollback.
	Spirit = entities.Spirit

	// SpiritModel specifies which Oracle provider and model a Spirit uses, along with
	// generation parameters (Temperature, MaxTokens, TopP).
	SpiritModel = entities.SpiritModel

	// ActionDefinition describes a tool schema as it is sent to an Oracle provider.
	ActionDefinition = entities.ActionDefinition

	// Pulse is the unit of work flowing through the Weave — an inbound event from any
	// channel. Build one with NewPulse and send it via Weave.ProcessEventByKey.
	Pulse = entities.Pulse

	// PulseBuilder constructs a Pulse with a fluent API. Obtain one via NewPulse.
	PulseBuilder = entities.PulseBuilder

	// MemoryKey uniquely identifies a conversation context. Channel is the inbound
	// channel name (e.g. "whatsapp", "api"); User is the end-user identifier.
	MemoryKey = entities.MemoryKey

	// SubjectKey identifies the business entity being discussed in a conversation
	// (e.g. an order ID or ticket number). Set by the update_subject built-in action.
	SubjectKey = entities.SubjectKey

	// Link wires an event key to Scouts, a Pathfinder, and allowed Spirits.
	// Build with NewLink and register via Weave.RegisterEventConfiguration.
	Link = entities.Link

	// Guard holds allow/block rules for a single field (e.g. ContactPhone).
	Guard = entities.Guard

	// CompiledGuard is a pre-compiled Guard ready for fast evaluation in the pipeline.
	CompiledGuard = entities.CompiledGuard

	// Memory is the ephemeral per-user conversation state held in Redis. It contains
	// the recent message Thread and any Scout-injected Knowledge.
	Memory = entities.Memory

	// Thread is the ordered list of messages in the current conversation window.
	// When the window fills, the Archivist summarises older messages.
	Thread = entities.Thread

	// Echo is a single persisted message record (user or assistant) stored in MongoDB.
	Echo = entities.Echo

	// Artifact is a media attachment on an Echo — image, audio, video, or document.
	Artifact = entities.Artifact

	// ArtifactType classifies the media kind of an Artifact.
	ArtifactType = entities.ArtifactType

	// Response is the result returned by Weave.ProcessEventByKey. It contains the
	// assistant message, which Spirit was used, Actions executed, and token usage.
	Response = entities.Response

	// ResponseStatus classifies the outcome of a Weave processing call.
	ResponseStatus = entities.ResponseStatus

	// ActionResult records the outcome of a single Action (tool call) execution.
	ActionResult = entities.ActionResult

	// Ritual is a scheduled or recurring event managed by the Keeper. Created by the
	// schedule_ritual built-in action; delivered back as a Pulse at the scheduled time.
	Ritual = entities.Ritual

	// RitualStatus is the lifecycle state of a Ritual.
	RitualStatus = entities.RitualStatus

	// RecurrenceConfig defines cron-based recurrence for a Ritual (timezone, expression,
	// max runs, end date).
	RecurrenceConfig = entities.RecurrenceConfig

	// Chronicle is an immutable audit record written after every Weave processing call.
	// Query via ChronicleQueryRepository or the /chronicle management API.
	Chronicle = entities.Chronicle

	// SpiritType controls which built-in capabilities a Spirit has.
	// Defaults to SpiritTypeConversational.
	SpiritType = entities.SpiritType

	// ConversationalConfig holds optional tuning for conversational Spirits.
	ConversationalConfig = entities.ConversationalConfig

	// ExecutorConfig configures batch-execution Spirits that run without a user turn.
	ExecutorConfig = entities.ExecutorConfig

	// NotifierConfig configures proactive-notification Spirits that fire on conditions.
	NotifierConfig = entities.NotifierConfig

	// NotifierCondition is a trigger rule evaluated by a NotifierConfig Spirit.
	NotifierCondition = entities.NotifierCondition

	// OrchestratorConfig configures a SpiritTypeOrchestrator. SubSpirits lists the
	// names the orchestrator may summon; MaxDepth caps delegation chains.
	OrchestratorConfig = entities.OrchestratorConfig

	// Channel pairs an inbound Receptor with an outbound Voice under a shared name.
	Channel = orchestrator.Channel

	// Lore is a named knowledge base that Spirits can search via the search_lore action.
	// Ingest documents as LoreChunks; the LoreEmbedder indexes them for retrieval.
	Lore = entities.Lore

	// LoreChunk is a single document fragment stored in a Lore knowledge base.
	LoreChunk = entities.LoreChunk

	// LoreIngestion tracks the status of an async Lore ingestion job.
	LoreIngestion = entities.LoreIngestion

	// Imprint is a persistent user fact stored across sessions. Spirits read Imprints
	// as context before each response; the remember_fact action creates new ones.
	Imprint = entities.Imprint

	// ImprintExtractionConfig controls automatic fact extraction. When Enabled, the
	// engine extracts Imprints from user messages without requiring an explicit Action.
	ImprintExtractionConfig = orchestrator.ImprintExtractionConfig

	// TokenBudget sets a monthly token limit for a Spirit with configurable enforcement:
	// "block" (reject when exceeded), "downgrade" (switch to a cheaper model), or "alert".
	TokenBudget = entities.TokenBudget

	// LedgerEntry records token usage and estimated cost for a Spirit in a given month.
	LedgerEntry = entities.LedgerEntry

	// ModelRoutingRule routes Pulses matching a condition to a specific model, enabling
	// automatic cost optimisation (e.g. route long inputs to a cheaper model).
	ModelRoutingRule = entities.ModelRoutingRule

	// ModelRoutingCondition is the match criteria for a ModelRoutingRule.
	ModelRoutingCondition = entities.ModelRoutingCondition

	// TrialCase is a single eval test case with input, context, and scoring expectations.
	TrialCase = entities.TrialCase

	// TrialSuite is a collection of TrialCases loaded from YAML or JSON via LoadTrialSuite.
	TrialSuite = entities.TrialSuite

	// TrialExpectations declares what a TrialCase response must satisfy to pass.
	TrialExpectations = entities.TrialExpectations

	// LLMJudgeExpect configures an LLM-as-judge scorer for a TrialCase.
	LLMJudgeExpect = entities.LLMJudgeExpect

	// TrialResult holds the outcome of a single TrialCase execution.
	TrialResult = entities.TrialResult

	// TrialReport aggregates TrialResults from a full TrialSuite run.
	TrialReport = entities.TrialReport

	// HTTPTool defines a parameterised HTTP endpoint that Spirits can invoke as an Action.
	// Stored and managed via the /http-tools management API.
	HTTPTool = entities.HTTPTool

	// HTTPToolParam is a single parameter definition for an HTTPTool.
	HTTPToolParam = entities.HTTPToolParam

	// Vigil represents an operator's active seat on a conversation. While a Vigil seat
	// is held, the Weave returns ErrSessionHeld and the operator handles messages directly.
	Vigil = entities.Vigil

	// Rite is an async approval request created by the request_rite built-in action.
	// An operator approves or rejects it via RiteRepository.Decide or the management API.
	Rite = entities.Rite

	// RiteStatus is the lifecycle state of a Rite.
	RiteStatus = entities.RiteStatus

	// AllowedAction names a single Action (or MCP tool) a Spirit is permitted to call.
	AllowedAction = entities.AllowedAction
)

var (
	// RitePending is the initial state of a Rite — awaiting operator decision.
	RitePending = entities.RitePending

	// RiteApproved means an operator approved the Rite.
	RiteApproved = entities.RiteApproved

	// RiteRejected means an operator rejected the Rite.
	RiteRejected = entities.RiteRejected

	// RiteExpired means the Rite TTL elapsed before a decision was made.
	RiteExpired = entities.RiteExpired
)

// Response status constants returned in Response.Status.
const (
	ResponseSuccess = entities.ResponseSuccess
	ResponsePartial = entities.ResponsePartial
	ResponseFailed  = entities.ResponseFailed
)

// Ritual lifecycle status constants.
const (
	RitualPending   = entities.RitualPending
	RitualRunning   = entities.RitualRunning
	RitualExecuted  = entities.RitualExecuted
	RitualCancelled = entities.RitualCancelled
	RitualFailed    = entities.RitualFailed
)

// Artifact media-type constants for Artifact.Type.
const (
	ArtifactTypeImage    = entities.ArtifactTypeImage
	ArtifactTypeAudio    = entities.ArtifactTypeAudio
	ArtifactTypeVideo    = entities.ArtifactTypeVideo
	ArtifactTypeDocument = entities.ArtifactTypeDocument
)

// MetadataKeyRitualID is the Pulse.Metadata key that carries the originating Ritual ID
// when a scheduled event fires.
const MetadataKeyRitualID = entities.MetadataKeyRitualID

// Spirit type constants for Spirit.Type.
const (
	SpiritTypeConversational = entities.SpiritTypeConversational
	SpiritTypeExecutor       = entities.SpiritTypeExecutor
	SpiritTypeNotifier       = entities.SpiritTypeNotifier

	// SpiritTypeOrchestrator enables the summon_spirit built-in tool. Configure
	// which sub-spirits can be summoned via Spirit.OrchestratorConfig.
	SpiritTypeOrchestrator = entities.SpiritTypeOrchestrator
)

// NewLink returns a builder for constructing a Link (event configuration).
// eventKey must match the key passed to Weave.ProcessEventByKey.
var NewLink = entities.NewLink
