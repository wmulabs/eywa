package orchestrator

import (
	"context"
	"strings"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

const conversationSummaryKey = "conversation_summary"

type SessionSetupStep struct {
	memoryManager MemoryManager
	timeout       time.Duration
	logger        *zap.SugaredLogger
}

func NewSessionSetupStep(memoryManager MemoryManager, timeout time.Duration, logger *zap.SugaredLogger) *SessionSetupStep {
	return &SessionSetupStep{memoryManager: memoryManager, timeout: timeout, logger: logger}
}

func (s *SessionSetupStep) Name() string           { return "SessionSetup" }
func (s *SessionSetupStep) Timeout() time.Duration { return s.timeout }

func (s *SessionSetupStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit != nil && !state.Spirit.NeedsSession() {
		return nil
	}

	// The event carries the topic when a converter or enricher resolved it deterministically.
	// When empty, restore continuity from the last known topic in the message store.
	effectiveTopic := state.Event.SubjectKey
	if effectiveTopic == "" {
		effectiveTopic = s.memoryManager.FindLastTopicKey(ctx, state.Event.MemoryKey)
	}

	// Each (memory_key, subject_key) pair has its own isolated Threads in Redis.
	session, err := s.memoryManager.GetOrCreate(ctx, state.Event.MemoryKey, effectiveTopic)
	if err != nil {
		return ErrMemoryOperationFailed("get_or_create", err)
	}
	state.Session = session

	s.exposeTopicToLLM(session, state.Event)

	return nil
}

// exposeTopicToLLM merges session state into event.Knowledge so the LLM can reason about it.
//
// Because sessions are isolated by composite key (memory_key + subject_key),
// TopicFacts and Summary always belong to the active topic — no stale data can bleed through.
//
// Merge strategy — scout knowledge takes precedence, session data fills gaps:
//  1. session.TopicFacts are merged non-destructively: existing event.Knowledge keys are not overwritten,
//     so scouts always win on conflicts.
//  2. session.Summary is injected only if no summary is already in event.Knowledge.
//  3. active_topic is always set last, unconditionally.
func (s *SessionSetupStep) exposeTopicToLLM(session *entities.Memory, event *entities.Pulse) {
	if session.SubjectKey == "" {
		return
	}

	for k, v := range session.TopicFacts {
		if _, exists := event.Knowledge[k]; !exists {
			event.AddKnowledge(k, v)
		}
	}

	if session.Summary != "" {
		if _, exists := event.Knowledge[conversationSummaryKey]; !exists {
			event.AddKnowledge(conversationSummaryKey, session.Summary)
		}
	}

	event.AddKnowledge("active_topic", session.SubjectKey)
}

// ArchivistStep compresses the oldest messages in Threads when the memory approaches
// the configured threshold. The resulting summary is stored in session.Summary
// and injected into event.Knowledge["conversation_summary"] for the LLM to use in this cycle.
//
// It is a no-op when:
//   - No summarizer is configured
//   - Threads length is below the threshold
//   - There are not enough messages to summarize (len <= keepRecent)
type ArchivistStep struct {
	summarizer ports.Archivist
	threshold  int
	keepRecent int
	timeout    time.Duration
	logger     *zap.SugaredLogger
}

func NewArchivistStep(
	summarizer ports.Archivist,
	threshold int,
	keepRecent int,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *ArchivistStep {
	if keepRecent < 1 {
		keepRecent = 1
	}
	return &ArchivistStep{
		summarizer: summarizer,
		threshold:  threshold,
		keepRecent: keepRecent,
		timeout:    timeout,
		logger:     logger,
	}
}

func (s *ArchivistStep) Name() string           { return "Archivist" }
func (s *ArchivistStep) Timeout() time.Duration { return s.timeout }

func (s *ArchivistStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit != nil && !state.Spirit.NeedsSession() {
		return nil
	}
	if s.summarizer == nil || state.Session == nil {
		return nil
	}

	memory := state.Session.Threads
	if len(memory) < s.threshold {
		return nil
	}

	splitAt := len(memory) - s.keepRecent
	if splitAt <= 0 {
		return nil
	}

	toSummarize := memory[:splitAt]
	toKeep := memory[splitAt:]

	s.logger.Infow("summarizing conversation segment",
		"memory_key", state.Event.MemoryKey,
		"subject_key", state.Session.SubjectKey,
		"messages_to_summarize", len(toSummarize),
		"messages_to_keep", len(toKeep))

	summary, err := s.summarizer.Summarize(ctx, state.Event.MemoryKey, state.Session.SubjectKey, toSummarize)
	if err != nil {
		s.logger.Errorw("summarization failed, skipping",
			"memory_key", state.Event.MemoryKey,
			"error", err)
		return nil // Non-critical: don't fail the pipeline
	}

	if summary == "" {
		return nil
	}

	state.Session.Summary = summary
	state.Session.Threads = toKeep
	state.ArchivistApplied = true

	if _, exists := state.Event.Knowledge[conversationSummaryKey]; !exists {
		state.Event.AddKnowledge(conversationSummaryKey, summary)
	}

	s.logger.Infow("summarization complete",
		"memory_key", state.Event.MemoryKey,
		"subject_key", state.Session.SubjectKey,
		"remaining_messages", len(toKeep))

	return nil
}

// MessageCoalescingStep is responsible for two things:
//  1. Draining the session inbox and merging buffered messages into a single user turn.
//  2. Adding the final UserMessage to session memory so it always reflects the coalesced value.
//
// minWindow guarantees a minimum elapsed time from pipeline start before draining, giving
// late-arriving messages a chance to accumulate in the inbox. Time spent in previous steps
// (enrichment, session setup, etc.) counts toward the window.
type MessageCoalescingStep struct {
	memoryManager MemoryManager
	messageInbox  ports.Inbox // optional; nil disables coalescing
	minWindow     time.Duration
	timeout       time.Duration
	logger        *zap.SugaredLogger
}

func NewMessageCoalescingStep(
	memoryManager MemoryManager,
	messageInbox ports.Inbox,
	minWindow time.Duration,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *MessageCoalescingStep {
	return &MessageCoalescingStep{
		memoryManager: memoryManager,
		messageInbox:  messageInbox,
		minWindow:     minWindow,
		timeout:       timeout,
		logger:        logger,
	}
}

func (s *MessageCoalescingStep) Name() string           { return "MessageCoalescing" }
func (s *MessageCoalescingStep) Timeout() time.Duration { return s.timeout }

func (s *MessageCoalescingStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Spirit != nil && !state.Spirit.NeedsMessageCoalescing() {
		return nil
	}
	if s.messageInbox != nil {
		if s.minWindow > 0 {
			if remaining := s.minWindow - time.Since(state.StartTime); remaining > 0 {
				select {
				case <-time.After(remaining):
				case <-ctx.Done():
				}
			}
		}

		messages, err := s.messageInbox.PopAll(ctx, state.Event.MemoryKey)
		if err != nil {
			s.logger.Warnw("failed to pop inbox messages, continuing with original",
				"error", err,
				"memory_key", state.Event.MemoryKey)
		} else if len(messages) > 1 {
			state.Event.UserMessage = strings.Join(messages, "\n")
			state.CoalescedMessagesCount = len(messages)
			s.logger.Infow("coalesced inbox messages into single user turn",
				"memory_key", state.Event.MemoryKey,
				"message_count", len(messages))
		}
	}

	if state.Event.UserMessage != "" && state.Session != nil {
		if err := s.memoryManager.UpdateMemory(ctx, state.Session, entities.Thread{
			Role:         ports.RoleUser,
			Content:      state.Event.UserMessage,
			IsUserFacing: true,
		}); err != nil {
			s.logger.Warnw("failed to update session memory with user message", "error", err)
		}
	}

	return nil
}
