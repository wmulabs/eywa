package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"github.com/wmulabs/eywa/internal/domain/ports"
)

// --- SessionSetupStep ---

func TestSessionSetupStep_NotifierSpirit_SkipsSession(t *testing.T) {
	mm := &stubMemoryManager{}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{EventType: "notification", MemoryKey: "user:1"},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeNotifier},
		EventConfig: &entities.Link{},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Session != nil {
		t.Error("expected session not loaded for notifier spirit")
	}
}

func TestSessionSetupStep_ConversationalSpirit_LoadsSession(t *testing.T) {
	session := &entities.Memory{MemoryKey: "user:1", SubjectKey: "default"}
	mm := &stubMemoryManager{session: session}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{EventType: "user_message", MemoryKey: "user:1", SubjectKey: "default"},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Session == nil {
		t.Fatal("expected session loaded")
	}
	if state.Session.MemoryKey != "user:1" {
		t.Errorf("unexpected memory key: %s", state.Session.MemoryKey)
	}
}

func TestSessionSetupStep_GetOrCreateError_ReturnsError(t *testing.T) {
	mm := &stubMemoryManager{getOrCreateErr: errors.New("redis down")}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{EventType: "user_message", MemoryKey: "user:1"},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	if err := step.Execute(context.Background(), state); err == nil {
		t.Error("expected error when GetOrCreate fails")
	}
}

func TestSessionSetupStep_EmptySubjectKey_UsesLastTopicKey(t *testing.T) {
	session := &entities.Memory{MemoryKey: "user:1", SubjectKey: "prior-topic"}
	mm := &stubMemoryManager{session: session, lastTopicKey: "prior-topic"}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:       &entities.Pulse{EventType: "user_message", MemoryKey: "user:1", SubjectKey: ""},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// exposeTopicToLLM is exercised via SessionSetupStep.Execute

func TestSessionSetupStep_ExposeTopicToLLM_MergesTopicFacts(t *testing.T) {
	session := &entities.Memory{
		MemoryKey:  "user:1",
		SubjectKey: "billing",
		TopicFacts: map[string]any{"plan": "premium"},
	}
	mm := &stubMemoryManager{session: session}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			EventType:  "user_message",
			MemoryKey:  "user:1",
			SubjectKey: "billing",
			Knowledge:  map[string]any{},
		},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	_ = step.Execute(context.Background(), state)

	if state.Event.Knowledge["plan"] != "premium" {
		t.Errorf("expected plan=premium in knowledge, got: %v", state.Event.Knowledge["plan"])
	}
	if state.Event.Knowledge["active_topic"] != "billing" {
		t.Errorf("expected active_topic=billing in knowledge, got: %v", state.Event.Knowledge["active_topic"])
	}
}

func TestSessionSetupStep_ExposeTopicToLLM_ExistingKeyNotOverwritten(t *testing.T) {
	session := &entities.Memory{
		MemoryKey:  "user:1",
		SubjectKey: "billing",
		TopicFacts: map[string]any{"plan": "free"},
	}
	mm := &stubMemoryManager{session: session}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			EventType:  "user_message",
			MemoryKey:  "user:1",
			SubjectKey: "billing",
			Knowledge:  map[string]any{"plan": "premium"}, // scout wins
		},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	_ = step.Execute(context.Background(), state)

	if state.Event.Knowledge["plan"] != "premium" {
		t.Errorf("expected scout value to be preserved, got: %v", state.Event.Knowledge["plan"])
	}
}

func TestSessionSetupStep_ExposeTopicToLLM_InjectsSummary(t *testing.T) {
	session := &entities.Memory{
		MemoryKey:  "user:1",
		SubjectKey: "support",
		Summary:    "User asked about refund last week.",
	}
	mm := &stubMemoryManager{session: session}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			EventType:  "user_message",
			MemoryKey:  "user:1",
			SubjectKey: "support",
			Knowledge:  map[string]any{},
		},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	_ = step.Execute(context.Background(), state)

	if state.Event.Knowledge["conversation_summary"] != "User asked about refund last week." {
		t.Errorf("expected summary injected, got: %v", state.Event.Knowledge["conversation_summary"])
	}
}

// --- ArchivistStep ---

type stubArchivist struct {
	summary string
	err     error
}

var _ ports.Archivist = (*stubArchivist)(nil)

func (a *stubArchivist) Summarize(_ context.Context, _, _ string, _ []entities.Thread) (string, error) {
	return a.summary, a.err
}

func archivistSession(threadCount int) *entities.Memory {
	threads := make([]entities.Thread, threadCount)
	for i := range threads {
		role := ports.RoleUser
		if i%2 == 1 {
			role = ports.RoleAssistant
		}
		threads[i] = entities.Thread{Role: role, Content: "msg", IsUserFacing: true}
	}
	return &entities.Memory{MemoryKey: "user:1", SubjectKey: "topic", Threads: threads}
}

func TestArchivistStep_NilSummarizer_NoOp(t *testing.T) {
	step := NewArchivistStep(nil, 5, 2, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Session: archivistSession(10),
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving when summarizer is nil")
	}
}

