package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// LockAcquisitionStep acquires the distributed session lock.
// When a MessageInbox is configured, the incoming user message is pushed to the inbox
// before the lock attempt. If the lock is already held by another cycle, ErrMemoryBusy
// is returned (non-retriable) — the message is safely in the inbox for the next cycle.
type LockAcquisitionStep struct {
	distributedLock ports.Bond
	messageInbox    ports.Inbox // optional; nil disables inbox support
	lockTTL         time.Duration
	timeout         time.Duration
	logger          *zap.SugaredLogger
}

func NewLockAcquisitionStep(
	distributedLock ports.Bond,
	messageInbox ports.Inbox,
	lockTTL time.Duration,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *LockAcquisitionStep {
	return &LockAcquisitionStep{
		distributedLock: distributedLock,
		messageInbox:    messageInbox,
		lockTTL:         lockTTL,
		timeout:         timeout,
		logger:          logger,
	}
}

func (s *LockAcquisitionStep) Name() string           { return "LockAcquisition" }
func (s *LockAcquisitionStep) Timeout() time.Duration { return s.timeout }

func (s *LockAcquisitionStep) Execute(ctx context.Context, state *ProcessingState) error {
	if s.messageInbox != nil && state.Event.UserMessage != "" {
		if err := s.messageInbox.Push(ctx, state.Event.MemoryKey, state.Event.UserMessage); err != nil {
			// Non-critical: log and continue — inbox failure must not block processing.
			s.logger.Warnw("failed to push message to inbox",
				"error", err,
				"memory_key", state.Event.MemoryKey)
		}
	}

	state.LockKey = fmt.Sprintf("memory:%s", state.Event.MemoryKey)
	acquired, err := s.distributedLock.AcquireLock(ctx, state.LockKey, s.lockTTL)
	if err != nil {
		return ErrLockAcquisitionFailed(state.Event.MemoryKey, err)
	}
	if !acquired {
		return ErrMemoryBusy(state.Event.MemoryKey)
	}
	state.LockAcquired = true
	return nil
}

type LockReleaseStep struct {
	distributedLock ports.Bond
	timeout         time.Duration
	logger          *zap.SugaredLogger
}

func NewLockReleaseStep(
	distributedLock ports.Bond,
	timeout time.Duration,
	logger *zap.SugaredLogger,
) *LockReleaseStep {
	return &LockReleaseStep{distributedLock: distributedLock, timeout: timeout, logger: logger}
}

func (s *LockReleaseStep) Name() string           { return "LockRelease" }
func (s *LockReleaseStep) Timeout() time.Duration { return s.timeout }

func (s *LockReleaseStep) Execute(ctx context.Context, state *ProcessingState) error {
	if !state.LockAcquired {
		return nil
	}

	if err := s.distributedLock.ReleaseLock(ctx, state.LockKey); err != nil {
		s.logger.Errorw("failed to release lock", "error", err, "memory_key", state.Event.MemoryKey, "lock_key", state.LockKey)
	} else {
		s.logger.Infow("lock released", "lock_key", state.LockKey)
	}

	for _, additionalLockKey := range state.AdditionalLocks {
		if err := s.distributedLock.ReleaseLock(ctx, additionalLockKey); err != nil {
			s.logger.Errorw("failed to release additional lock", "error", err, "lock_key", additionalLockKey)
		} else {
			s.logger.Infow("additional lock released", "lock_key", additionalLockKey)
		}
	}

	state.LockAcquired = false
	return nil
}
