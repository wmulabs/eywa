package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/wmulabs/eywa/internal/domain/entities"
	"go.opentelemetry.io/otel/trace"
)

// --- stubs ---

type stubMemoryRepo struct {
	memory  *entities.Memory
	getErr  error
	saveErr error
}

func (r *stubMemoryRepo) GetMemory(_ context.Context, _ string) (*entities.Memory, error) {
	return r.memory, r.getErr
}
func (r *stubMemoryRepo) SaveMemory(_ context.Context, _ string, _ *entities.Memory) error {
	return r.saveErr
}
func (r *stubMemoryRepo) DeleteMemory(_ context.Context, _ string) error { return nil }

type stubEchoRepo struct {
	messages  []*entities.Echo
	appendErr error
	findErr   error
}

func (r *stubEchoRepo) Append(_ context.Context, _ entities.Echo) error { return r.appendErr }
func (r *stubEchoRepo) FindRecentByMemoryKey(_ context.Context, _ string, _ int) ([]*entities.Echo, error) {
	return r.messages, r.findErr
}
func (r *stubEchoRepo) FindRecentByMemoryKeyAndSubject(_ context.Context, _, _ string, _ int) ([]*entities.Echo, error) {
	return r.messages, r.findErr
}
func (r *stubEchoRepo) FindByMemoryKey(_ context.Context, _ string, _, _ int) ([]*entities.Echo, error) {
	return r.messages, r.findErr
}
func (r *stubEchoRepo) FindByMemoryKeyAndSubject(_ context.Context, _, _ string, _, _ int) ([]*entities.Echo, error) {
	return r.messages, r.findErr
}
func (r *stubEchoRepo) FindBySubjectKey(_ context.Context, _ string, _, _ int) ([]*entities.Echo, error) {
	return r.messages, r.findErr
}
func (r *stubEchoRepo) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return int64(len(r.messages)), r.findErr
}
func (r *stubEchoRepo) CountByMemoryKeyAndSubject(_ context.Context, _, _ string) (int64, error) {
	return int64(len(r.messages)), r.findErr
}
func (r *stubEchoRepo) CountBySubjectKey(_ context.Context, _ string) (int64, error) {
	return int64(len(r.messages)), r.findErr
}

func noopTracer() trace.Tracer {
	return trace.NewNoopTracerProvider().Tracer("test")
}

func newTestMemoryManager(t *testing.T, memRepo *stubMemoryRepo, echoRepo *stubEchoRepo) *DefaultMemoryManager {
	t.Helper()
	mgr := NewMemoryManager(memRepo, echoRepo, 5, testLogger(t), noopTracer())
	return mgr
}

// --- buildRedisKey ---

func TestBuildRedisKey_NoSubject(t *testing.T) {
	if got := buildRedisKey("whatsapp:+55", ""); got != "whatsapp:+55" {
		t.Errorf("expected whatsapp:+55, got %s", got)
	}
}

func TestBuildRedisKey_WithSubject(t *testing.T) {
	if got := buildRedisKey("whatsapp:+55", "order:1"); got != "whatsapp:+55:order:1" {
		t.Errorf("unexpected key: %s", got)
	}
}

// --- NewMemoryManager ---

func TestNewMemoryManager_DefaultsWhenZeroMaxMessages(t *testing.T) {
	mgr := NewMemoryManager(&stubMemoryRepo{}, &stubEchoRepo{}, 0, testLogger(t), noopTracer())
	if mgr.maxMemoryMessages != DefaultMaxMemoryMessages {
		t.Errorf("expected default %d, got %d", DefaultMaxMemoryMessages, mgr.maxMemoryMessages)
	}
}

func TestNewMemoryManager_CustomMaxMessages(t *testing.T) {
	mgr := NewMemoryManager(&stubMemoryRepo{}, &stubEchoRepo{}, 20, testLogger(t), noopTracer())
	if mgr.maxMemoryMessages != 20 {
		t.Errorf("expected 20, got %d", mgr.maxMemoryMessages)
	}
}

// --- WithMemoryReconstruction ---

func TestWithMemoryReconstruction_SetsFields(t *testing.T) {
	mgr := NewMemoryManager(&stubMemoryRepo{}, &stubEchoRepo{}, 5, testLogger(t), noopTracer())
	mgr.WithMemoryReconstruction(false, 25)
	if mgr.enableMemoryReconstruction {
		t.Error("expected enableMemoryReconstruction=false")
	}
	if mgr.memoryReconstructionLimit != 25 {
		t.Errorf("expected limit=25, got %d", mgr.memoryReconstructionLimit)
	}
}

