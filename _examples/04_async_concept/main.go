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

// Example 04: Sync vs Async Processing
//
// SYNC (this example):
//   - Client calls ProcessEventByKey → Weave processes immediately → returns Response
//   - Good for dev, testing, and low-volume use cases.
//
// ASYNC (production pattern):
//   - Client sends webhook → POST /api/v1/events/:event_key/async
//   - Weave dispatches to Keeper (Cloud Tasks) → returns 200 OK in < 100ms
//   - Keeper fires POST /internal/execute-event → Weave processes in the background
//   - Automatic retry, deduplication, scales to high-volume webhooks
//
// This example demonstrates SYNC processing of multiple Pulses concurrently
// via ProcessMultipleEventsByKey.

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Sync vs Async Processing ===")

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

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		AddOracle(eywaopenai.NewOracle(apiKey)).
		Build()
	if err != nil {
		log.Fatalf("failed to build Weave: %v", err)
	}

	// Spirit
	spirit := &eywa.Spirit{
		Name:         "demo_assistant",
		Description:  "Demo assistant — answers questions concisely",
		SystemPrompt: "You are a helpful AI assistant. Answer each question concisely in one sentence.",
		ModelConfig:  eywa.SpiritModel{Provider: "openai", Model: "gpt-4o-mini", Temperature: 0.7, MaxTokens: 100},
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists: %v\n", err)
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("demo_message").
			WithDefaultSpirit("demo_assistant").
			Build(),
	)

	// Three independent Pulses (different MemoryKeys = different users)
	// ProcessMultipleEventsByKey processes them concurrently via goroutines.
	pulses := []*eywa.Pulse{
		eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_001"}).
			WithUserMessage("What is 5 + 7?").
			WithSource("webhook").
			Build(),
		eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_002"}).
			WithUserMessage("What is the capital of Brazil?").
			WithSource("webhook").
			Build(),
		eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_003"}).
			WithUserMessage("Who wrote Hamlet?").
			WithSource("webhook").
			Build(),
	}

	fmt.Println("\nProcessing 3 Pulses concurrently (SYNC mode)...")
	fmt.Println("Caller is waiting for all results...")

	start := time.Now()
	results, err := weave.ProcessMultipleEventsByKey(ctx, "demo_message", pulses)
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("processing failed: %v", err)
	}

	successCount := 0
	for _, r := range results {
		if r.Status == eywa.ResponseSuccess {
			successCount++
		}
	}

	fmt.Printf("\nCompleted in %v — %d/%d succeeded\n", elapsed, successCount, len(results))
	for i, r := range results {
		fmt.Printf("\n[%d] %s\n    → %s\n", i+1, pulses[i].UserMessage, truncate(r.Message, 100))
	}

	fmt.Println("\n─────────────────────────────────────────")
	fmt.Println("SYNC  — client waits, immediate results")
	fmt.Println("        good for: dev, testing, low-volume")
	fmt.Println()
	fmt.Println("ASYNC — POST /api/v1/events/:key/async")
	fmt.Println("        returns 200 OK in < 100ms")
	fmt.Println("        Keeper (Cloud Tasks) fires processing later")
	fmt.Println("        good for: production, high-volume webhooks")
	fmt.Println("        built-in: retry, deduplication, OIDC auth")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
