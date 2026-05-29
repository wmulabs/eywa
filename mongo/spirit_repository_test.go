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

func newSpiritRepo(mt *mtest.T) *SpiritRepository {
	return &SpiritRepository{
		collection: mt.DB.Collection("spirits"),
		logger:     zap.NewNop().Sugar(),
	}
}

func newSpiritRepoWithDB(mt *mtest.T) *SpiritRepository {
	return &SpiritRepository{
		collection: mt.DB.Collection("spirits"),
		db:         mt.DB,
		logger:     zap.NewNop().Sugar(),
	}
}

func spiritDoc(name string, version int, isActive bool) bson.D {
	return bson.D{
		{Key: "_id", Value: name + "-v1"},
		{Key: "name", Value: name},
		{Key: "version", Value: int32(version)},
		{Key: "is_active", Value: isActive},
		{Key: "is_deleted", Value: false},
		{Key: "allowed_actions", Value: bson.A{}},
		{Key: "model_config", Value: bson.D{}},
	}
}

func TestSpiritRepository_FindActiveByName_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)))

		spirit, err := newSpiritRepo(mt).FindActiveByName(context.Background(), "bot")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spirit.Name != "bot" || spirit.Version != 1 {
			t.Errorf("unexpected spirit: %+v", spirit)
		}
	})
}

func TestSpiritRepository_FindActiveByName_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch))

		_, err := newSpiritRepo(mt).FindActiveByName(context.Background(), "ghost")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestSpiritRepository_FindActiveByName_EmptyName_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty name", func(mt *mtest.T) {
		_, err := newSpiritRepo(mt).FindActiveByName(context.Background(), "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestSpiritRepository_FindActiveByNames_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch,
			spiritDoc("bot-a", 1, true),
			spiritDoc("bot-b", 2, true),
		))

		result, err := newSpiritRepo(mt).FindActiveByNames(context.Background(), []string{"bot-a", "bot-b"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 spirits, got %d", len(result))
		}
	})
}

func TestSpiritRepository_FindActiveByNames_Empty_ReturnsEmptyMap(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty", func(mt *mtest.T) {
		result, err := newSpiritRepo(mt).FindActiveByNames(context.Background(), nil)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d", len(result))
		}
	})
}

func TestSpiritRepository_Create_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		// FindActiveByName returns not found → proceed with insert
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch), // not found
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}},           // InsertOne
		)

		spirit := &eywa.Spirit{Name: "new-bot"}
		if err := newSpiritRepo(mt).Create(context.Background(), spirit); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSpiritRepository_Create_AlreadyExists_ReturnsConflictError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("conflict", func(mt *mtest.T) {
		// FindActiveByName finds existing spirit
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)))

		spirit := &eywa.Spirit{Name: "bot"}
		err := newSpiritRepo(mt).Create(context.Background(), spirit)

		var conflict *eywa.ConflictError
		if !errors.As(err, &conflict) {
			t.Errorf("expected ConflictError, got %v", err)
		}
	})
}

func TestSpiritRepository_Create_EmptyName_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty name", func(mt *mtest.T) {
		err := newSpiritRepo(mt).Create(context.Background(), &eywa.Spirit{})
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestSpiritRepository_Update_InvalidInput_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("invalid", func(mt *mtest.T) {
		if _, err := newSpiritRepo(mt).Update(context.Background(), "", nil, ""); err == nil {
			t.Error("expected error for empty name/nil spirit")
		}
	})
}

func TestSpiritRepository_Update_SpiritNotFound_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		// FindActiveByName → not found
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch))

		_, err := newSpiritRepo(mt).Update(context.Background(), "ghost", &eywa.Spirit{}, "")
		if err == nil {
			t.Error("expected error when spirit not found")
		}
	})
}

func TestSpiritRepository_Update_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(
			// FindActiveByName (current version)
			mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)),
			// UpdateOne (deactivate)
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}},
			// InsertOne (new version)
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}},
		)

		updated, err := newSpiritRepo(mt).Update(context.Background(), "bot", &eywa.Spirit{Description: "updated"}, "v2")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Version != 2 {
			t.Errorf("expected version=2, got %d", updated.Version)
		}
	})
}

func TestSpiritRepository_ListActive_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch,
			spiritDoc("a", 1, true),
			spiritDoc("b", 1, true),
		))

		spirits, err := newSpiritRepo(mt).ListActive(context.Background(), 10, 0)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(spirits) != 2 {
			t.Errorf("expected 2, got %d", len(spirits))
		}
	})
}

func TestSpiritRepository_CountActive(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(7)}},
		))

		count, err := newSpiritRepo(mt).CountActive(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 7 {
			t.Errorf("expected 7, got %d", count)
		}
	})
}

