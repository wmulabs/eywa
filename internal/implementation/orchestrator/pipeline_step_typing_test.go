package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.uber.org/zap"
)

type mockTypingIndicator struct {
	startCalled bool
	stopCalled  bool
	startErr    error
	stopErr     error
}

func (m *mockTypingIndicator) StartTyping(_ context.Context, _ string) error {
	m.startCalled = true
	return m.startErr
}

func (m *mockTypingIndicator) StopTyping(_ context.Context, _ string) error {
	m.stopCalled = true
	return m.stopErr
}

func testLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

func stateWithPhone(phone string) *ProcessingState {
	return &ProcessingState{Event: &entities.Pulse{ContactPhone: phone}}
}

func TestTypingStartStep_NilIndicator_NoOp(t *testing.T) {
	step := NewTypingStartStep(nil, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTypingStartStep_EmptyPhone_DoesNotCall(t *testing.T) {
	mock := &mockTypingIndicator{}
	step := NewTypingStartStep(mock, testLogger(t))
	step.Execute(context.Background(), stateWithPhone(""))
	if mock.startCalled {
		t.Error("StartTyping must not be called when ContactPhone is empty")
	}
}

func TestTypingStartStep_CallsStart(t *testing.T) {
	mock := &mockTypingIndicator{}
	step := NewTypingStartStep(mock, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mock.startCalled {
		t.Error("StartTyping was not called")
	}
}

func TestTypingStartStep_IndicatorError_DoesNotFailPipeline(t *testing.T) {
	mock := &mockTypingIndicator{startErr: errors.New("network error")}
	step := NewTypingStartStep(mock, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Error("typing indicator error must not fail the pipeline")
	}
}

func TestTypingStopStep_NilIndicator_NoOp(t *testing.T) {
	step := NewTypingStopStep(nil, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTypingStopStep_IndicatorError_DoesNotFailPipeline(t *testing.T) {
	mock := &mockTypingIndicator{stopErr: errors.New("network error")}
	step := NewTypingStopStep(mock, testLogger(t))
	if err := step.Execute(context.Background(), stateWithPhone("+5511999999999")); err != nil {
		t.Error("typing stop error must not fail the pipeline")
	}
}
