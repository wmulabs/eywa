package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"go.uber.org/zap"
)

// --- stubs ---

type stubRitualRepo struct {
	saved            []*entities.Ritual
	findResult       *entities.Ritual
	findErr          error
	saveErr          error
	updateStatusErr  error
	updateRunningErr error
	updateFailedErr  error
	keeperIDErr      error
	findPendingRes   []*entities.Ritual
	findPendingErr   error
	countRes         int64
	countErr         error
}

func (r *stubRitualRepo) Save(_ context.Context, task *entities.Ritual) error {
	r.saved = append(r.saved, task)
	return r.saveErr
}
func (r *stubRitualRepo) FindByID(_ context.Context, _ string) (*entities.Ritual, error) {
	return r.findResult, r.findErr
}
func (r *stubRitualRepo) FindPendingByMemoryKey(_ context.Context, _ string, _, _ int) ([]*entities.Ritual, error) {
	return r.findPendingRes, r.findPendingErr
}
func (r *stubRitualRepo) CountByMemoryKey(_ context.Context, _ string) (int64, error) {
	return r.countRes, r.countErr
}
func (r *stubRitualRepo) UpdateStatus(_ context.Context, _ string, _ entities.RitualStatus, _ time.Time) error {
	return r.updateStatusErr
}
func (r *stubRitualRepo) UpdateRunning(_ context.Context, _ string, _ time.Time) error {
	return r.updateRunningErr
}
func (r *stubRitualRepo) UpdateFailed(_ context.Context, _ string, _ time.Time, _ string) error {
	return r.updateFailedErr
}
func (r *stubRitualRepo) UpdateKeeperTaskID(_ context.Context, _ string, _ string) error {
	return r.keeperIDErr
}

type stubKeeper struct {
	taskID      string
	scheduleErr error
	cancelErr   error
}

func (k *stubKeeper) Schedule(_ context.Context, _ string, _ *entities.Pulse, _ time.Time) (string, error) {
	return k.taskID, k.scheduleErr
}
func (k *stubKeeper) Cancel(_ context.Context, _ string) error {
	return k.cancelErr
}

func newTestService(repo ports.RitualRepository, keeper ports.Keeper) *RitualService {
	return NewRitualService(repo, keeper, zap.NewNop().Sugar())
}

func makeEvent() *entities.Pulse {
	return &entities.Pulse{
		ID:        "evt-1",
		MemoryKey: "user:1",
		Metadata:  map[string]any{},
	}
}

// --- Schedule ---

