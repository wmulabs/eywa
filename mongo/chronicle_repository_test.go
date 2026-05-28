package mongo

import (
	"context"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newChronicleRepo(mt *mtest.T) *ChronicleRepository {
	return &ChronicleRepository{
		collection: mt.DB.Collection("interaction_logs"),
		logger:     zap.NewNop().Sugar(),
	}
}

func TestChronicleRepository_LogInteraction_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		log := &eywa.Chronicle{MemoryKey: "user:1", EventKey: "chat"}
		if err := newChronicleRepo(mt).LogInteraction(context.Background(), log); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if log.ID == "" {
			t.Error("expected ID set after log")
		}
	})
}

func TestChronicleRepository_LogInteraction_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))

		if err := newChronicleRepo(mt).LogInteraction(context.Background(), &eywa.Chronicle{}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_FindByMemoryKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "log-1"},
			{Key: "memory_key", Value: "user:1"},
			{Key: "event_key", Value: "chat"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		logs, err := newChronicleRepo(mt).FindByMemoryKey(context.Background(), "user:1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 || logs[0].MemoryKey != "user:1" {
			t.Errorf("unexpected logs: %v", logs)
		}
	})
}

func TestChronicleRepository_FindByEventKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "log-2"},
			{Key: "memory_key", Value: "user:2"},
			{Key: "event_key", Value: "order"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		logs, err := newChronicleRepo(mt).FindByEventKey(context.Background(), "order")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 || logs[0].EventKey != "order" {
			t.Errorf("unexpected logs: %v", logs)
		}
	})
}

func TestChronicleRepository_FindByTimeRange_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "log-3"},
			{Key: "memory_key", Value: "user:3"},
			{Key: "event_key", Value: "chat"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		start := time.Now().Add(-time.Hour)
		end := time.Now()
		logs, err := newChronicleRepo(mt).FindByTimeRange(context.Background(), start, end)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 1 {
			t.Errorf("expected 1 log, got %d", len(logs))
		}
	})
}

func TestChronicleRepository_FindByMemoryKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newChronicleRepo(mt).FindByMemoryKey(context.Background(), "user:1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_FindByEventKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newChronicleRepo(mt).FindByEventKey(context.Background(), "chat"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_FindByTimeRange_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		_, err := newChronicleRepo(mt).FindByTimeRange(context.Background(), time.Now().Add(-time.Hour), time.Now())
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_FindByMemoryKey_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))
		logs, err := newChronicleRepo(mt).FindByMemoryKey(context.Background(), "user:nobody")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 0 {
			t.Errorf("expected 0, got %d", len(logs))
		}
	})
}

func TestChronicleRepository_FindByEventKey_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))
		logs, err := newChronicleRepo(mt).FindByEventKey(context.Background(), "unknown")
		if err != nil {
			mt.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 0 {
			mt.Errorf("expected 0, got %d", len(logs))
		}
	})
}

func TestChronicleRepository_FindByTimeRange_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))
		logs, err := newChronicleRepo(mt).FindByTimeRange(context.Background(), time.Now().Add(-time.Hour), time.Now())
		if err != nil {
			mt.Fatalf("unexpected error: %v", err)
		}
		if len(logs) != 0 {
			mt.Errorf("expected 0, got %d", len(logs))
		}
	})
}
