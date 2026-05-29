package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newImprintRepo(mt *mtest.T) *ImprintRepository {
	return &ImprintRepository{
		collection: mt.DB.Collection("imprints"),
		logger:     zap.NewNop().Sugar(),
	}
}

func TestImprintRepository_Save_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		imp := eywa.Imprint{UserKey: "user:1", SpiritName: "bot", Fact: "prefers short answers"}
		if err := newImprintRepo(mt).Save(context.Background(), imp); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestImprintRepository_GetByUserKey_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "imp-1"},
			{Key: "user_key", Value: "user:1"},
			{Key: "spirit_name", Value: "bot"},
			{Key: "content", Value: "prefers English"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch, doc))

		imprints, err := newImprintRepo(mt).GetByUserKey(context.Background(), "user:1", "bot")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(imprints) != 1 || imprints[0].UserKey != "user:1" {
			t.Errorf("unexpected imprints: %v", imprints)
		}
	})
}

func TestImprintRepository_Delete_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		if err := newImprintRepo(mt).Delete(context.Background(), "imp-1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestImprintRepository_Prune_UnderLimit_NoOp(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("under limit", func(mt *mtest.T) {
		// GetByUserKey returns 1 imprint, maxCount=5 → no deletes needed
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch,
			bson.D{{Key: "_id", Value: "imp-1"}, {Key: "user_key", Value: "u:1"}, {Key: "spirit_name", Value: "bot"}},
		))

		if err := newImprintRepo(mt).Prune(context.Background(), "u:1", "bot", 5); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestImprintRepository_Prune_OverLimit_Deletes(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("over limit", func(mt *mtest.T) {
		// GetByUserKey returns 3 imprints, maxCount=1 → 2 deletes
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch,
			bson.D{{Key: "_id", Value: "imp-1"}, {Key: "user_key", Value: "u:1"}},
			bson.D{{Key: "_id", Value: "imp-2"}, {Key: "user_key", Value: "u:1"}},
			bson.D{{Key: "_id", Value: "imp-3"}, {Key: "user_key", Value: "u:1"}},
		))
		// 2 delete responses
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		if err := newImprintRepo(mt).Prune(context.Background(), "u:1", "bot", 1); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestImprintRepository_List_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		// CountDocuments
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		// Find
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "imp-1"},
				{Key: "user_key", Value: "user:1"},
				{Key: "spirit_name", Value: "bot"},
			},
		))

		imprints, total, err := newImprintRepo(mt).List(context.Background(), eywa.ImprintListOptions{UserKey: "user:1"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(imprints) != 1 {
			t.Errorf("expected total=1 len=1, got total=%d len=%d", total, len(imprints))
		}
	})
}

func TestImprintRepository_Save_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newImprintRepo(mt).Save(context.Background(), eywa.Imprint{UserKey: "u:1", SpiritName: "bot", Fact: "x"}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestImprintRepository_GetByUserKey_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newImprintRepo(mt).GetByUserKey(context.Background(), "u:1", "bot"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestImprintRepository_Delete_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newImprintRepo(mt).Delete(context.Background(), "imp-1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestImprintRepository_List_CountError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, _, err := newImprintRepo(mt).List(context.Background(), eywa.ImprintListOptions{}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestImprintRepository_List_WithAllFilters(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("all filters", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(1)}},
		))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.imprints", mtest.FirstBatch,
			bson.D{{Key: "_id", Value: "imp-1"}, {Key: "user_key", Value: "u:1"}, {Key: "spirit_name", Value: "bot"}},
		))

		opts := eywa.ImprintListOptions{UserKey: "u:1", SpiritName: "bot", Category: "pref", Limit: 10, Offset: 0}
		imprints, total, err := newImprintRepo(mt).List(context.Background(), opts)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1 || len(imprints) != 1 {
			t.Errorf("expected total=1 len=1, got total=%d len=%d", total, len(imprints))
		}
	})
}
