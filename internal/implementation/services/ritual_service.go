package services

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/wmulabs/eywa/internal/domain/entities"
	domainerrors "github.com/wmulabs/eywa/internal/domain/errors"
	"github.com/wmulabs/eywa/internal/domain/ports"
	"github.com/wmulabs/eywa/internal/helpers"
	"go.uber.org/zap"
)

// Single authority for ritual lifecycle: scheduling, cancellation, execution tracking, and recurrence.
type RitualService struct {
	repo      ports.RitualRepository
	scheduler ports.Keeper
	logger    *zap.SugaredLogger
}

func NewRitualService(
	repo ports.RitualRepository,
	scheduler ports.Keeper,
	logger *zap.SugaredLogger,
) *RitualService {
	return &RitualService{repo: repo, scheduler: scheduler, logger: logger}
}

func (s *RitualService) Schedule(ctx context.Context, req ports.RitualRequest) (*entities.Ritual, error) {
	taskID := helpers.GenerateRandomID()

	if req.Event.Metadata == nil {
		req.Event.Metadata = make(map[string]any)
	}
	req.Event.Metadata[entities.MetadataKeyRitualID] = taskID

	// Persist before the external Cloud Tasks call so the record always exists.
	// KeeperTaskID is patched in a follow-up update once scheduling succeeds.
	task := &entities.Ritual{
		ID:            taskID,
		MemoryKey:     req.Event.MemoryKey,
		EventKey:      req.EventKey,
		KeeperTaskID:  "",
		ExecuteAt:     req.ExecuteAt,
		Status:        entities.RitualPending,
		CreatedBy:     req.CreatedBy,
		EventSnapshot: req.Event,
		Recurrence:    req.Recurrence,
		CreatedAt:     helpers.NowUTC(),
		Metadata:      req.Metadata,
	}

	if err := s.repo.Save(ctx, task); err != nil {
		return nil, fmt.Errorf("persist scheduled task: %w", err)
	}

	cloudTaskID, err := s.scheduler.Schedule(ctx, req.EventKey, req.Event, req.ExecuteAt)
	if err != nil {
		// Roll back the DB record so it does not linger as a phantom pending task.
		if statusErr := s.repo.UpdateStatus(ctx, taskID, entities.RitualCancelled, helpers.NowUTC()); statusErr != nil {
			s.logger.Errorw("failed to cancel phantom task after Keeper scheduling failure",
				"task_id", taskID,
				"memory_key", req.Event.MemoryKey,
				"status_err", statusErr,
			)
		}
		return nil, fmt.Errorf("schedule task: %w", err)
	}

	// Best-effort update — if this fails the task will still execute (MarkExecuted finds
	// by internal ID from metadata), but Cancel will not be able to delete the Cloud Task.
	if err := s.repo.UpdateKeeperTaskID(ctx, taskID, cloudTaskID); err != nil {
		s.logger.Warnw("failed to update keeper task ID on ritual",
			"task_id", taskID,
			"keeper_task_id", cloudTaskID,
			"error", err,
		)
	}

	task.KeeperTaskID = cloudTaskID
	return task, nil
}

func (s *RitualService) Cancel(ctx context.Context, id, memoryKey string) error {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	if task.MemoryKey != memoryKey {
		return domainerrors.NewBusinessError("task does not belong to this memory")
	}

	if task.Status != entities.RitualPending {
		return nil
	}

	// Cancel is best-effort — NotFound means already dispatched, which is fine.
	if err := s.scheduler.Cancel(ctx, task.KeeperTaskID); err != nil {
		s.logger.Warnw("failed to cancel Keeper task (may have already fired)",
			"task_id", id,
			"keeper_task_id", task.KeeperTaskID,
			"error", err,
		)
	}

	now := helpers.NowUTC()
	return s.repo.UpdateStatus(ctx, id, entities.RitualCancelled, now)
}

