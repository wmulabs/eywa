package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	eywa "github.com/wmulabs/eywa"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

// Example 01: Basic Setup
//
// Minimal setup to process a Pulse through Eywa's pipeline.
//
// What you'll learn:
// - Connect to MongoDB and Redis via opt-in sub-modules
// - Assemble a Weave with WeaveBuilder
// - Register a Spirit and a Link
// - Build a Pulse with the Pulse builder
// - Process a Pulse and inspect the Response

func main() {
	ctx := context.Background()

	fmt.Println("=== Eywa — Basic Setup ===")

	// Infrastructure
	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017")
	mongoDatabase := getEnv("MONGO_DATABASE", "eywa_example")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	environment := getEnv("ENVIRONMENT", "lcl")
	serviceName := getEnv("SERVICE_NAME", "eywa")

	mongoConn, err := eywamongo.NewMongoConnection(ctx, mongoURL, mongoDatabase, serviceName)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer mongoConn.DisconnectMongoDB(ctx)

	redisConn, err := eywaredis.NewRedisConnection(ctx, redisURL, serviceName)
	if err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	defer redisConn.DisconnectRedisDB(ctx)

	// Repositories
	spiritRepo := eywamongo.NewSpiritRepository(mongoConn.GetDatabase())
	memoryRepo := eywaredis.NewMemoryRepository(redisConn.GetClient(), serviceName, environment, 3600, nil)
	echoRepo := eywamongo.NewEchoRepository(mongoConn.GetDatabase())
	chronicleRepo := eywamongo.NewChronicleRepository(mongoConn.GetDatabase())
	bond := eywaredis.NewBondManager(redisConn.GetClient())

	// Weave
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	config := eywa.DefaultWeaveConfig()
	config.ReasoningTimeout = 60 * time.Second
	config.MaxReasoningIterations = 5
	config.MaxMemoryMessages = 10

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		AddOracle(eywaopenai.NewOracle(apiKey)).
		WithConfig(config).
		Build()
	if err != nil {
		log.Fatalf("failed to build Weave: %v", err)
	}

	fmt.Println("Weave ready")

	// Spirit
	spirit := &eywa.Spirit{
		Name:         "assistant",
		Description:  "A simple conversational assistant",
		SystemPrompt: "You are a helpful AI assistant built with Eywa. Answer questions concisely.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   500,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists: %v\n", err)
	}

	// Link: wire the "user_message" event key to the "assistant" Spirit
	weave.RegisterEventConfiguration(
		eywa.NewLink("user_message").
			WithDefaultSpirit("assistant").
			Build(),
	)

	// Pulse — use the builder, not a struct literal
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_001"}).
		WithUserMessage("What is Eywa?").
		WithSource("api").
		Build()

	result, err := weave.ProcessEventByKey(ctx, "user_message", pulse)
	if err != nil {
		log.Fatalf("processing failed: %v", err)
	}

	fmt.Printf("\nStatus:   %v\n", result.Status)
	fmt.Printf("Spirit:   %s\n", result.SpiritUsed)
	fmt.Printf("Time:     %dms\n", result.ProcessingTimeMs)
	fmt.Printf("Reply:    %s\n", result.Message)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
