package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	eywa "github.com/wmulabs/eywa"
)

// Versioned Spirit storage — each Update inserts a new version rather than overwriting.
type SpiritRepository struct {
	collection *mongo.Collection
	db         *mongo.Database
	logger     *zap.SugaredLogger
}

func NewSpiritRepository(db *mongo.Database) eywa.SpiritRepository {
	repo := &SpiritRepository{
		collection: db.Collection("spirits"),
		db:         db,
		logger:     newLogger(),
	}

	repo.ensureIndexes()

	return repo
}

func (r *SpiritRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := r.getIndexModels()

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warnw("failed to create indexes for spirits collection", "error", err)
	} else {
		r.logger.Infow("indexes created successfully for spirits collection", "count", len(indexes))
	}
}

func (r *SpiritRepository) getIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			// Optimizes: FindActiveByName, FindActiveByNames, ListActive
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "is_active", Value: 1},
				{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().
				SetName("idx_name_active_deleted"),
		},
		{
			// Optimizes: GetVersion, FindVersionHistory
			// Ensures: No duplicate versions for the same Spirit
			Keys: bson.D{
				{Key: "name", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().
				SetName("idx_name_version").
				SetUnique(true),
		},
		{
			// Optimizes: ListActive
			Keys: bson.D{
				{Key: "is_active", Value: 1},
				{Key: "is_deleted", Value: 1},
			},
			Options: options.Index().
				SetName("idx_active_deleted"),
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName("idx_created_at"),
		},
	}
}

func (r *SpiritRepository) Create(ctx context.Context, spirit *eywa.Spirit) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/Create")
	defer span.End()

	if spirit == nil || spirit.Name == "" {
		r.logger.Errorw("invalid spirit: name is required")
		return fmt.Errorf("invalid spirit: name is required")
	}

	existing, err := r.FindActiveByName(ctx, spirit.Name)
	if err == nil && existing != nil {
		r.logger.Warnw("spirit already exists", "name", spirit.Name)
		return &eywa.ConflictError{Entity: "spirit", ID: spirit.Name}
	}

	spirit.Version = 1
	spirit.IsActive = true
	spirit.IsDeleted = false
	spirit.CreatedAt = eywa.NowUTC()

	result, err := r.collection.InsertOne(ctx, spirit)
	if err != nil {
		r.logger.Errorw("failed to create spirit", "error", err, "name", spirit.Name)
		return fmt.Errorf("failed to create spirit: %w", err)
	}

	r.logger.Infow("spirit created", "id", result.InsertedID, "name", spirit.Name)
	return nil
}

// Update deactivates the current active version and inserts a new one with an incremented version number.
func (r *SpiritRepository) Update(ctx context.Context, spiritName string, spirit *eywa.Spirit, changeLog string) (*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/Update")
	defer span.End()

	if spiritName == "" || spirit == nil {
		r.logger.Errorw("invalid input: spiritName and spirit are required")
		return nil, fmt.Errorf("invalid input: spiritName and spirit are required")
	}

	current, err := r.FindActiveByName(ctx, spiritName)
	if err != nil {
		r.logger.Errorw("spirit not found", "name", spiritName, "error", err)
		return nil, fmt.Errorf("spirit not found: %w", err)
	}

	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"name": spiritName, "is_active": true},
		bson.M{"$set": bson.M{"is_active": false}},
	)
	if err != nil {
		r.logger.Errorw("failed to deactivate current version", "name", spiritName, "error", err)
		return nil, fmt.Errorf("failed to deactivate current version: %w", err)
	}

	newVersion := &eywa.Spirit{
		Name:           spiritName,
		Version:        current.Version + 1,
		IsActive:       true,
		IsDeleted:      false,
		Description:    spirit.Description,
		Specialization: spirit.Specialization,
		SystemPrompt:   spirit.SystemPrompt,
		AllowedActions: spirit.AllowedActions,
		ModelConfig:    spirit.ModelConfig,
		Metadata:       spirit.Metadata,
		CreatedAt:      eywa.NowUTC(),
		CreatedBy:      spirit.CreatedBy,
		ChangeLog:      changeLog,
	}

	result, err := r.collection.InsertOne(ctx, newVersion)
	if err != nil {
		r.logger.Errorw("failed to create new version", "error", err, "name", spiritName)
		return nil, fmt.Errorf("failed to create new version: %w", err)
	}

	r.logger.Infow("spirit updated", "id", result.InsertedID, "name", spiritName, "version", newVersion.Version)
	return newVersion, nil
}

