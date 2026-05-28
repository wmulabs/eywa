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

func newLoreRepo(mt *mtest.T) *LoreRepository {
	return &LoreRepository{
		collection: mt.DB.Collection("lores"),
		logger:     zap.NewNop().Sugar(),
	}
}

func loreDoc(id, name string) bson.D {
	return bson.D{
		{Key: "_id", Value: id},
		{Key: "name", Value: name},
		{Key: "description", Value: "test lore"},
	}
}

func TestLoreRepository_Create_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		if err := newLoreRepo(mt).Create(context.Background(), eywa.Lore{ID: "lore-1", Name: "kb-faq"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoreRepository_Create_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 11000, Message: "duplicate"}))

		err := newLoreRepo(mt).Create(context.Background(), eywa.Lore{ID: "lore-1", Name: "kb-faq"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreRepository_GetByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lores", mtest.FirstBatch, loreDoc("lore-1", "kb-faq")))

		lore, err := newLoreRepo(mt).GetByID(context.Background(), "lore-1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lore.ID != "lore-1" || lore.Name != "kb-faq" {
			t.Errorf("unexpected lore: %+v", lore)
		}
	})
}

func TestLoreRepository_GetByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lores", mtest.FirstBatch))

		_, err := newLoreRepo(mt).GetByID(context.Background(), "missing")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestLoreRepository_GetByName_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lores", mtest.FirstBatch, loreDoc("lore-1", "kb-faq")))

		lore, err := newLoreRepo(mt).GetByName(context.Background(), "kb-faq")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lore.Name != "kb-faq" {
			t.Errorf("expected name=kb-faq, got %s", lore.Name)
		}
	})
}

func TestLoreRepository_GetBySpiritID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lores", mtest.FirstBatch, loreDoc("lore-1", "kb")))

		lores, err := newLoreRepo(mt).GetBySpiritID(context.Background(), "spirit-1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lores) != 1 {
			t.Errorf("expected 1 lore, got %d", len(lores))
		}
	})
}

func TestLoreRepository_GetByIDs_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lores", mtest.FirstBatch,
			loreDoc("lore-1", "kb-a"),
			loreDoc("lore-2", "kb-b"),
		))

		lores, err := newLoreRepo(mt).GetByIDs(context.Background(), []string{"lore-1", "lore-2"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lores) != 2 {
			t.Errorf("expected 2 lores, got %d", len(lores))
		}
	})
}

func TestLoreRepository_Update_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		if err := newLoreRepo(mt).Update(context.Background(), eywa.Lore{ID: "lore-1", Name: "updated"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoreRepository_Delete_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		if err := newLoreRepo(mt).Delete(context.Background(), "lore-1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoreRepository_GetByName_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.lores", mtest.FirstBatch))

		_, err := newLoreRepo(mt).GetByName(context.Background(), "missing")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestLoreRepository_GetByName_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newLoreRepo(mt).GetByName(context.Background(), "kb-faq"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreRepository_GetByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newLoreRepo(mt).GetByID(context.Background(), "lore-1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreRepository_GetBySpiritID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newLoreRepo(mt).GetBySpiritID(context.Background(), "spirit-1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreRepository_GetByIDs_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newLoreRepo(mt).GetByIDs(context.Background(), []string{"lore-1"}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreRepository_Update_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newLoreRepo(mt).Update(context.Background(), eywa.Lore{ID: "lore-1"}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLoreRepository_Delete_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newLoreRepo(mt).Delete(context.Background(), "lore-1"); err == nil {
			t.Error("expected error")
		}
	})
}
