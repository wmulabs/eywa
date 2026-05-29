package orchestrator

import (
	"context"
	"fmt"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	DefaultMaxMemoryMessages          = 10
	DefaultMemoryReconstructionLimit  = 10
	DefaultEnableMemoryReconstruction = true
)

// buildRedisKey uses ":" as separator — keys are write-only (never parsed back), so collisions are impossible in practice.
//
//	buildRedisKey("whatsapp:+5521999999", "")               → "whatsapp:+5521999999"
//	buildRedisKey("whatsapp:+5521999999", "shipment:12345") → "whatsapp:+5521999999:shipment:12345"
func buildRedisKey(memoryKey, subjectKey string) string {
	if subjectKey == "" {
		return memoryKey
	}
	return fmt.Sprintf("%s:%s", memoryKey, subjectKey)
}

type MemoryManager interface {
	GetOrCreate(ctx context.Context, memoryKey, subjectKey string) (*entities.Memory, error)
	RebuildForTopic(ctx context.Context, memoryKey, subjectKey string) (*entities.Memory, error)
	FindLastTopicKey(ctx context.Context, memoryKey string) string
	UpdateMemory(ctx context.Context, session *entities.Memory, message entities.Thread) error
	Save(ctx context.Context, session *entities.Memory) error
}

type DefaultMemoryManager struct {
	memoryRepo                 ports.MemoryRepository
	messageRepo                ports.EchoRepository
	maxMemoryMessages          int
	memoryReconstructionLimit  int
	enableMemoryReconstruction bool
	logger                     *zap.SugaredLogger
	tracer                     trace.Tracer
}

func NewMemoryManager(
	memoryRepo ports.MemoryRepository,
	messageRepo ports.EchoRepository,
	maxMemoryMessages int,
	logger *zap.SugaredLogger,
	tracer trace.Tracer,
) *DefaultMemoryManager {
	if maxMemoryMessages <= 0 {
		maxMemoryMessages = DefaultMaxMemoryMessages
	}

	return &DefaultMemoryManager{
		memoryRepo:                 memoryRepo,
		messageRepo:                messageRepo,
		maxMemoryMessages:          maxMemoryMessages,
		memoryReconstructionLimit:  DefaultMemoryReconstructionLimit,
		enableMemoryReconstruction: DefaultEnableMemoryReconstruction,
		logger:                     logger,
		tracer:                     tracer,
	}
}

func (s *DefaultMemoryManager) WithMemoryReconstruction(enabled bool, limit int) *DefaultMemoryManager {
	s.enableMemoryReconstruction = enabled
	if limit > 0 {
		s.memoryReconstructionLimit = limit
	}
	return s
}

// GetOrCreate loads the memory for the given (memoryKey, subjectKey) pair from Redis.
// On miss it rebuilds from the message store if reconstruction is enabled, or returns empty memory.
// The composite Redis key encodes both identity and topic — memories for different topics
// are completely isolated.
func (s *DefaultMemoryManager) GetOrCreate(ctx context.Context, memoryKey, subjectKey string) (*entities.Memory, error) {
	ctx, span := s.tracer.Start(ctx, "MemoryManager/GetOrCreate")
	defer span.End()

	key := buildRedisKey(memoryKey, subjectKey)

	session, err := s.memoryRepo.GetMemory(ctx, key)
	if err != nil {
		if s.enableMemoryReconstruction && s.messageRepo != nil {
			session = s.rebuildMemoryFromMessages(ctx, memoryKey, subjectKey)
		} else {
			session = emptySession(memoryKey, subjectKey)
		}

		if err := s.memoryRepo.SaveMemory(ctx, key, session); err != nil {
			return nil, fmt.Errorf("initialize memory: %w", err)
		}

		s.logger.Infow("memory initialized",
			"key", key,
			"messages_reconstructed", len(session.Threads))
	}

	return session, nil
}

// FindLastTopicKey returns the subject_key from the most recent persisted message for the memory.
// Used by SessionSetupStep to restore continuity when the incoming event carries no explicit topic.
// Returns empty string when no messages exist or the last message had no topic.
func (s *DefaultMemoryManager) FindLastTopicKey(ctx context.Context, memoryKey string) string {
	if s.messageRepo == nil {
		return ""
	}
	messages, err := s.messageRepo.FindRecentByMemoryKey(ctx, memoryKey, 1)
	if err != nil || len(messages) == 0 {
		return ""
	}
	return messages[0].SubjectKey
}