func (r *SpiritRepository) FindActiveByName(ctx context.Context, spiritName string) (*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/FindActiveByName")
	defer span.End()

	if spiritName == "" {
		r.logger.Errorw("invalid input: spiritName is required")
		return nil, fmt.Errorf("invalid input: spiritName is required")
	}

	var spirit eywa.Spirit

	err := r.collection.FindOne(
		ctx,
		bson.M{
			"name":       spiritName,
			"is_active":  true,
			"is_deleted": false,
		},
	).Decode(&spirit)

	if err == mongo.ErrNoDocuments {
		r.logger.Debugw("active spirit not found", "name", spiritName)
		return nil, &eywa.NotFoundError{Entity: "spirit", ID: spiritName}
	}

	if err != nil {
		r.logger.Errorw("failed to find spirit", "name", spiritName, "error", err)
		return nil, fmt.Errorf("failed to find spirit: %w", err)
	}

	return &spirit, nil
}

// FindActiveByNames fetches multiple Spirits in a single query — avoids N+1 round trips.
func (r *SpiritRepository) FindActiveByNames(ctx context.Context, spiritNames []string) (map[string]*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/FindActiveByNames")
	defer span.End()

	if len(spiritNames) == 0 {
		r.logger.Debugw("empty spirit names list provided")
		return make(map[string]*eywa.Spirit), nil
	}

	cursor, err := r.collection.Find(
		ctx,
		bson.M{
			"name":       bson.M{"$in": spiritNames},
			"is_active":  true,
			"is_deleted": false,
		},
	)
	if err != nil {
		r.logger.Errorw("failed to find spirits by names", "names", spiritNames, "error", err)
		return nil, fmt.Errorf("failed to find spirits by names: %w", err)
	}
	defer cursor.Close(ctx)

	spiritMap := make(map[string]*eywa.Spirit)
	for cursor.Next(ctx) {
		var spirit eywa.Spirit
		if err := cursor.Decode(&spirit); err != nil {
			r.logger.Warnw("failed to decode spirit", "error", err)
			continue
		}
		spiritMap[spirit.Name] = &spirit
	}

	if err := cursor.Err(); err != nil {
		r.logger.Errorw("cursor error", "error", err)
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	r.logger.Debugw("found spirits by names",
		"requested", len(spiritNames),
		"found", len(spiritMap))

	return spiritMap, nil
}

func (r *SpiritRepository) FindByID(ctx context.Context, id string) (*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/FindByID")
	defer span.End()

	if id == "" {
		r.logger.Errorw("invalid input: id is required")
		return nil, fmt.Errorf("invalid input: id is required")
	}

	var spirit eywa.Spirit

	err := r.collection.FindOne(
		ctx,
		bson.M{"_id": id},
	).Decode(&spirit)

	if err == mongo.ErrNoDocuments {
		r.logger.Debugw("spirit not found", "id", id)
		return nil, &eywa.NotFoundError{Entity: "spirit", ID: id}
	}

	if err != nil {
		r.logger.Errorw("failed to find spirit", "id", id, "error", err)
		return nil, fmt.Errorf("failed to find spirit: %w", err)
	}

	return &spirit, nil
}

func (r *SpiritRepository) ListActive(ctx context.Context, limit, offset int) ([]*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/ListActive")
	defer span.End()

	opts := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(
		ctx,
		bson.M{
			"is_active":  true,
			"is_deleted": false,
		},
		opts,
	)
	if err != nil {
		r.logger.Errorw("failed to list active spirits", "error", err)
		return nil, fmt.Errorf("failed to list active spirits: %w", err)
	}
	defer cursor.Close(ctx)

	var spirits []*eywa.Spirit
	if err := cursor.All(ctx, &spirits); err != nil {
		r.logger.Errorw("failed to decode spirits", "error", err)
		return nil, fmt.Errorf("failed to decode spirits: %w", err)
	}

	r.logger.Infow("listed active spirits", "count", len(spirits), "limit", limit, "offset", offset)
	return spirits, nil
}

func (r *SpiritRepository) CountActive(ctx context.Context) (int64, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/CountActive")
	defer span.End()

	count, err := r.collection.CountDocuments(ctx, bson.M{
		"is_active":  true,
		"is_deleted": false,
	})
	if err != nil {
		r.logger.Errorw("failed to count active spirits", "error", err)
		return 0, fmt.Errorf("failed to count active spirits: %w", err)
	}
	return count, nil
}

func (r *SpiritRepository) ListAll(ctx context.Context) ([]*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/ListAll")
	defer span.End()

	cursor, err := r.collection.Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}, {Key: "version", Value: -1}}),
	)
	if err != nil {
		r.logger.Errorw("failed to list all spirits", "error", err)
		return nil, fmt.Errorf("failed to list all spirits: %w", err)
	}
	defer cursor.Close(ctx)

	var spirits []*eywa.Spirit
	if err := cursor.All(ctx, &spirits); err != nil {
		r.logger.Errorw("failed to decode spirits", "error", err)
		return nil, fmt.Errorf("failed to decode spirits: %w", err)
	}

	r.logger.Infow("listed all spirits", "count", len(spirits))
	return spirits, nil
}

