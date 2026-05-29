package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func newHTTPToolRepo(mt *mtest.T) *HTTPToolRepository {
	return NewHTTPToolRepository(mt.DB)
}

func httpToolDoc(id, name string) bson.D {
	return bson.D{
		{Key: "_id", Value: id},
		{Key: "name", Value: name},
		{Key: "method", Value: "GET"},
		{Key: "url", Value: "https://api.example.com"},
		{Key: "parameters", Value: bson.A{}},
		{Key: "headers", Value: bson.D{}},
		{Key: "spirit_ids", Value: bson.A{}},
	}
}

func TestHTTPToolRepository_List_ReturnTools(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("list", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.http_tools", mtest.FirstBatch,
			httpToolDoc("tool-1", "get-weather"),
			httpToolDoc("tool-2", "send-email"),
		))

		tools, err := newHTTPToolRepo(mt).List(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tools) != 2 {
			t.Errorf("expected 2 tools, got %d", len(tools))
		}
	})
}

func TestHTTPToolRepository_FindByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.http_tools", mtest.FirstBatch,
			httpToolDoc("tool-1", "get-weather"),
		))

		tool, err := newHTTPToolRepo(mt).FindByID(context.Background(), "tool-1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tool.ID != "tool-1" || tool.Name != "get-weather" {
			t.Errorf("unexpected tool: %+v", tool)
		}
	})
}

func TestHTTPToolRepository_FindByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.http_tools", mtest.FirstBatch))

		_, err := newHTTPToolRepo(mt).FindByID(context.Background(), "missing")

		if err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestHTTPToolRepository_Save_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("save", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})

		tool := eywa.HTTPTool{ID: "t-1", Name: "my-tool", Method: "POST", URL: "https://x.com"}
		if err := newHTTPToolRepo(mt).Save(context.Background(), tool); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestHTTPToolRepository_Update_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(1)},
			{Key: "nModified", Value: int32(1)},
		})

		tool := eywa.HTTPTool{ID: "t-1", Name: "updated", Method: "PUT", URL: "https://x.com"}
		if err := newHTTPToolRepo(mt).Update(context.Background(), tool); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestHTTPToolRepository_FindBySpiritID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.http_tools", mtest.FirstBatch,
			httpToolDoc("tool-1", "get-weather"),
		))

		tools, err := newHTTPToolRepo(mt).FindBySpiritID(context.Background(), "spirit-1")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tools) != 1 || tools[0].Name != "get-weather" {
			t.Errorf("unexpected tools: %v", tools)
		}
	})
}

func TestHTTPToolRepository_Update_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: int32(0)},
			{Key: "nModified", Value: int32(0)},
		})

		tool := eywa.HTTPTool{ID: "ghost", Name: "x", Method: "GET", URL: "https://x.com"}
		if err := newHTTPToolRepo(mt).Update(context.Background(), tool); err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestHTTPToolRepository_Delete_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(0)}})

		if err := newHTTPToolRepo(mt).Delete(context.Background(), "ghost"); err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestHTTPToolRepository_Delete_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: int32(1)}})

		if err := newHTTPToolRepo(mt).Delete(context.Background(), "t-1"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestHTTPToolRepository_List_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newHTTPToolRepo(mt).List(context.Background()); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHTTPToolRepository_FindBySpiritID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newHTTPToolRepo(mt).FindBySpiritID(context.Background(), "spirit-1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHTTPToolRepository_Save_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		tool := eywa.HTTPTool{ID: "tool-1", Name: "test", Method: "GET", URL: "https://example.com"}
		if err := newHTTPToolRepo(mt).Save(context.Background(), tool); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHTTPToolRepository_List_WithParameters(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("with parameters", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "tool-2"},
			{Key: "name", Value: "search"},
			{Key: "method", Value: "POST"},
			{Key: "url", Value: "https://api.example.com/search"},
			{Key: "headers", Value: bson.D{{Key: "Authorization", Value: "Bearer token"}}},
			{Key: "spirit_ids", Value: bson.A{"spirit-1"}},
			{Key: "parameters", Value: bson.A{
				bson.D{
					{Key: "name", Value: "query"},
					{Key: "type", Value: "string"},
					{Key: "description", Value: "search query"},
					{Key: "required", Value: true},
				},
			}},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.http_tools", mtest.FirstBatch, doc))

		tools, err := newHTTPToolRepo(mt).List(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tools) != 1 || len(tools[0].Parameters) != 1 {
			t.Errorf("expected 1 tool with 1 param, got %d tools", len(tools))
		}
		if tools[0].Parameters[0].Name != "query" {
			t.Errorf("expected param name=query, got %s", tools[0].Parameters[0].Name)
		}
	})
}

func TestHTTPToolRepository_Update_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		tool := eywa.HTTPTool{ID: "tool-1", Name: "test", Method: "GET", URL: "https://example.com"}
		if err := newHTTPToolRepo(mt).Update(context.Background(), tool); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHTTPToolRepository_Delete_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if err := newHTTPToolRepo(mt).Delete(context.Background(), "tool-1"); err == nil {
			t.Error("expected error")
		}
	})
}

func TestHTTPToolRepository_FindByID_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))

		if _, err := newHTTPToolRepo(mt).FindByID(context.Background(), "tool-1"); err == nil {
			t.Error("expected error")
		}
	})
}
