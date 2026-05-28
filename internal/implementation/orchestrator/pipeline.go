package orchestrator

import (
	"context"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ProcessingState struct {
	// Input
	Event       *entities.Pulse
	EventKey    string
	EventConfig *entities.Link

	// Processing artifacts
	SpiritName string
	Spirit     *entities.Spirit
	Session    *entities.Memory

	// Reasoning results
	ReasoningResult *ReasoningResult
	Response        string   // Final response
	FinalError      string   // Final error that stopped processing
	ExecutionErrors []string // General execution errors (not iteration-specific)

	// Timing
	StartTime time.Time

	// Lock management
	LockKey         string
	LockAcquired    bool
	AdditionalLocks []string // Additional locks acquired if memory_key changes

	ResponseDelivered bool // true once the end-user received a response this cycle
	Skipped           bool // true when a ConditionEvaluationStep vetoes notifier delivery

	// Token usage from media processing fallbacks (audio transcription, image analysis, document extraction)
	MediaTokensUsed ports.OracleUsage

	// Audit fields — populated by steps for InteractionLog
	ProcessingStatus       string // "success", "rate_limited", "validation_failed", etc.
	PipelineFailedAtStep   string // name of the step that caused pipeline failure
	PathfinderUsed         string // Pathfinder that selected the Spirit
	SessionMessagesCount   int    // Threads depth at reasoning start
	CoalescedMessagesCount int    // messages merged by inbox coalescing (0 = no coalescing)
	ArchivistApplied       bool   // true when summarization ran this turn
}

type ProcessingStep interface {
	Name() string
	Execute(ctx context.Context, state *ProcessingState) error
	Timeout() time.Duration // 0 = no timeout
}

type Pipeline struct {
	steps         []ProcessingStep
	deferredSteps []ProcessingStep
	logger        *zap.SugaredLogger
	tracer        trace.Tracer
}

func NewPipeline(logger *zap.SugaredLogger, tracer trace.Tracer) *Pipeline {
	return &Pipeline{logger: logger, tracer: tracer}
}

func (p *Pipeline) AddStep(step ProcessingStep) *Pipeline {
	p.steps = append(p.steps, step)
	return p
}

func (p *Pipeline) AddDeferredStep(step ProcessingStep) *Pipeline {
	p.deferredSteps = append(p.deferredSteps, step)
	return p
}

func (p *Pipeline) Execute(ctx context.Context, state *ProcessingState) (returnErr error) {
	ctx, span := p.tracer.Start(ctx, "Pipeline/Execute")
	defer span.End()

	defer func() {
		for _, step := range p.deferredSteps {
			if err := p.executeStep(ctx, step, state); err != nil {
				p.logger.Errorw("deferred step failed",
					"step", step.Name(),
					"error", err,
					"event_id", state.Event.ID,
					"memory_key", state.Event.MemoryKey,
				)
				span.AddEvent("deferred_step_failed",
					trace.WithAttributes(
						attribute.String("step", step.Name()),
						attribute.String("error", err.Error()),
					),
				)
			}
		}
	}()

	state.StartTime = time.Now()
	state.ProcessingStatus = "success"

	for _, step := range p.steps {
		if err := p.executeStep(ctx, step, state); err != nil {
			p.logger.Errorw("pipeline step failed",
				"step", step.Name(),
				"error", err,
				"event_id", state.Event.ID,
				"memory_key", state.Event.MemoryKey,
			)
			state.PipelineFailedAtStep = step.Name()
			return err
		}
	}

	return nil
}

func (p *Pipeline) executeStep(ctx context.Context, step ProcessingStep, state *ProcessingState) error {
	ctx, span := p.tracer.Start(ctx, "Pipeline/"+step.Name())
	defer span.End()

	timeout := step.Timeout()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	p.logger.Debugw("executing pipeline step",
		"step", step.Name(),
		"timeout", timeout,
		"event_id", state.Event.ID,
	)

	stepStart := time.Now()
	err := step.Execute(ctx, state)
	stepDuration := time.Since(stepStart)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ErrTimeoutExceeded(step.Name())
		}
		return err
	}

	p.logger.Debugw("pipeline step completed",
		"step", step.Name(),
		"duration_ms", stepDuration.Milliseconds(),
		"event_id", state.Event.ID,
	)

	return nil
}
