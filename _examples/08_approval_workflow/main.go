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

// Example 08: Approval Workflow (Rite)
//
// Demonstrates Rite — Eywa's async approval workflow primitive.
//
// Key concepts:
// - RequestRiteAction is a built-in action the Oracle calls to pause and request approval
// - The Rite is stored in MongoDB with status "pending"
// - An operator reviews and calls Decide(approved/rejected) via the management API
// - The original Pulse can be resumed after approval
// - Use cases: financial transactions, sensitive data access, escalations

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Approval Workflow (Rite) ===")

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
	riteRepo := eywamongo.NewRiteRepository(mongoConn.GetDatabase())

	// NewRequestRiteAction is auto-registered by the builder when WithRiteRepository is set.

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithRiteRepository(riteRepo).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	spirit := &eywa.Spirit{
		Name:        "finance_spirit",
		Description: "Financial assistant that requires approval for large transactions",
		SystemPrompt: `You are a financial assistant.
For any transaction over $1,000, you MUST call request_rite before proceeding.
Use request_rite with:
  reason: explain why approval is needed
  context: include transaction details (amount, recipient, purpose)
After requesting approval, tell the user the request is pending review.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.3,
			MaxTokens:   400,
		},
		AllowedActions: []eywa.AllowedAction{
			{Name: "request_rite", IsCritical: boolPtr(true)},
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists\n")
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("finance_message").
			WithDefaultSpirit("finance_spirit").
			Build(),
	)

	// ── Phase 1: Small transaction — no approval needed ───────────────────────

	fmt.Println("\n--- Phase 1: Small transaction (no approval) ---")
	pulse1 := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_finance"}).
		WithUserMessage("Transfer $50 to Alice for lunch.").
		WithSource("api").
		Build()

	r1, err := weave.ProcessEventByKey(ctx, "finance_message", pulse1)
	if err != nil {
		log.Printf("error: %v", err)
	} else {
		fmt.Printf("Actions: %v\n", r1.ActionsExecuted)
		fmt.Printf("Reply:   %s\n", r1.Message)
	}

	// ── Phase 2: Large transaction — approval requested ───────────────────────

	fmt.Println("\n--- Phase 2: Large transaction (approval required) ---")
	pulse2 := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_finance"}).
		WithUserMessage("Transfer $5,000 to the vendor account for the server upgrade.").
		WithSource("api").
		Build()

	r2, err := weave.ProcessEventByKey(ctx, "finance_message", pulse2)
	if err != nil {
		log.Printf("error: %v", err)
	} else {
		fmt.Printf("Actions: %v\n", r2.ActionsExecuted)
		fmt.Printf("Reply:   %s\n", r2.Message)
	}

	// ── Phase 3: Operator reviews pending rites ───────────────────────────────

	fmt.Println("\n--- Phase 3: Operator review ---")
	rites, _, err := riteRepo.List(ctx, eywa.RiteListOptions{
		Status: eywa.RitePending,
		Page:   1,
		Limit:  10,
	})
	if err != nil {
		log.Printf("list rites: %v", err)
	} else {
		fmt.Printf("Pending rites: %d\n", len(rites))
		for _, rite := range rites {
			fmt.Printf("  Rite %s | Reason: %s | Context: %v\n", rite.ID, rite.Reason, rite.Context)

			// Operator approves
			if err := riteRepo.Decide(ctx, rite.ID, "operator_bob", eywa.RiteApproved); err != nil {
				log.Printf("decide rite: %v", err)
			} else {
				fmt.Printf("  → Approved by operator_bob\n")
			}
		}
	}
}

func boolPtr(b bool) *bool { return &b }

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
