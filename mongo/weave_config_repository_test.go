package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestWeaveConfigRepository_Find_ReturnsConfig(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "default"},
			{Key: "max_reasoning_iterations", Value: int32(10)},
			{Key: "lock_ttl_ns", Value: int64(5000000000)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.engine_config", mtest.FirstBatch, doc))
		repo := NewWeaveConfigRepository(mt.DB)

		cfg, err := repo.Find(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.MaxReasoningIterations != 10 {
			t.Errorf("expected MaxReasoningIterations=10, got %d", cfg.MaxReasoningIterations)
		}
	})
}

func TestWeaveConfigRepository_Find_NoDocument_ReturnsDefault(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.engine_config", mtest.FirstBatch))
		repo := NewWeaveConfigRepository(mt.DB)

		cfg, err := repo.Find(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected default config, got nil")
		}
	})
}

func TestWeaveConfigRepository_Find_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "bad"}))
		repo := NewWeaveConfigRepository(mt.DB)

		_, err := repo.Find(context.Background())

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestWeaveConfigRepository_Save_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("save", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})
		repo := NewWeaveConfigRepository(mt.DB)

		if err := repo.Save(context.Background(), eywa.DefaultWeaveConfig()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
