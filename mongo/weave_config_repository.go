package mongo

import (
	"context"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ eywa.WeaveConfigRepository = (*WeaveConfigRepository)(nil)

type WeaveConfigRepository struct {
	collection *mongodriver.Collection
}

func NewWeaveConfigRepository(database *mongodriver.Database) *WeaveConfigRepository {
	return &WeaveConfigRepository{
		collection: database.Collection("engine_config"),
	}
}

type weaveConfigDocument struct {
	ID                         string    `bson:"_id"`
	LockTTLNs                  int64     `bson:"lock_ttl_ns"`
	LockAcquireTimeoutNs       int64     `bson:"lock_acquire_timeout_ns"`
	ScoutTimeoutNs             int64     `bson:"scout_timeout_ns"`
	SpiritSelectionTimeoutNs   int64     `bson:"spirit_selection_timeout_ns"`
	SpiritLoadTimeoutNs        int64     `bson:"spirit_load_timeout_ns"`
	SessionTimeoutNs           int64     `bson:"session_timeout_ns"`
	ReasoningTimeoutNs         int64     `bson:"reasoning_timeout_ns"`
	PersistenceTimeoutNs       int64     `bson:"persistence_timeout_ns"`
	MaxReasoningIterations     int       `bson:"max_reasoning_iterations"`
	MaxMemoryMessages          int       `bson:"max_memory_messages"`
	MaxIterationsMessage       string    `bson:"max_iterations_message"`
	EnableMemoryReconstruction bool      `bson:"enable_memory_reconstruction"`
	MemoryReconstructionLimit  int       `bson:"memory_reconstruction_limit"`
	ParallelActionExecution    bool      `bson:"parallel_action_execution"`
	ActionRetryMaxAttempts     int       `bson:"action_retry_max_attempts"`
	ActionRetryBaseDelayNs     int64     `bson:"action_retry_base_delay_ns"`
	MaxActionsPerCycle         int       `bson:"max_actions_per_cycle"`
	InboxMinWindowNs           int64     `bson:"inbox_min_window_ns"`
	MessageCoalescingTimeoutNs int64     `bson:"message_coalescing_timeout_ns"`
	MaxSessionKeyLength        int       `bson:"max_session_key_length"`
	MaxUserMessageLength       int       `bson:"max_user_message_length"`
	MaxEventContextSize        int       `bson:"max_event_context_size"`
	PromptInjectionDetection   bool      `bson:"prompt_injection_detection"`
	MaxLineCount               int       `bson:"max_line_count"`
	UpdatedAt                  time.Time `bson:"updated_at"`
}

func configToDocument(cfg *eywa.WeaveConfig) weaveConfigDocument {
	return weaveConfigDocument{
		ID:                         "default",
		LockTTLNs:                  cfg.LockTTL.Nanoseconds(),
		LockAcquireTimeoutNs:       cfg.LockAcquireTimeout.Nanoseconds(),
		ScoutTimeoutNs:             cfg.ScoutTimeout.Nanoseconds(),
		SpiritSelectionTimeoutNs:   cfg.SpiritSelectionTimeout.Nanoseconds(),
		SpiritLoadTimeoutNs:        cfg.SpiritLoadTimeout.Nanoseconds(),
		SessionTimeoutNs:           cfg.SessionTimeout.Nanoseconds(),
		ReasoningTimeoutNs:         cfg.ReasoningTimeout.Nanoseconds(),
		PersistenceTimeoutNs:       cfg.PersistenceTimeout.Nanoseconds(),
		MaxReasoningIterations:     cfg.MaxReasoningIterations,
		MaxMemoryMessages:          cfg.MaxMemoryMessages,
		MaxIterationsMessage:       cfg.MaxIterationsMessage,
		EnableMemoryReconstruction: cfg.EnableMemoryReconstruction,
		MemoryReconstructionLimit:  cfg.MemoryReconstructionLimit,
		ParallelActionExecution:    cfg.ParallelActionExecution,
		ActionRetryMaxAttempts:     cfg.ActionRetryMaxAttempts,
		ActionRetryBaseDelayNs:     cfg.ActionRetryBaseDelay.Nanoseconds(),
		MaxActionsPerCycle:         cfg.MaxActionsPerCycle,
		InboxMinWindowNs:           cfg.InboxMinWindow.Nanoseconds(),
		MessageCoalescingTimeoutNs: cfg.MessageCoalescingTimeout.Nanoseconds(),
		MaxSessionKeyLength:        cfg.MaxSessionKeyLength,
		MaxUserMessageLength:       cfg.MaxUserMessageLength,
		MaxEventContextSize:        cfg.MaxEventContextSize,
		PromptInjectionDetection:   cfg.InputGuard.PromptInjectionDetection,
		MaxLineCount:               cfg.InputGuard.MaxLineCount,
		UpdatedAt:                  time.Now().UTC(),
	}
}

func documentToConfig(doc weaveConfigDocument) *eywa.WeaveConfig {
	return &eywa.WeaveConfig{
		LockTTL:                    time.Duration(doc.LockTTLNs),
		LockAcquireTimeout:         time.Duration(doc.LockAcquireTimeoutNs),
		ScoutTimeout:               time.Duration(doc.ScoutTimeoutNs),
		SpiritSelectionTimeout:     time.Duration(doc.SpiritSelectionTimeoutNs),
		SpiritLoadTimeout:          time.Duration(doc.SpiritLoadTimeoutNs),
		SessionTimeout:             time.Duration(doc.SessionTimeoutNs),
		ReasoningTimeout:           time.Duration(doc.ReasoningTimeoutNs),
		PersistenceTimeout:         time.Duration(doc.PersistenceTimeoutNs),
		MaxReasoningIterations:     doc.MaxReasoningIterations,
		MaxMemoryMessages:          doc.MaxMemoryMessages,
		MaxIterationsMessage:       doc.MaxIterationsMessage,
		EnableMemoryReconstruction: doc.EnableMemoryReconstruction,
		MemoryReconstructionLimit:  doc.MemoryReconstructionLimit,
		ParallelActionExecution:    doc.ParallelActionExecution,
		ActionRetryMaxAttempts:     doc.ActionRetryMaxAttempts,
		ActionRetryBaseDelay:       time.Duration(doc.ActionRetryBaseDelayNs),
		MaxActionsPerCycle:         doc.MaxActionsPerCycle,
		InboxMinWindow:             time.Duration(doc.InboxMinWindowNs),
		MessageCoalescingTimeout:   time.Duration(doc.MessageCoalescingTimeoutNs),
		MaxSessionKeyLength:        doc.MaxSessionKeyLength,
		MaxUserMessageLength:       doc.MaxUserMessageLength,
		MaxEventContextSize:        doc.MaxEventContextSize,
		InputGuard: eywa.GuardConfig{
			PromptInjectionDetection: doc.PromptInjectionDetection,
			MaxLineCount:             doc.MaxLineCount,
		},
	}
}

func (r *WeaveConfigRepository) Find(ctx context.Context) (*eywa.WeaveConfig, error) {
	var doc weaveConfigDocument
	err := r.collection.FindOne(ctx, bson.M{"_id": "default"}).Decode(&doc)
	if err == mongodriver.ErrNoDocuments {
		return eywa.DefaultWeaveConfig(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("decode weave config: %w", err)
	}
	return documentToConfig(doc), nil
}

func (r *WeaveConfigRepository) Save(ctx context.Context, config *eywa.WeaveConfig) error {
	doc := configToDocument(config)
	opts := options.Replace().SetUpsert(true)
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": "default"}, doc, opts)
	if err != nil {
		return fmt.Errorf("replace weave config: %w", err)
	}
	return nil
}
