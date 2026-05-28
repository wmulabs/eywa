package mongo

import (
	"context"
	"testing"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestEchoRepository_ListSessions_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		// countPipeline cursor
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{{Key: "total", Value: int64(2)}},
		))
		// main pipeline cursor
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "user:1"},
				{Key: "message_count", Value: int64(5)},
				{Key: "last_message_at", Value: time.Now()},
			},
		))

		sessions, total, err := newEchoRepo(mt).ListSessions(context.Background(), eywa.SessionListOptions{Page: 1, Limit: 20})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 2 || len(sessions) != 1 {
			t.Errorf("expected total=2 len=1, got total=%d len=%d", total, len(sessions))
		}
		if sessions[0].MemoryKey != "user:1" || sessions[0].MessageCount != 5 {
			t.Errorf("unexpected session: %+v", sessions[0])
		}
	})
}

func TestEchoRepository_ListSessions_WithFilters(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with filters", func(mt *mtest.T) {
		now := time.Now()
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch)) // count = 0
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch)) // empty results

		_, total, err := newEchoRepo(mt).ListSessions(context.Background(), eywa.SessionListOptions{
			MemoryKey: "user:1",
			DateFrom:  &now,
			DateTo:    &now,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 {
			t.Errorf("expected 0, got %d", total)
		}
	})
}

func TestEchoRepository_FindByMemoryKeyBefore_NoBeforeID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("no before id", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: primitive.NewObjectID()},
			{Key: "memory_key", Value: "user:1"},
			{Key: "is_user_facing", Value: true},
			{Key: "role", Value: "user"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch, doc))

		msgs, err := newEchoRepo(mt).FindByMemoryKeyBefore(context.Background(), "user:1", "", 10)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 1 {
			t.Errorf("expected 1 message, got %d", len(msgs))
		}
	})
}

func TestEchoRepository_FindByMemoryKeyBefore_WithBeforeID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with before id", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch))

		beforeID := primitive.NewObjectID().Hex()
		msgs, err := newEchoRepo(mt).FindByMemoryKeyBefore(context.Background(), "user:1", beforeID, 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected empty, got %d", len(msgs))
		}
	})
}

func TestEchoRepository_FindByMemoryKeyBefore_EmptyKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty key", func(mt *mtest.T) {
		if _, err := newEchoRepo(mt).FindByMemoryKeyBefore(context.Background(), "", "", 10); err == nil {
			t.Error("expected error for empty memoryKey")
		}
	})
}

func TestEchoRepository_FindByMemoryKeyBefore_InvalidBeforeID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("invalid before id", func(mt *mtest.T) {
		if _, err := newEchoRepo(mt).FindByMemoryKeyBefore(context.Background(), "user:1", "not-valid-oid", 10); err == nil {
			t.Error("expected error for invalid beforeID")
		}
	})
}

func TestEchoRepository_ListSessions_CountError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, _, err := newEchoRepo(mt).ListSessions(context.Background(), eywa.SessionListOptions{}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_ListSessions_MainPipelineError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("pipeline error", func(mt *mtest.T) {
		// Count succeeds with 0
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch))
		// Main pipeline fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, _, err := newEchoRepo(mt).ListSessions(context.Background(), eywa.SessionListOptions{}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_ListSessions_WithDateFiltersAndOverLimit(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with date filters", func(mt *mtest.T) {
		now := time.Now()
		from := now.Add(-24 * time.Hour)
		to := now

		// Count pipeline
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{{Key: "total", Value: int64(1)}},
		))
		// Main pipeline
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{{Key: "_id", Value: "user:1"}, {Key: "message_count", Value: int64(3)}, {Key: "last_message_at", Value: now}},
		))

		// Limit > 100 → clamped to 100; DateFrom and DateTo set
		opts := eywa.SessionListOptions{MemoryKey: "user:1", DateFrom: &from, DateTo: &to, Limit: 200}
		sessions, total, err := newEchoRepo(mt).ListSessions(context.Background(), opts)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(sessions) != 1 {
			t.Errorf("expected total=1 len=1, got total=%d len=%d", total, len(sessions))
		}
	})
}

func TestEchoRepository_FindByMemoryKeyBefore_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newEchoRepo(mt).FindByMemoryKeyBefore(context.Background(), "user:1", "", 10); err == nil {
			t.Error("expected error")
		}
	})
}
