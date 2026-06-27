package mongo

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newHandoffStore(mt *mtest.T) *HandoffStore {
	return &HandoffStore{
		col:    mt.DB.Collection("handoffs"),
		logger: zap.NewNop().Sugar(),
	}
}

func TestNewHandoffStore_Constructor(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ctor", func(mt *mtest.T) {
		// Index creation failure is swallowed (logged) — exercises the ensureIndexes error branch.
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))
		if NewHandoffStore(mt.DB) == nil {
			t.Fatal("expected non-nil store")
		}
	})
}

func TestHandoffStore_GetActiveSpirit_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("get error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))
		if _, err := newHandoffStore(mt).GetActiveSpirit(context.Background(), "user:1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHandoffStore_ClearActiveSpirit_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("clear error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))
		if err := newHandoffStore(mt).ClearActiveSpirit(context.Background(), "user:1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHandoffStore_GetActiveSpirit_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{{Key: "session_key", Value: "user:1"}, {Key: "spirit_name", Value: "billing"}}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.handoffs", mtest.FirstBatch, doc))

		got, err := newHandoffStore(mt).GetActiveSpirit(context.Background(), "user:1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "billing" {
			t.Errorf("expected billing, got %q", got)
		}
	})
}

func TestHandoffStore_GetActiveSpirit_NotFound_ReturnsEmpty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.handoffs", mtest.FirstBatch))

		got, err := newHandoffStore(mt).GetActiveSpirit(context.Background(), "user:1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestHandoffStore_SetActiveSpirit_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("set", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		if err := newHandoffStore(mt).SetActiveSpirit(context.Background(), "user:1", "billing"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestHandoffStore_SetActiveSpirit_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))

		if err := newHandoffStore(mt).SetActiveSpirit(context.Background(), "user:1", "billing"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHandoffStore_ClearActiveSpirit_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("clear", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		if err := newHandoffStore(mt).ClearActiveSpirit(context.Background(), "user:1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
