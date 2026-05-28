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

// Example 09: Long-Term Memory (Imprint)
//
// Demonstrates Imprint — Eywa's per-user long-term memory system.
//
// Key concepts:
// - Imprint stores persistent facts about a user across sessions
// - ImprintExtraction auto-extracts facts from conversations using an LLM
// - RememberFactAction lets the Oracle explicitly store a fact
// - ForgetFactAction lets the Oracle remove a fact
// - ImprintScout automatically loads relevant facts into each Pulse context
// - Facts are scoped by user key and spirit ID

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Long-Term Memory (Imprint) ===")

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
	imprintRepo := eywamongo.NewImprintRepository(mongoConn.GetDatabase())

	spirit := &eywa.Spirit{
		Name:        "personal_assistant",
		Description: "Personal assistant that remembers user preferences",
		SystemPrompt: `You are a personal assistant that remembers important facts about the user.
Use remember_fact when the user shares personal information, preferences, or context you should recall later.
Use forget_fact when the user asks you to forget something.
Always personalize responses based on what you know about the user.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   400,
		},
		AllowedActions: []eywa.AllowedAction{
			{Name: "remember_fact"},
			{Name: "forget_fact"},
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	// RememberFact and ForgetFact are bound to a specific spirit.
	// Facts stored by this spirit are only retrieved for this spirit.
	actionRegistry := eywa.NewActionRegistry()
	actionRegistry.Register(eywa.NewRememberFactAction(imprintRepo, spirit.Name))
	actionRegistry.Register(eywa.NewForgetFactAction(imprintRepo, spirit.Name))

	// ImprintExtractionConfig enables automatic fact extraction after each turn.
	// The extraction LLM reads the conversation and identifies facts worth storing.
	extractionConfig := eywa.ImprintExtractionConfig{
		Enabled: true,
		Model: eywa.SpiritModel{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			MaxTokens: 200,
		},
		MaxImprints: 50,
	}

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(actionRegistry).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithImprintRepository(imprintRepo).
		WithImprintExtraction(extractionConfig).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists\n")
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("chat").
			WithDefaultSpirit("personal_assistant").
			Build(),
	)

	userKey := eywa.MemoryKey{Channel: "api", User: "user_alice"}

	// ── Session 1: User shares preferences ───────────────────────────────────

	fmt.Println("\n--- Session 1: User shares context ---")
	messages := []string{
		"Hi! I'm Alice, I'm a software engineer working on Go backends.",
		"I prefer concise answers — no bullet points, just direct responses.",
		"My timezone is UTC-3 (São Paulo).",
	}
	for _, msg := range messages {
		send(ctx, weave, userKey, msg)
	}

	// ── Inspect stored facts ──────────────────────────────────────────────────

	fmt.Println("\n--- Stored imprints ---")
	imprints, err := imprintRepo.GetByUserKey(ctx, userKey.String(), spirit.Name)
	if err != nil {
		log.Printf("find imprints: %v", err)
	} else {
		for _, imp := range imprints {
			fmt.Printf("  [%s] %s\n", imp.Category, imp.Fact)
		}
	}

	// ── Session 2: New session — facts recalled automatically ─────────────────
	// Clear Redis memory so Session 2 starts fresh (no short-term context).
	// Imprints persist in MongoDB and are injected by ImprintScout.

	fmt.Println("\n--- Session 2: New session (facts recalled automatically) ---")
	newSession := eywa.MemoryKey{Channel: "api", User: "user_alice_s2"}
	send(ctx, weave, newSession, "What do you know about me?")
	send(ctx, weave, newSession, "What time is it for me right now?")
}

func send(ctx context.Context, weave *eywa.Weave, key eywa.MemoryKey, msg string) {
	pulse := eywa.NewPulse(key).
		WithUserMessage(msg).
		WithSource("api").
		Build()

	result, err := weave.ProcessEventByKey(ctx, "chat", pulse)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	fmt.Printf("User: %s\nReply: %s\n\n", msg, result.Message)
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
