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

// Example 07: Human Takeover (Vigil)
//
// Demonstrates Vigil — Eywa's human takeover mechanism.
//
// Key concepts:
// - VigilRepository stores the active operator seat in Redis with a TTL
// - When an operator holds a seat, Pulses are blocked from reaching the Oracle
// - The operator communicates directly via the Echo API (not through the Weave)
// - Releasing the seat returns the conversation to the AI
// - VigilConfig.InactivityTimeout controls the seat TTL

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Human Takeover (Vigil) ===")

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

	// VigilRepository uses Redis to store the active seat with a TTL.
	// If the operator goes idle for InactivityTimeout, the seat is released automatically.
	vigilRepo := eywaredis.NewVigilRepository(redisConn.GetClient(), serviceName, environment)

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithVigilRepository(vigilRepo).
		WithVigilConfig(eywa.VigilConfig{InactivityTimeout: 30 * time.Minute}).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	spirit := &eywa.Spirit{
		Name:         "support_spirit",
		Description:  "Customer support spirit",
		SystemPrompt: "You are a helpful support agent. Answer customer questions concisely.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   300,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists\n")
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("user_message").
			WithDefaultSpirit("support_spirit").
			Build(),
	)

	memoryKey := eywa.MemoryKey{Channel: "api", User: "customer_001"}
	operatorID := "operator_alice"
	takeover := 30 * time.Minute

	// ── Phase 1: AI handles the conversation ─────────────────────────────────

	fmt.Println("\n--- Phase 1: AI mode ---")
	send(ctx, weave, memoryKey, "Hello! I have a question about your product.")

	// ── Phase 2: Operator takes over ─────────────────────────────────────────

	fmt.Println("\n--- Phase 2: Operator takes seat ---")
	if err := vigilRepo.Acquire(ctx, memoryKey.String(), operatorID, takeover); err != nil {
		log.Fatalf("acquire seat: %v", err)
	}

	seat, _ := vigilRepo.Get(ctx, memoryKey.String())
	fmt.Printf("Seat acquired by: %s (expires: %s)\n", seat.OperatorID, seat.ExpiresAt.Format(time.RFC3339))

	// Any Pulse sent while a seat is held returns without reaching the Oracle.
	// The response indicates the session is in human-controlled mode.
	fmt.Println("Sending message while seat is held...")
	send(ctx, weave, memoryKey, "Can you help me with billing?")

	// The operator communicates directly via the Echo API (not through Weave).
	// In production this happens via POST /vigil/:memoryKey/echoes on the management API.
	fmt.Println("Operator sends direct message via Echo API (out of band)")

	// ── Phase 3: Operator releases seat — AI resumes ─────────────────────────

	fmt.Println("\n--- Phase 3: Release seat — AI resumes ---")
	if err := vigilRepo.Release(ctx, memoryKey.String()); err != nil {
		log.Fatalf("release seat: %v", err)
	}
	fmt.Println("Seat released")

	send(ctx, weave, memoryKey, "Thanks for the help! One more question: how do I cancel?")
}

func send(ctx context.Context, weave *eywa.Weave, key eywa.MemoryKey, msg string) {
	pulse := eywa.NewPulse(key).
		WithUserMessage(msg).
		WithSource("api").
		Build()

	result, err := weave.ProcessEventByKey(ctx, "user_message", pulse)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	fmt.Printf("Status: %s | Reply: %s\n", result.Status, result.Message)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
