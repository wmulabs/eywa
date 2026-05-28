package mongo

import (
	"context"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var _ eywa.OperatorRepository = (*OperatorRepository)(nil)

type OperatorRepository struct {
	collection *mongodriver.Collection
	logger     *zap.SugaredLogger
}

func NewOperatorRepository(database *mongodriver.Database) *OperatorRepository {
	repo := &OperatorRepository{
		collection: database.Collection("operators"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *OperatorRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongodriver.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetName("idx_email_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "is_active", Value: 1}},
			Options: options.Index().SetName("idx_is_active"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create operators indexes", "error", err)
	}
}

func (r *OperatorRepository) Create(ctx context.Context, op *eywa.Operator) error {
	if op.ID == "" {
		op.ID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	op.CreatedAt = now
	op.UpdatedAt = now
	_, err := r.collection.InsertOne(ctx, op)
	return err
}

func (r *OperatorRepository) FindByEmail(ctx context.Context, email string) (*eywa.Operator, error) {
	var op eywa.Operator
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&op)
	if err == mongodriver.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "operator", ID: email}
	}
	return &op, err
}

func (r *OperatorRepository) FindByID(ctx context.Context, id string) (*eywa.Operator, error) {
	var op eywa.Operator
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&op)
	if err == mongodriver.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "operator", ID: id}
	}
	return &op, err
}

func (r *OperatorRepository) List(ctx context.Context, page, limit int) ([]*eywa.Operator, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	cursor, err := r.collection.Find(ctx, bson.M{},
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetSkip(skip).
			SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var ops []*eywa.Operator
	if err := cursor.All(ctx, &ops); err != nil {
		return nil, 0, err
	}
	return ops, total, nil
}

func (r *OperatorRepository) Update(ctx context.Context, op *eywa.Operator) error {
	op.UpdatedAt = time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"name":       op.Name,
			"email":      op.Email,
			"role":       op.Role,
			"is_active":  op.IsActive,
			"updated_at": op.UpdatedAt,
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": op.ID}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return &eywa.NotFoundError{Entity: "operator", ID: op.ID}
	}
	return nil
}

func (r *OperatorRepository) Deactivate(ctx context.Context, id string) error {
	update := bson.M{
		"$set": bson.M{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return &eywa.NotFoundError{Entity: "operator", ID: id}
	}
	return nil
}
