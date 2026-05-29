package orchestrator

import (
	"context"
	"fmt"
	"time"
)

// GuardConfig controls input validation checks applied before each Pulse enters the pipeline.
// Both fields are enabled by default in DefaultWeaveConfig.
// To disable for development or trusted internal traffic:
//
//	builder.WithConfig(func(c *WeaveConfig) { c.InputGuard = GuardConfig{} })
type GuardConfig struct {
	// PromptInjectionDetection enables matching against known prompt injection patterns.
	// Conservative — only explicit attack phrases are flagged to minimise false positives.
	PromptInjectionDetection bool `json:"prompt_injection_detection"`
	// MaxLineCount rejects messages that exceed this many newline-separated lines. 0 = disabled.
	MaxLineCount int `json:"max_line_count"`
}

type WeaveConfig struct {
	LockTTL            time.Duration `json:"lock_ttl"`
	LockAcquireTimeout time.Duration `json:"lock_acquire_timeout"`

	ScoutTimeout           time.Duration `json:"scout_timeout"`
	SpiritSelectionTimeout time.Duration `json:"spirit_selection_timeout"`
	SpiritLoadTimeout      time.Duration `json:"spirit_load_timeout"`
	SessionTimeout         time.Duration `json:"session_timeout"`
	// ReasoningTimeout bounds a single complete reasoning cycle (all iterations).
	// Size it as: MaxReasoningIterations × MaxActionsPerCycle × (avg action latency + avg Oracle latency).
	// Example: 5 iterations × 20 actions × 2 s/action = 200 s minimum.
	// LockTTL must be set at least 15 s above ReasoningTimeout (enforced by Validate).
	ReasoningTimeout   time.Duration `json:"reasoning_timeout"`
	PersistenceTimeout time.Duration `json:"persistence_timeout"`

	MaxReasoningIterations int    `json:"max_reasoning_iterations"`
	MaxMemoryMessages      int    `json:"max_memory_messages"`
	MaxIterationsMessage   string `json:"max_iterations_message"`

	// Rebuild Memory from message store when Redis session expires.
	EnableMemoryReconstruction bool `json:"enable_memory_reconstruction"`
	MemoryReconstructionLimit  int  `json:"memory_reconstruction_limit"`

	ParallelActionExecution bool `json:"parallel_action_execution"`

	// Applies to transient (infrastructure) errors only.
	// Business errors are never retried regardless of these values.
	// Defaults to no retry (ActionRetryMaxAttempts = 1).
	ActionRetryMaxAttempts int           `json:"action_retry_max_attempts"`
	ActionRetryBaseDelay   time.Duration `json:"action_retry_base_delay"`

	// MaxActionsPerCycle caps total Action calls across all iterations in one reasoning cycle.
	// Prevents runaway Spirits from burning tokens in infinite Action loops. 0 disables the limit.
	MaxActionsPerCycle int         `json:"max_actions_per_cycle"`
	InputGuard         GuardConfig `json:"input_guard"`

	// InboxMinWindow is the minimum time elapsed from pipeline start before draining.
	// Pipeline steps count toward the window; actual added wait = max(0, InboxMinWindow - elapsed).
	// 0 disables the wait (coalescing still occurs for messages already in the inbox).
	InboxMinWindow time.Duration `json:"inbox_min_window"`
	// MessageCoalescingTimeout bounds the entire coalescing step including any minWindow wait.
	// Must be > InboxMinWindow to leave time for the inbox drain and memory update.
	MessageCoalescingTimeout time.Duration `json:"message_coalescing_timeout"`

	MaxSessionKeyLength  int `json:"max_session_key_length"`
	MaxUserMessageLength int `json:"max_user_message_length"`
	MaxEventContextSize  int `json:"max_event_context_size"`

	// MaxConcurrentEvents caps the number of goroutines spawned by ProcessMultipleEventsByKey.
	// 0 uses the default (100). Set to -1 to disable the limit (unbounded).
	MaxConcurrentEvents int `json:"max_concurrent_events"`
}

func DefaultWeaveConfig() *WeaveConfig {
	const defaultReasoningTimeout = 120 * time.Second
	return &WeaveConfig{
		LockTTL:            defaultReasoningTimeout + 30*time.Second, // must exceed ReasoningTimeout
		LockAcquireTimeout: 5 * time.Second,

		ScoutTimeout:           10 * time.Second,
		SpiritSelectionTimeout: 30 * time.Second,
		SpiritLoadTimeout:      5 * time.Second,
		SessionTimeout:         5 * time.Second,
		ReasoningTimeout:       defaultReasoningTimeout,
		PersistenceTimeout:     10 * time.Second,

		MaxReasoningIterations: MaxReasoningIterations,
		MaxMemoryMessages:      DefaultMaxMemoryMessages,
		MaxIterationsMessage:   "I've reached my processing limit. Please try again with a more specific question.",

		EnableMemoryReconstruction: DefaultEnableMemoryReconstruction,
		MemoryReconstructionLimit:  DefaultMemoryReconstructionLimit,

		ParallelActionExecution: false,

		ActionRetryMaxAttempts: 1,
		ActionRetryBaseDelay:   0,

		MaxActionsPerCycle: 20,
		InputGuard: GuardConfig{
			PromptInjectionDetection: true,
			MaxLineCount:             200,
		},

		InboxMinWindow:           0,
		MessageCoalescingTimeout: 10 * time.Second,

		MaxSessionKeyLength:  256,
		MaxUserMessageLength: 50000,
		MaxEventContextSize:  100000,

		MaxConcurrentEvents: 100,
	}
}

func (c *WeaveConfig) Validate() error {
	if c.LockTTL <= 0 {
		return ErrInvalidConfig("LockTTL must be positive")
	}
	if c.ReasoningTimeout > 0 && c.LockTTL < c.ReasoningTimeout+15*time.Second {
		return ErrInvalidConfig(fmt.Sprintf(
			"LockTTL (%s) must be at least ReasoningTimeout (%s) + 15s to prevent lock expiry during reasoning; set LockTTL to %s or higher",
			c.LockTTL, c.ReasoningTimeout, c.ReasoningTimeout+30*time.Second,
		))
	}
	if c.MaxReasoningIterations <= 0 {
		return ErrInvalidConfig("MaxReasoningIterations must be positive")
	}
	if c.MaxMemoryMessages <= 0 {
		return ErrInvalidConfig("MaxMemoryMessages must be positive")
	}
	if c.MaxSessionKeyLength <= 0 {
		return ErrInvalidConfig("MaxSessionKeyLength must be positive")
	}
	if c.MaxUserMessageLength <= 0 {
		return ErrInvalidConfig("MaxUserMessageLength must be positive")
	}
	return nil
}

// WeaveConfigRepository persists and retrieves WeaveConfig from a durable store.
// Find returns DefaultWeaveConfig() if no document exists yet.
type WeaveConfigRepository interface {
	Find(ctx context.Context) (*WeaveConfig, error)
	Save(ctx context.Context, config *WeaveConfig) error
}