func TestSpiritRepository_Activate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("activate", func(mt *mtest.T) {
		mt.AddMockResponses(
			// UpdateMany (deactivate all)
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}},
			// UpdateOne (activate version)
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}},
			// commitTransaction
			bson.D{{Key: "ok", Value: 1}},
		)

		if err := newSpiritRepoWithDB(mt).Activate(context.Background(), "bot", 2); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSpiritRepository_Activate_InvalidInput_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("invalid", func(mt *mtest.T) {
		if err := newSpiritRepo(mt).Activate(context.Background(), "", 0); err == nil {
			t.Error("expected error for empty name/version")
		}
	})
}

func TestSpiritRepository_GetVersion_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 3, false)))

		spirit, err := newSpiritRepo(mt).GetVersion(context.Background(), "bot", 3)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spirit.Version != 3 {
			t.Errorf("expected version=3, got %d", spirit.Version)
		}
	})
}

func TestSpiritRepository_GetVersion_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch))

		_, err := newSpiritRepo(mt).GetVersion(context.Background(), "bot", 99)

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestSpiritRepository_FindByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)))

		spirit, err := newSpiritRepo(mt).FindByID(context.Background(), "bot-v1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if spirit.Name != "bot" {
			t.Errorf("expected name=bot, got %s", spirit.Name)
		}
	})
}

func TestSpiritRepository_FindByID_EmptyID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty id", func(mt *mtest.T) {
		if _, err := newSpiritRepo(mt).FindByID(context.Background(), ""); err == nil {
			t.Error("expected error for empty id")
		}
	})
}

func TestSpiritRepository_ListAll_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list all", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch,
			spiritDoc("bot", 1, true),
			spiritDoc("bot", 2, false),
		))

		spirits, err := newSpiritRepo(mt).ListAll(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(spirits) != 2 {
			t.Errorf("expected 2, got %d", len(spirits))
		}
	})
}

func TestSpiritRepository_Deactivate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		if err := newSpiritRepo(mt).Deactivate(context.Background(), "bot-v1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSpiritRepository_Deactivate_EmptyID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty id", func(mt *mtest.T) {
		if err := newSpiritRepo(mt).Deactivate(context.Background(), ""); err == nil {
			t.Error("expected error for empty id")
		}
	})
}

func TestSpiritRepository_FindVersionHistory_EmptyName_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty name", func(mt *mtest.T) {
		if _, err := newSpiritRepo(mt).FindVersionHistory(context.Background(), ""); err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestSpiritRepository_FindVersionHistory_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).FindVersionHistory(context.Background(), "bot"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_Activate_VersionNotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("version not found", func(mt *mtest.T) {
		mt.AddMockResponses(
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}}, // UpdateMany OK
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(0)}, {Key: "nModified", Value: int32(0)}}, // UpdateOne MatchedCount=0
			// transaction aborts on NotFoundError — no commitTransaction response needed
		)

		err := newSpiritRepoWithDB(mt).Activate(context.Background(), "bot", 99)
		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestSpiritRepository_ListAll_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).ListAll(context.Background()); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_CountActive_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).CountActive(context.Background()); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_ListActive_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).ListActive(context.Background(), 10, 0); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_FindActiveByNames_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).FindActiveByNames(context.Background(), []string{"bot"}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_FindVersionHistory_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch,
			spiritDoc("bot", 2, true),
			spiritDoc("bot", 1, false),
		))

		history, err := newSpiritRepo(mt).FindVersionHistory(context.Background(), "bot")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(history) != 2 {
			t.Errorf("expected 2 versions, got %d", len(history))
		}
	})
}

func TestSpiritRepository_RestoreVersion_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		// FindByID
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 2, false)))
		// UpdateMany (deactivate all versions)
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(2)}, {Key: "nModified", Value: int32(2)}})
		// UpdateOne (restore requested version)
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		// commitTransaction
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}})

		if err := newSpiritRepoWithDB(mt).RestoreVersion(context.Background(), "bot-v1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSpiritRepository_RestoreVersion_EmptyID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty id", func(mt *mtest.T) {
		if err := newSpiritRepo(mt).RestoreVersion(context.Background(), ""); err == nil {
			t.Error("expected error for empty id")
		}
	})
}

