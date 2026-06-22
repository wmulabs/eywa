package eywa

import (
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/implementation/media"
)

type (
	// Oracle is the LLM provider port. Implement it to integrate any language model.
	// Register implementations via WeaveBuilder.AddOracle.
	Oracle = ports.Oracle

	// OracleFactory selects the correct Oracle for a given provider name.
	// Build with NewOracleFactory; pass via WeaveBuilder.WithOracleFactory.
	OracleFactory = ports.OracleFactory

	// OracleRequest is the input sent to an Oracle. It carries the conversation thread,
	// available tools, attachments, and generation parameters.
	OracleRequest = ports.OracleRequest

	// OracleResponse is the output returned by an Oracle after one generation step.
	OracleResponse = ports.OracleResponse

	// StreamingOracle is the optional streaming capability of an Oracle. Implement GenerateStream
	// alongside Oracle to enable token streaming; the loop falls back to GenerateResponse otherwise.
	StreamingOracle = ports.StreamingOracle

	// StreamEvent is one incremental event from GenerateStream (delta, done, or error).
	StreamEvent = ports.StreamEvent

	// StreamEventType classifies a StreamEvent.
	StreamEventType = ports.StreamEventType

	// AgentStreamEvent is one incremental event from a streaming agent turn (delta, tool status,
	// done, or error), surfaced by Weave.ProcessEventByKeyStream.
	AgentStreamEvent = ports.AgentStreamEvent

	// AgentStreamEventType classifies an AgentStreamEvent.
	AgentStreamEventType = ports.AgentStreamEventType

	// ResponseFormat requests a response conforming to a JSON Schema. Set it on OracleRequest to get
	// validated structured output; nil = free-form generation.
	ResponseFormat = ports.ResponseFormat

	// StructuredOracle is the optional native structured-output capability of an Oracle.
	StructuredOracle = ports.StructuredOracle

	// OracleMessage is a single message in the conversation thread sent to an Oracle.
	OracleMessage = ports.OracleMessage

	// OracleTool describes a tool (Action) schema as sent to the Oracle provider.
	OracleTool = ports.OracleTool

	// OracleToolCall is a tool invocation requested by the Oracle in its response.
	OracleToolCall = ports.OracleToolCall

	// OracleUsage records token consumption for a single Oracle call.
	OracleUsage = ports.OracleUsage

	// LLMAttachment is a media file (image, audio, document) sent to the Oracle alongside
	// the text prompt. Created from Artifacts via ConvertToLLMAttachments.
	LLMAttachment = ports.LLMAttachment

	// Action is the tool-use port. Implement it to expose capabilities to Spirits.
	// Register implementations via ActionRegistry.Register.
	Action = ports.Action

	// ActionCategory classifies an Action by its effect (retrieval, modification, delivery).
	ActionCategory = ports.ActionCategory

	// ActionRegistry holds the set of Actions available to the Weave engine.
	// Create with NewActionRegistry; pass via WeaveBuilder.WithActionRegistry.
	ActionRegistry = ports.ActionRegistry

	// ToolResultLimits bounds how much a single Action result contributes to the reasoning
	// context. Pass via WeaveBuilder.WithToolResultLimits.
	ToolResultLimits = ports.ToolResultLimits

	// ToolShapeStrategy selects how an oversized Action result is reduced (truncate or summarize).
	ToolShapeStrategy = ports.ToolShapeStrategy

	// ToolResultShaper is an optional interface an Action may implement to override the global
	// ToolResultLimits for its own results.
	ToolResultShaper = ports.ToolResultShaper

	// Scout is the context-enrichment port. Implement it to inject knowledge into
	// a Pulse before Spirit selection. Register via ScoutRegistry.Register.
	Scout = ports.Scout

	// ScoutRegistry holds the set of Scouts available to the Weave engine.
	// Create with NewScoutRegistry; pass via WeaveBuilder.WithScoutRegistry.
	ScoutRegistry = ports.ScoutRegistry

	// Pathfinder is the Spirit-selection port. Implement it for custom routing logic.
	// Register via PathfinderRegistry.Register.
	Pathfinder = ports.Pathfinder

	// PathfinderRegistry holds the set of Pathfinders available to the Weave engine.
	PathfinderRegistry = ports.PathfinderRegistry

	// Receptor converts a raw inbound payload (webhook body, queue message) into a Pulse.
	// Register via Weave.RegisterReceptor.
	Receptor = ports.Receptor

	// Voice delivers a Spirit's response to an external channel (WhatsApp, HTTP, etc.).
	// Register via VoiceRegistry.Register.
	Voice = ports.Voice

	// VoiceRegistry holds the set of Voices available to the Weave engine.
	// Create with NewVoiceRegistry; pass via WeaveBuilder.WithVoiceRegistry.
	VoiceRegistry = ports.VoiceRegistry

	// Bond is the distributed lock port. It prevents concurrent processing of Pulses
	// for the same MemoryKey. Use the Redis adapter (eywaredis.NewBondManager) in production.
	Bond = ports.Bond

	// Inbox is the message-coalescing port. When multiple Pulses arrive in quick
	// succession for the same user, the Inbox accumulates them into a single Pulse.
	Inbox = ports.Inbox

	// Limiter is the rate-limiting port. Implement to throttle Pulse processing per user.
	Limiter = ports.Limiter

	// IdempotencyStore is the duplicate-suppression port. It records processed IdempotencyKeys
	// so redelivered webhooks (e.g. WhatsApp) are handled at most once. Wire via
	// WeaveBuilder.WithIdempotencyStore. Use the Redis adapter for multi-instance deployments.
	IdempotencyStore = ports.IdempotencyStore

	// Keeper is the scheduler port. It enqueues Pulses for future delivery and handles
	// async dispatch. Use the Cloud Tasks adapter for GCP workloads.
	Keeper = ports.Keeper

	// CheckpointStore persists in-progress reasoning-turn state for durable execution: an
	// interrupted turn (deploy, crash, request timeout) resumes from its last completed iteration
	// instead of restarting. Wire via WeaveBuilder.WithCheckpointStore; nil = durability off (default).
	CheckpointStore = ports.CheckpointStore

	// RitualManager coordinates Ritual lifecycle: create, list, cancel, and fire scheduled
	// events. Create with NewRitualService.
	RitualManager = ports.RitualManager

	// RitualRequest is the input for scheduling a new Ritual.
	RitualRequest = ports.RitualRequest

	// LogicRouter applies post-processing logic (redaction, formatting) to a Response
	// before it is delivered via Voice.
	LogicRouter = ports.LogicRouter

	// LogicRouterRegistry holds the set of LogicRouters active in the Weave engine.
	LogicRouterRegistry = ports.LogicRouterRegistry

	// LoreRepository persists Lore documents and chunks for RAG retrieval.
	// The MongoDB adapter supports full-text search out of the box.
	LoreRepository = ports.LoreRepository

	// LoreStore is the vector search backend port for semantic Lore retrieval.
	// Implement for pgvector, Qdrant, Pinecone, or Weaviate.
	LoreStore = ports.LoreStore

	// FilterableLoreStore is the optional metadata-filtering capability of a LoreStore, used by
	// Weave.SearchLore. Stores whose backend supports payload filters implement it.
	FilterableLoreStore = ports.FilterableLoreStore

	// LoreFilter constrains a Lore search by chunk Metadata (equality + numeric ranges).
	LoreFilter = ports.LoreFilter

	// LoreRange constrains a numeric metadata field to [Min, Max]; a nil bound is open on that side.
	LoreRange = ports.LoreRange

	// LoreSearchOptions carries the parameters of Weave.SearchLore (TopK, MinScore, Filter).
	LoreSearchOptions = ports.LoreSearchOptions

	// LoreEmbedder converts text into a vector embedding for Lore indexing and search.
	// Implement using your preferred embedding model API.
	LoreEmbedder = ports.LoreEmbedder

	// LoreHarvester pre-processes raw documents into LoreChunks before ingestion.
	LoreHarvester = ports.LoreHarvester

	// ImprintRepository persists long-term user facts (Imprints) across sessions.
	ImprintRepository = ports.ImprintRepository

	// ImprintListOptions filters for ImprintRepository.List.
	ImprintListOptions = ports.ImprintListOptions

	// ImprintHarvester extracts structured facts from raw conversation text.
	// Used by the automatic ImprintExtractionConfig pipeline.
	ImprintHarvester = ports.ImprintHarvester

	// Conduit is the MCP client port. It connects to an external MCP server, discovers
	// its tools, and makes them available as Actions in the Weave.
	// Create with eywamcp.NewConduit; register via WeaveBuilder.WithConduit.
	Conduit = ports.Conduit

	// ConduitRegistry manages the set of active Conduits in the Weave engine.
	ConduitRegistry = ports.ConduitRegistry

	// LedgerRepository persists monthly token usage and manages TokenBudgets per Spirit.
	LedgerRepository = ports.LedgerRepository

	// CostAlertHook is called when a Spirit's token usage reaches its AlertThreshold.
	CostAlertHook = ports.CostAlertHook

	// Scorer evaluates a single TrialCase response. Implement for custom eval metrics.
	// Built-in scorers: NewOutputContainsScorer, NewActionSequenceScorer, NewLLMJudgeScorer.
	Scorer = ports.Scorer

	// Archivist summarises a conversation Thread when it exceeds a message threshold,
	// preventing context overflow on long conversations.
	Archivist = ports.Archivist

	// Vault is the object storage port for media files (images, audio, documents).
	// Use the GCS adapter for Google Cloud Storage.
	Vault = ports.Vault

	// Lens is the media processing port. It transcribes audio, describes images,
	// and extracts text from documents for the Oracle.
	Lens = ports.Lens

	// Ear is the audio transcription port used by a Lens implementation.
	Ear = ports.Ear

	// Eye is the image analysis port used by a Lens implementation.
	Eye = ports.Eye

	// WhatsAppClient sends WhatsApp messages and templates via a provider's API.
	WhatsAppClient = ports.WhatsAppClient

	// TemplateComponent is a section of a WhatsApp template message.
	TemplateComponent = ports.TemplateComponent

	// TemplateMessage is a WhatsApp template message with its components.
	TemplateMessage = ports.TemplateMessage

	// TemplateParameter is a single variable parameter within a WhatsApp template.
	TemplateParameter = ports.TemplateParameter

	// TypingIndicator sends a typing-in-progress signal on a channel while the Oracle reasons.
	TypingIndicator = ports.TypingIndicator

	// TokenValidator authenticates inbound API requests. Implementations include
	// NewAPIKeyValidator, NewJWTValidator, and NewJWKSValidator.
	TokenValidator = ports.TokenValidator

	// AuthClaims carries identity claims extracted by a TokenValidator.
	AuthClaims = ports.AuthClaims

	// RequestVerifier authenticates an inbound event request from its full content (headers + raw
	// body), enabling signature schemes (HMAC, provider webhooks) a bearer TokenValidator cannot.
	// Wire via RouteDeps.EventVerifiers. Implementations: NewHMACVerifier, channel signature verifiers.
	RequestVerifier = ports.RequestVerifier

	// VerifiableRequest is the transport-agnostic request view a RequestVerifier inspects.
	VerifiableRequest = ports.VerifiableRequest

	// AppTokenRepository persists revocable app tokens for authenticating inbound event requests.
	// Use the Mongo adapter (eywamongo.NewAppTokenRepository) in production.
	AppTokenRepository = ports.AppTokenRepository

	// OperatorRepository persists Operator records and validates credentials.
	OperatorRepository = ports.OperatorRepository

	// LinkRepository persists Link (event configuration) records, enabling the
	// management API to read and update event routing at runtime.
	LinkRepository = ports.LinkRepository

	// PubSub is the publish/subscribe port used by ConfigCache for live-update
	// notifications when event configurations change.
	PubSub = ports.PubSub
)

