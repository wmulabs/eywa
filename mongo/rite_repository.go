package mongo

import (
	"context"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var _ eywa.RiteRepository = (*RiteRepository)(nil)

type RiteRepository struct {
	collection *mongodriver.Collection
	logger     *zap.SugaredLogger
}

func NewRiteRepository(database *mongodriver.Database) *RiteRepository {
	repo := &RiteRepository{
		collection: database.Collection("rites"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *RiteRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongodriver.IndexModel{
		{
			Keys: bson.D{
				{Key: "memory_key", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("idx_memory_key_status"),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "requested_at", Value: -1},
			},
			Options: options.Index().SetName("idx_status_requested_at"),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetName("idx_expires_at_ttl").SetExpireAfterSeconds(0),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create rites indexes", "error", err)
	}
}

func (r *RiteRepository) Create(ctx context.Context, rite *eywa.Rite) error {
	if rite.ID == "" {
		rite.ID = primitive.NewObjectID().Hex()
	}
	_, err := r.collection.InsertOne(ctx, rite)
	if err != nil {
		return fmt.Errorf("insert rite: %w", err)
	}
	return nil
}

func (r *RiteRepository) FindByID(ctx context.Context, id string) (*eywa.Rite, error) {
	var rite eywa.Rite
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&rite)
	if err == mongodriver.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "rite", ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("decode rite: %w", err)
	}
	return &rite, nil
}

func (r *RiteRepository) List(ctx context.Context, opts eywa.RiteListOptions) ([]*eywa.Rite, int64, error) {
	filter := bson.M{}
	if opts.MemoryKey != "" {
		filter["memory_key"] = opts.MemoryKey
	}
	if opts.Status != "" {
		filter["status"] = opts.Status
	}

	page := opts.Page
	if page < 1 {
		page = 1
	}
	limit := opts.Limit
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count rites: %w", err)
	}

	cursor, err := r.collection.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "requested_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, fmt.Errorf("find rites: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var rites []*eywa.Rite
	if err := cursor.All(ctx, &rites); err != nil {
		return nil, 0, fmt.Errorf("decode rites: %w", err)
	}
	return rites, total, nil
}

func (r *RiteRepository) Decide(ctx context.Context, id, operatorID string, status eywa.RiteStatus) error {
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"operator_id": operatorID,
			"decided_at":  now,
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("decide rite: %w", err)
	}
	if res.MatchedCount == 0 {
		return &eywa.NotFoundError{Entity: "rite", ID: id}
	}
	return nil
}
