package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- stubs ---

type stubBond struct {
	acquired    bool
	acquireErr  error
	releaseErr  error
}

var _ ports.Bond = (*stubBond)(nil)

func (b *stubBond) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return b.acquired, b.acquireErr
}
func (b *stubBond) ReleaseLock(_ context.Context, _ string) error { return b.releaseErr }
func (b *stubBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error {
	panic("not implemented")
}

type stubInbox struct {
	pushErr  error
	messages []string
	popErr   error
	pushed   []string
}

var _ ports.Inbox = (*stubInbox)(nil)

func (i *stubInbox) Push(_ context.Context, _ string, msg string) error {
	i.pushed = append(i.pushed, msg)
	return i.pushErr
}
func (i *stubInbox) PopAll(_ context.Context, _ string) ([]string, error) {
	return i.messages, i.popErr
}

func lockState(memoryKey, userMsg string) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{
			EventType:   "user_message",
			MemoryKey:   memoryKey,
			UserMessage: userMsg,
		},
		EventConfig: &entities.Link{},
		Spirit:      &entities.Spirit{},
	}
}

// --- LockAcquisitionStep ---

func TestLockAcquisitionStep_Acquired_SetsLockKey(t *testing.T) {
	bond := &stubBond{acquired: true}
	step := NewLockAcquisitionStep(bond, nil, time.Second, time.Second, testLogger(t))
	state := lockState("user:1", "")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.LockKey != fmt.Sprintf("memory:%s", "user:1") {
		t.Errorf("unexpected LockKey: %s", state.LockKey)
	}
	if !state.LockAcquired {
		t.Error("expected LockAcquired=true")
	}
}

func TestLockAcquisitionStep_NotAcquired_ReturnsMemoryBusyError(t *testing.T) {
	bond := &stubBond{acquired: false}
	step := NewLockAcquisitionStep(bond, nil, time.Second, time.Second, testLogger(t))
	state := lockState("user:2", "")

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected error when lock not acquired")
	}
	if !IsMemoryBusy(err) {
		t.Errorf("expected MemoryBusy error, got: %v", err)
	}
}

func TestLockAcquisitionStep_AcquireError_ReturnsOrchestrationError(t *testing.T) {
	bond := &stubBond{acquireErr: errors.New("redis down")}
	step := NewLockAcquisitionStep(bond, nil, time.Second, time.Second, testLogger(t))
	state := lockState("user:3", "")

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected error on acquire failure")
	}
	if !IsRetriable(err) {
		t.Errorf("expected retriable error, got: %v", err)
	}
}

func TestLockAcquisitionStep_WithInbox_PushesUserMessage(t *testing.T) {
	bond := &stubBond{acquired: true}
	inbox := &stubInbox{}
	step := NewLockAcquisitionStep(bond, inbox, time.Second, time.Second, testLogger(t))
	state := lockState("user:4", "hello world")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inbox.pushed) != 1 || inbox.pushed[0] != "hello world" {
		t.Errorf("expected message pushed to inbox, got: %v", inbox.pushed)
	}
}

func TestLockAcquisitionStep_WithInbox_PushError_DoesNotFail(t *testing.T) {
	bond := &stubBond{acquired: true}
	inbox := &stubInbox{pushErr: errors.New("inbox down")}
	step := NewLockAcquisitionStep(bond, inbox, time.Second, time.Second, testLogger(t))
	state := lockState("user:5", "msg")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected push error to be swallowed, got: %v", err)
	}
}

func TestLockAcquisitionStep_NoUserMessage_DoesNotPush(t *testing.T) {
	bond := &stubBond{acquired: true}
	inbox := &stubInbox{}
	step := NewLockAcquisitionStep(bond, inbox, time.Second, time.Second, testLogger(t))
	state := lockState("user:6", "") // no message

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inbox.pushed) != 0 {
		t.Errorf("expected no push for empty user message, got: %v", inbox.pushed)
	}
}

// --- LockReleaseStep ---

func TestLockReleaseStep_NotAcquired_NoOp(t *testing.T) {
	bond := &stubBond{}
	step := NewLockReleaseStep(bond, time.Second, testLogger(t))
	state := lockState("user:7", "")
	state.LockAcquired = false

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLockReleaseStep_Acquired_ReleasesLockAndClearsFlag(t *testing.T) {
	bond := &stubBond{}
	step := NewLockReleaseStep(bond, time.Second, testLogger(t))
	state := lockState("user:8", "")
	state.LockAcquired = true
	state.LockKey = "memory:user:8"

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.LockAcquired {
		t.Error("expected LockAcquired=false after release")
	}
}

func TestLockReleaseStep_ReleaseError_DoesNotFail(t *testing.T) {
	bond := &stubBond{releaseErr: errors.New("redis timeout")}
	step := NewLockReleaseStep(bond, time.Second, testLogger(t))
	state := lockState("user:9", "")
	state.LockAcquired = true
	state.LockKey = "memory:user:9"

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected release error to be swallowed, got: %v", err)
	}
}

func TestLockReleaseStep_AdditionalLocks_ReleasesAll(t *testing.T) {
	released := map[string]int{}
	bond := &stubBond{}
	// Override with a counting bond
	countingBond := &countingBond{released: released}
	step := NewLockReleaseStep(countingBond, time.Second, testLogger(t))
	state := lockState("user:10", "")
	state.LockAcquired = true
	state.LockKey = "memory:user:10"
	state.AdditionalLocks = []string{"memory:extra:1", "memory:extra:2"}
	_ = bond

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if released["memory:user:10"] != 1 {
		t.Error("expected primary lock released once")
	}
	if released["memory:extra:1"] != 1 || released["memory:extra:2"] != 1 {
		t.Error("expected additional locks released")
	}
}

type countingBond struct {
	released map[string]int
}

var _ ports.Bond = (*countingBond)(nil)

func (b *countingBond) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}
func (b *countingBond) ReleaseLock(_ context.Context, key string) error {
	b.released[key]++
	return nil
}
func (b *countingBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error {
	panic("not implemented")
}

func TestLockReleaseStep_AdditionalLock_ReleaseError_IsLogged(t *testing.T) {
	bond := &errBond{err: errors.New("release failed")}
	step := NewLockReleaseStep(bond, time.Second, testLogger(t))
	state := lockState("user:1", "")
	state.LockAcquired = true
	state.LockKey = "memory:user:1"
	state.AdditionalLocks = []string{"memory:extra:1"}

	// Should not return error even when ReleaseLock fails for additional locks
	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type errBond struct {
	err error
}

var _ ports.Bond = (*errBond)(nil)

func (b *errBond) AcquireLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}
func (b *errBond) ReleaseLock(_ context.Context, _ string) error {
	return b.err
}
func (b *errBond) ExtendLock(_ context.Context, _ string, _ time.Duration) error {
	panic("not implemented")
}
