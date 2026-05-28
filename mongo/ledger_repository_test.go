package mongo

import (
	"context"
	"testing"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"go.uber.org/zap"
)

func newLedgerRepo(mt *mtest.T) *LedgerRepository {
	return &LedgerRepository{
		entries: mt.DB.Collection("ledger_entries"),
		budgets: mt.DB.Collection("ledger_budgets"),
		logger:  zap.NewNop().Sugar(),
	}
}

func TestLedgerRepository_IncrementUsage_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		if err := newLedgerRepo(mt).IncrementUsage(context.Background(), "spirit-a", 100, 0.01); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLedgerRepository_IncrementUsage_Error(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("error", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 10, Message: "fail"}))

		if err := newLedgerRepo(mt).IncrementUsage(context.Background(), "spirit-a", 100, 0.01); err == nil {
			t.Error("expected error")
		}
	})
}

func TestLedgerRepository_GetMonthUsage_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "spirit_name", Value: "bot"},
			{Key: "month", Value: "2026-05"},
			{Key: "tokens_used", Value: int64(500)},
			{Key: "est_cost_usd", Value: float64(0.05)},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch, doc))

		entry, err := newLedgerRepo(mt).GetMonthUsage(context.Background(), "bot", "2026-05")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.TokensUsed != 500 {
			t.Errorf("expected TokensUsed=500, got %d", entry.TokensUsed)
		}
	})
}

func TestLedgerRepository_GetMonthUsage_NotFound_ReturnsEmpty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch))

		entry, err := newLedgerRepo(mt).GetMonthUsage(context.Background(), "bot", "2026-05")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.SpiritName != "bot" || entry.Month != "2026-05" {
			t.Errorf("unexpected empty entry: %+v", entry)
		}
	})
}

func TestLedgerRepository_GetBudget_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("found", func(mt *mtest.T) {
		doc := bson.D{
			{Key: "spirit_name", Value: "bot"},
			{Key: "monthly_token_limit", Value: int64(10000)},
			{Key: "on_exceed", Value: "block"},
		}
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch, doc))

		budget, err := newLedgerRepo(mt).GetBudget(context.Background(), "bot")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if budget.MonthlyTokenLimit != 10000 || budget.OnExceed != "block" {
			t.Errorf("unexpected budget: %+v", budget)
		}
	})
}

func TestLedgerRepository_GetBudget_NotFound_ReturnsEmpty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("not found", func(mt *mtest.T) {
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "db.c", mtest.FirstBatch))

		budget, err := newLedgerRepo(mt).GetBudget(context.Background(), "bot")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if budget.SpiritName != "bot" {
			t.Errorf("expected empty budget for bot, got %+v", budget)
		}
	})
}

func TestLedgerRepository_SetBudget_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("success", func(mt *mtest.T) {
		mt.AddMockResponses(bson.D{{Key: "ok", Value: 1}, {Key: "n", Value: 1}, {Key: "nModified", Value: 1}})

		budget := eywa.TokenBudget{SpiritName: "bot", MonthlyTokenLimit: 5000, OnExceed: "block"}
		if err := newLedgerRepo(mt).SetBudget(context.Background(), budget); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
