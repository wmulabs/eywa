// Package eywa is a Go library for building event-driven, multi-agent AI systems.
//
// Inspired by the neural network in Avatar that connects all living things on Pandora,
// Eywa orchestrates a network of AI Spirits (agents) that perceive Pulses (events),
// are enriched by Scouts, routed by a Pathfinder, consult an Oracle (LLM), execute
// Actions (tools), and respond through a Voice (output channel).
//
// Pipeline:
//
//	Pulse → Scouts → Pathfinder → Spirit → Oracle → Actions → Voice
//
// The central runtime is the Weave, assembled via NewWeaveBuilder. It wires together
// repositories (Spirit, Memory, Echo, Chronicle), a Bond (distributed lock), one or
// more Oracle providers, and optional registries for Actions, Scouts, Pathfinders, and
// Voices.
//
// Quick start:
//
//	weave, err := eywa.NewWeaveBuilder(ctx).
//	    WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
//	    WithBond(distributedLock).
//	    AddOracle(anthropicOracle).
//	    Build()
//
// See the _examples directory for runnable examples.
package eywa

import (
	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/implementation/archivists"
	"github.com/wmulabs/eywa/internal/implementation/orchestrator"
	"github.com/wmulabs/eywa/internal/implementation/registries"
	"github.com/wmulabs/eywa/internal/implementation/services/ingestion"
	"github.com/wmulabs/eywa/internal/implementation/trial"
	"github.com/wmulabs/eywa/internal/implementation/voices"
	"github.com/wmulabs/eywa/internal/infrastructure/driven/auth"
)

type (
	// Weave is the central runtime engine. Assemble it via NewWeaveBuilder, then call
	// ProcessEventByKey to handle Pulses. A single Weave instance is safe for concurrent use.
	Weave = orchestrator.Weave

	// WeaveBuilder assembles a Weave via a fluent API. Obtain one with NewWeaveBuilder,
	// chain With*/Add*/Register* methods, and call Build to produce the Weave.
	WeaveBuilder = orchestrator.WeaveBuilder

	// WeaveConfig holds global engine settings (timeouts, iteration limits, etc.).
	// Use DefaultWeaveConfig for sensible defaults, then override individual fields.
	WeaveConfig = orchestrator.WeaveConfig

	// GuardConfig defines allow/block rules that gate Pulses before processing begins.
	// Set on WeaveBuilder.WithInputGuard.
	GuardConfig = orchestrator.GuardConfig

	// ProgressPolicy configures reasoning-loop stall detection.
	// Set on WeaveBuilder.WithProgressPolicy.
	ProgressPolicy = orchestrator.ProgressPolicy

	// CompressionPolicy configures in-loop working-context compression.
	// Set on WeaveBuilder.WithCompressionPolicy.
	CompressionPolicy = orchestrator.CompressionPolicy

	// AppInfo carries service identity metadata logged with every Chronicle entry.
	AppInfo = orchestrator.AppInfo

	// WeaveConfigRepository persists WeaveConfig to a durable store (e.g. MongoDB).
	// Used by the management API to support hot-reload.
	WeaveConfigRepository = orchestrator.WeaveConfigRepository

	// ConfigCache holds the in-memory event-configuration cache. Changes pushed through
	// the management API take effect immediately via the cache without a restart.
	ConfigCache = orchestrator.ConfigCache

	// HTTPToolExecutor executes HTTPTool definitions against live HTTP endpoints.
	// Used internally by the management API test endpoint.
	HTTPToolExecutor = orchestrator.HTTPToolExecutor

	// HTTPToolTestResult holds the outcome of a single HTTPTool test execution.
	HTTPToolTestResult = orchestrator.HTTPToolTestResult

	// VigilConfig controls human-takeover behaviour. InactivityTimeout sets how long
	// an operator seat stays active without activity before auto-releasing.
	VigilConfig = orchestrator.VigilConfig
)

type (
	// Operator is an authenticated human operator who can manage Spirits, review Rites,
	// and take over conversations via Vigil.
	Operator = entities.Operator

	// OperatorAuth provides JWT issuance and validation for built-in operator accounts.
	// Create with NewOperatorAuth; pass the same secret used at signing time for validation.
	OperatorAuth = auth.OperatorAuth
)

