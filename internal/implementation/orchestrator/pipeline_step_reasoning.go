package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

type ReasoningStep struct {
	reasoningService ReasoningExecutor
	timeout          time.Duration
	bond             ports.Bond
	lockTTL          time.Duration
	logger           *zap.SugaredLogger
}

func NewReasoningStep(
	reasoningService ReasoningExecutor,
	timeout time.Duration,
	bond ports.Bond,
	lockTTL time.Duration,
	logger *zap.SugaredLogger,
) *ReasoningStep {
	return &ReasoningStep{
		reasoningService: reasoningService,
		timeout:          timeout,
		bond:             bond,
		lockTTL:          lockTTL,
		logger:           logger,
	}
}

func (s *ReasoningStep) Name() string           { return "Reasoning" }
func (s *ReasoningStep) Timeout() time.Duration { return s.timeout }

func (s *ReasoningStep) Execute(ctx context.Context, state *ProcessingState) error {
	if state.Skipped {
		return nil
	}
	if state.Spirit != nil && state.Spirit.IsNotifier() {
		return nil
	}
	ctx = context.WithValue(ctx, ports.SessionContextKey{}, state.Session)
	ctx = context.WithValue(ctx, ports.PulseContextKey{}, state.Event)
	ctx = context.WithValue(ctx, ports.LinkContextKey{}, state.EventConfig)
	if state.Session != nil {
		state.SessionMessagesCount = len(state.Session.Threads)
	}

	if s.bond != nil && state.LockAcquired && state.LockKey != "" {
		done := make(chan struct{})
		defer close(done)
		go s.heartbeat(done, state.LockKey)
	}

	reasoningReq := s.buildReasoningRequest(state)

	var (
		result *ReasoningResult
		err    error
	)
	if state.streamSink != nil {
		result, err = s.executeStreaming(ctx, reasoningReq, state.streamSink)
	} else {
		result, err = s.reasoningService.Execute(ctx, reasoningReq)
	}

	s.populateStateFromResult(state, result)

	if err != nil {
		return ErrReasoningFailed(err)
	}

	return nil
}

// executeStreaming drives the streaming reasoning loop, forwarding answer tokens and tool-execution
// status to sink, and returns the same assembled ReasoningResult the buffered path produces.
func (s *ReasoningStep) executeStreaming(ctx context.Context, req *ReasoningRequest, sink reasoningEmitter) (*ReasoningResult, error) {
	events, err := s.reasoningService.ExecuteStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("opening reasoning stream: %w", err)
	}

	var result *ReasoningResult
	for ev := range events {
		switch ev.Type {
		case ReasoningEventDelta, ReasoningEventToolStatus:
			sink(ev)
		case ReasoningEventDone:
			result = ev.Result
		case ReasoningEventError:
			return ev.Result, ev.Err
		}
	}
	return result, nil
}

func (s *ReasoningStep) heartbeat(done <-chan struct{}, lockKey string) {
	interval := s.lockTTL / 3
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// Background ctx: the step ctx may cancel while the tick is in flight.
			if err := s.bond.ExtendLock(context.Background(), lockKey, s.lockTTL); err != nil {
				s.logger.Warnw("failed to extend reasoning lock", "error", err, "lock_key", lockKey)
			}
		}
	}
}

func (s *ReasoningStep) buildReasoningRequest(state *ProcessingState) *ReasoningRequest {
	return &ReasoningRequest{
		Event:               state.Event,
		Spirit:              state.Spirit,
		Session:             state.Session,
		SystemPrompt:        buildSystemPrompt(state.Spirit, state.Event),
		ConversationContext: buildMessageHistory(state.Session),
	}
}

func (s *ReasoningStep) populateStateFromResult(state *ProcessingState, result *ReasoningResult) {
	if result == nil {
		return
	}

	state.ReasoningResult = result
	state.Response = result.FinalResponse
	state.FinalError = result.FinalError
	state.ExecutionErrors = result.ExecutionErrors

	// Propagate the final session so that PersistenceStep always operates on
	// the session that was active at the end of the reasoning loop.
	// This matters when a tool triggered a topic switch mid-loop:
	// req.Session inside ReasoningService gets replaced with a freshly rebuilt
	// session, but state.Session would still point to the original — causing
	// PersistenceStep to clobber the rebuilt Redis entry with stale Threads.
	if result.FinalSession != nil {
		state.Session = result.FinalSession
	}

	if result.ResponseDelivered {
		state.ResponseDelivered = true
	}
}
