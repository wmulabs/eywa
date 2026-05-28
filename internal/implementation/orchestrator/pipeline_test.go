package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

type testStep struct {
	name   string
	err    error
	called *bool
}

func (s *testStep) Name() string           { return s.name }
func (s *testStep) Timeout() time.Duration { return 0 }
func (s *testStep) Execute(_ context.Context, _ *ProcessingState) error {
	if s.called != nil {
		*s.called = true
	}
	return s.err
}

func newTestPipeline() *Pipeline {
	logger, _ := zap.NewDevelopment()
	return NewPipeline(logger.Sugar(), noop.NewTracerProvider().Tracer("test"))
}

func minimalState() *ProcessingState {
	return &ProcessingState{Event: &entities.Pulse{}}
}

func TestPipeline_DeferredStep_RunsOnSuccess(t *testing.T) {
	p := newTestPipeline()
	called := false
	p.AddDeferredStep(&testStep{name: "d", called: &called})

	if err := p.Execute(context.Background(), minimalState()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("deferred step not called on success")
	}
}

func TestPipeline_DeferredStep_RunsOnStepFailure(t *testing.T) {
	p := newTestPipeline()
	mainCalled, deferredCalled := false, false

	p.AddStep(&testStep{name: "main", err: errors.New("fail"), called: &mainCalled})
	p.AddDeferredStep(&testStep{name: "d", called: &deferredCalled})

	err := p.Execute(context.Background(), minimalState())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !deferredCalled {
		t.Error("deferred step not called on failure")
	}
}

func TestPipeline_DeferredStep_ErrorDoesNotOverwriteMainError(t *testing.T) {
	p := newTestPipeline()
	mainErr := errors.New("main")
	mainCalled, dCalled := false, false

	p.AddStep(&testStep{name: "main", err: mainErr, called: &mainCalled})
	p.AddDeferredStep(&testStep{name: "d", err: errors.New("deferred-err"), called: &dCalled})

	err := p.Execute(context.Background(), minimalState())
	if !errors.Is(err, mainErr) {
		t.Errorf("want main error %v, got %v", mainErr, err)
	}
	if !dCalled {
		t.Error("deferred step not called")
	}
}

func TestPipeline_ExecuteStep_WithTimeout_Completes(t *testing.T) {
	p := newTestPipeline()
	called := false
	step := &testStepWithTimeout{name: "timed", timeout: time.Second, called: &called}
	p.AddStep(step)

	if err := p.Execute(context.Background(), minimalState()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected step to be called")
	}
}

func TestPipeline_ExecuteStep_DeadlineExceeded_WrapsError(t *testing.T) {
	p := newTestPipeline()
	// Step takes 200ms but timeout is 1ms → deadline exceeded
	step := &sleepStep{name: "slow", timeout: time.Millisecond, sleep: 200 * time.Millisecond}
	p.AddStep(step)

	err := p.Execute(context.Background(), minimalState())
	if err == nil {
		t.Fatal("expected error on deadline exceeded")
	}
	orchErr, ok := err.(*OrchestrationError)
	if !ok {
		t.Fatalf("expected *OrchestrationError, got %T: %v", err, err)
	}
	if orchErr.Code != "TIMEOUT_EXCEEDED" {
		t.Errorf("expected code TIMEOUT_EXCEEDED, got %s", orchErr.Code)
	}
}

type testStepWithTimeout struct {
	name    string
	timeout time.Duration
	err     error
	called  *bool
}

func (s *testStepWithTimeout) Name() string           { return s.name }
func (s *testStepWithTimeout) Timeout() time.Duration { return s.timeout }
func (s *testStepWithTimeout) Execute(_ context.Context, _ *ProcessingState) error {
	if s.called != nil {
		*s.called = true
	}
	return s.err
}

type sleepStep struct {
	name    string
	timeout time.Duration
	sleep   time.Duration
}

func (s *sleepStep) Name() string           { return s.name }
func (s *sleepStep) Timeout() time.Duration { return s.timeout }
func (s *sleepStep) Execute(ctx context.Context, _ *ProcessingState) error {
	select {
	case <-time.After(s.sleep):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestPipeline_AllDeferredSteps_RunOnFailure(t *testing.T) {
	p := newTestPipeline()
	called1, called2 := false, false
	mainCalled := false

	p.AddStep(&testStep{name: "main", err: errors.New("fail"), called: &mainCalled})
	p.AddDeferredStep(&testStep{name: "d1", called: &called1})
	p.AddDeferredStep(&testStep{name: "d2", called: &called2})

	p.Execute(context.Background(), minimalState())

	if !called1 || !called2 {
		t.Errorf("not all deferred steps ran: d1=%v d2=%v", called1, called2)
	}
}