type (
	// SpiritRepository persists Spirit definitions.
	SpiritRepository = ports.SpiritRepository

	// MemoryRepository persists ephemeral per-user conversation state (Thread + Knowledge).
	// TTL-based — use the Redis adapter for production workloads.
	MemoryRepository = ports.MemoryRepository

	// EchoRepository persists individual message records for conversation history.
	EchoRepository = ports.EchoRepository

	// ChronicleRepository persists immutable audit records of every Weave processing call.
	ChronicleRepository = ports.ChronicleRepository

	// RitualRepository persists Ritual definitions and tracks their execution state.
	RitualRepository = ports.RitualRepository

	// ChronicleListOptions filters a Chronicle query by time range, Spirit, user, etc.
	ChronicleListOptions = ports.ChronicleListOptions

	// ChronicleQueryRepository provides read-only analytics queries over Chronicle data.
	ChronicleQueryRepository = ports.ChronicleQueryRepository

	// TokenSeries is a time-bucketed token usage series returned by analytics queries.
	TokenSeries = ports.TokenSeries

	// ActionStats aggregates Action invocation counts and latency.
	ActionStats = ports.ActionStats

	// SpiritStats aggregates per-Spirit usage metrics.
	SpiritStats = ports.SpiritStats

	// SessionListOptions filters a conversation session query.
	SessionListOptions = ports.SessionListOptions

	// SessionSummary is a lightweight summary of a conversation session for listing.
	SessionSummary = ports.SessionSummary

	// EchoQueryRepository provides read-only queries over Echo (message history) data.
	EchoQueryRepository = ports.EchoQueryRepository

	// HTTPToolRepository persists HTTPTool definitions managed via the management API.
	HTTPToolRepository = ports.HTTPToolRepository

	// VigilRepository persists active Vigil seats (operator takeover state) in Redis.
	VigilRepository = ports.VigilRepository

	// RiteRepository persists Rite (async approval) records.
	RiteRepository = ports.RiteRepository

	// RiteListOptions filters a Rite query by status, Spirit, or user.
	RiteListOptions = ports.RiteListOptions
)

