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

var _ eywa.AppTokenRepository = (*AppTokenRepository)(nil)

// AppTokenRepository implements eywa.AppTokenRepository using MongoDB. Each token is one document
// keyed by its ID; the secret hash is uniquely indexed for validation lookups, and a TTL index on
// expires_at evicts expired tokens automatically (tokens with no expiry are never evicted).
type AppTokenRepository struct {
	collection *mongodriver.Collection
	logger     *zap.SugaredLogger
}

func NewAppTokenRepository(database *mongodriver.Database) *AppTokenRepository {
	repo := &AppTokenRepository{
		collection: database.Collection("app_tokens"),
		logger:     newLogger(),
	}
	repo.ensureIndexes()
	return repo
}

func (r *AppTokenRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongodriver.IndexModel{
		{
			Keys:    bson.D{{Key: "hash", Value: 1}},
			Options: options.Index().SetName("idx_hash_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetName("idx_expires_at_ttl").SetExpireAfterSeconds(0),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create app_tokens indexes", "error", err)
	}
}

func (r *AppTokenRepository) Create(ctx context.Context, token *eywa.AppToken) error {
	if _, err := r.collection.InsertOne(ctx, token); err != nil {
		return fmt.Errorf("create app token: %w", err)
	}
	return nil
}

func (r *AppTokenRepository) FindByHash(ctx context.Context, hash []byte) (*eywa.AppToken, error) {
	var token eywa.AppToken
	err := r.collection.FindOne(ctx, bson.M{"hash": hash}).Decode(&token)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, eywa.ErrNotFound
		}
		return nil, fmt.Errorf("find app token: %w", err)
	}
	return &token, nil
}

func (r *AppTokenRepository) List(ctx context.Context) ([]*eywa.AppToken, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("list app tokens: %w", err)
	}
	defer cursor.Close(ctx) //nolint:errcheck

	var tokens []*eywa.AppToken
	if err := cursor.All(ctx, &tokens); err != nil {
		return nil, fmt.Errorf("decode app tokens: %w", err)
	}
	return tokens, nil
}

func (r *AppTokenRepository) Revoke(ctx context.Context, id string) error {
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"revoked_at": time.Now().UTC()}})
	if err != nil {
		return fmt.Errorf("revoke app token: %w", err)
	}
	if result.MatchedCount == 0 {
		return eywa.ErrNotFound
	}
	return nil
}