func TestWithMemoryReconstruction_ZeroLimitIgnored(t *testing.T) {
	mgr := NewMemoryManager(&stubMemoryRepo{}, &stubEchoRepo{}, 5, testLogger(t), noopTracer())
	original := mgr.memoryReconstructionLimit
	mgr.WithMemoryReconstruction(true, 0)
	if mgr.memoryReconstructionLimit != original {
		t.Errorf("expected limit unchanged at %d, got %d", original, mgr.memoryReconstructionLimit)
	}
}

// --- GetOrCreate ---

func TestGetOrCreate_CacheHit(t *testing.T) {
	mem := &entities.Memory{MemoryKey: "user:1"}
	mgr := newTestMemoryManager(t, &stubMemoryRepo{memory: mem}, nil)

	result, err := mgr.GetOrCreate(context.Background(), "user:1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MemoryKey != "user:1" {
		t.Errorf("expected user:1, got %s", result.MemoryKey)
	}
}

func TestGetOrCreate_CacheMiss_ReconstructionDisabled_ReturnsEmpty(t *testing.T) {
	memRepo := &stubMemoryRepo{getErr: errors.New("miss")}
	mgr := newTestMemoryManager(t, memRepo, nil)
	mgr.enableMemoryReconstruction = false

	result, err := mgr.GetOrCreate(context.Background(), "user:1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MemoryKey != "user:1" {
		t.Errorf("expected user:1, got %s", result.MemoryKey)
	}
	if len(result.Threads) != 0 {
		t.Error("expected empty threads")
	}
}

func TestGetOrCreate_CacheMiss_ReconstructionEnabled_NoMessages(t *testing.T) {
	memRepo := &stubMemoryRepo{getErr: errors.New("miss")}
	echoRepo := &stubEchoRepo{messages: nil}
	mgr := newTestMemoryManager(t, memRepo, echoRepo)

	result, err := mgr.GetOrCreate(context.Background(), "user:1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Threads) != 0 {
		t.Error("expected empty threads when no messages")
	}
}

func TestGetOrCreate_CacheMiss_ReconstructionEnabled_WithMessages(t *testing.T) {
	memRepo := &stubMemoryRepo{getErr: errors.New("miss")}
	echoRepo := &stubEchoRepo{messages: []*entities.Echo{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}}
	mgr := newTestMemoryManager(t, memRepo, echoRepo)

	result, err := mgr.GetOrCreate(context.Background(), "user:1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Threads) != 2 {
		t.Errorf("expected 2 threads, got %d", len(result.Threads))
	}
}

func TestGetOrCreate_CacheMiss_SaveFails_ReturnsError(t *testing.T) {
	memRepo := &stubMemoryRepo{getErr: errors.New("miss"), saveErr: errors.New("save fail")}
	mgr := newTestMemoryManager(t, memRepo, nil)
	mgr.enableMemoryReconstruction = false

	_, err := mgr.GetOrCreate(context.Background(), "user:1", "")
	if err == nil {
		t.Fatal("expected error when save fails")
	}
}

// --- FindLastTopicKey ---

func TestFindLastTopicKey_NilMessageRepo_ReturnsEmpty(t *testing.T) {
	mgr := NewMemoryManager(&stubMemoryRepo{}, nil, 5, testLogger(t), noopTracer())
	key := mgr.FindLastTopicKey(context.Background(), "user:1")
	if key != "" {
		t.Errorf("expected empty, got %q", key)
	}
}

func TestFindLastTopicKey_FindFails_ReturnsEmpty(t *testing.T) {
	echoRepo := &stubEchoRepo{findErr: errors.New("db error")}
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, echoRepo)
	key := mgr.FindLastTopicKey(context.Background(), "user:1")
	if key != "" {
		t.Errorf("expected empty on error, got %q", key)
	}
}

func TestFindLastTopicKey_NoMessages_ReturnsEmpty(t *testing.T) {
	echoRepo := &stubEchoRepo{messages: []*entities.Echo{}}
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, echoRepo)
	key := mgr.FindLastTopicKey(context.Background(), "user:1")
	if key != "" {
		t.Errorf("expected empty for no messages, got %q", key)
	}
}

func TestFindLastTopicKey_ReturnsLastSubjectKey(t *testing.T) {
	echoRepo := &stubEchoRepo{messages: []*entities.Echo{{SubjectKey: "order:42"}}}
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, echoRepo)
	key := mgr.FindLastTopicKey(context.Background(), "user:1")
	if key != "order:42" {
		t.Errorf("expected order:42, got %q", key)
	}
}

