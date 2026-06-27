package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	eywa "github.com/wmulabs/eywa"
)

type handoffDoc struct {
	SessionKey string    `bson:"session_key"`
	SpiritName string    `bson:"spirit_name"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

// HandoffStore implements eywa.HandoffStore using a MongoDB collection keyed by session. It records
// which Spirit owns a conversation after a handoff so subsequent Pulses route to it across instances.
type HandoffStore struct {
	col    *mongo.Collection
	logger *zap.SugaredLogger
}

func NewHandoffStore(db *mongo.Database) eywa.HandoffStore {
	store := &HandoffStore{
		col:    db.Collection("handoffs"),
		logger: newLogger(),
	}
	store.ensureIndexes()
	return store
}

func (s *HandoffStore) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := s.col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "session_key", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		s.logger.Warnw("failed to create handoffs index", "error", err)
	}
}

func (s *HandoffStore) GetActiveSpirit(ctx context.Context, sessionKey string) (string, error) {
	var doc handoffDoc
	err := s.col.FindOne(ctx, bson.M{"session_key": sessionKey}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("handoff find: %w", err)
	}
	return doc.SpiritName, nil
}

func (s *HandoffStore) SetActiveSpirit(ctx context.Context, sessionKey, spiritName string) error {
	_, err := s.col.UpdateOne(ctx,
		bson.M{"session_key": sessionKey},
		bson.M{"$set": bson.M{"spirit_name": spiritName, "updated_at": time.Now().UTC()}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("handoff upsert: %w", err)
	}
	return nil
}

func (s *HandoffStore) ClearActiveSpirit(ctx context.Context, sessionKey string) error {
	if _, err := s.col.DeleteOne(ctx, bson.M{"session_key": sessionKey}); err != nil {
		return fmt.Errorf("handoff delete: %w", err)
	}
	return nil
}