// RebuildForTopic forces a memory reconstruction for the given (memoryKey, subjectKey) pair,
// loading the most recent user-facing messages from MongoDB and saving the result to Redis.
// Called by the ReasoningService when a topic switch is detected mid-reasoning loop,
// ensuring the next LLM iteration has full history of the new topic.
func (s *DefaultMemoryManager) RebuildForTopic(ctx context.Context, memoryKey, subjectKey string) (*entities.Memory, error) {
	ctx, span := s.tracer.Start(ctx, "MemoryManager/RebuildForTopic")
	defer span.End()

	session := s.rebuildMemoryFromMessages(ctx, memoryKey, subjectKey)

	key := buildRedisKey(memoryKey, subjectKey)
	if err := s.memoryRepo.SaveMemory(ctx, key, session); err != nil {
		return nil, fmt.Errorf("rebuild memory: %w", err)
	}

	return session, nil
}

func (s *DefaultMemoryManager) rebuildMemoryFromMessages(ctx context.Context, memoryKey, subjectKey string) *entities.Memory {
	var messages []*entities.Echo
	var err error

	if subjectKey != "" {
		messages, err = s.messageRepo.FindRecentByMemoryKeyAndSubject(ctx, memoryKey, subjectKey, s.memoryReconstructionLimit)
	} else {
		messages, err = s.messageRepo.FindRecentByMemoryKey(ctx, memoryKey, s.memoryReconstructionLimit)
	}

	if err != nil || len(messages) == 0 {
		s.logger.Debugw("no messages found for memory reconstruction",
			"memory_key", memoryKey,
			"subject_key", subjectKey,
			"error", err)
		return emptySession(memoryKey, subjectKey)
	}

	shortTermMemory := toChatMessages(messages)

	s.logger.Infow("memory reconstructed",
		"memory_key", memoryKey,
		"subject_key", subjectKey,
		"messages_count", len(shortTermMemory))

	return &entities.Memory{
		MemoryKey:       memoryKey,
		SubjectKey:      subjectKey,
		Threads:         shortTermMemory,
		Summary:         "",
		TopicFacts:      make(map[string]any),
		LastInteraction: helpers.NowUTC(),
	}
}

func (s *DefaultMemoryManager) UpdateMemory(ctx context.Context, session *entities.Memory, message entities.Thread) error {
	_, span := s.tracer.Start(ctx, "MemoryManager/UpdateMemory")
	defer span.End()

	now := helpers.NowUTC()
	message.Timestamp = now
	session.Threads = append(session.Threads, message)
	session.LastInteraction = now

	if len(session.Threads) > s.maxMemoryMessages {
		session.Threads = session.Threads[len(session.Threads)-s.maxMemoryMessages:]
		s.logger.Debugw("memory trimmed",
			"memory_key", session.MemoryKey,
			"max_messages", s.maxMemoryMessages)
	}

	return nil
}

func (s *DefaultMemoryManager) Save(ctx context.Context, session *entities.Memory) error {
	ctx, span := s.tracer.Start(ctx, "MemoryManager/Save")
	defer span.End()

	key := buildRedisKey(session.MemoryKey, session.SubjectKey)

	if err := s.memoryRepo.SaveMemory(ctx, key, session); err != nil {
		return fmt.Errorf("save memory: %w", err)
	}
	return nil
}

func toChatMessages(messages []*entities.Echo) []entities.Thread {
	result := make([]entities.Thread, 0, len(messages))
	for _, msg := range messages {
		result = append(result, msg.ToThread())
	}
	return result
}

func emptySession(memoryKey, subjectKey string) *entities.Memory {
	return &entities.Memory{
		MemoryKey:       memoryKey,
		SubjectKey:      subjectKey,
		Threads:         []entities.Thread{},
		Summary:         "",
		TopicFacts:      make(map[string]any),
		LastInteraction: helpers.NowUTC(),
	}
}