// --- RebuildForTopic ---

func TestRebuildForTopic_Success(t *testing.T) {
	echoRepo := &stubEchoRepo{messages: []*entities.Echo{
		{Role: "user", Content: "track order"},
	}}
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, echoRepo)

	mem, err := mgr.RebuildForTopic(context.Background(), "user:1", "order:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mem.Threads) != 1 {
		t.Errorf("expected 1 thread, got %d", len(mem.Threads))
	}
}

func TestRebuildForTopic_SaveFails_ReturnsError(t *testing.T) {
	memRepo := &stubMemoryRepo{saveErr: errors.New("save fail")}
	mgr := newTestMemoryManager(t, memRepo, &stubEchoRepo{})

	_, err := mgr.RebuildForTopic(context.Background(), "user:1", "order:1")
	if err == nil {
		t.Fatal("expected error when save fails")
	}
}

// --- UpdateMemory ---

func TestUpdateMemory_AppendsThread(t *testing.T) {
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, &stubEchoRepo{})
	session := &entities.Memory{MemoryKey: "user:1", Threads: []entities.Thread{}}
	msg := entities.Thread{Role: "user", Content: "hello"}

	if err := mgr.UpdateMemory(context.Background(), session, msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(session.Threads) != 1 {
		t.Errorf("expected 1 thread, got %d", len(session.Threads))
	}
}

func TestUpdateMemory_TrimsWhenExceedsMax(t *testing.T) {
	mgr := NewMemoryManager(&stubMemoryRepo{}, &stubEchoRepo{}, 3, testLogger(t), noopTracer())
	session := &entities.Memory{
		MemoryKey: "user:1",
		Threads: []entities.Thread{
			{Role: "user", Content: "1"},
			{Role: "assistant", Content: "2"},
			{Role: "user", Content: "3"},
		},
	}
	// Adding a 4th message should trim to last 3.
	if err := mgr.UpdateMemory(context.Background(), session, entities.Thread{Role: "assistant", Content: "4"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(session.Threads) != 3 {
		t.Errorf("expected 3 threads after trim, got %d", len(session.Threads))
	}
	if session.Threads[2].Content != "4" {
		t.Error("expected last thread to be the newly added message")
	}
}

// --- Save ---

func TestSave_Success(t *testing.T) {
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, &stubEchoRepo{})
	session := &entities.Memory{MemoryKey: "user:1", SubjectKey: ""}

	if err := mgr.Save(context.Background(), session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSave_Error(t *testing.T) {
	memRepo := &stubMemoryRepo{saveErr: errors.New("save fail")}
	mgr := newTestMemoryManager(t, memRepo, &stubEchoRepo{})
	session := &entities.Memory{MemoryKey: "user:1"}

	if err := mgr.Save(context.Background(), session); err == nil {
		t.Fatal("expected error")
	}
}

// --- emptySession ---

func TestEmptySession_HasCorrectFields(t *testing.T) {
	s := emptySession("user:1", "order:1")
	if s.MemoryKey != "user:1" {
		t.Errorf("expected user:1, got %s", s.MemoryKey)
	}
	if s.SubjectKey != "order:1" {
		t.Errorf("expected order:1, got %s", s.SubjectKey)
	}
	if len(s.Threads) != 0 {
		t.Error("expected empty threads")
	}
	if s.TopicFacts == nil {
		t.Error("expected non-nil TopicFacts")
	}
}

// --- toChatMessages ---

func TestToChatMessages_MapsEchoes(t *testing.T) {
	echoes := []*entities.Echo{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	threads := toChatMessages(echoes)
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	if threads[0].Role != "user" {
		t.Errorf("expected user, got %s", threads[0].Role)
	}
}

// --- rebuildMemoryFromMessages with subjectKey ---

func TestRebuildMemory_WithSubjectKey(t *testing.T) {
	echoRepo := &stubEchoRepo{messages: []*entities.Echo{
		{Role: "user", Content: "where is order?"},
	}}
	mgr := newTestMemoryManager(t, &stubMemoryRepo{}, echoRepo)

	mem := mgr.rebuildMemoryFromMessages(context.Background(), "user:1", "order:99")
	if mem.SubjectKey != "order:99" {
		t.Errorf("expected subject order:99, got %s", mem.SubjectKey)
	}
	if len(mem.Threads) != 1 {
		t.Errorf("expected 1 thread, got %d", len(mem.Threads))
	}
}