func (s *RitualService) MarkRunning(ctx context.Context, id string) error {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	switch task.Status {
	case entities.RitualExecuted, entities.RitualFailed, entities.RitualCancelled:
		return fmt.Errorf("task %s is %q: %w", id, task.Status, domainerrors.ErrRitualTerminal)
	}

	now := helpers.NowUTC()
	return s.repo.UpdateRunning(ctx, id, now)
}

func (s *RitualService) MarkExecuted(ctx context.Context, id string) error {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	if task.Status != entities.RitualPending && task.Status != entities.RitualRunning {
		return nil
	}

	now := helpers.NowUTC()
	if err := s.repo.UpdateStatus(ctx, id, entities.RitualExecuted, now); err != nil {
		return err
	}

	if task.Recurrence != nil {
		s.scheduleNextOccurrence(ctx, task)
	}

	return nil
}

func (s *RitualService) MarkFailed(ctx context.Context, id string, reason string) error {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	if task.Status == entities.RitualFailed {
		return nil
	}

	now := helpers.NowUTC()
	return s.repo.UpdateFailed(ctx, id, now, reason)
}

func (s *RitualService) ListPendingByMemoryKey(ctx context.Context, memoryKey string, limit, offset int) ([]*entities.Ritual, error) {
	return s.repo.FindPendingByMemoryKey(ctx, memoryKey, limit, offset)
}

func (s *RitualService) CountByMemoryKey(ctx context.Context, memoryKey string) (int64, error) {
	return s.repo.CountByMemoryKey(ctx, memoryKey)
}

// Failures are logged but not propagated — the current execution already succeeded.
func (s *RitualService) scheduleNextOccurrence(ctx context.Context, task *entities.Ritual) {
	if task.EventSnapshot == nil {
		s.logger.Warnw("cannot schedule recurrence: no event snapshot", "task_id", task.ID)
		return
	}

	rec := task.Recurrence

	if rec.MaxRuns > 0 && rec.RunCount+1 >= rec.MaxRuns {
		return
	}

	nextAt, err := s.nextCronTick(rec.Cron, rec.Timezone, task.ExecuteAt)
	if err != nil {
		s.logger.Warnw("failed to parse recurrence cron", "task_id", task.ID, "cron", rec.Cron, "error", err)
		return
	}

	if rec.EndsAt != nil && nextAt.After(*rec.EndsAt) {
		return
	}

	// Clone the event snapshot: fresh ID and timestamp, clear the old scheduled_task_id
	// (Schedule() will inject a new one for the next occurrence).
	nextEvent := *task.EventSnapshot
	nextEvent.ID = helpers.GenerateRandomID()
	nextEvent.Timestamp = helpers.NowUTC()
	if nextEvent.Metadata != nil {
		clonedMeta := make(map[string]any, len(nextEvent.Metadata))
		for k, v := range nextEvent.Metadata {
			clonedMeta[k] = v
		}
		delete(clonedMeta, entities.MetadataKeyRitualID)
		nextEvent.Metadata = clonedMeta
	}

	nextRecurrence := &entities.RecurrenceConfig{
		Cron:     rec.Cron,
		Timezone: rec.Timezone,
		EndsAt:   rec.EndsAt,
		MaxRuns:  rec.MaxRuns,
		RunCount: rec.RunCount + 1,
	}

	_, err = s.Schedule(ctx, ports.RitualRequest{
		EventKey:   task.EventKey,
		Event:      &nextEvent,
		ExecuteAt:  nextAt,
		CreatedBy:  "system",
		Recurrence: nextRecurrence,
		Metadata:   task.Metadata,
	})
	if err != nil {
		s.logger.Errorw("failed to schedule next recurrence", "task_id", task.ID, "next_at", nextAt, "error", err)
	}
}

func (s *RitualService) nextCronTick(expr, timezone string, after time.Time) (time.Time, error) {
	loc := time.UTC
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
		}
	}

	schedule, err := cron.ParseStandard(expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}

	return schedule.Next(after.In(loc)), nil
}