func TestArchivistStep_BelowThreshold_NoOp(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: "summary"}, 10, 2, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Session: archivistSession(3), // below threshold of 10
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving below threshold")
	}
}

func TestArchivistStep_AboveThreshold_SummarizesAndTrimsThreads(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: "Past: user asked about plans."}, 5, 2, time.Second, testLogger(t))
	session := archivistSession(8) // 8 threads, keep 2 → summarize 6
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", Knowledge: map[string]any{}},
		Session: session,
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !state.ArchivistApplied {
		t.Error("expected ArchivistApplied=true")
	}
	if len(state.Session.Threads) != 2 {
		t.Errorf("expected 2 threads kept, got %d", len(state.Session.Threads))
	}
	if state.Session.Summary != "Past: user asked about plans." {
		t.Errorf("unexpected summary: %s", state.Session.Summary)
	}
	if state.Event.Knowledge["conversation_summary"] != "Past: user asked about plans." {
		t.Errorf("expected summary in knowledge, got: %v", state.Event.Knowledge["conversation_summary"])
	}
}

func TestArchivistStep_SummarizerError_IsSwallowed(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{err: errors.New("llm down")}, 3, 1, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Session: archivistSession(5),
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected summarizer error swallowed, got: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving on error")
	}
}

func TestArchivistStep_EmptySummaryResult_NoOp(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: ""}, 3, 1, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Session: archivistSession(5),
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving when summary is empty")
	}
}

func TestArchivistStep_NotifierSpirit_SkipsArchiving(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: "summary"}, 2, 1, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Spirit:  &entities.Spirit{Type: entities.SpiritTypeNotifier},
		Session: archivistSession(5),
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving for notifier spirit")
	}
}

// --- MessageCoalescingStep ---

func TestMessageCoalescingStep_NilInbox_AddsUserMessageToSession(t *testing.T) {
	mm := &stubMemoryManager{}
	step := NewMessageCoalescingStep(mm, nil, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "hello"},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mm.updatedThreads) == 0 {
		t.Error("expected user message added to session memory")
	}
	if mm.updatedThreads[0].Content != "hello" {
		t.Errorf("unexpected thread content: %s", mm.updatedThreads[0].Content)
	}
}

func TestMessageCoalescingStep_WithInbox_CoalescesMessages(t *testing.T) {
	mm := &stubMemoryManager{}
	inbox := &stubInbox{messages: []string{"msg1", "msg2", "msg3"}}
	step := NewMessageCoalescingStep(mm, inbox, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "msg1"},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.CoalescedMessagesCount != 3 {
		t.Errorf("expected 3 coalesced messages, got %d", state.CoalescedMessagesCount)
	}
	expected := "msg1\nmsg2\nmsg3"
	if state.Event.UserMessage != expected {
		t.Errorf("expected coalesced message '%s', got '%s'", expected, state.Event.UserMessage)
	}
}

func TestMessageCoalescingStep_WithInbox_SingleMessage_NoCoalescing(t *testing.T) {
	mm := &stubMemoryManager{}
	inbox := &stubInbox{messages: []string{"only-msg"}}
	step := NewMessageCoalescingStep(mm, inbox, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "only-msg"},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	_ = step.Execute(context.Background(), state)

	if state.CoalescedMessagesCount != 0 {
		t.Errorf("expected no coalescing for single message, got count=%d", state.CoalescedMessagesCount)
	}
}

func TestMessageCoalescingStep_NotifierSpirit_SkipsCoalescing(t *testing.T) {
	mm := &stubMemoryManager{}
	inbox := &stubInbox{messages: []string{"msg1", "msg2"}}
	step := NewMessageCoalescingStep(mm, inbox, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "msg1"},
		Spirit:  &entities.Spirit{Type: entities.SpiritTypeNotifier},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	_ = step.Execute(context.Background(), state)

	if state.CoalescedMessagesCount != 0 {
		t.Error("expected no coalescing for notifier spirit")
	}
}

func TestMessageCoalescingStep_InboxPopError_ContinuesWithOriginal(t *testing.T) {
	mm := &stubMemoryManager{}
	inbox := &stubInbox{popErr: errors.New("inbox down")}
	step := NewMessageCoalescingStep(mm, inbox, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "original"},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected inbox pop error swallowed, got: %v", err)
	}
	if state.Event.UserMessage != "original" {
		t.Errorf("expected original message preserved, got: %s", state.Event.UserMessage)
	}
}

func TestMessageCoalescingStep_NilSession_SkipsMemoryUpdate(t *testing.T) {
	mm := &stubMemoryManager{}
	step := NewMessageCoalescingStep(mm, nil, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "hello"},
		Session: nil,
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mm.updatedThreads) != 0 {
		t.Error("expected no update when session is nil")
	}
}

