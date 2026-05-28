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

// Example 13: Multi-Spirit Orchestration
//
// Demonstrates Eywa's multi-spirit orchestration:
// - An Orchestrator spirit dispatches tasks to specialist spirits
// - The Orchestrator uses the summon_spirit built-in tool to delegate
// - SubSpirits lists which spirits can be summoned
// - ParallelSummon enables concurrent execution across sub-spirits
// - MaxDepth limits delegation chain depth
//
// This example uses a research pipeline:
//   coordinator (orchestrator)
//     └── researcher  (searches and summarizes)
//     └── writer      (formats the final response)

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Multi-Spirit Orchestration ===")

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

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		AddOracle(eywaopenai.NewOracle(mustEnv("OPENAI_API_KEY"))).
		WithConfig(eywa.DefaultWeaveConfig()).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	// ── Sub-spirits ───────────────────────────────────────────────────────────

	researcher := &eywa.Spirit{
		Name:        "researcher",
		Description: "Researches topics and extracts key facts",
		SystemPrompt: `You are a research specialist. When summoned, investigate the given topic and return a structured summary with:
1. Key facts (bullet points)
2. Main concepts to understand
3. Potential limitations or caveats
Be thorough but concise.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.3,
			MaxTokens:   600,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	writer := &eywa.Spirit{
		Name:        "writer",
		Description: "Formats information into clear, readable prose",
		SystemPrompt: `You are a writing specialist. When summoned, take the provided information and:
1. Format it into clear, engaging prose
2. Use an appropriate tone for the audience
3. Ensure logical flow and readability
Return a polished final response.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   500,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	// ── Orchestrator spirit ───────────────────────────────────────────────────
	//
	// Type: orchestrator enables the summon_spirit built-in tool.
	// SubSpirits lists which spirits this orchestrator can delegate to.
	// ParallelSummon: true dispatches all summons concurrently.
	// MaxDepth: 2 prevents infinite delegation chains.

	coordinator := &eywa.Spirit{
		Name:        "coordinator",
		Description: "Orchestrates research and writing tasks",
		SystemPrompt: `You are a coordinator that breaks complex requests into specialist tasks.
For each request:
1. Summon the researcher to gather and analyze information
2. Pass the research to the writer to format the final response
3. Return the writer's output as your response

Always use both specialists — don't answer directly.`,
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.3,
			MaxTokens:   200,
		},
		Type: eywa.SpiritTypeOrchestrator,
		OrchestratorConfig: eywa.OrchestratorConfig{
			SubSpirits:     []string{"researcher", "writer"},
			MaxDepth:       2,
			ParallelSummon: false, // sequential: researcher first, then writer
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	for _, s := range []*eywa.Spirit{researcher, writer, coordinator} {
		if err := spiritRepo.Create(ctx, s); err != nil {
			fmt.Printf("note: spirit %q already exists\n", s.Name)
		}
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("research_request").
			WithDefaultSpirit("coordinator").
			Build(),
	)

	// ── Run ───────────────────────────────────────────────────────────────────

	questions := []string{
		"Explain how vector search works in RAG systems.",
		"What are the key differences between goroutines and OS threads?",
	}

	for _, q := range questions {
		fmt.Printf("\n--- Request: %s ---\n", q)
		pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: "user_research"}).
			WithUserMessage(q).
			WithSource("api").
			Build()

		result, err := weave.ProcessEventByKey(ctx, "research_request", pulse)
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		fmt.Printf("Spirit chain: %s\n", result.SpiritUsed)
		fmt.Printf("Actions:      %v\n", result.ActionsExecuted)
		fmt.Printf("Reply:\n%s\n", result.Message)
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
