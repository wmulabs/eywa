package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	eywa "github.com/wmulabs/eywa"
)

type EchoRepository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewEchoRepository(database *mongo.Database) *EchoRepository {
	repo := &EchoRepository{
		collection: database.Collection("messages"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *EchoRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "memory_key", Value: 1}, {Key: "timestamp", Value: 1}},
			Options: options.Index().SetName("idx_memory_key_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "memory_key", Value: 1}, {Key: "subject_key", Value: 1}, {Key: "timestamp", Value: 1}},
			Options: options.Index().SetName("idx_memory_key_subject_key_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "subject_key", Value: 1}, {Key: "timestamp", Value: 1}},
			Options: options.Index().SetName("idx_subject_key_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "memory_key", Value: 1}, {Key: "is_user_facing", Value: 1}, {Key: "timestamp", Value: 1}},
			Options: options.Index().SetName("idx_memory_key_userfacing_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "memory_key", Value: 1}, {Key: "subject_key", Value: 1}, {Key: "is_user_facing", Value: 1}, {Key: "timestamp", Value: 1}},
			Options: options.Index().SetName("idx_memory_key_subject_key_userfacing_timestamp"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create indexes for messages collection", "error", err)
	} else {
		r.logger.Infow("indexes created for messages collection", "count", len(indexes))
	}
}

func (r *EchoRepository) Append(ctx context.Context, message eywa.Echo) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "EchoRepository/Append")
	defer span.End()

	if !message.IsUserFacing {
		return nil
	}

	if message.MemoryKey == "" {
		return fmt.Errorf("invalid input: memory_key is required")
	}

	if message.ID == "" {
		message.ID = primitive.NewObjectID().Hex()
	}
	message.Timestamp = eywa.NowUTC()

	if _, err := r.collection.InsertOne(ctx, message); err != nil {
		r.logger.Errorw("failed to append message", "error", err, "memory_key", message.MemoryKey)
		return fmt.Errorf("failed to append message: %w", err)
	}

	r.logger.Infow("message appended", "memory_key", message.MemoryKey, "subject_key", message.SubjectKey, "role", message.Role)
	return nil
}

func (r *EchoRepository) FindByMemoryKey(ctx context.Context, memoryKey string, limit, offset int) ([]*eywa.Echo, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "EchoRepository/FindByMemoryKey")
	defer span.End()

	if memoryKey == "" {
		return nil, fmt.Errorf("invalid input: memory_key is required")
	}

	filter := bson.M{"memory_key": memoryKey, "is_user_facing": true}
	return r.find(ctx, filter, limit, offset)
}

func (r *EchoRepository) FindByMemoryKeyAndSubject(ctx context.Context, memoryKey, subjectKey string, limit, offset int) ([]*eywa.Echo, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "EchoRepository/FindByMemoryKeyAndSubject")
	defer span.End()

	if memoryKey == "" || subjectKey == "" {
		return nil, fmt.Errorf("invalid input: memory_key and subject_key are required")
	}

	filter := bson.M{"memory_key": memoryKey, "subject_key": subjectKey, "is_user_facing": true}
	return r.find(ctx, filter, limit, offset)
}

func (r *EchoRepository) FindRecentByMemoryKey(ctx context.Context, memoryKey string, limit int) ([]*eywa.Echo, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "EchoRepository/FindRecentByMemoryKey")
	defer span.End()

	if memoryKey == "" {
		return nil, fmt.Errorf("invalid input: memory_key is required")
	}

	filter := bson.M{"memory_key": memoryKey, "is_user_facing": true}
	return r.findRecent(ctx, filter, limit)
}

func (r *EchoRepository) FindRecentByMemoryKeyAndSubject(ctx context.Context, memoryKey, subjectKey string, limit int) ([]*eywa.Echo, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "EchoRepository/FindRecentByMemoryKeyAndSubject")
	defer span.End()

	if memoryKey == "" || subjectKey == "" {
		return nil, fmt.Errorf("invalid input: memory_key and subject_key are required")
	}

	filter := bson.M{"memory_key": memoryKey, "subject_key": subjectKey, "is_user_facing": true}
	return r.findRecent(ctx, filter, limit)
}

func (r *EchoRepository) FindBySubjectKey(ctx context.Context, subjectKey string, limit, offset int) ([]*eywa.Echo, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "EchoRepository/FindBySubjectKey")
	defer span.End()

	if subjectKey == "" {
		return nil, fmt.Errorf("invalid input: subject_key is required")
	}

	filter := bson.M{"subject_key": subjectKey, "is_user_facing": true}
	return r.find(ctx, filter, limit, offset)
}

const (
	maxMessageLimit     = 1000
	defaultMessageLimit = 50
)

func (r *EchoRepository) find(ctx context.Context, filter bson.M, limit, offset int) ([]*eywa.Echo, error) {
	if limit <= 0 {
		limit = defaultMessageLimit
	}
	if limit > maxMessageLimit {
		limit = maxMessageLimit
	}
	if offset < 0 {
		offset = 0
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: 1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Errorw("failed to query messages", "error", err, "filter", filter)
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var messages []*eywa.Echo
	if err := cursor.All(ctx, &messages); err != nil {
		r.logger.Errorw("failed to decode messages", "error", err)
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// findRecent returns the last N messages in chronological (ASC) order.
// Uses an aggregation pipeline to sort DESC, limit, then re-sort ASC at the
// database level — avoiding in-application slice reversal.
func (r *EchoRepository) findRecent(ctx context.Context, filter bson.M, limit int) ([]*eywa.Echo, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: -1}}}},
		{{Key: "$limit", Value: int64(limit)}},
		{{Key: "$sort", Value: bson.D{{Key: "timestamp", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		r.logger.Errorw("failed to query recent messages", "error", err, "filter", filter)
		return nil, fmt.Errorf("failed to query recent messages: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var messages []*eywa.Echo
	if err := cursor.All(ctx, &messages); err != nil {
		r.logger.Errorw("failed to decode recent messages", "error", err)
		return nil, fmt.Errorf("failed to decode recent messages: %w", err)
	}

	return messages, nil
}

func (r *EchoRepository) CountByMemoryKey(ctx context.Context, memoryKey string) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"memory_key": memoryKey, "is_user_facing": true})
	if err != nil {
		return 0, fmt.Errorf("count messages by memory key: %w", err)
	}
	return count, nil
}

func (r *EchoRepository) CountByMemoryKeyAndSubject(ctx context.Context, memoryKey, subjectKey string) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"memory_key": memoryKey, "subject_key": subjectKey, "is_user_facing": true})
	if err != nil {
		return 0, fmt.Errorf("count messages by memory key and subject: %w", err)
	}
	return count, nil
}

func (r *EchoRepository) CountBySubjectKey(ctx context.Context, subjectKey string) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"subject_key": subjectKey, "is_user_facing": true})
	if err != nil {
		return 0, fmt.Errorf("count messages by subject key: %w", err)
	}
	return count, nil
}