// Oracle role constants — used by Oracle provider implementations to populate OracleMessage.Role.
const (
	RoleUser      = ports.RoleUser
	RoleAssistant = ports.RoleAssistant
	RoleSystem    = ports.RoleSystem
	RoleTool      = ports.RoleTool
)

// Normalized stop reason constants — Oracle provider implementations must map
// provider-native values to these before returning OracleResponse.StopReason.
const (
	StopReasonComplete      = ports.StopReasonComplete
	StopReasonLength        = ports.StopReasonLength
	StopReasonToolCalls     = ports.StopReasonToolCalls
	StopReasonContentFilter = ports.StopReasonContentFilter
)

// Action category constants for Action.GetCategory.
const (
	ActionDelivery     = ports.ActionDelivery
	ActionRetrieval    = ports.ActionRetrieval
	ActionModification = ports.ActionModification
	ActionGeneral      = ports.ActionGeneral
)

// Tool-result shaping strategy constants for ToolResultLimits.Strategy.
const (
	ToolShapeTruncate  = ports.ToolShapeTruncate
	ToolShapeSummarize = ports.ToolShapeSummarize
)

// Stream event type constants for StreamEvent.Type.
const (
	StreamEventDelta = ports.StreamEventDelta
	StreamEventDone  = ports.StreamEventDone
	StreamEventError = ports.StreamEventError
)

// Agent stream event type constants for AgentStreamEvent.Type.
const (
	AgentStreamDelta      = ports.AgentStreamDelta
	AgentStreamToolStatus = ports.AgentStreamToolStatus
	AgentStreamDone       = ports.AgentStreamDone
	AgentStreamError      = ports.AgentStreamError
)

// Operator role constants for AuthClaims.Role.
const (
	RoleAdmin    = ports.RoleAdmin
	RoleOperator = ports.RoleOperator
)

// ConvertToLLMAttachments filters and maps Artifacts to LLMAttachment for Oracle calls.
var ConvertToLLMAttachments = media.ConvertToLLMAttachments
