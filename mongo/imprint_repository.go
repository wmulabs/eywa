package mongo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	eywa "github.com/wmulabs/eywa"
)

type ImprintRepository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewImprintRepository(db *mongo.Database) eywa.ImprintRepository {
	repo := &ImprintRepository{
		collection: db.Collection("imprints"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *ImprintRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_key", Value: 1},
				{Key: "spirit_name", Value: 1},
			},
			Options: options.Index().SetName("idx_user_spirit"),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_created_at"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create imprints indexes", "error", err)
	}
}

func (r *ImprintRepository) Save(ctx context.Context, imprint eywa.Imprint) error {
	now := time.Now().UTC()
	imprint.CreatedAt = now
	imprint.UpdatedAt = now
	_, err := r.collection.InsertOne(ctx, imprint)
	if err != nil {
		return fmt.Errorf("save imprint: %w", err)
	}
	return nil
}

func (r *ImprintRepository) GetByUserKey(ctx context.Context, userKey string, spiritName string) ([]eywa.Imprint, error) {
	filter := bson.M{"user_key": userKey, "spirit_name": spiritName}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("get imprints: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var imprints []eywa.Imprint
	if err := cursor.All(ctx, &imprints); err != nil {
		return nil, fmt.Errorf("decode imprints: %w", err)
	}
	return imprints, nil
}

func (r *ImprintRepository) Delete(ctx context.Context, id string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete imprint: %w", err)
	}
	return nil
}

func (r *ImprintRepository) List(ctx context.Context, opts eywa.ImprintListOptions) ([]eywa.Imprint, int64, error) {
	filter := bson.M{}
	if opts.UserKey != "" {
		filter["user_key"] = opts.UserKey
	}
	if opts.SpiritName != "" {
		filter["spirit_name"] = opts.SpiritName
	}
	if opts.Category != "" {
		filter["category"] = opts.Category
	}

	limit := int64(50)
	if opts.Limit > 0 {
		limit = int64(opts.Limit)
	}
	skip := int64(opts.Offset)

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count imprints: %w", err)
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(skip)

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("list imprints: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var imprints []eywa.Imprint
	if err := cursor.All(ctx, &imprints); err != nil {
		return nil, 0, fmt.Errorf("decode imprints: %w", err)
	}
	return imprints, total, nil
}

func (r *ImprintRepository) Prune(ctx context.Context, userKey string, spiritName string, maxCount int) error {
	imprints, err := r.GetByUserKey(ctx, userKey, spiritName)
	if err != nil || len(imprints) <= maxCount {
		return err
	}

	sort.Slice(imprints, func(i, j int) bool {
		return imprints[i].CreatedAt.After(imprints[j].CreatedAt)
	})

	toDelete := imprints[maxCount:]
	for _, imp := range toDelete {
		_ = r.Delete(ctx, imp.ID)
	}
	return nil
}
