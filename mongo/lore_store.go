package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	eywa "github.com/wmulabs/eywa"
)

// LoreStore implements LoreStore via MongoDB Atlas $vectorSearch on the lore_chunks collection.
// Requires an Atlas Vector Search index named "lore_embedding_index" on the embedding field.
type LoreStore struct {
	collection *mongo.Collection
	logger     *zap.SugaredLogger
}

func NewLoreStore(db *mongo.Database) eywa.LoreStore {
	return &LoreStore{
		collection: db.Collection("lore_chunks"),
		logger:     newLogger(),
	}
}

func (s *LoreStore) Upsert(ctx context.Context, chunks []eywa.LoreChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	now := time.Now().UTC()
	models := make([]mongo.WriteModel, len(chunks))
	for i, chunk := range chunks {
		chunk.CreatedAt = now
		models[i] = mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": chunk.ID}).
			SetReplacement(chunk).
			SetUpsert(true)
	}

	_, err := s.collection.BulkWrite(ctx, models)
	if err != nil {
		return fmt.Errorf("upsert lore chunks: %w", err)
	}
	return nil
}

func (s *LoreStore) Search(ctx context.Context, loreID string, queryVec []float32, topK int, minScore float64) ([]eywa.LoreChunk, error) {
	if topK <= 0 {
		topK = 5
	}

	vecSlice := make([]any, len(queryVec))
	for i, v := range queryVec {
		vecSlice[i] = v
	}

	pipeline := mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: "lore_embedding_index"},
			{Key: "path", Value: "embedding"},
			{Key: "queryVector", Value: vecSlice},
			{Key: "numCandidates", Value: topK * 10},
			{Key: "limit", Value: topK},
			{Key: "filter", Value: bson.D{{Key: "lore_id", Value: loreID}}},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "vectorSearchScore"}}},
		}}},
	}

	if minScore > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{
			{Key: "score", Value: bson.D{{Key: "$gte", Value: minScore}}},
		}}})
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var chunks []eywa.LoreChunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, fmt.Errorf("decode lore chunks: %w", err)
	}
	return chunks, nil
}

func (s *LoreStore) Delete(ctx context.Context, loreID string) error {
	_, err := s.collection.DeleteMany(ctx, bson.M{"lore_id": loreID})
	if err != nil {
		return fmt.Errorf("delete lore chunks: %w", err)
	}
	return nil
}