func TestMessageCoalescingStep_EmptyUserMessage_SkipsMemoryUpdate(t *testing.T) {
	mm := &stubMemoryManager{}
	step := NewMessageCoalescingStep(mm, nil, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: ""},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mm.updatedThreads) != 0 {
		t.Error("expected no update when user message is empty")
	}
}

func TestMessageCoalescingStep_UpdateMemoryError_IsSwallowed(t *testing.T) {
	mm := &stubMemoryManager{updateErr: errors.New("redis down")}
	step := NewMessageCoalescingStep(mm, nil, 0, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1", UserMessage: "hello"},
		Session: &entities.Memory{MemoryKey: "user:1"},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Errorf("expected update error swallowed, got: %v", err)
	}
}

func TestMessageCoalescingStep_MinWindow_PastStartTime_NoSleep(t *testing.T) {
	mm := &stubMemoryManager{}
	inbox := &stubInbox{messages: []string{"msg1", "msg2"}}
	step := NewMessageCoalescingStep(mm, inbox, 50*time.Millisecond, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:     &entities.Pulse{MemoryKey: "user:1", UserMessage: "msg1"},
		Session:   &entities.Memory{MemoryKey: "user:1"},
		StartTime: time.Now().Add(-200 * time.Millisecond), // far in past → remaining <= 0
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMessageCoalescingStep_MinWindow_CtxCancelled_SelectsCtxDone(t *testing.T) {
	mm := &stubMemoryManager{}
	inbox := &stubInbox{messages: []string{"msg1", "msg2"}}
	step := NewMessageCoalescingStep(mm, inbox, 500*time.Millisecond, time.Second, testLogger(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	state := &ProcessingState{
		Event:     &entities.Pulse{MemoryKey: "user:1", UserMessage: "msg1"},
		Session:   &entities.Memory{MemoryKey: "user:1"},
		StartTime: time.Now(), // fresh start → remaining > 0
	}

	if err := step.Execute(ctx, state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- exposeTopicToLLM edge cases ---

func TestSessionSetupStep_ExposeTopicToLLM_EmptySubjectKey_NoOp(t *testing.T) {
	session := &entities.Memory{
		MemoryKey:  "user:1",
		SubjectKey: "", // triggers early return
		TopicFacts: map[string]any{"plan": "premium"},
	}
	mm := &stubMemoryManager{session: session}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			EventType: "user_message",
			MemoryKey: "user:1",
			Knowledge: map[string]any{},
		},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := state.Event.Knowledge["plan"]; exists {
		t.Error("expected no facts injected when SubjectKey is empty")
	}
	if _, exists := state.Event.Knowledge["active_topic"]; exists {
		t.Error("expected active_topic not set when SubjectKey is empty")
	}
}

func TestSessionSetupStep_ExposeTopicToLLM_SummaryAlreadyInKnowledge_NotOverwritten(t *testing.T) {
	session := &entities.Memory{
		MemoryKey:  "user:1",
		SubjectKey: "support",
		Summary:    "new summary from session",
	}
	mm := &stubMemoryManager{session: session}
	step := NewSessionSetupStep(mm, time.Second, testLogger(t))
	state := &ProcessingState{
		Event: &entities.Pulse{
			EventType: "user_message",
			MemoryKey: "user:1",
			Knowledge: map[string]any{
				conversationSummaryKey: "scout summary wins",
			},
		},
		Spirit:      &entities.Spirit{Type: entities.SpiritTypeConversational},
		EventConfig: &entities.Link{},
	}

	_ = step.Execute(context.Background(), state)

	if state.Event.Knowledge[conversationSummaryKey] != "scout summary wins" {
		t.Errorf("expected scout summary preserved, got: %v", state.Event.Knowledge[conversationSummaryKey])
	}
}

// --- ArchivistStep edge cases ---

func TestArchivistStep_NilSession_NoOp(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: "summary"}, 3, 1, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Session: nil,
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving when session is nil")
	}
}

func TestArchivistStep_SplitAtZero_NoOp(t *testing.T) {
	// keepRecent(5) >= len(messages)(5) → splitAt = 0 → early return
	step := NewArchivistStep(&stubArchivist{summary: "summary"}, 3, 5, time.Second, testLogger(t))
	state := &ProcessingState{
		Event:   &entities.Pulse{MemoryKey: "user:1"},
		Session: archivistSession(5),
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.ArchivistApplied {
		t.Error("expected no archiving when splitAt <= 0")
	}
}

func TestArchivistStep_SummaryAlreadyInKnowledge_NotOverwritten(t *testing.T) {
	step := NewArchivistStep(&stubArchivist{summary: "session summary"}, 3, 1, time.Second, testLogger(t))
	session := archivistSession(5)
	state := &ProcessingState{
		Event: &entities.Pulse{
			MemoryKey: "user:1",
			Knowledge: map[string]any{
				conversationSummaryKey: "existing summary",
			},
		},
		Session: session,
	}

	if err := step.Execute(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Event.Knowledge[conversationSummaryKey] != "existing summary" {
		t.Errorf("expected existing summary preserved, got: %v", state.Event.Knowledge[conversationSummaryKey])
	}
}
