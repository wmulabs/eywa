package mongo

import (
	"context"
	"fmt"
	"time"

	eywa "github.com/wmulabs/eywa"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var _ eywa.CheckpointStore = (*CheckpointStore)(nil)

// defaultCheckpointTTL bounds how long an orphaned checkpoint (interrupted turn never retried) lingers
// before the TTL monitor reclaims it.
const defaultCheckpointTTL = 24 * time.Hour

// CheckpointStore implements eywa.CheckpointStore using MongoDB, persisting reasoning-turn snapshots so
// an interrupted turn can resume. Each turn is one document keyed by turnID; a TTL index on expires_at
// evicts orphaned checkpoints.
type CheckpointStore struct {
	collection *mongodriver.Collection
	ttl        time.Duration
	logger     *zap.SugaredLogger
}

type checkpointDocument struct {
	ID        string    `bson:"_id"`
	Data      []byte    `bson:"data"`
	ExpiresAt time.Time `bson:"expires_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// NewCheckpointStore creates a MongoDB-backed CheckpointStore with the default 24h checkpoint TTL.
func NewCheckpointStore(database *mongodriver.Database) *CheckpointStore {
	return NewCheckpointStoreWithTTL(database, defaultCheckpointTTL)
}

// NewCheckpointStoreWithTTL creates a MongoDB-backed CheckpointStore with a custom TTL for orphan
// cleanup. A non-positive ttl falls back to the default.
func NewCheckpointStoreWithTTL(database *mongodriver.Database, ttl time.Duration) *CheckpointStore {
	if ttl <= 0 {
		ttl = defaultCheckpointTTL
	}
	store := &CheckpointStore{
		collection: database.Collection("checkpoints"),
		ttl:        ttl,
		logger:     newLogger(),
	}
	store.ensureIndexes()
	return store
}

func (s *CheckpointStore) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// expireAfterSeconds(0) makes Mongo evict each document at the wall-clock time stored in expires_at.
	index := mongodriver.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetName("idx_expires_at_ttl").SetExpireAfterSeconds(0),
	}
	if _, err := s.collection.Indexes().CreateOne(ctx, index); err != nil {
		s.logger.Warnw("failed to create checkpoints TTL index", "error", err)
	}
}

func (s *CheckpointStore) Save(ctx context.Context, turnID string, data []byte) error {
	now := time.Now().UTC()
	doc := checkpointDocument{
		ID:        turnID,
		Data:      data,
		ExpiresAt: now.Add(s.ttl),
		UpdatedAt: now,
	}
	_, err := s.collection.ReplaceOne(ctx, bson.M{"_id": turnID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("checkpoint save: %w", err)
	}
	return nil
}

func (s *CheckpointStore) Load(ctx context.Context, turnID string) ([]byte, bool, error) {
	var doc checkpointDocument
	err := s.collection.FindOne(ctx, bson.M{"_id": turnID}).Decode(&doc)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("checkpoint load: %w", err)
	}
	return doc.Data, true, nil
}

func (s *CheckpointStore) Delete(ctx context.Context, turnID string) error {
	// A missing document is not an error: Delete is idempotent per the CheckpointStore contract.
	if _, err := s.collection.DeleteOne(ctx, bson.M{"_id": turnID}); err != nil {
		return fmt.Errorf("checkpoint delete: %w", err)
	}
	return nil
}
