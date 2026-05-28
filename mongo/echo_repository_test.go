package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newEchoRepo(mt *mtest.T) *EchoRepository {
	return &EchoRepository{
		collection: mt.DB.Collection("messages"),
		logger:     zap.NewNop().Sugar(),
	}
}

func TestEchoRepository_Append_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		msg := eywa.Echo{MemoryKey: "user:1", Role: "user", Content: "hello", IsUserFacing: true}
		if err := newEchoRepo(mt).Append(context.Background(), msg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestEchoRepository_Append_NotUserFacing_Skips(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("skip", func(mt *mtest.T) {
		// No mock responses needed — should return nil without calling DB
		msg := eywa.Echo{MemoryKey: "user:1", Role: "assistant", Content: "thinking", IsUserFacing: false}
		if err := newEchoRepo(mt).Append(context.Background(), msg); err != nil {
			t.Errorf("expected nil for non-user-facing, got %v", err)
		}
	})
}

func TestEchoRepository_Append_EmptyMemoryKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty key", func(mt *mtest.T) {
		msg := eywa.Echo{MemoryKey: "", IsUserFacing: true}
		if err := newEchoRepo(mt).Append(context.Background(), msg); err == nil {
			t.Error("expected error for empty memory_key")
		}
	})
}

func TestEchoRepository_FindByMemoryKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "msg-1"},
			{Key: "memory_key", Value: "user:1"},
			{Key: "role", Value: "user"},
			{Key: "content", Value: "hi"},
			{Key: "is_user_facing", Value: true},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch, doc))

		msgs, err := newEchoRepo(mt).FindByMemoryKey(context.Background(), "user:1", 10, 0)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 1 || msgs[0].MemoryKey != "user:1" {
			t.Errorf("unexpected messages: %v", msgs)
		}
	})
}

func TestEchoRepository_FindByMemoryKey_EmptyKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty key", func(mt *mtest.T) {
		if _, err := newEchoRepo(mt).FindByMemoryKey(context.Background(), "", 10, 0); err == nil {
			t.Error("expected error for empty memory_key")
		}
	})
}

func TestEchoRepository_FindRecentByMemoryKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "memory_key", Value: "user:2"},
			{Key: "role", Value: "assistant"},
			{Key: "content", Value: "hello"},
			{Key: "is_user_facing", Value: true},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch, doc))

		msgs, err := newEchoRepo(mt).FindRecentByMemoryKey(context.Background(), "user:2", 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 1 {
			t.Errorf("expected 1 message, got %d", len(msgs))
		}
	})
}

func TestEchoRepository_FindByMemoryKey_ZeroLimit_UsesDefault(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("zero limit", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch))
		// limit=0 → defaults to defaultMessageLimit=50
		msgs, err := newEchoRepo(mt).FindByMemoryKey(context.Background(), "user:1", 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected empty, got %v", msgs)
		}
	})
}

func TestEchoRepository_FindByMemoryKey_OverMaxLimit_Clamps(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("over max limit", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch))
		// limit > maxMessageLimit=1000 → clamped
		msgs, err := newEchoRepo(mt).FindByMemoryKey(context.Background(), "user:1", 2000, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected empty, got %v", msgs)
		}
	})
}

func TestEchoRepository_FindByMemoryKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newEchoRepo(mt).FindByMemoryKey(context.Background(), "user:1", 10, 0); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_FindRecentByMemoryKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newEchoRepo(mt).FindRecentByMemoryKey(context.Background(), "user:1", 5); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_FindByMemoryKeyAndSubject_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "memory_key", Value: "user:1"},
			{Key: "subject_key", Value: "order:99"},
			{Key: "role", Value: "user"},
			{Key: "is_user_facing", Value: true},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch, doc))

		msgs, err := newEchoRepo(mt).FindByMemoryKeyAndSubject(context.Background(), "user:1", "order:99", 10, 0)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 1 || msgs[0].SubjectKey != "order:99" {
			t.Errorf("unexpected messages: %v", msgs)
		}
	})
}

func TestEchoRepository_FindRecentByMemoryKeyAndSubject_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "memory_key", Value: "user:1"},
			{Key: "subject_key", Value: "order:99"},
			{Key: "is_user_facing", Value: true},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch, doc))

		msgs, err := newEchoRepo(mt).FindRecentByMemoryKeyAndSubject(context.Background(), "user:1", "order:99", 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 1 {
			t.Errorf("expected 1 message, got %d", len(msgs))
		}
	})
}

func TestEchoRepository_FindBySubjectKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "subject_key", Value: "order:99"},
			{Key: "is_user_facing", Value: true},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch, doc))

		msgs, err := newEchoRepo(mt).FindBySubjectKey(context.Background(), "order:99", 10, 0)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 1 {
			t.Errorf("expected 1 message, got %d", len(msgs))
		}
	})
}

func TestEchoRepository_CountByMemoryKeyAndSubject(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(5)}},
		))

		count, err := newEchoRepo(mt).CountByMemoryKeyAndSubject(context.Background(), "user:1", "order:99")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 5 {
			t.Errorf("expected 5, got %d", count)
		}
	})
}

func TestEchoRepository_CountBySubjectKey(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(2)}},
		))

		count, err := newEchoRepo(mt).CountBySubjectKey(context.Background(), "order:99")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2, got %d", count)
		}
	})
}

func TestEchoRepository_CountByMemoryKey(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(3)}},
		))

		count, err := newEchoRepo(mt).CountByMemoryKey(context.Background(), "user:1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3, got %d", count)
		}
	})
}

func TestEchoRepository_FindByMemoryKey_NegativeOffset_Clamps(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("negative offset", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.messages", mtest.FirstBatch))
		msgs, err := newEchoRepo(mt).FindByMemoryKey(context.Background(), "user:1", 10, -5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected empty, got %v", msgs)
		}
	})
}

func TestEchoRepository_Append_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		msg := eywa.Echo{MemoryKey: "user:1", Role: "user", Content: "hi", IsUserFacing: true}
		if err := newEchoRepo(mt).Append(context.Background(), msg); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_FindByMemoryKeyAndSubject_EmptyKeys_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty keys", func(mt *mtest.T) {
		if _, err := newEchoRepo(mt).FindByMemoryKeyAndSubject(context.Background(), "", "order:1", 10, 0); err == nil {
			t.Error("expected error for empty memory_key")
		}
		if _, err := newEchoRepo(mt).FindByMemoryKeyAndSubject(context.Background(), "user:1", "", 10, 0); err == nil {
			t.Error("expected error for empty subject_key")
		}
	})
}

func TestEchoRepository_FindRecentByMemoryKeyAndSubject_EmptyKeys_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty keys", func(mt *mtest.T) {
		if _, err := newEchoRepo(mt).FindRecentByMemoryKeyAndSubject(context.Background(), "", "order:1", 5); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_FindBySubjectKey_EmptyKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty key", func(mt *mtest.T) {
		if _, err := newEchoRepo(mt).FindBySubjectKey(context.Background(), "", 10, 0); err == nil {
			t.Error("expected error")
		}
	})
}

func TestEchoRepository_findRecent_AggregateError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("aggregate error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "agg fail"}))

		if _, err := newEchoRepo(mt).FindRecentByMemoryKeyAndSubject(context.Background(), "user:1", "order:1", 5); err == nil {
			mt.Error("expected error")
		}
	})
}

func TestEchoRepository_FindBySubjectKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newEchoRepo(mt).FindBySubjectKey(context.Background(), "order:1", 10, 0); err == nil {
			t.Error("expected error")
		}
	})
}
