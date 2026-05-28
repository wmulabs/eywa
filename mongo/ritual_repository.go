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
	"go.uber.org/zap"
)

type RitualRepository struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewRitualRepository(database *mongo.Database) eywa.RitualRepository {
	repo := &RitualRepository{
		collection: database.Collection("scheduled_tasks"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *RitualRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "memory_key", Value: 1},
				{Key: "status", Value: 1},
				{Key: "execute_at", Value: 1},
			},
			Options: options.Index().SetName("idx_memory_key_status_execute_at"),
		},
		{
			Keys:    bson.D{{Key: "execute_at", Value: 1}},
			Options: options.Index().SetName("idx_execute_at"),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create indexes for scheduled_tasks", "error", err)
	}
}

func (r *RitualRepository) Save(ctx context.Context, task *eywa.Ritual) error {
	if task.ID == "" {
		task.ID = primitive.NewObjectID().Hex()
	}
	_, err := r.collection.InsertOne(ctx, task)
	return err
}

func (r *RitualRepository) FindByID(ctx context.Context, id string) (*eywa.Ritual, error) {
	var task eywa.Ritual
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, &eywa.NotFoundError{Entity: "ritual", ID: id}
	}
	return &task, err
}

func (r *RitualRepository) FindPendingByMemoryKey(ctx context.Context, memoryKey string, limit, offset int) ([]*eywa.Ritual, error) {
	filter := bson.M{
		"memory_key": memoryKey,
		"status":     eywa.RitualPending,
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "execute_at", Value: 1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*eywa.Ritual
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *RitualRepository) CountByMemoryKey(ctx context.Context, memoryKey string) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{
		"memory_key": memoryKey,
		"status":     eywa.RitualPending,
	})
}

func (r *RitualRepository) UpdateStatus(ctx context.Context, id string, status eywa.RitualStatus, ts time.Time) error {
	tsField, err := statusTimestampField(status)
	if err != nil {
		return err
	}
	update := bson.M{"$set": bson.M{"status": status, tsField: ts}}
	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *RitualRepository) UpdateRunning(ctx context.Context, id string, ts time.Time) error {
	update := bson.M{
		"$set": bson.M{"status": eywa.RitualRunning, "started_at": ts},
		"$inc": bson.M{"attempt_count": 1},
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *RitualRepository) UpdateFailed(ctx context.Context, id string, ts time.Time, reason string) error {
	update := bson.M{"$set": bson.M{
		"status":         eywa.RitualFailed,
		"failed_at":      ts,
		"failure_reason": reason,
	}}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *RitualRepository) UpdateKeeperTaskID(ctx context.Context, id string, keeperTaskID string) error {
	update := bson.M{"$set": bson.M{"keeper_task_id": keeperTaskID}}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func statusTimestampField(status eywa.RitualStatus) (string, error) {
	switch status {
	case eywa.RitualRunning:
		return "started_at", nil
	case eywa.RitualExecuted:
		return "executed_at", nil
	case eywa.RitualCancelled:
		return "cancelled_at", nil
	default:
		return "", fmt.Errorf("UpdateStatus: unhandled status %q", status)
	}
}
