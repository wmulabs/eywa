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

func newOperatorRepo(mt *mtest.T) *OperatorRepository {
	return &OperatorRepository{
		collection: mt.DB.Collection("operators"),
		logger:     zap.NewNop().Sugar(),
	}
}

func operatorDoc(id, email string) bson.D {
	return bson.D{
		{Key: "_id", Value: id},
		{Key: "name", Value: "Test Op"},
		{Key: "email", Value: email},
		{Key: "role", Value: "admin"},
		{Key: "is_active", Value: true},
	}
}

func TestOperatorRepository_Create_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		op := &eywa.Operator{Name: "Alice", Email: "alice@example.com", Role: "admin"}
		if err := newOperatorRepo(mt).Create(context.Background(), op); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestOperatorRepository_FindByEmail_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch,
			operatorDoc("op-1", "alice@example.com"),
		))

		op, err := newOperatorRepo(mt).FindByEmail(context.Background(), "alice@example.com")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if op.Email != "alice@example.com" {
			t.Errorf("expected alice@example.com, got %s", op.Email)
		}
	})
}

func TestOperatorRepository_FindByEmail_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch))

		_, err := newOperatorRepo(mt).FindByEmail(context.Background(), "ghost@example.com")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestOperatorRepository_FindByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch,
			operatorDoc("op-42", "bob@example.com"),
		))

		op, err := newOperatorRepo(mt).FindByID(context.Background(), "op-42")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if op.ID != "op-42" {
			t.Errorf("expected id=op-42, got %s", op.ID)
		}
	})
}

func TestOperatorRepository_Update_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(1)},
			{Key: "nModified", Value: int32(1)},
		})

		op := &eywa.Operator{ID: "op-1", Name: "Updated", Email: "x@x.com", Role: "admin", IsActive: true}
		if err := newOperatorRepo(mt).Update(context.Background(), op); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestOperatorRepository_Update_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(0)},
			{Key: "nModified", Value: int32(0)},
		})

		op := &eywa.Operator{ID: "ghost"}
		err := newOperatorRepo(mt).Update(context.Background(), op)

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestOperatorRepository_Deactivate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(1)},
			{Key: "nModified", Value: int32(1)},
		})

		if err := newOperatorRepo(mt).Deactivate(context.Background(), "op-1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestOperatorRepository_List_ReturnsPaginated(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		// CountDocuments
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(2)}},
		))
		// Find
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch,
			operatorDoc("op-1", "a@a.com"),
			operatorDoc("op-2", "b@b.com"),
		))

		ops, total, err := newOperatorRepo(mt).List(context.Background(), 1, 20)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 2 || len(ops) != 2 {
			t.Errorf("expected total=2 len=2, got total=%d len=%d", total, len(ops))
		}
	})
}

func TestOperatorRepository_List_CountError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("count error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, _, err := newOperatorRepo(mt).List(context.Background(), 1, 20); err == nil {
			t.Error("expected error")
		}
	})
}

func TestOperatorRepository_List_DefaultPagination(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("defaults", func(mt *mtest.T) {
		// page<1 and limit<1 → default to page=1, limit=20
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(0)}},
		))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch))

		ops, total, err := newOperatorRepo(mt).List(context.Background(), 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 0 || len(ops) != 0 {
			t.Errorf("expected empty, got total=%d len=%d", total, len(ops))
		}
	})
}

func TestOperatorRepository_Deactivate_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(0)},
			{Key: "nModified", Value: int32(0)},
		})

		err := newOperatorRepo(mt).Deactivate(context.Background(), "ghost")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestOperatorRepository_Deactivate_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newOperatorRepo(mt).Deactivate(context.Background(), "op-1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestOperatorRepository_FindByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch))

		_, err := newOperatorRepo(mt).FindByID(context.Background(), "ghost")

		var notFound *eywa.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("expected NotFoundError, got %v", err)
		}
	})
}

func TestOperatorRepository_Update_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newOperatorRepo(mt).Update(context.Background(), &eywa.Operator{ID: "op-1"}); err == nil {
			t.Error("expected error")
		}
	})
}

func TestOperatorRepository_List_FindError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("find error", func(mt *mtest.T) {
		// CountDocuments succeeds
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.operators", mtest.FirstBatch,
			bson.D{{Key: "n", Value: int32(5)}},
		))
		// Find fails
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "find fail"}))

		if _, _, err := newOperatorRepo(mt).List(context.Background(), 1, 20); err == nil {
			mt.Error("expected error on Find failure")
		}
	})
}

func TestOperatorRepository_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newOperatorRepo(mt).FindByID(context.Background(), "op-1"); err == nil {
			mt.Error("expected error")
		}
	})
}

func TestOperatorRepository_FindByEmail_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		if _, err := newOperatorRepo(mt).FindByEmail(context.Background(), "a@b.com"); err == nil {
			mt.Error("expected error")
		}
	})
}
