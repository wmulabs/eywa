package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newLoreStore(mt *mtest.T) *LoreStore {
	return &LoreStore{
		collection: mt.DB.Collection("lore_chunks"),
		logger:     zap.NewNop().Sugar(),
	}
}

func TestLoreStore_Upsert_Empty_Skips(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty", func(mt *mtest.T) {
		// No mock responses needed — should return nil without DB call
		if err := newLoreStore(mt).Upsert(context.Background(), nil); err != nil {
			t.Errorf("expected nil for empty chunks, got %v", err)
		}
	})
}

func TestLoreStore_Upsert_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nInserted", Value: 0},
			{Key: "nMatched", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "nRemoved", Value: 0},
			{Key: "nUpserted", Value: 0},
		})

		chunks := []eywa.LoreChunk{
			{ID: "chunk-1", LoreID: "lore-1", Content: "fact about X"},
		}
		if err := newLoreStore(mt).Upsert(context.Background(), chunks); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoreStore_Search_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "chunk-1"},
			{Key: "lore_id", Value: "lore-1"},
			{Key: "content", Value: "fact about X"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lore_chunks", mtest.FirstBatch, doc))

		chunks, err := newLoreStore(mt).Search(context.Background(), "lore-1", []float32{0.1, 0.2}, 5, 0)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chunks) != 1 || chunks[0].LoreID != "lore-1" {
			t.Errorf("unexpected chunks: %v", chunks)
		}
	})
}

func TestLoreStore_Search_WithMinScore(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with min score", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lore_chunks", mtest.FirstBatch))

		chunks, err := newLoreStore(mt).Search(context.Background(), "lore-1", []float32{0.5}, 3, 0.8)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(chunks) != 0 {
			t.Errorf("expected empty, got %d", len(chunks))
		}
	})
}

func TestLoreStore_Search_DefaultTopK(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default topK", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lore_chunks", mtest.FirstBatch))

		// topK=0 → defaults to 5
		if _, err := newLoreStore(mt).Search(context.Background(), "lore-1", []float32{}, 0, 0); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoreStore_Delete_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 3}})

		if err := newLoreStore(mt).Delete(context.Background(), "lore-1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoreStore_Upsert_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		chunks := []eywa.LoreChunk{{ID: "c-1", LoreID: "lore-1", Content: "fact"}}
		if err := newLoreStore(mt).Upsert(context.Background(), chunks); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreStore_Search_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newLoreStore(mt).Search(context.Background(), "lore-1", []float32{0.1}, 5, 0); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreStore_Delete_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newLoreStore(mt).Delete(context.Background(), "lore-1"); err == nil {
			t.Error("expected error")
		}
	})
}
