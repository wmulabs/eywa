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

type LoreRepository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewLoreRepository(db *mongo.Database) eywa.LoreRepository {
	repo := &LoreRepository{
		collection: db.Collection("lores"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *LoreRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetName("idx_name").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "spirit_ids", Value: 1}},
			Options: options.Index().SetName("idx_spirit_ids"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create lores indexes", "error", err)
	}
}

func (r *LoreRepository) Create(ctx context.Context, lore eywa.Lore) error {
	now := time.Now().UTC()
	lore.CreatedAt = now
	lore.UpdatedAt = now
	_, err := r.collection.InsertOne(ctx, lore)
	if err != nil {
		return fmt.Errorf("create lore: %w", err)
	}
	return nil
}

func (r *LoreRepository) GetByID(ctx context.Context, id string) (eywa.Lore, error) {
	var lore eywa.Lore
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&lore)
	if err == mongo.ErrNoDocuments {
		return lore, &eywa.NotFoundError{Entity: "lore", ID: id}
	}
	if err != nil {
		return lore, fmt.Errorf("get lore %q: %w", id, err)
	}
	return lore, nil
}

func (r *LoreRepository) GetByName(ctx context.Context, name string) (eywa.Lore, error) {
	var lore eywa.Lore
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&lore)
	if err == mongo.ErrNoDocuments {
		return lore, &eywa.NotFoundError{Entity: "lore", ID: name}
	}
	if err != nil {
		return lore, fmt.Errorf("get lore %q: %w", name, err)
	}
	return lore, nil
}

func (r *LoreRepository) GetBySpiritID(ctx context.Context, spiritID string) ([]eywa.Lore, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"spirit_ids": spiritID})
	if err != nil {
		return nil, fmt.Errorf("get lores by spirit: %w", err)
	}
	defer cursor.Close(ctx)

	var lores []eywa.Lore
	if err := cursor.All(ctx, &lores); err != nil {
		return nil, fmt.Errorf("decode lores: %w", err)
	}
	return lores, nil
}

func (r *LoreRepository) GetByIDs(ctx context.Context, ids []string) ([]eywa.Lore, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("get lores by ids: %w", err)
	}
	defer cursor.Close(ctx)

	var lores []eywa.Lore
	if err := cursor.All(ctx, &lores); err != nil {
		return nil, fmt.Errorf("decode lores: %w", err)
	}
	return lores, nil
}

func (r *LoreRepository) Update(ctx context.Context, lore eywa.Lore) error {
	lore.UpdatedAt = time.Now().UTC()
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": lore.ID}, lore)
	if err != nil {
		return fmt.Errorf("update lore: %w", err)
	}
	return nil
}

func (r *LoreRepository) Delete(ctx context.Context, id string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete lore: %w", err)
	}
	return nil
}