type (
	// AsyncIngestionService ingests Lore documents asynchronously, chunking and
	// embedding them via the configured LoreEmbedder.
	AsyncIngestionService = ingestion.AsyncIngestionService

	// IngestionResult carries the outcome of a single async Lore ingestion job.
	IngestionResult = ingestion.IngestionResult

	// TrialRunner executes eval suites against a live Weave, scoring responses with
	// deterministic and LLM-as-judge Scorers.
	TrialRunner = trial.TrialRunner
)

var (
	// NewWeaveBuilder returns a WeaveBuilder ready to wire infrastructure and build a Weave.
	NewWeaveBuilder = orchestrator.NewWeaveBuilder

	// DefaultWeaveConfig returns a WeaveConfig with production-safe defaults.
	// Override individual fields before passing to WeaveBuilder.WithConfig.
	DefaultWeaveConfig = orchestrator.DefaultWeaveConfig

	// IsRetriable reports whether err is a transient error that the Keeper should retry.
	IsRetriable = orchestrator.IsRetriable

	// NewConfigCache creates a ConfigCache backed by linkRepo. Pass nil for pubSub and
	// logger to skip live-update notifications; they are optional.
	NewConfigCache = orchestrator.NewConfigCache

	// NewPulse returns a PulseBuilder for constructing a Pulse to send into the Weave.
	NewPulse = entities.NewPulse

	// NewAsyncIngestionService creates an AsyncIngestionService that chunks and embeds
	// Lore documents in the background using the provided LoreEmbedder.
	NewAsyncIngestionService = ingestion.NewAsyncIngestionService

	// NewArchivist creates an LLM-backed Archivist that summarizes conversation history
	// once it exceeds the configured message threshold.
	NewArchivist = archivists.NewLLMArchivist

	// NewVoiceRegistry creates an empty VoiceRegistry. Register Voice implementations
	// to enable automatic response delivery on named channels.
	NewVoiceRegistry = registries.NewVoiceRegistry

	// NewHTTPVoice creates a Voice that POSTs the response body to an HTTP endpoint.
	NewHTTPVoice = voices.NewHTTPVoice

	// NewTrialRunner creates a TrialRunner that executes eval suites against weave.
	NewTrialRunner = trial.NewTrialRunner

	// LoadTrialSuite loads a TrialSuite from a YAML or JSON file at path.
	LoadTrialSuite = trial.LoadTrialSuite

	// NewAPIKeyValidator returns a TokenValidator that accepts static API keys.
	// keys maps raw key strings to role names (e.g. "admin", "operator").
	NewAPIKeyValidator = auth.NewAPIKeyValidator

	// NewOperatorAuth creates an OperatorAuth that signs and validates operator JWTs
	// using secret. The same secret must be used across all instances.
	NewOperatorAuth = auth.NewOperatorAuth

	// NewJWTValidator returns a TokenValidator for HS256-signed JWTs.
	NewJWTValidator = auth.NewJWTValidator

	// NewJWTValidatorRS256 returns a TokenValidator for RS256-signed JWTs.
	NewJWTValidatorRS256 = auth.NewJWTValidatorRS256

	// NewJWKSValidator returns a TokenValidator that fetches public keys from a JWKS URL.
	// Suitable for OIDC/SSO integrations.
	NewJWKSValidator = auth.NewJWKSValidator

	// HashPassword returns a bcrypt hash of password. Use when creating Operator records.
	HashPassword = auth.HashPassword

	// NewHTTPToolExecutor creates an HTTPToolExecutor for testing HTTPTool definitions
	// against live endpoints.
	NewHTTPToolExecutor = orchestrator.NewHTTPToolExecutor

	// NewNoOpBond returns an in-process Bond backed by a sync.Mutex map.
	// Use for single-instance deployments, local development, and tests.
	// For multi-instance production use redis.NewBondManager from the redis sub-module.
	NewNoOpBond = orchestrator.NewNoOpBond

	// NewInMemoryIdempotencyStore returns an in-process IdempotencyStore for duplicate-event
	// suppression. Use for single-instance deployments and tests. For multi-instance production
	// use redis.NewIdempotencyStore from the redis sub-module.
	NewInMemoryIdempotencyStore = orchestrator.NewInMemoryIdempotencyStore
)
