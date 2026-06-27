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

// Example 10: Cost Tracking (Ledger)
//
// Demonstrates Ledger — Eywa's built-in token usage and cost tracking.
//
// Key concepts:
// - LedgerRepository stores monthly token usage per spirit
// - TokenBudget sets a monthly token limit with an enforcement action (block/downgrade/alert)
// - ModelRoutingRules route requests to cheaper models based on conditions
//   (input length, topic, attachments, event type)
// - CostAlertHook fires when usage reaches a configurable threshold

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Cost Tracking (Ledger) ===")

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
	ledgerRepo := eywamongo.NewLedgerRepository(mongoConn.GetDatabase())

	// TokenBudget: block requests once the monthly limit is exceeded.
	// "downgrade" would route to a cheaper model; "alert" fires the hook.
	budget := eywa.TokenBudget{
		SpiritName:        "assistant",
		MonthlyTokenLimit: 100_000,
		OnExceed:          "downgrade",
		DowngradeModel: eywa.SpiritModel{
			Provider:  "openai",
			Model:     "gpt-4o-mini",
			MaxTokens: 200,
		},
		AlertThreshold:       0.8,  // fire hook at 80% usage
		DowngradeAtThreshold: true, // switch to DowngradeModel at 80%, before the hard limit
	}
	if err := ledgerRepo.SetBudget(ctx, budget); err != nil {
		log.Printf("set budget: %v", err)
	}

	// ModelRoutingRules: automatically switch models based on request characteristics.
	// Rules are evaluated in order; first match wins.
	routingRules := []eywa.ModelRoutingRule{
		{
			// Long inputs are expensive on GPT-4o — route to mini automatically.
			Name: "long_input_downgrade",
			Condition: eywa.ModelRoutingCondition{
				InputLengthGte: 2000,
			},
			Model: eywa.SpiritModel{
				Provider:  "openai",
				Model:     "gpt-4o-mini",
				MaxTokens: 500,
			},
		},
		{
			// Requests with attachments route to vision-capable model.
			Name: "vision_upgrade",
			Condition: eywa.ModelRoutingCondition{
				HasAttachments: true,
			},
			Model: eywa.SpiritModel{
				Provider:  "openai",
				Model:     "gpt-4o",
				MaxTokens: 1000,
			},
		},
	}

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		WithLedgerRepository(ledgerRepo).
		WithModelRoutingRules(routingRules).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	spirit := &eywa.Spirit{
		Name:         "assistant",
		Description:  "Cost-tracked assistant",
		SystemPrompt: "You are a helpful assistant. Answer concisely.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o",
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
		eywa.NewLink("user_message").WithDefaultSpirit("assistant").Build(),
	)

	// ── Send some messages ────────────────────────────────────────────────────

	questions := []string{
		"What is the capital of Brazil?",
		"Explain the difference between a goroutine and a thread in Go.",
	}

	for _, q := range questions {
		fmt.Printf("\nQ: %s\n", q)
		pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_cost"}).
			WithUserMessage(q).
			WithSource("api").
			Build()

		result, err := weave.ProcessEventByKey(ctx, "user_message", pulse)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		fmt.Printf("Reply: %s\n", result.Message)
	}

	// ── Inspect ledger ────────────────────────────────────────────────────────

	fmt.Println("\n--- Ledger ---")
	month := time.Now().Format("2006-01")
	entry, err := ledgerRepo.GetMonthUsage(ctx, "assistant", month)
	if err != nil {
		log.Printf("get usage: %v", err)
	} else {
		fmt.Printf("Spirit: %s | Month: %s | Tokens: %d | Est. cost: $%.4f\n",
			entry.SpiritName, entry.Month, entry.TokensUsed, entry.EstCostUSD)
	}

	b, err := ledgerRepo.GetBudget(ctx, "assistant")
	if err != nil {
		log.Printf("get budget: %v", err)
	} else {
		used := float64(entry.TokensUsed) / float64(b.MonthlyTokenLimit) * 100
		fmt.Printf("Budget: %d tokens/mo | Used: %.1f%%\n", b.MonthlyTokenLimit, used)
	}
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