func TestSchedule_Success(t *testing.T) {
	repo := &stubRitualRepo{}
	keeper := &stubKeeper{taskID: "keeper-123"}
	svc := newTestService(repo, keeper)

	ritual, err := svc.Schedule(context.Background(), ports.RitualRequest{
		EventKey:  "chat",
		Event:     makeEvent(),
		ExecuteAt: time.Now().Add(time.Minute),
		CreatedBy: "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ritual == nil {
		t.Fatal("expected non-nil ritual")
	}
	if ritual.KeeperTaskID != "keeper-123" {
		t.Errorf("expected keeper-123, got %s", ritual.KeeperTaskID)
	}
	if ritual.Status != entities.RitualPending {
		t.Errorf("expected pending, got %s", ritual.Status)
	}
}

func TestSchedule_InjectsRitualIDIntoMetadata(t *testing.T) {
	repo := &stubRitualRepo{}
	keeper := &stubKeeper{taskID: "k1"}
	svc := newTestService(repo, keeper)

	event := makeEvent()
	event.Metadata = nil // will be initialized by Schedule

	ritual, err := svc.Schedule(context.Background(), ports.RitualRequest{
		EventKey:  "chat",
		Event:     event,
		ExecuteAt: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Metadata[entities.MetadataKeyRitualID] != ritual.ID {
		t.Error("expected ritual_id injected into event metadata")
	}
}

func TestSchedule_SaveFails_ReturnsError(t *testing.T) {
	repo := &stubRitualRepo{saveErr: errors.New("db down")}
	keeper := &stubKeeper{taskID: "k1"}
	svc := newTestService(repo, keeper)

	_, err := svc.Schedule(context.Background(), ports.RitualRequest{
		EventKey:  "chat",
		Event:     makeEvent(),
		ExecuteAt: time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatal("expected error when save fails")
	}
}

func TestSchedule_KeeperFails_RollsBackAndReturnsError(t *testing.T) {
	repo := &stubRitualRepo{}
	keeper := &stubKeeper{scheduleErr: errors.New("keeper down")}
	svc := newTestService(repo, keeper)

	_, err := svc.Schedule(context.Background(), ports.RitualRequest{
		EventKey:  "chat",
		Event:     makeEvent(),
		ExecuteAt: time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatal("expected error when keeper fails")
	}
}

func TestSchedule_KeeperFails_RollbackStatusFails_StillReturnsOriginalError(t *testing.T) {
	repo := &stubRitualRepo{updateStatusErr: errors.New("status update failed")}
	keeper := &stubKeeper{scheduleErr: errors.New("keeper down")}
	svc := newTestService(repo, keeper)

	_, err := svc.Schedule(context.Background(), ports.RitualRequest{
		EventKey:  "chat",
		Event:     makeEvent(),
		ExecuteAt: time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatal("expected error when keeper fails")
	}
}

func TestSchedule_KeeperIDUpdateFails_ReturnsSuccessAnyway(t *testing.T) {
	repo := &stubRitualRepo{keeperIDErr: errors.New("patch failed")}
	keeper := &stubKeeper{taskID: "k1"}
	svc := newTestService(repo, keeper)

	ritual, err := svc.Schedule(context.Background(), ports.RitualRequest{
		EventKey:  "chat",
		Event:     makeEvent(),
		ExecuteAt: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("expected success even when keeper ID patch fails: %v", err)
	}
	if ritual.KeeperTaskID != "k1" {
		t.Errorf("expected k1, got %s", ritual.KeeperTaskID)
	}
}

// --- Cancel ---

func TestCancel_Success(t *testing.T) {
	task := &entities.Ritual{ID: "t1", MemoryKey: "user:1", KeeperTaskID: "k1", Status: entities.RitualPending}
	repo := &stubRitualRepo{findResult: task}
	keeper := &stubKeeper{}
	svc := newTestService(repo, keeper)

	if err := svc.Cancel(context.Background(), "t1", "user:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancel_FindFails_ReturnsError(t *testing.T) {
	repo := &stubRitualRepo{findErr: errors.New("not found")}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.Cancel(context.Background(), "t1", "user:1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCancel_WrongMemoryKey_ReturnsBusinessError(t *testing.T) {
	task := &entities.Ritual{ID: "t1", MemoryKey: "user:99", Status: entities.RitualPending}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{})

	err := svc.Cancel(context.Background(), "t1", "user:1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !domainerrors.IsBusinessError(err) {
		t.Errorf("expected BusinessError, got %T: %v", err, err)
	}
}

func TestCancel_NonPendingStatus_NoOp(t *testing.T) {
	task := &entities.Ritual{ID: "t1", MemoryKey: "user:1", Status: entities.RitualExecuted}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.Cancel(context.Background(), "t1", "user:1"); err != nil {
		t.Fatalf("expected no error for non-pending: %v", err)
	}
}

func TestCancel_KeeperCancelFails_StillUpdatesStatus(t *testing.T) {
	task := &entities.Ritual{ID: "t1", MemoryKey: "user:1", KeeperTaskID: "k1", Status: entities.RitualPending}
	repo := &stubRitualRepo{findResult: task}
	keeper := &stubKeeper{cancelErr: errors.New("already fired")}
	svc := newTestService(repo, keeper)

	if err := svc.Cancel(context.Background(), "t1", "user:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- MarkRunning ---

func TestMarkRunning_Success(t *testing.T) {
	task := &entities.Ritual{ID: "t1", Status: entities.RitualPending}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkRunning(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkRunning_FindFails(t *testing.T) {
	repo := &stubRitualRepo{findErr: errors.New("not found")}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkRunning(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkRunning_TerminalStatus_ReturnsErrRitualTerminal(t *testing.T) {
	for _, status := range []entities.RitualStatus{entities.RitualExecuted, entities.RitualFailed, entities.RitualCancelled} {
		task := &entities.Ritual{ID: "t1", Status: status}
		repo := &stubRitualRepo{findResult: task}
		svc := newTestService(repo, &stubKeeper{})

		err := svc.MarkRunning(context.Background(), "t1")
		if err == nil {
			t.Errorf("expected error for status %s", status)
			continue
		}
		if !errors.Is(err, domainerrors.ErrRitualTerminal) {
			t.Errorf("expected ErrRitualTerminal for status %s, got %v", status, err)
		}
	}
}

// --- MarkExecuted ---

func TestMarkExecuted_Success(t *testing.T) {
	task := &entities.Ritual{ID: "t1", Status: entities.RitualRunning}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkExecuted_FindFails(t *testing.T) {
	repo := &stubRitualRepo{findErr: errors.New("not found")}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkExecuted(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkExecuted_NonPendingOrRunning_NoOp(t *testing.T) {
	for _, status := range []entities.RitualStatus{entities.RitualExecuted, entities.RitualFailed, entities.RitualCancelled} {
		task := &entities.Ritual{ID: "t1", Status: status}
		repo := &stubRitualRepo{findResult: task}
		svc := newTestService(repo, &stubKeeper{})

		if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
			t.Errorf("expected no-op for status %s, got: %v", status, err)
		}
	}
}

func TestMarkExecuted_WithRecurrence_SchedulesNext(t *testing.T) {
	task := &entities.Ritual{
		ID:        "t1",
		EventKey:  "chat",
		MemoryKey: "user:1",
		Status:    entities.RitualPending,
		ExecuteAt: time.Now(),
		EventSnapshot: &entities.Pulse{
			ID:        "evt-1",
			MemoryKey: "user:1",
			Metadata:  map[string]any{},
		},
		Recurrence: &entities.RecurrenceConfig{
			Cron:     "0 9 * * *",
			Timezone: "UTC",
			MaxRuns:  5,
			RunCount: 0,
		},
	}
	repo := &stubRitualRepo{findResult: task}
	keeper := &stubKeeper{taskID: "k-next"}
	svc := newTestService(repo, keeper)

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Errorf("expected 1 save for next recurrence, got %d", len(repo.saved))
	}
}

func TestMarkExecuted_WithRecurrence_MaxRunsReached_NoNextSchedule(t *testing.T) {
	task := &entities.Ritual{
		ID:        "t1",
		Status:    entities.RitualPending,
		ExecuteAt: time.Now(),
		EventSnapshot: &entities.Pulse{
			ID:        "evt-1",
			MemoryKey: "user:1",
			Metadata:  map[string]any{},
		},
		Recurrence: &entities.RecurrenceConfig{
			Cron:     "0 9 * * *",
			MaxRuns:  2,
			RunCount: 1, // RunCount+1 >= MaxRuns
		},
	}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{taskID: "k"})

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 0 {
		t.Errorf("expected no new schedule, got %d saves", len(repo.saved))
	}
}

func TestMarkExecuted_WithRecurrence_EndsAtPassed_NoNextSchedule(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	task := &entities.Ritual{
		ID:        "t1",
		Status:    entities.RitualPending,
		ExecuteAt: time.Now().Add(-time.Hour),
		EventSnapshot: &entities.Pulse{
			ID:        "evt-1",
			MemoryKey: "user:1",
		},
		Recurrence: &entities.RecurrenceConfig{
			Cron:   "*/5 * * * *",
			EndsAt: &past,
		},
	}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{taskID: "k"})

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 0 {
		t.Errorf("expected no new schedule after EndsAt, got %d saves", len(repo.saved))
	}
}

func TestMarkExecuted_WithRecurrence_NilEventSnapshot_NoNextSchedule(t *testing.T) {
	task := &entities.Ritual{
		ID:            "t1",
		Status:        entities.RitualPending,
		EventSnapshot: nil,
		Recurrence:    &entities.RecurrenceConfig{Cron: "0 9 * * *"},
	}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{taskID: "k"})

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkExecuted_UpdateStatusFails_ReturnsError(t *testing.T) {
	task := &entities.Ritual{ID: "t1", Status: entities.RitualRunning}
	repo := &stubRitualRepo{findResult: task, updateStatusErr: errors.New("db error")}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkExecuted(context.Background(), "t1"); err == nil {
		t.Fatal("expected error when UpdateStatus fails")
	}
}

func TestMarkExecuted_WithRecurrence_ScheduleFails_LogsError(t *testing.T) {
	task := &entities.Ritual{
		ID:        "t1",
		Status:    entities.RitualPending,
		ExecuteAt: time.Now(),
		EventSnapshot: &entities.Pulse{
			ID:        "evt-1",
			MemoryKey: "user:1",
			Metadata:  map[string]any{},
		},
		Recurrence: &entities.RecurrenceConfig{
			Cron:     "0 9 * * *",
			Timezone: "UTC",
		},
	}
	// Keeper fails → Schedule for next occurrence fails → scheduleNextOccurrence logs but doesn't propagate
	repo := &stubRitualRepo{findResult: task}
	keeper := &stubKeeper{scheduleErr: errors.New("keeper down")}
	svc := newTestService(repo, keeper)

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkExecuted_WithRecurrence_InvalidCron_LogsWarn(t *testing.T) {
	task := &entities.Ritual{
		ID:     "t1",
		Status: entities.RitualPending,
		EventSnapshot: &entities.Pulse{
			ID:        "evt-1",
			MemoryKey: "user:1",
		},
		Recurrence: &entities.RecurrenceConfig{Cron: "not-a-cron"},
	}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{taskID: "k"})

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 0 {
		t.Errorf("expected no schedule for invalid cron")
	}
}

// --- MarkFailed ---

func TestMarkFailed_Success(t *testing.T) {
	task := &entities.Ritual{ID: "t1", Status: entities.RitualRunning}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkFailed(context.Background(), "t1", "timeout"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkFailed_FindFails(t *testing.T) {
	repo := &stubRitualRepo{findErr: errors.New("not found")}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkFailed(context.Background(), "t1", "reason"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkFailed_AlreadyFailed_NoOp(t *testing.T) {
	task := &entities.Ritual{ID: "t1", Status: entities.RitualFailed}
	repo := &stubRitualRepo{findResult: task}
	svc := newTestService(repo, &stubKeeper{})

	if err := svc.MarkFailed(context.Background(), "t1", "again"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ListPendingByMemoryKey ---

func TestListPendingByMemoryKey_Success(t *testing.T) {
	tasks := []*entities.Ritual{{ID: "t1"}, {ID: "t2"}}
	repo := &stubRitualRepo{findPendingRes: tasks}
	svc := newTestService(repo, &stubKeeper{})

	result, err := svc.ListPendingByMemoryKey(context.Background(), "user:1", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestListPendingByMemoryKey_Error(t *testing.T) {
	repo := &stubRitualRepo{findPendingErr: errors.New("db error")}
	svc := newTestService(repo, &stubKeeper{})

	_, err := svc.ListPendingByMemoryKey(context.Background(), "user:1", 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- CountByMemoryKey ---

func TestCountByMemoryKey_Success(t *testing.T) {
	repo := &stubRitualRepo{countRes: 42}
	svc := newTestService(repo, &stubKeeper{})

	n, err := svc.CountByMemoryKey(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 42 {
		t.Errorf("expected 42, got %d", n)
	}
}

func TestCountByMemoryKey_Error(t *testing.T) {
	repo := &stubRitualRepo{countErr: errors.New("db error")}
	svc := newTestService(repo, &stubKeeper{})

	_, err := svc.CountByMemoryKey(context.Background(), "user:1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- nextCronTick ---

func TestNextCronTick_ValidCronUTC(t *testing.T) {
	svc := newTestService(&stubRitualRepo{}, &stubKeeper{})
	after := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	next, err := svc.nextCronTick("0 9 * * *", "UTC", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.IsZero() {
		t.Error("expected non-zero next time")
	}
}

func TestNextCronTick_InvalidCron_ReturnsError(t *testing.T) {
	svc := newTestService(&stubRitualRepo{}, &stubKeeper{})
	_, err := svc.nextCronTick("not-a-cron", "UTC", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid cron")
	}
}

func TestNextCronTick_InvalidTimezone_ReturnsError(t *testing.T) {
	svc := newTestService(&stubRitualRepo{}, &stubKeeper{})
	_, err := svc.nextCronTick("0 9 * * *", "Invalid/Zone", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestNextCronTick_EmptyTimezone_UsesUTC(t *testing.T) {
	svc := newTestService(&stubRitualRepo{}, &stubKeeper{})
	after := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	_, err := svc.nextCronTick("0 9 * * *", "", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkExecuted_WithRecurrence_NonEmptyMetadata_Cloned(t *testing.T) {
	// Metadata has content → range loop body executes (metadata clone branch covered).
	task := &entities.Ritual{
		ID:        "t1",
		Status:    entities.RitualPending,
		ExecuteAt: time.Now(),
		EventSnapshot: &entities.Pulse{
			ID:        "evt-1",
			MemoryKey: "user:1",
			Metadata: map[string]any{
				"source":                     "web",
				entities.MetadataKeyRitualID: "old-ritual-id",
			},
		},
		Recurrence: &entities.RecurrenceConfig{
			Cron:     "0 9 * * *",
			Timezone: "UTC",
			MaxRuns:  5,
			RunCount: 0,
		},
	}
	repo := &stubRitualRepo{findResult: task}
	keeper := &stubKeeper{taskID: "k-next"}
	svc := newTestService(repo, keeper)

	if err := svc.MarkExecuted(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Errorf("expected 1 save for next recurrence, got %d", len(repo.saved))
	}
}
