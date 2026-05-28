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

func TestChronicleRepository_FindByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		oid := primitive.NewObjectID()
		doc := bson.D{
			{Key: "_id", Value: oid},
			{Key: "memory_key", Value: "user:1"},
			{Key: "event_key", Value: "chat"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		ch, err := newChronicleRepo(mt).FindByID(context.Background(), oid.Hex())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch.MemoryKey != "user:1" {
			t.Errorf("expected memory_key=user:1, got %s", ch.MemoryKey)
		}
	})
}

func TestChronicleRepository_FindByID_InvalidID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("invalid id", func(mt *mtest.T) {
		_, err := newChronicleRepo(mt).FindByID(context.Background(), "not-an-oid")
		if err == nil {
			t.Error("expected error for invalid id")
		}
	})
}

func TestChronicleRepository_FindByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))

		oid := primitive.NewObjectID()
		_, err := newChronicleRepo(mt).FindByID(context.Background(), oid.Hex())

		if err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestChronicleRepository_List_ReturnsPaginated(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: primitive.NewObjectID()},
			{Key: "memory_key", Value: "user:1"},
		}
		// CountDocuments
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		// Find
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		chronicles, total, err := newChronicleRepo(mt).List(context.Background(), eywa.ChronicleListOptions{Page: 1, Limit: 10})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(chronicles) != 1 {
			t.Errorf("expected total=1 len=1, got total=%d len=%d", total, len(chronicles))
		}
	})
}

func TestChronicleRepository_List_WithFilters(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with filters", func(mt *mtest.T) {
		now := time.Now()
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(0)}},
		))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))

		_, total, err := newChronicleRepo(mt).List(context.Background(), eywa.ChronicleListOptions{
			SpiritName:    "bot",
			MemoryKey:     "user:1",
			HasError:      true,
			MinIterations: 2,
			DateFrom:      &now,
			DateTo:        &now,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 {
			t.Errorf("expected 0, got %d", total)
		}
	})
}

func TestChronicleRepository_AggregateTokens_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		now := time.Now()
		doc := bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "date", Value: now},
				{Key: "spirit", Value: "bot"},
			}},
			{Key: "prompt_tokens", Value: int32(100)},
			{Key: "completion_tokens", Value: int32(50)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		results, err := newChronicleRepo(mt).AggregateTokens(context.Background(), "bot", now.Add(-time.Hour), now, "day")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].SpiritName != "bot" {
			t.Errorf("unexpected results: %v", results)
		}
	})
}

func TestChronicleRepository_AggregateTokens_WeekGranularity(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("week", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))

		now := time.Now()
		results, err := newChronicleRepo(mt).AggregateTokens(context.Background(), "", now.Add(-time.Hour), now, "week")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected empty, got %v", results)
		}
	})
}

func TestChronicleRepository_AggregateActions_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "action_name", Value: "search_lore"},
			{Key: "call_count", Value: int32(10)},
			{Key: "error_count", Value: int32(1)},
			{Key: "avg_latency_ms", Value: float64(45.0)},
			{Key: "p95_latency_ms", Value: float64(120.0)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		now := time.Now()
		results, err := newChronicleRepo(mt).AggregateActions(context.Background(), "bot", now.Add(-time.Hour), now)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].ActionName != "search_lore" {
			t.Errorf("unexpected results: %v", results)
		}
	})
}

func TestChronicleRepository_AggregateSpirits_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch,
			bson.D{
				{Key: "spirit_name", Value: "bot"},
				{Key: "avg_iterations", Value: float64(2.5)},
				{Key: "error_rate", Value: float64(0.05)},
				{Key: "avg_duration_ms", Value: float64(300.0)},
			},
		))

		now := time.Now()
		results, err := newChronicleRepo(mt).AggregateSpirits(context.Background(), now.Add(-time.Hour), now)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].SpiritName != "bot" {
			t.Errorf("unexpected results: %v", results)
		}
	})
}

func TestChronicleRepository_List_CountError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, _, err := newChronicleRepo(mt).List(context.Background(), eywa.ChronicleListOptions{}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_List_OverLimitClamped(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("over limit", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(0)}},
		))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))

		// limit > 100 → clamped to 100
		_, total, err := newChronicleRepo(mt).List(context.Background(), eywa.ChronicleListOptions{Limit: 200})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 {
			t.Errorf("expected total=0, got %d", total)
		}
	})
}

func TestChronicleRepository_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		oid := primitive.NewObjectID()
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newChronicleRepo(mt).FindByID(context.Background(), oid.Hex()); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_AggregateTokens_MonthGranularity(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("month", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))

		now := time.Now()
		_, err := newChronicleRepo(mt).AggregateTokens(context.Background(), "", now.Add(-24*time.Hour), now, "month")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestChronicleRepository_AggregateTokens_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		now := time.Now()
		if _, err := newChronicleRepo(mt).AggregateTokens(context.Background(), "bot", now.Add(-time.Hour), now, "day"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_AggregateActions_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		now := time.Now()
		if _, err := newChronicleRepo(mt).AggregateActions(context.Background(), "", now.Add(-time.Hour), now); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_AggregateSpirits_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		now := time.Now()
		if _, err := newChronicleRepo(mt).AggregateSpirits(context.Background(), now.Add(-time.Hour), now); err == nil {
			t.Error("expected error")
		}
	})
}

func TestChronicleRepository_List_FindError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("find error", func(mt *mtest.T) {
		// CountDocuments succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		// Find fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "find fail"}))

		if _, _, err := newChronicleRepo(mt).List(context.Background(), eywa.ChronicleListOptions{}); err == nil {
			mt.Error("expected error on Find failure")
		}
	})
}

func TestChronicleRepository_AggregateTokens_NoSpiritName(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("no spirit name", func(mt *mtest.T) {
		now := time.Now()
		doc := bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "date", Value: now},
				{Key: "spirit", Value: "bot"},
			}},
			{Key: "prompt_tokens", Value: int32(50)},
			{Key: "completion_tokens", Value: int32(25)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch, doc))

		results, err := newChronicleRepo(mt).AggregateTokens(context.Background(), "", now.Add(-time.Hour), now, "day")
		if err != nil {
			mt.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			mt.Errorf("expected 1 result, got %d", len(results))
		}
	})
}

func TestChronicleRepository_AggregateActions_NoSpiritName(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("no spirit name", func(mt *mtest.T) {
		now := time.Now()
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.interaction_logs", mtest.FirstBatch))

		results, err := newChronicleRepo(mt).AggregateActions(context.Background(), "", now.Add(-time.Hour), now)
		if err != nil {
			mt.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			mt.Errorf("expected empty, got %v", results)
		}
	})
}
