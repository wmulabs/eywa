// Command eywa-starter is a minimal, runnable Eywa agent: a terminal chat backed by MongoDB, Redis,
// and an LLM, with one example tool. Fork it and make it yours — see README.md.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	eywa "github.com/wmulabs/eywa"
	eywamongo "github.com/wmulabs/eywa/mongo"
	eywaopenai "github.com/wmulabs/eywa/providers/openai"
	eywaredis "github.com/wmulabs/eywa/redis"
)

func main() {
	ctx := context.Background()

	apiKey := mustEnv("OPENAI_API_KEY")
	mongoURL := getEnv("MONGO_URL", "mongodb://localhost:27017")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	mongoDB := getEnv("MONGO_DATABASE", "eywa_starter")
	serviceName := getEnv("SERVICE_NAME", "eywa-starter")

	mongoConn, err := eywamongo.NewMongoConnection(ctx, mongoURL, mongoDB, serviceName)
	if err != nil {
		log.Fatalf("connect MongoDB: %v", err)
	}
	defer mongoConn.DisconnectMongoDB(ctx)

	redisConn, err := eywaredis.NewRedisConnection(ctx, redisURL, serviceName)
	if err != nil {
		log.Fatalf("connect Redis: %v", err)
	}
	defer redisConn.DisconnectRedisDB(ctx)

	db := mongoConn.GetDatabase()
	spiritRepo := eywamongo.NewSpiritRepository(db)

	// Register your tools here. Each implements the eywa.Action interface (see timeAction below).
	actions := eywa.NewActionRegistry()
	if err := actions.Register(&timeAction{}); err != nil {
		log.Fatalf("register action: %v", err)
	}

	weave, err := eywa.NewWeaveBuilder(ctx).
		WithRepositories(
			spiritRepo,
			eywaredis.NewMemoryRepository(redisConn.GetClient(), serviceName, "lcl", 3600, nil),
			eywamongo.NewEchoRepository(db),
			eywamongo.NewChronicleRepository(db),
		).
		WithBond(eywaredis.NewBondManager(redisConn.GetClient())).
		WithActionRegistry(actions).
		WithScoutRegistry(eywa.NewScoutRegistry()).
		AddOracle(eywaopenai.NewOracle(apiKey)).
		Build()
	if err != nil {
		log.Fatalf("build weave: %v", err)
	}

	// Define your agent. Tweak the prompt, model, and allowed tools to make it yours.
	spirit := &eywa.Spirit{
		Name:         "assistant",
		Description:  "Starter assistant",
		SystemPrompt: "You are a concise, helpful assistant built with Eywa. Use the get_time tool when asked about the current time.",
		ModelConfig: eywa.SpiritModel{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.7,
			MaxTokens:   500,
		},
		AllowedActions: []eywa.AllowedAction{{Name: "get_time"}},
		IsActive:       true,
		CreatedAt:      time.Now(),
	}
	if err := spiritRepo.Create(ctx, spirit); err != nil {
		fmt.Printf("note: spirit already exists: %v\n", err)
	}

	weave.RegisterEventConfiguration(
		eywa.NewLink("chat").WithDefaultSpirit("assistant").Build(),
	)

	// Interactive chat loop. Type a message; Ctrl-D or "exit" to quit.
	fmt.Println("Eywa starter — type a message (exit to quit)")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nyou> ")
		if !scanner.Scan() {
			return
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if text == "exit" {
			return
		}

		pulse := eywa.NewPulse(eywa.MemoryKey{Channel: "cli", User: "local"}).
			WithUserMessage(text).
			WithSource("cli").
			Build()

		result, err := weave.ProcessEventByKey(ctx, "chat", pulse)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		fmt.Printf("bot> %s\n", result.FinalResponse)
	}
}

// timeAction is an example tool. Copy this shape for your own Actions: name, description, JSON-schema
// parameters, and an Execute that returns a string the model reads back.
type timeAction struct{}

func (a *timeAction) GetName() string               { return "get_time" }
func (a *timeAction) GetDescription() string        { return "Returns the current UTC time in RFC3339." }
func (a *timeAction) GetParameters() map[string]any { return map[string]any{"type": "object"} }
func (a *timeAction) IsCritical() bool              { return false }
func (a *timeAction) GetCategory() eywa.ActionCategory {
	return eywa.ActionRetrieval
}
func (a *timeAction) Validate(map[string]any) error { return nil }
func (a *timeAction) Execute(context.Context, map[string]any) (string, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required (copy .env.example to .env and fill it in)", key)
	}
	return v
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
