package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	eywa "github.com/wmulabs/eywa"
)

type LedgerRepository struct {
	entries *mongo.Collection
	budgets *mongo.Collection
	logger  *zap.SugaredLogger
}

func NewLedgerRepository(db *mongo.Database) eywa.LedgerRepository {
	repo := &LedgerRepository{
		entries: db.Collection("ledger_entries"),
		budgets: db.Collection("ledger_budgets"),
		logger:  newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *LedgerRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := r.entries.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "spirit_name", Value: 1}, {Key: "month", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		r.logger.Warnw("failed to create ledger_entries index", "error", err)
	}

	if _, err := r.budgets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "spirit_name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		r.logger.Warnw("failed to create ledger_budgets index", "error", err)
	}
}

func (r *LedgerRepository) IncrementUsage(ctx context.Context, spiritName string, tokens int64, costUSD float64) error {
	month := time.Now().UTC().Format("2006-01")

	filter := bson.M{"spirit_name": spiritName, "month": month}
	update := bson.M{
		"$inc": bson.M{
			"tokens_used":  tokens,
			"est_cost_usd": costUSD,
		},
		"$set": bson.M{
			"updated_at": time.Now().UTC(),
		},
		"$setOnInsert": bson.M{
			"spirit_name": spiritName,
			"month":       month,
		},
	}

	_, err := r.entries.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("update ledger entry: %w", err)
	}
	return nil
}

func (r *LedgerRepository) GetMonthUsage(ctx context.Context, spiritName string, month string) (eywa.LedgerEntry, error) {
	filter := bson.M{"spirit_name": spiritName, "month": month}

	var entry eywa.LedgerEntry
	err := r.entries.FindOne(ctx, filter).Decode(&entry)
	if err == mongo.ErrNoDocuments {
		return eywa.LedgerEntry{SpiritName: spiritName, Month: month}, nil
	}
	if err != nil {
		return eywa.LedgerEntry{}, fmt.Errorf("decode ledger entry: %w", err)
	}
	return entry, nil
}

func (r *LedgerRepository) GetBudget(ctx context.Context, spiritName string) (eywa.TokenBudget, error) {
	filter := bson.M{"spirit_name": spiritName}

	var budget eywa.TokenBudget
	err := r.budgets.FindOne(ctx, filter).Decode(&budget)
	if err == mongo.ErrNoDocuments {
		return eywa.TokenBudget{SpiritName: spiritName}, nil
	}
	if err != nil {
		return eywa.TokenBudget{}, fmt.Errorf("decode ledger budget: %w", err)
	}
	return budget, nil
}

func (r *LedgerRepository) SetBudget(ctx context.Context, budget eywa.TokenBudget) error {
	filter := bson.M{"spirit_name": budget.SpiritName}
	update := bson.M{
		"$set": budget,
	}

	_, err := r.budgets.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert ledger entry: %w", err)
	}
	return nil
}
