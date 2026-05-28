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
)

// Compile-time interface satisfaction check.
var _ eywa.EchoQueryRepository = (*EchoRepository)(nil)

// ListSessions aggregates user-facing messages to one SessionSummary per memory_key.
// SpiritName filtering is not supported (Echo documents do not carry spirit_name).
func (r *EchoRepository) ListSessions(ctx context.Context, opts eywa.SessionListOptions) ([]*eywa.SessionSummary, int64, error) {
	matchFilter := bson.M{"is_user_facing": true}
	if opts.MemoryKey != "" {
		matchFilter["memory_key"] = opts.MemoryKey
	}
	timeFilter := bson.M{}
	if opts.DateFrom != nil {
		timeFilter["$gte"] = *opts.DateFrom
	}
	if opts.DateTo != nil {
		timeFilter["$lte"] = *opts.DateTo
	}
	if len(timeFilter) > 0 {
		matchFilter["timestamp"] = timeFilter
	}

	// Count distinct sessions.
	countPipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$memory_key"}}}},
		{{Key: "$count", Value: "total"}},
	}
	countCursor, err := r.collection.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer countCursor.Close(ctx)
	var countResult []struct {
		Total int64 `bson:"total"`
	}
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}
	var total int64
	if len(countResult) > 0 {
		total = countResult[0].Total
	}

	page := opts.Page
	if page < 1 {
		page = 1
	}
	limit := opts.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	skip := int64((page - 1) * limit)

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$memory_key"},
			{Key: "message_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "last_message_at", Value: bson.D{{Key: "$max", Value: "$timestamp"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "last_message_at", Value: -1}}}},
		{{Key: "$skip", Value: skip}},
		{{Key: "$limit", Value: int64(limit)}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	type sessionResult struct {
		MemoryKey     string    `bson:"_id"`
		MessageCount  int64     `bson:"message_count"`
		LastMessageAt time.Time `bson:"last_message_at"`
	}

	var raw []sessionResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, 0, err
	}

	result := make([]*eywa.SessionSummary, len(raw))
	for i, r := range raw {
		result[i] = &eywa.SessionSummary{
			MemoryKey:     r.MemoryKey,
			MessageCount:  r.MessageCount,
			LastMessageAt: r.LastMessageAt,
		}
	}
	return result, total, nil
}

// FindByMemoryKeyBefore returns user-facing messages for memoryKey with IDs before beforeID,
// sorted newest-first. Pass empty beforeID to get the most recent messages.
func (r *EchoRepository) FindByMemoryKeyBefore(ctx context.Context, memoryKey, beforeID string, limit int) ([]*eywa.Echo, error) {
	if memoryKey == "" {
		return nil, fmt.Errorf("memoryKey is required")
	}
	filter := bson.M{"memory_key": memoryKey, "is_user_facing": true}
	if beforeID != "" {
		oid, err := primitive.ObjectIDFromHex(beforeID)
		if err != nil {
			return nil, fmt.Errorf("invalid before_id: %w", err)
		}
		filter["_id"] = bson.M{"$lt": oid}
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []*eywa.Echo
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}
