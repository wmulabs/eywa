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

func newAppTokenRepo(mt *mtest.T) *AppTokenRepository {
	return &AppTokenRepository{
		collection: mt.DB.Collection("app_tokens"),
		logger:     zap.NewNop().Sugar(),
	}
}

func TestAppTokenRepository_Create(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("create", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}})
		err := newAppTokenRepo(mt).Create(context.Background(), &eywa.AppToken{ID: "t1", Name: "ci", Hash: []byte("h"), CreatedAt: time.Now()})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestAppTokenRepository_FindByHash_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		hash := []byte("the-hash")
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.app_tokens", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: "t1"},
			{Key: "name", Value: "ci"},
			{Key: "hash", Value: hash},
		}))
		tok, err := newAppTokenRepo(mt).FindByHash(context.Background(), hash)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.ID != "t1" {
			t.Errorf("ID = %q, want t1", tok.ID)
		}
	})
}

func TestAppTokenRepository_FindByHash_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.app_tokens", mtest.FirstBatch))
		_, err := newAppTokenRepo(mt).FindByHash(context.Background(), []byte("missing"))
		if err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestAppTokenRepository_List(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.app_tokens", mtest.FirstBatch,
			bson.D{{Key: "_id", Value: "t1"}, {Key: "name", Value: "a"}},
			bson.D{{Key: "_id", Value: "t2"}, {Key: "name", Value: "b"}},
		))
		tokens, err := newAppTokenRepo(mt).List(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tokens) != 2 {
			t.Errorf("len = %d, want 2", len(tokens))
		}
	})
}

func TestAppTokenRepository_Revoke(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})
		if err := newAppTokenRepo(mt).Revoke(context.Background(), "t1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestAppTokenRepository_Revoke_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 0}})
		if err := newAppTokenRepo(mt).Revoke(context.Background(), "missing"); err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestAppTokenRepository_Create_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{Index: 0, Code: 11000, Message: "dup"}))
		err := newAppTokenRepo(mt).Create(context.Background(), &eywa.AppToken{ID: "t1", Hash: []byte("h")})
		if err == nil {
			t.Error("expected error from a failed insert")
		}
	})
}

func TestAppTokenRepository_FindByHash_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))
		_, err := newAppTokenRepo(mt).FindByHash(context.Background(), []byte("h"))
		if err == nil || err == eywa.ErrNotFound {
			t.Errorf("expected a non-not-found error, got %v", err)
		}
	})
}

func TestAppTokenRepository_List_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))
		if _, err := newAppTokenRepo(mt).List(context.Background()); err == nil {
			t.Error("expected error when the find command fails")
		}
	})
}

func TestAppTokenRepository_Revoke_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))
		if err := newAppTokenRepo(mt).Revoke(context.Background(), "t1"); err == nil {
			t.Error("expected error when the update command fails")
		}
	})
}

func TestNewAppTokenRepository_IndexError_LogsAndContinues(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("index error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 1, Message: "boom"}))
		if repo := NewAppTokenRepository(mt.DB); repo == nil {
			t.Fatal("constructor must not fail when index creation errors")
		}
	})
}

func TestNewAppTokenRepository_Constructs(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("constructs", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}})
		if repo := NewAppTokenRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil repo")
		}
	})
}
