package mongo

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newCheckpointStore(mt *mtest.T) *CheckpointStore {
	return &CheckpointStore{
		collection: mt.DB.Collection("checkpoints"),
		ttl:        defaultCheckpointTTL,
		logger:     zap.NewNop().Sugar(),
	}
}

func TestCheckpointStore_Save_Upserts(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("upsert", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		err := newCheckpointStore(mt).Save(context.Background(), "turn-1", []byte(`{"iterations_used":2}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCheckpointStore_Save_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{Index: 0, Code: 11000, Message: "dup"}))

		err := newCheckpointStore(mt).Save(context.Background(), "turn-1", []byte("v1"))
		if err == nil {
			t.Error("expected error from a failed write")
		}
	})
}

func TestCheckpointStore_Load_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		payload := []byte(`{"iterations_used":3}`)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.checkpoints", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: "turn-1"},
			{Key: "data", Value: payload},
			{Key: "expires_at", Value: time.Now().Add(time.Hour)},
		}))

		got, found, err := newCheckpointStore(mt).Load(context.Background(), "turn-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found {
			t.Fatal("expected the checkpoint to be found")
		}
		if string(got) != string(payload) {
			t.Errorf("Load = %q, want %q", got, payload)
		}
	})
}

func TestCheckpointStore_Load_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.checkpoints", mtest.FirstBatch))

		got, found, err := newCheckpointStore(mt).Load(context.Background(), "missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found || got != nil {
			t.Errorf("expected (nil, false) for an absent checkpoint, got (%q, %v)", got, found)
		}
	})
}

func TestCheckpointStore_Load_DecodeError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("decode error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))

		_, _, err := newCheckpointStore(mt).Load(context.Background(), "turn-1")
		if err == nil {
			t.Error("expected error when the find command fails")
		}
	})
}

func TestCheckpointStore_Delete(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("delete", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		if err := newCheckpointStore(mt).Delete(context.Background(), "turn-1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCheckpointStore_Delete_AbsentIsNoError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("absent", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 0}})

		if err := newCheckpointStore(mt).Delete(context.Background(), "missing"); err != nil {
			t.Errorf("deleting an absent checkpoint must not error, got %v", err)
		}
	})
}

func TestCheckpointStore_Delete_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))

		if err := newCheckpointStore(mt).Delete(context.Background(), "turn-1"); err == nil {
			t.Error("expected error when the delete command fails")
		}
	})
}

func TestNewCheckpointStore_Constructs(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("constructs", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}}) // index creation
		if store := NewCheckpointStore(mt.DB); store == nil {
			t.Fatal("expected non-nil store")
		}
	})
}

func TestNewCheckpointStore_IndexError_LogsAndContinues(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("index error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))
		if store := NewCheckpointStore(mt.DB); store == nil {
			t.Fatal("constructor must not fail when index creation errors")
		}
	})
}

func TestNewCheckpointStoreWithTTL_NonPositiveFallsBack(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default ttl", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}})
		store := NewCheckpointStoreWithTTL(mt.DB, 0)
		if store.ttl != defaultCheckpointTTL {
			t.Errorf("expected default TTL %v, got %v", defaultCheckpointTTL, store.ttl)
		}
	})
}