// Activate sets only the requested version to active — deactivates all other versions of the same Spirit.
// Both operations run inside a session transaction to prevent a window where no version is active.
func (r *SpiritRepository) Activate(ctx context.Context, spiritName string, version int) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/Activate")
	defer span.End()

	if spiritName == "" || version < 1 {
		r.logger.Errorw("invalid input: spiritName and version are required", "name", spiritName, "version", version)
		return fmt.Errorf("invalid input: spiritName and version are required")
	}

	session, err := r.db.Client().StartSession()
	if err != nil {
		r.logger.Errorw("failed to start session for activate", "name", spiritName, "version", version, "error", err)
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		_, txErr := r.collection.UpdateMany(
			sc,
			bson.M{"name": spiritName},
			bson.M{"$set": bson.M{"is_active": false}},
		)
		if txErr != nil {
			return nil, fmt.Errorf("failed to deactivate versions: %w", txErr)
		}

		result, txErr := r.collection.UpdateOne(
			sc,
			bson.M{"name": spiritName, "version": version},
			bson.M{"$set": bson.M{"is_active": true}},
		)
		if txErr != nil {
			return nil, fmt.Errorf("failed to activate version: %w", txErr)
		}

		if result.MatchedCount == 0 {
			return nil, &eywa.NotFoundError{Entity: "spirit version", ID: fmt.Sprintf("%s v%d", spiritName, version)}
		}

		return nil, nil
	})
	if err != nil {
		r.logger.Errorw("failed to activate spirit", "name", spiritName, "version", version, "error", err)
		return err
	}

	r.logger.Infow("spirit activated", "name", spiritName, "version", version)
	return nil
}

func (r *SpiritRepository) Deactivate(ctx context.Context, id string) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/Deactivate")
	defer span.End()

	if id == "" {
		r.logger.Errorw("invalid input: id is required")
		return fmt.Errorf("invalid input: id is required")
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"is_active": false, "deactivated_at": eywa.NowUTC()}},
	)

	if err != nil {
		r.logger.Errorw("failed to deactivate spirit", "id", id, "error", err)
		return fmt.Errorf("failed to deactivate spirit: %w", err)
	}

	if result.ModifiedCount == 0 {
		r.logger.Warnw("spirit not found or already deactivated", "id", id)
	}

	r.logger.Infow("spirit deactivated", "id", id, "modified_count", result.ModifiedCount)
	return nil
}

