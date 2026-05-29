package mongo

import (
	"context"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

// ChronicleRepository persists interaction audit records to MongoDB.
//
// Immutability note: Chronicle records are append-only by application convention —
// this repository exposes no Update or Delete methods. However, MongoDB itself does
// not enforce write protection; records can be modified or deleted by any process
// with direct database write access. For compliance use cases (GDPR, HIPAA, SOC2)
// that require a legally-binding audit trail, supplement with an external append-only
// log service or enable MongoDB Atlas audit log streaming to object storage.
type ChronicleRepository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewChronicleRepository(database *mongo.Database) *ChronicleRepository {
	repo := &ChronicleRepository{
		collection: database.Collection("interaction_logs"),
		logger:     newLogger(),
	}

	repo.ensureIndexes()

	return repo
}

func (r *ChronicleRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := r.getIndexModels()

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create indexes for interaction_logs collection", "error", err)
	} else {
		r.logger.Infow("indexes created successfully for interaction_logs collection", "count", len(indexes))
	}
}

func (r *ChronicleRepository) getIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			// Optimizes: FindByMemoryKey
			Keys: bson.D{
				{Key: "memory_key", Value: 1},
				{Key: "timestamp", Value: 1},
			},
			Options: options.Index().
				SetName("idx_memory_key_timestamp"),
		},
		{
			// Optimizes: FindByEventKey
			Keys: bson.D{
				{Key: "event_key", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().
				SetName("idx_event_key_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "spirit.id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_spirit_id_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "spirit.name", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_spirit_name_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "spirit.provider", Value: 1},
				{Key: "spirit.model", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_spirit_provider_model_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "processing.status", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_processing_status_timestamp"),
		},
		{
			Keys: bson.D{
				{Key: "event.contact_phone", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index().SetName("idx_event_contact_phone_timestamp"),
		},
	}
}

func (r *ChronicleRepository) LogInteraction(ctx context.Context, log *eywa.Chronicle) error {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "Repository/Interaction/LogInteraction")
	defer span.End()

	log.Timestamp = eywa.NowUTC()

	result, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		r.logger.Errorw("error logging interaction", "error", err)
		return fmt.Errorf("insert interaction log: %w", err)
	}

	log.ID = result.InsertedID.(primitive.ObjectID).Hex()
	r.logger.Infow("interaction logged", "id", log.ID, "memory_key", log.MemoryKey, "event_key", log.EventKey)
	return nil
}

func (r *ChronicleRepository) FindByMemoryKey(ctx context.Context, memoryKey string) ([]*eywa.Chronicle, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "Repository/Interaction/FindByMemoryKey")
	defer span.End()

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := r.collection.Find(ctx, bson.M{"memory_key": memoryKey}, opts)
	if err != nil {
		r.logger.Errorw("error finding interactions", "error", err, "memory_key", memoryKey)
		return nil, fmt.Errorf("find interactions: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var interactions []*eywa.Chronicle
	if err = cursor.All(ctx, &interactions); err != nil {
		r.logger.Errorw("error decoding interactions", "error", err)
		return nil, fmt.Errorf("find interactions: %w", err)
	}

	return interactions, nil
}

func (r *ChronicleRepository) FindByEventKey(ctx context.Context, eventKey string) ([]*eywa.Chronicle, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "Repository/Interaction/FindByEventKey")
	defer span.End()

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := r.collection.Find(ctx, bson.M{"event_key": eventKey}, opts)
	if err != nil {
		r.logger.Errorw("error finding interactions", "error", err, "event_key", eventKey)
		return nil, fmt.Errorf("find interactions by event key: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var interactions []*eywa.Chronicle
	if err = cursor.All(ctx, &interactions); err != nil {
		r.logger.Errorw("error decoding interactions", "error", err)
		return nil, fmt.Errorf("find interactions by event key: %w", err)
	}

	return interactions, nil
}

func (r *ChronicleRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*eywa.Chronicle, error) {
	ctx, span := noop.NewTracerProvider().Tracer("eywa").Start(ctx, "Repository/Interaction/FindByTimeRange")
	defer span.End()

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	filter := bson.M{
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Errorw("error finding interactions by time range", "error", err)
		return nil, fmt.Errorf("find interactions: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var interactions []*eywa.Chronicle
	if err = cursor.All(ctx, &interactions); err != nil {
		r.logger.Errorw("error decoding interactions", "error", err)
		return nil, fmt.Errorf("find interactions: %w", err)
	}

	return interactions, nil
}
