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

// Example 05: Multi-Provider
//
// One Weave running two spirits backed by different LLM providers.
//
// Key concepts:
// - AddOracle registers providers by name (openai, groq, ollama, …)
// - Spirit.ModelConfig.Provider selects which provider handles that spirit
// - OpenAI-compatible providers (Groq, Ollama, Mistral, Together) need no extra code
// - Ollama is shown as a local alternative; Groq as a cloud alternative

func main() {
	ctx := context.Background()
	fmt.Println("=== Eywa — Multi-Provider ===")

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

	// ── Oracles ────────────────────────────────────────────────────────────────
	//
	// Each provider is registered under its own name.
	// Spirit.ModelConfig.Provider references that name at runtime.

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	builder := eywa.NewWeaveBuilder(ctx).
		WithRepositories(spiritRepo, memoryRepo, echoRepo, chronicleRepo).
		WithBond(bond).
		WithActionRegistry(eywa.NewActionRegistry()).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		AddOracle(eywaopenai.NewOracle(openaiKey)) // provider name: "openai"

	// Add Groq if key is present (cloud, OpenAI-compatible, no extra deps).
	groqKey := os.Getenv("GROQ_API_KEY")
	useGroq := groqKey != ""
	if useGroq {
		builder = builder.AddOracle(eywaopenai.NewGroqOracle(groqKey)) // provider name: "groq"
		fmt.Println("Groq oracle registered")
	}

	// Add Ollama if a local instance is running (self-hosted, no API key needed).
	ollamaURL := getEnv("OLLAMA_URL", "")
	useOllama := ollamaURL != ""
	if useOllama {
		builder = builder.AddOracle(eywaopenai.NewOllamaOracle(ollamaURL)) // provider name: "ollama"
		fmt.Println("Ollama oracle registered")
	}

	config := eywa.DefaultWeaveConfig()
	config.ReasoningTimeout = 60 * time.Second

	weave, err := builder.WithConfig(config).Build()
	if err != nil {
		log.Fatalf("failed to build Weave: %v", err)
	}

	// ── Spirits ────────────────────────────────────────────────────────────────
	//
	// Each spirit names a provider in ModelConfig.Provider.
	// The engine resolves the Oracle at cycle time.

	openaiSpirit := &eywa.Spirit{
		Name:         "openai_assistant",
		Description:  "Assistant backed by OpenAI GPT-4o-mini",
		SystemPrompt: "You are a concise assistant. Answer in one sentence.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   200,
		},
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	mustCreate(ctx, spiritRepo, openaiSpirit)

	weave.RegisterEventConfiguration(
		eywa.NewLink("openai_message").
			WithDefaultSpirit("openai_assistant").
			Build(),
	)

	// Second spirit: Groq or Ollama depending on what's available.
	if useGroq {
		groqSpirit := &eywa.Spirit{
			Name:         "groq_assistant",
			Description:  "Assistant backed by Groq (llama-3.3-70b-versatile)",
			SystemPrompt: "You are a concise assistant. Answer in one sentence.",
			ModelConfig: eywa.SpiritModel{
				Provider:    "groq",
				Model:       "llama-3.3-70b-versatile",
				Temperature: 0.7,
				MaxTokens:   200,
			},
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		mustCreate(ctx, spiritRepo, groqSpirit)
		weave.RegisterEventConfiguration(
			eywa.NewLink("alt_message").
				WithDefaultSpirit("groq_assistant").
				Build(),
		)
		fmt.Println("\n--- Groq spirit registered ---")
	} else if useOllama {
		ollamaSpirit := &eywa.Spirit{
			Name:         "ollama_assistant",
			Description:  "Local assistant backed by Ollama",
			SystemPrompt: "You are a concise assistant. Answer in one sentence.",
			ModelConfig: eywa.SpiritModel{
				Provider:    "ollama",
				Model:       getEnv("OLLAMA_MODEL", "llama3.2"),
				Temperature: 0.7,
				MaxTokens:   200,
			},
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		mustCreate(ctx, spiritRepo, ollamaSpirit)
		weave.RegisterEventConfiguration(
			eywa.NewLink("alt_message").
				WithDefaultSpirit("ollama_assistant").
				Build(),
		)
		fmt.Println("\n--- Ollama spirit registered ---")
	}

	// ── Run ────────────────────────────────────────────────────────────────────

	question := "What is the capital of France?"

	fmt.Println("\n--- OpenAI ---")
	processAndPrint(ctx, weave, "openai_message", "user_openai", question)

	if useGroq || useOllama {
		fmt.Println("\n--- Alternative provider ---")
		processAndPrint(ctx, weave, "alt_message", "user_alt", question)
	} else {
		fmt.Println("\nSet GROQ_API_KEY or OLLAMA_URL to test a second provider.")
	}
}

func processAndPrint(ctx context.Context, weave *eywa.Weave, eventKey, user, message string) {
	pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "api", User: user}).
		WithUserMessage(message).
		WithSource("api").
		Build()

	result, err := weave.ProcessEventByKey(ctx, eventKey, pulse)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	fmt.Printf("Spirit:   %s\n", result.SpiritUsed)
	fmt.Printf("Time:     %dms\n", result.ProcessingTimeMs)
	fmt.Printf("Reply:    %s\n", result.Message)
}

func mustCreate(ctx context.Context, repo eywa.SpiritRepository, spirit *eywa.Spirit) {
	if err := repo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit %q already exists\n", spirit.Name)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
