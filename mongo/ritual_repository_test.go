package mongo

import (
	"context"
	"errors"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newRitualRepo(mt *mtest.T) *RitualRepository {
	return &RitualRepository{
		collection: mt.DB.Collection("scheduled_tasks"),
		logger:     zap.NewNop().Sugar(),
	}
}

func TestRitualRepository_Save_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		task := &eywa.Ritual{MemoryKey: "user:1", EventKey: "ping", Status: eywa.RitualPending}
		if err := newRitualRepo(mt).Save(context.Background(), task); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if task.ID == "" {
			t.Error("expected ID assigned")
		}
	})
}

func TestRitualRepository_FindByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "ritual-1"},
			{Key: "memory_key", Value: "user:1"},
			{Key: "event_key", Value: "ping"},
			{Key: "status", Value: string(eywa.RitualPending)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.scheduled_tasks", mtest.FirstBatch, doc))

		task, err := newRitualRepo(mt).FindByID(context.Background(), "ritual-1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if task.ID != "ritual-1" || task.MemoryKey != "user:1" {
			t.Errorf("unexpected task: %+v", task)
		}
	})
}

func TestRitualRepository_FindByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.scheduled_tasks", mtest.FirstBatch))

		_, err := newRitualRepo(mt).FindByID(context.Background(), "missing")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestRitualRepository_UpdateStatus_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		err := newRitualRepo(mt).UpdateStatus(context.Background(), "ritual-1", eywa.RitualExecuted, time.Now())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRitualRepository_UpdateStatus_UnknownStatus_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("unknown status", func(mt *mtest.T) {
		err := newRitualRepo(mt).UpdateStatus(context.Background(), "ritual-1", "unknown", time.Now())
		if err == nil {
			t.Error("expected error for unknown status")
		}
	})
}

func TestRitualRepository_UpdateStatus_Cancelled_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("cancelled", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		if err := newRitualRepo(mt).UpdateStatus(context.Background(), "r-1", eywa.RitualCancelled, time.Now()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRitualRepository_UpdateStatus_Running_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("running", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		if err := newRitualRepo(mt).UpdateStatus(context.Background(), "r-1", eywa.RitualRunning, time.Now()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRitualRepository_CountByMemoryKey(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.scheduled_tasks", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(4)}},
		))

		count, err := newRitualRepo(mt).CountByMemoryKey(context.Background(), "user:1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 4 {
			t.Errorf("expected 4, got %d", count)
		}
	})
}

func TestRitualRepository_FindPendingByMemoryKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "r-1"},
			{Key: "memory_key", Value: "user:1"},
			{Key: "status", Value: string(eywa.RitualPending)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.scheduled_tasks", mtest.FirstBatch, doc))

		tasks, err := newRitualRepo(mt).FindPendingByMemoryKey(context.Background(), "user:1", 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(tasks))
		}
	})
}

func TestRitualRepository_UpdateRunning_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		if err := newRitualRepo(mt).UpdateRunning(context.Background(), "r-1", time.Now()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRitualRepository_UpdateFailed_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		if err := newRitualRepo(mt).UpdateFailed(context.Background(), "r-1", time.Now(), "timeout"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRitualRepository_UpdateKeeperTaskID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		if err := newRitualRepo(mt).UpdateKeeperTaskID(context.Background(), "r-1", "keeper-99"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRitualRepository_FindPendingByMemoryKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newRitualRepo(mt).FindPendingByMemoryKey(context.Background(), "user:1", 10, 0); err == nil {
			mt.Error("expected error")
		}
	})
}

func TestRitualRepository_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newRitualRepo(mt).FindByID(context.Background(), "r-1"); err == nil {
			mt.Error("expected error")
		}
	})
}

func TestRitualRepository_UpdateStatus_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if err := newRitualRepo(mt).UpdateStatus(context.Background(), "r-1", eywa.RitualExecuted, time.Now()); err == nil {
			mt.Error("expected error")
		}
	})
}

func TestRitualRepository_CountByMemoryKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newRitualRepo(mt).CountByMemoryKey(context.Background(), "user:1"); err == nil {
			mt.Error("expected error")
		}
	})
}
