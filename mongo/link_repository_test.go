package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func newLinkMT(t *testing.T) *mtest.T {
	t.Helper()
	return mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
}

func TestLinkRepository_FindAll_ReturnsLinks(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("returns links", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "chat"},
			{Key: "voice_name", Value: "twilio"},
			{Key: "require_scouts", Value: bson.A{}},
			{Key: "allowed_spirits", Value: bson.A{}},
			{Key: "guards", Value: bson.A{}},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.event_configurations", mtest.FirstBatch, doc))
		repo := NewLinkRepository(mt.DB)

		links, err := repo.FindAll(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) != 1 || links[0].EventType != "chat" {
			t.Errorf("expected 1 link EventType=chat, got %v", links)
		}
	})
}

func TestLinkRepository_FindAll_Empty(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("empty", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch))
		repo := NewLinkRepository(mt.DB)

		links, err := repo.FindAll(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) != 0 {
			t.Errorf("expected empty, got %d", len(links))
		}
	})
}

func TestLinkRepository_FindAll_Error(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))
		repo := NewLinkRepository(mt.DB)

		_, err := repo.FindAll(context.Background())

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestLinkRepository_FindByKey_Found(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "order"},
			{Key: "default_spirit", Value: "bot"},
			{Key: "require_scouts", Value: bson.A{}},
			{Key: "allowed_spirits", Value: bson.A{}},
			{Key: "guards", Value: bson.A{}},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch, doc))
		repo := NewLinkRepository(mt.DB)

		link, err := repo.FindByKey(context.Background(), "order")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.EventType != "order" || link.DefaultSpirit != "bot" {
			t.Errorf("unexpected link: %+v", link)
		}
	})
}

func TestLinkRepository_FindByKey_NotFound(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch))
		repo := NewLinkRepository(mt.DB)

		_, err := repo.FindByKey(context.Background(), "missing")

		if err != eywa.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestLinkRepository_Save_Success(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("save", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})
		repo := NewLinkRepository(mt.DB)
		link := eywa.NewLink("chat").Build()

		if err := repo.Save(context.Background(), link); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLinkRepository_Delete_Success(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("delete", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}})
		repo := NewLinkRepository(mt.DB)

		if err := repo.Delete(context.Background(), "chat"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLinkRepository_FindAll_WithGuards(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("with guards", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "_id", Value: "chat"},
			{Key: "require_scouts", Value: bson.A{"scout-1"}},
			{Key: "allowed_spirits", Value: bson.A{"bot-a", "bot-b"}},
			{Key: "guards", Value: bson.A{
				bson.D{
					{Key: "field", Value: "region"},
					{Key: "block_list", Value: bson.A{"us-east"}},
					{Key: "allow_list", Value: bson.A{"eu-west"}},
				},
			}},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.event_configurations", mtest.FirstBatch, doc))
		repo := NewLinkRepository(mt.DB)

		links, err := repo.FindAll(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(links) != 1 || len(links[0].Guards) != 1 {
			t.Errorf("expected 1 link with 1 guard")
		}
	})
}

func TestLinkRepository_Save_WithGuards(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("save with guards", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})
		repo := NewLinkRepository(mt.DB)
		link := eywa.NewLink("chat").
			WithGuards(eywa.Guard{Field: "region", BlockList: []string{"us-east"}}).
			Build()

		if err := repo.Save(context.Background(), link); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLinkRepository_FindByKey_Error(t *testing.T) {
	mt := newLinkMT(t)
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "fail"}))
		repo := NewLinkRepository(mt.DB)

		if _, err := repo.FindByKey(context.Background(), "chat"); err == nil {
			t.Error("expected error")
		}
	})
}
