package mongo

import (
	"context"
	"errors"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newRiteRepo(mt *mtest.T) *RiteRepository {
	return &RiteRepository{
		collection: mt.DB.Collection("rites"),
		logger:     zap.NewNop().Sugar(),
	}
}

func riteDoc(id string) bson.D {
	return bson.D{
		{Key: "_id", Value: id},
		{Key: "memory_key", Value: "user:1"},
		{Key: "status", Value: string(eywa.RitePending)},
	}
}

func TestRiteRepository_Create_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		rite := &eywa.Rite{MemoryKey: "user:1"}
		if err := newRiteRepo(mt).Create(context.Background(), rite); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if rite.ID == "" {
			t.Error("expected ID assigned")
		}
	})
}

func TestRiteRepository_FindByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch, riteDoc("rite-1")))

		rite, err := newRiteRepo(mt).FindByID(context.Background(), "rite-1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rite.ID != "rite-1" {
			t.Errorf("expected ID=rite-1, got %s", rite.ID)
		}
	})
}

func TestRiteRepository_FindByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch))

		_, err := newRiteRepo(mt).FindByID(context.Background(), "missing")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestRiteRepository_List_ReturnsPaginated(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch, riteDoc("rite-1")))

		rites, total, err := newRiteRepo(mt).List(context.Background(), eywa.RiteListOptions{Page: 1, Limit: 10})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(rites) != 1 {
			t.Errorf("expected total=1 len=1, got total=%d len=%d", total, len(rites))
		}
	})
}

func TestRiteRepository_Decide_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(1)},
			{Key: "nModified", Value: int32(1)},
		})

		if err := newRiteRepo(mt).Decide(context.Background(), "rite-1", "op-1", eywa.RiteApproved); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRiteRepository_Decide_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(0)},
			{Key: "nModified", Value: int32(0)},
		})

		err := newRiteRepo(mt).Decide(context.Background(), "ghost", "op-1", eywa.RiteApproved)
		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestRiteRepository_List_CountError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, _, err := newRiteRepo(mt).List(context.Background(), eywa.RiteListOptions{}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestRiteRepository_List_WithFilters(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with filters", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch, riteDoc("rite-1")))

		opts := eywa.RiteListOptions{MemoryKey: "user:1", Status: eywa.RitePending, Page: 0, Limit: 0}
		rites, total, err := newRiteRepo(mt).List(context.Background(), opts)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(rites) != 1 {
			t.Errorf("expected total=1 len=1, got total=%d len=%d", total, len(rites))
		}
	})
}

func TestRiteRepository_Decide_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newRiteRepo(mt).Decide(context.Background(), "rite-1", "op-1", eywa.RiteApproved); err == nil {
			t.Error("expected error")
		}
	})
}

func TestRiteRepository_List_FindError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("find error", func(mt *mtest.T) {
		// CountDocuments succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.rites", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		// Find fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "find fail"}))

		if _, _, err := newRiteRepo(mt).List(context.Background(), eywa.RiteListOptions{}); err == nil {
			mt.Error("expected error on Find failure")
		}
	})
}

func TestRiteRepository_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newRiteRepo(mt).FindByID(context.Background(), "rite-1"); err == nil {
			mt.Error("expected error")
		}
	})
}

func TestRiteRepository_Create_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if err := newRiteRepo(mt).Create(context.Background(), &eywa.Rite{MemoryKey: "u:1", EventKey: "e:1"}); err == nil {
			mt.Error("expected error")
		}
	})
}
