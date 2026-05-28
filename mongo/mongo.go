package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

type MongoConnection struct {
	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database
}

func NewMongoConnection(ctx context.Context, mongoUrl string, database string, appName string) (*MongoConnection, error) {
	debugger := newLogger()
	debugger.Infow("MongoDB is starting...")

	clientOptions := options.Client()
	clientOptions.SetMonitor(otelmongo.NewMonitor())
	clientOptions.ApplyURI(mongoUrl)
	clientOptions.SetAppName(appName)
	clientOptions.SetCompressors([]string{"zstd", "zlib"})
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("create mongodb client: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	debugger.Infow("MongoDB is connected")
	return &MongoConnection{
		mongoClient:   client,
		mongoDatabase: client.Database(database),
	}, nil
}

func (m *MongoConnection) GetDatabase() *mongo.Database {
	return m.mongoDatabase
}

func (m *MongoConnection) DisconnectMongoDB(ctx context.Context) {
	debugger := newLogger()
	if m.mongoClient == nil {
		debugger.Errorw("DisconnectMongoDB called with nil client")
		return
	}

	if err := m.mongoClient.Disconnect(ctx); err != nil {
		debugger.Errorw("Error disconnecting from MongoDB", "error", err)
		return
	}

	debugger.Infow("MongoDB is disconnected")
}