func (r *SpiritRepository) GetVersion(ctx context.Context, spiritName string, version int) (*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/GetVersion")
	defer span.End()

	if spiritName == "" || version < 1 {
		r.logger.Errorw("invalid input: spiritName and version are required", "name", spiritName, "version", version)
		return nil, fmt.Errorf("invalid input: spiritName and version are required")
	}

	var spirit eywa.Spirit

	err := r.collection.FindOne(
		ctx,
		bson.M{
			"name":    spiritName,
			"version": version,
		},
	).Decode(&spirit)

	if err == mongo.ErrNoDocuments {
		r.logger.Debugw("spirit version not found", "name", spiritName, "version", version)
		return nil, &eywa.NotFoundError{Entity: "spirit version", ID: fmt.Sprintf("%s v%d", spiritName, version)}
	}

	if err != nil {
		r.logger.Errorw("failed to get spirit version", "name", spiritName, "version", version, "error", err)
		return nil, fmt.Errorf("failed to get spirit version: %w", err)
	}

	return &spirit, nil
}

func (r *SpiritRepository) FindVersionHistory(ctx context.Context, spiritName string) ([]*eywa.Spirit, error) {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/FindVersionHistory")
	defer span.End()

	if spiritName == "" {
		r.logger.Errorw("invalid input: spiritName is required")
		return nil, fmt.Errorf("invalid input: spiritName is required")
	}

	cursor, err := r.collection.Find(
		ctx,
		bson.M{"name": spiritName},
		options.Find().SetSort(bson.M{"version": -1}),
	)
	if err != nil {
		r.logger.Errorw("failed to find version history", "name", spiritName, "error", err)
		return nil, fmt.Errorf("failed to find version history: %w", err)
	}
	defer cursor.Close(ctx)

	var spirits []*eywa.Spirit
	if err := cursor.All(ctx, &spirits); err != nil {
		r.logger.Errorw("failed to decode spirits", "name", spiritName, "error", err)
		return nil, fmt.Errorf("failed to decode spirits: %w", err)
	}

	r.logger.Infow("found version history", "name", spiritName, "count", len(spirits))
	return spirits, nil
}

// RestoreVersion promotes a specific version to active, deactivating all other versions of the same Spirit.
// Both operations run inside a session transaction to prevent a window where no version is active.
func (r *SpiritRepository) RestoreVersion(ctx context.Context, id string) error {
	ctx, span := trace.NewNoopTracerProvider().Tracer("eywa").Start(ctx, "SpiritRepository/RestoreVersion")
	defer span.End()

	if id == "" {
		r.logger.Errorw("invalid input: id is required")
		return fmt.Errorf("invalid input: id is required")
	}

	spirit, err := r.FindByID(ctx, id)
	if err != nil {
		r.logger.Errorw("spirit not found", "id", id, "error", err)
		return err
	}

	session, err := r.db.Client().StartSession()
	if err != nil {
		r.logger.Errorw("failed to start session for restore", "id", id, "error", err)
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		if _, txErr := r.collection.UpdateMany(
			sc,
			bson.M{"name": spirit.Name},
			bson.M{"$set": bson.M{"is_active": false}},
		); txErr != nil {
			return nil, fmt.Errorf("failed to deactivate current versions: %w", txErr)
		}

		result, txErr := r.collection.UpdateOne(
			sc,
			bson.M{"_id": id},
			bson.M{
				"$set": bson.M{
					"is_active":      true,
					"deactivated_at": nil,
				},
			},
		)
		if txErr != nil {
			return nil, fmt.Errorf("failed to restore spirit version: %w", txErr)
		}

		if result.ModifiedCount == 0 {
			r.logger.Warnw("spirit not found or already active", "id", id)
		}

		return nil, nil
	})
	if err != nil {
		r.logger.Errorw("failed to restore spirit version", "id", id, "error", err)
		return err
	}

	r.logger.Infow("spirit version restored", "id", id, "name", spirit.Name, "version", spirit.Version)
	return nil
}
