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
var _ eywa.ChronicleQueryRepository = (*ChronicleRepository)(nil)

func (r *ChronicleRepository) FindByID(ctx context.Context, id string) (*eywa.Chronicle, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid chronicle id: %w", err)
	}
	var ch eywa.Chronicle
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&ch); err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, eywa.ErrNotFound
		}
		return nil, err
	}
	return &ch, nil
}

func (r *ChronicleRepository) List(ctx context.Context, opts eywa.ChronicleListOptions) ([]*eywa.Chronicle, int64, error) {
	filter := buildChronicleListFilter(opts)

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
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

	findOpts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []*eywa.Chronicle
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

func (r *ChronicleRepository) AggregateTokens(ctx context.Context, spiritName string, from, to time.Time, granularity string) ([]eywa.TokenSeries, error) {
	unit := "day"
	if granularity == "week" || granularity == "month" {
		unit = granularity
	}

	matchFilter := bson.M{"timestamp": bson.M{"$gte": from, "$lte": to}}
	if spiritName != "" {
		matchFilter["spirit.name"] = spiritName
	}

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "date", Value: bson.D{{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$timestamp"},
					{Key: "unit", Value: unit},
				}}}},
				{Key: "spirit", Value: "$spirit.name"},
			}},
			{Key: "prompt_tokens", Value: bson.D{{Key: "$sum", Value: "$token_usage.total.prompt_tokens"}}},
			{Key: "completion_tokens", Value: bson.D{{Key: "$sum", Value: "$token_usage.total.completion_tokens"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id.date", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type tokenResult struct {
		ID struct {
			Date   time.Time `bson:"date"`
			Spirit string    `bson:"spirit"`
		} `bson:"_id"`
		PromptTokens     int `bson:"prompt_tokens"`
		CompletionTokens int `bson:"completion_tokens"`
	}

	var raw []tokenResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]eywa.TokenSeries, len(raw))
	for i, r := range raw {
		result[i] = eywa.TokenSeries{
			Date:             r.ID.Date,
			SpiritName:       r.ID.Spirit,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
		}
	}
	return result, nil
}

func (r *ChronicleRepository) AggregateActions(ctx context.Context, spiritName string, from, to time.Time) ([]eywa.ActionStats, error) {
	matchFilter := bson.M{"timestamp": bson.M{"$gte": from, "$lte": to}}
	if spiritName != "" {
		matchFilter["spirit.name"] = spiritName
	}

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$unwind", Value: "$processing.iterations"}},
		{{Key: "$unwind", Value: "$processing.iterations.action_calls"}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$processing.iterations.action_calls.action_name"},
			{Key: "call_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "error_count", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				"$processing.iterations.action_calls.is_error", 1, 0,
			}}}}}},
			{Key: "avg_latency", Value: bson.D{{Key: "$avg", Value: "$processing.iterations.action_calls.duration_ms"}}},
			{Key: "durations", Value: bson.D{{Key: "$push", Value: "$processing.iterations.action_calls.duration_ms"}}},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "sorted_durations", Value: bson.D{{Key: "$sortArray", Value: bson.D{
				{Key: "input", Value: "$durations"},
				{Key: "sortBy", Value: 1},
			}}}},
			{Key: "dur_count", Value: bson.D{{Key: "$size", Value: "$durations"}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "action_name", Value: "$_id"},
			{Key: "call_count", Value: 1},
			{Key: "error_count", Value: 1},
			{Key: "avg_latency_ms", Value: "$avg_latency"},
			{Key: "p95_latency_ms", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{
				"$sorted_durations",
				bson.D{{Key: "$floor", Value: bson.D{{Key: "$multiply", Value: bson.A{0.95, "$dur_count"}}}}},
			}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "call_count", Value: -1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type actionResult struct {
		ActionName   string  `bson:"action_name"`
		CallCount    int     `bson:"call_count"`
		ErrorCount   int     `bson:"error_count"`
		AvgLatencyMs float64 `bson:"avg_latency_ms"`
		P95LatencyMs float64 `bson:"p95_latency_ms"`
	}

	var raw []actionResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]eywa.ActionStats, len(raw))
	for i, r := range raw {
		result[i] = eywa.ActionStats{
			ActionName:   r.ActionName,
			CallCount:    r.CallCount,
			ErrorCount:   r.ErrorCount,
			AvgLatencyMs: r.AvgLatencyMs,
			P95LatencyMs: r.P95LatencyMs,
		}
	}
	return result, nil
}

func (r *ChronicleRepository) AggregateSpirits(ctx context.Context, from, to time.Time) ([]eywa.SpiritStats, error) {
	matchFilter := bson.M{"timestamp": bson.M{"$gte": from, "$lte": to}}

	pipeline := mongodriver.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$spirit.name"},
			{Key: "total_count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "error_count", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$ne", Value: bson.A{"$processing.status", "success"}}}, 1, 0,
			}}}}}},
			{Key: "avg_iterations", Value: bson.D{{Key: "$avg", Value: "$processing.iterations_used"}}},
			{Key: "avg_duration", Value: bson.D{{Key: "$avg", Value: "$processing.processing_time_ms"}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "spirit_name", Value: "$_id"},
			{Key: "avg_iterations", Value: 1},
			{Key: "error_rate", Value: bson.D{{Key: "$divide", Value: bson.A{"$error_count", "$total_count"}}}},
			{Key: "avg_duration_ms", Value: "$avg_duration"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "spirit_name", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type spiritResult struct {
		SpiritName    string  `bson:"spirit_name"`
		AvgIterations float64 `bson:"avg_iterations"`
		ErrorRate     float64 `bson:"error_rate"`
		AvgDurationMs float64 `bson:"avg_duration_ms"`
	}

	var raw []spiritResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, err
	}

	result := make([]eywa.SpiritStats, len(raw))
	for i, r := range raw {
		result[i] = eywa.SpiritStats{
			SpiritName:    r.SpiritName,
			AvgIterations: r.AvgIterations,
			ErrorRate:     r.ErrorRate,
			AvgDurationMs: r.AvgDurationMs,
		}
	}
	return result, nil
}

func buildChronicleListFilter(opts eywa.ChronicleListOptions) bson.M {
	filter := bson.M{}
	if opts.SpiritName != "" {
		filter["spirit.name"] = opts.SpiritName
	}
	if opts.MemoryKey != "" {
		filter["memory_key"] = opts.MemoryKey
	}
	if opts.HasError {
		filter["processing.status"] = bson.M{"$ne": "success"}
	}
	if opts.MinIterations > 0 {
		filter["processing.iterations_used"] = bson.M{"$gte": opts.MinIterations}
	}
	timeFilter := bson.M{}
	if opts.DateFrom != nil {
		timeFilter["$gte"] = *opts.DateFrom
	}
	if opts.DateTo != nil {
		timeFilter["$lte"] = *opts.DateTo
	}
	if len(timeFilter) > 0 {
		filter["timestamp"] = timeFilter
	}
	return filter
}