func TestSpiritRepository_RestoreVersion_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("findbyid error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newSpiritRepo(mt).RestoreVersion(context.Background(), "bot-v1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_RestoreVersion_UsesTransaction(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("uses transaction", func(mt *mtest.T) {
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 2, false)),       // FindByID
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(2)}, {Key: "nModified", Value: int32(2)}}, // UpdateMany
			bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}}, // UpdateOne
			bson.D{{Key: "ok", Value: 1}}, // commitTransaction
		)

		if err := newSpiritRepoWithDB(mt).RestoreVersion(context.Background(), "bot-v1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSpiritRepository_RestoreVersion_UpdateMany_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("updatemany error", func(mt *mtest.T) {
		// FindByID succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 2, true)))
		// UpdateMany fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newSpiritRepoWithDB(mt).RestoreVersion(context.Background(), "bot-v1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_RestoreVersion_UpdateOne_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("updateone error", func(mt *mtest.T) {
		// FindByID succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 2, true)))
		// UpdateMany succeeds
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		// UpdateOne fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newSpiritRepoWithDB(mt).RestoreVersion(context.Background(), "bot-v1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_FindByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch))

		_, err := newSpiritRepo(mt).FindByID(context.Background(), "ghost")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestSpiritRepository_GetVersion_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).GetVersion(context.Background(), "bot", 1); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_Deactivate_AlreadyInactive_NoError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("already inactive", func(mt *mtest.T) {
		// ModifiedCount=0 only triggers a warning log, not an error
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(0)}})

		if err := newSpiritRepo(mt).Deactivate(context.Background(), "bot-v1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSpiritRepository_Deactivate_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if err := newSpiritRepo(mt).Deactivate(context.Background(), "bot-v1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_Activate_UpdateManyError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("updatemany error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newSpiritRepoWithDB(mt).Activate(context.Background(), "bot", 1); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_Activate_UpdateOneError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("updateone error", func(mt *mtest.T) {
		// UpdateMany succeeds
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		// UpdateOne fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newSpiritRepoWithDB(mt).Activate(context.Background(), "bot", 1); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_Create_InsertError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("insert error", func(mt *mtest.T) {
		// FindActiveByName → not found (ErrNotFound, so err != nil → skip conflict check)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch))
		// InsertOne fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newSpiritRepo(mt).Create(context.Background(), &eywa.Spirit{Name: "bot"}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_FindActiveByName_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).FindActiveByName(context.Background(), "bot"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_GetVersion_InvalidInput_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("invalid", func(mt *mtest.T) {
		if _, err := newSpiritRepo(mt).GetVersion(context.Background(), "", 0); err == nil {
			t.Error("expected error for empty name/invalid version")
		}
		if _, err := newSpiritRepo(mt).GetVersion(context.Background(), "bot", 0); err == nil {
			t.Error("expected error for version < 1")
		}
	})
}

func TestSpiritRepository_Update_UpdateOneError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("updateone error", func(mt *mtest.T) {
		// FindActiveByName succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)))
		// UpdateOne fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "deactivate fail"}))

		if _, err := newSpiritRepo(mt).Update(context.Background(), "bot", &eywa.Spirit{}, ""); err == nil {
			t.Error("expected error on UpdateOne failure")
		}
	})
}

func TestSpiritRepository_Update_InsertOneError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("insertone error", func(mt *mtest.T) {
		// FindActiveByName succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)))
		// UpdateOne succeeds
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		// InsertOne fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "insert fail"}))

		if _, err := newSpiritRepo(mt).Update(context.Background(), "bot", &eywa.Spirit{}, ""); err == nil {
			t.Error("expected error on InsertOne failure")
		}
	})
}

func TestSpiritRepository_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newSpiritRepo(mt).FindByID(context.Background(), "bot-v1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestSpiritRepository_ListActive_DefaultPagination(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default pagination", func(mt *mtest.T) {
		// limit<=0 → defaults to 20, offset<0 → 0
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch))

		spirits, err := newSpiritRepo(mt).ListActive(context.Background(), 0, -5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(spirits) != 0 {
			t.Errorf("expected empty, got %d", len(spirits))
		}
	})
}

func TestSpiritRepository_Activate_InvalidInput_ReturnsError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("empty name returns error", func(mt *mtest.T) {
		repo := newSpiritRepo(mt)
		err := repo.Activate(context.Background(), "", 1)
		if err == nil {
			t.Error("expected error for empty spiritName, got nil")
		}
	})
}

func TestSpiritRepository_Activate_ZeroVersion_ReturnsError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("zero version returns error", func(mt *mtest.T) {
		repo := newSpiritRepo(mt)
		err := repo.Activate(context.Background(), "bot", 0)
		if err == nil {
			t.Error("expected error for version=0, got nil")
		}
	})
}

func TestSpiritRepository_RestoreVersion_AlreadyActive_NoError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("already active", func(mt *mtest.T) {
		// FindByID
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.spirits", mtest.FirstBatch, spiritDoc("bot", 1, true)))
		// UpdateMany (deactivate all)
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(1)}})
		// UpdateOne returns ModifiedCount=0 (already active — should log warn but no error)
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}, {Key: "nModified", Value: int32(0)}})
		// commitTransaction
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}})

		if err := newSpiritRepoWithDB(mt).RestoreVersion(context.Background(), "bot-v1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
