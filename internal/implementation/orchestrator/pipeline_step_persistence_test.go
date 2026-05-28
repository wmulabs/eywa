package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
)

// --- stubs ---

type stubMemoryManager struct {
	session        *entities.Memory
	getOrCreateErr error
	saveErr        error
	updateErr      error
	lastTopicKey   string
	savedSession   *entities.Memory
	updatedThreads []entities.Thread
	rebuildResult  *entities.Memory
	rebuildErr     error
}

var _ MemoryManager = (*stubMemoryManager)(nil)

func (m *stubMemoryManager) GetOrCreate(_ context.Context, _, _ string) (*entities.Memory, error) {
	return m.session, m.getOrCreateErr
}
func (m *stubMemoryManager) RebuildForTopic(_ context.Context, _, _ string) (*entities.Memory, error) {
	return m.rebuildResult, m.rebuildErr
}
func (m *stubMemoryManager) FindLastTopicKey(_ context.Context, _ string) string {
	return m.lastTopicKey
}
func (m *stubMemoryManager) UpdateMemory(_ context.Context, _ *entities.Memory, thread entities.Thread) error {
	m.updatedThreads = append(m.updatedThreads, thread)
	return m.updateErr
}
func (m *stubMemoryManager) Save(_ context.Context, session *entities.Memory) error {
	m.savedSession = session
	return m.saveErr
}

type stubMessageManager struct {
	appended  []entities.Echo
	appendErr error
}

var _ MessageManager = (*stubMessageManager)(nil)

func (m *stubMessageManager) Append(_ context.Context, msg entities.Echo) error {
	m.appended = append(m.appended, msg)
	return m.appendErr
}

func persistenceState(response, userMsg string) *ProcessingState {
	return &ProcessingState{
		Event: &entities.Pulse{
			EventType:   "user_message",
			MemoryKey:   "user:1",
			UserMessage: userMsg,
		},
		Spirit:      &entities.Spirit{},
		EventConfig: &entities.Link{},
		Session:     &entities.Memory{MemoryKey: "user:1", SubjectKey: "default"},
		Response:    response,
	}
}

// --- PersistenceStep ---

func TestPersistenceStep_WithResponse_SavesMemory(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("hello back", "hello")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mm.savedSession == nil {
		t.Error("expected session to be saved")
	}
}

func TestPersistenceStep_WithResponse_AddsAssistantThread(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("hi there", "hi")

	_ = step.Execute(context.Background(), state)

	if len(mm.updatedThreads) == 0 {
		t.Fatal("expected UpdateMemory called with assistant thread")
	}
	thread := mm.updatedThreads[0]
	if thread.Content != "hi there" {
		t.Errorf("expected assistant content 'hi there', got '%s'", thread.Content)
	}
}

func TestPersistenceStep_SaveError_ReturnsError(t *testing.T) {
	mm := &stubMemoryManager{saveErr: errors.New("redis down")}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("", "")

	err := step.Execute(context.Background(), state)
	if err == nil {
		t.Error("expected error on save failure")
	}
}

func TestPersistenceStep_AppendsUserMessage(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("", "what's the weather?")

	_ = step.Execute(context.Background(), state)

	found := false
	for _, echo := range msgm.appended {
		if echo.Role == "user" && echo.Content == "what's the weather?" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected user message appended, got: %v", msgm.appended)
	}
}

func TestPersistenceStep_AppendsAssistantResponse(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("here is the answer", "question")

	_ = step.Execute(context.Background(), state)

	found := false
	for _, echo := range msgm.appended {
		if echo.Role == "assistant" && echo.Content == "here is the answer" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected assistant response appended, got: %v", msgm.appended)
	}
}

func TestPersistenceStep_AppendError_IsSwallowed(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{appendErr: errors.New("store down")}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("response", "user msg")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected append error to be swallowed, got: %v", err)
	}
}

func TestPersistenceStep_NoResponse_NoAssistantThread(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("", "user msg")

	_ = step.Execute(context.Background(), state)

	for _, t2 := range mm.updatedThreads {
		if t2.Role == "assistant" {
			t.Error("expected no assistant thread when response is empty")
		}
	}
}

func TestPersistenceStep_NilSession_Skips(t *testing.T) {
	mm := &stubMemoryManager{}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("response", "user msg")
	state.Session = nil

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected nil session to be skipped, got: %v", err)
	}
	if mm.savedSession != nil {
		t.Error("expected Save not called when session is nil")
	}
}

func TestPersistenceStep_UpdateMemoryError_IsSwallowed(t *testing.T) {
	mm := &stubMemoryManager{updateErr: errors.New("memory update failed")}
	msgm := &stubMessageManager{}
	step := NewPersistenceStep(mm, msgm, time.Second, testLogger(t))
	state := persistenceState("assistant reply", "user msg")

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected update memory error to be swallowed, got: %v", err)
	}
}
